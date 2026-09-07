package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

// ErrUnavailable never includes SMTP replies, credentials, codes or recipients.
// Callers may safely map it to the public MAIL_UNAVAILABLE response.
var ErrUnavailable = errors.New("mail unavailable")

// Config selects an implicit-TLS SMTP server. Port 465 is the production default;
// other implicit-TLS ports support local fixtures. STARTTLS on 587 is unsupported.
// TLSConfig can add trusted roots, but cannot disable certificate verification.
type Config struct {
	Host      string
	Port      int
	Username  string
	AuthCode  string
	From      string
	Timeout   time.Duration
	TLSConfig *tls.Config
}

// SMTP opens one authenticated TLS connection per message and never retries.
type SMTP struct{ config Config }

func New(config Config) (*SMTP, error) {
	if config.Host == "" {
		config.Host = "smtp.qq.com"
	}
	if config.Port == 0 {
		config.Port = 465
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if !validHost(config.Host) || config.Port < 1 || config.Port > 65535 || config.Port == 587 || config.Timeout < 0 || config.Timeout > 10*time.Second {
		return nil, ErrUnavailable
	}
	var ok bool
	config.From, ok = normalizeAddress(config.From)
	if !ok {
		return nil, ErrUnavailable
	}
	config.Username, ok = normalizeAddress(config.Username)
	if !ok || strings.TrimSpace(config.AuthCode) == "" || hasControl(config.AuthCode) {
		return nil, ErrUnavailable
	}
	if config.TLSConfig == nil {
		config.TLSConfig = &tls.Config{}
	} else {
		config.TLSConfig = config.TLSConfig.Clone()
		if config.TLSConfig.InsecureSkipVerify {
			return nil, ErrUnavailable
		}
		if config.TLSConfig.RootCAs != nil {
			config.TLSConfig.RootCAs = config.TLSConfig.RootCAs.Clone()
		}
	}
	config.TLSConfig.ServerName = config.Host
	if config.TLSConfig.MinVersion < tls.VersionTLS12 {
		config.TLSConfig.MinVersion = tls.VersionTLS12
	}
	return &SMTP{config: config}, nil
}

// LoadFromEnv disables email only when every SMTP setting is absent. A partial
// configuration fails closed. It reads environment variables, never dotenv files.
func LoadFromEnv() (*SMTP, error) {
	configured := false
	for _, name := range []string{"SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_AUTH_CODE", "MAIL_FROM", "SMTP_TIMEOUT_SECONDS", "SMTP_CA_FILE"} {
		if _, exists := os.LookupEnv(name); exists {
			configured = true
		}
	}
	if !configured {
		return nil, nil
	}
	config := Config{Host: os.Getenv("SMTP_HOST"), Username: os.Getenv("SMTP_USERNAME"), AuthCode: os.Getenv("SMTP_AUTH_CODE"), From: os.Getenv("MAIL_FROM")}
	if value := os.Getenv("SMTP_PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return nil, ErrUnavailable
		}
		config.Port = port
	}
	if value := os.Getenv("SMTP_TIMEOUT_SECONDS"); value != "" {
		seconds, err := strconv.Atoi(value)
		if err != nil || seconds < 1 || seconds > 10 {
			return nil, ErrUnavailable
		}
		config.Timeout = time.Duration(seconds) * time.Second
	}
	if caFile := os.Getenv("SMTP_CA_FILE"); caFile != "" {
		if os.Getenv("ALLOW_LOCAL_PROVIDERS") != "true" || (config.Host != "127.0.0.1" && config.Host != "localhost") {
			return nil, ErrUnavailable
		}
		ca, err := os.ReadFile(caFile)
		if err != nil {
			return nil, ErrUnavailable
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(ca) {
			return nil, ErrUnavailable
		}
		config.TLSConfig = &tls.Config{RootCAs: roots}
	}
	return New(config)
}

func (s *SMTP) SendCode(ctx context.Context, to, code, purpose string, expiresAt time.Time) error {
	if s == nil || ctx == nil {
		return ErrUnavailable
	}
	to, valid := normalizeAddress(to)
	if !valid || !validCode(code) || (purpose != "register" && purpose != "bind") || !expiresAt.After(time.Now()) {
		return ErrUnavailable
	}
	if ctx.Err() != nil {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	dialer := tls.Dialer{Config: s.config.TLSConfig}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port)))
	if err != nil {
		return ErrUnavailable
	}
	defer conn.Close()
	stopCancel := context.AfterFunc(ctx, func() { conn.Close() })
	defer stopCancel()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return ErrUnavailable
		}
	}
	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		return ErrUnavailable
	}
	defer client.Close()
	if err := client.Auth(smtp.PlainAuth("", s.config.Username, s.config.AuthCode, s.config.Host)); err != nil {
		return ErrUnavailable
	}
	if err := client.Mail(s.config.From); err != nil {
		return ErrUnavailable
	}
	if err := client.Rcpt(to); err != nil {
		return ErrUnavailable
	}
	writer, err := client.Data()
	if err != nil {
		return ErrUnavailable
	}
	message := buildMessage(s.config.From, to, code, purpose, expiresAt)
	if _, err := writer.Write(message); err != nil {
		return ErrUnavailable
	}
	if err := writer.Close(); err != nil {
		return ErrUnavailable
	}
	// DATA's final 250 response is the acceptance boundary. Close errors are
	// ignored; sending QUIT or retrying here could misreport or duplicate a send.
	return nil
}

func normalizeAddress(value string) (string, bool) {
	if hasControl(value) {
		return "", false
	}
	for i := range len(value) {
		if value[i] >= 128 {
			return "", false
		}
	}
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 254 {
		return "", false
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Name != "" || address.Address != value {
		return "", false
	}
	at := strings.LastIndexByte(value, '@')
	if at < 1 || at > 64 || at == len(value)-1 {
		return "", false
	}
	return value[:at+1] + strings.ToLower(value[at+1:]), true
}

func hasControl(value string) bool {
	for i := range len(value) {
		if value[i] < 32 || value[i] == 127 {
			return true
		}
	}
	return false
}

func validCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for i := range len(code) {
		if code[i] < '0' || code[i] > '9' {
			return false
		}
	}
	return true
}

func validHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := range len(label) {
			c := label[i]
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

func buildMessage(from, to, code, purpose string, expiresAt time.Time) []byte {
	label := "注册"
	if purpose == "bind" {
		label = "绑定"
	}
	var message bytes.Buffer
	fmt.Fprintf(&message, "From: <%s>\r\nTo: <%s>\r\nSubject: %s\r\n", from, to, mime.BEncoding.Encode("UTF-8", "行者"+label+"邮箱验证码"))
	message.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n")
	body := quotedprintable.NewWriter(&message)
	fmt.Fprintf(body, "你的行者%s邮箱验证码是：%s\r\n请在 %s（UTC）前使用，过期失效。\r\n如果此邮箱已经注册，请直接登录。此验证码不会修改已有账号的密码。\r\n如果并非你本人操作，请忽略此邮件。\r\n", label, code, expiresAt.UTC().Format(time.RFC3339))
	body.Close()
	return message.Bytes()
}
