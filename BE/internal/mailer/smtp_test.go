package mailer

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type smtpDelivery struct {
	auth, from, to, data string
}

type fixtureOptions struct {
	rejectAuth      bool
	rejectRecipient bool
	rejectData      bool
	stall           string
	dropAfterData   bool
}

// smtpFixture runs the actual TLS, SMTP authentication, envelope and DATA protocol.
type smtpFixture struct {
	listener    net.Listener
	roots       *x509.CertPool
	caPEM       []byte
	options     fixtureOptions
	deliveries  chan smtpDelivery
	reached     chan string
	connections atomic.Int32
	mu          sync.Mutex
	active      map[net.Conn]struct{}
	wg          sync.WaitGroup
}

func newSMTPFixture(t *testing.T, options fixtureOptions) *smtpFixture {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "local SMTP fixture"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true, IsCA: true,
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, DNSNames: []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("test certificate invalid")
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
	})
	if err != nil {
		t.Fatal(err)
	}
	f := &smtpFixture{listener: listener, roots: roots, caPEM: caPEM, options: options,
		deliveries: make(chan smtpDelivery, 32), reached: make(chan string, 8), active: make(map[net.Conn]struct{})}
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			f.connections.Add(1)
			f.mu.Lock()
			f.active[conn] = struct{}{}
			f.mu.Unlock()
			f.wg.Add(1)
			go f.serve(conn)
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		f.mu.Lock()
		for conn := range f.active {
			conn.Close()
		}
		f.mu.Unlock()
		f.wg.Wait()
	})
	return f
}

func (f *smtpFixture) config() Config {
	_, port, _ := net.SplitHostPort(f.listener.Addr().String())
	p, _ := strconv.Atoi(port)
	return Config{Host: "127.0.0.1", Port: p, Username: "sender@qq.com", AuthCode: "fixture-auth-code",
		From: "sender@qq.com", Timeout: 2 * time.Second, TLSConfig: &tls.Config{RootCAs: f.roots}}
}

func (f *smtpFixture) serve(conn net.Conn) {
	defer f.wg.Done()
	defer conn.Close()
	defer func() { f.mu.Lock(); delete(f.active, conn); f.mu.Unlock() }()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := conn.(*tls.Conn).Handshake(); err != nil {
		return
	}
	reader := textproto.NewReader(bufio.NewReader(conn))
	if f.options.stall == "greeting" {
		f.reached <- "greeting"
		io.Copy(io.Discard, conn)
		return
	}
	if _, err := fmt.Fprint(conn, "220 fixture ESMTP\r\n"); err != nil {
		return
	}
	var delivery smtpDelivery
	for {
		line, err := reader.ReadLine()
		if err != nil {
			return
		}
		verb, arg, _ := strings.Cut(line, " ")
		switch verb {
		case "EHLO":
			fmt.Fprint(conn, "250-fixture\r\n250 AUTH PLAIN\r\n")
		case "AUTH":
			_, encoded, _ := strings.Cut(arg, " ")
			decoded, _ := base64.StdEncoding.DecodeString(encoded)
			delivery.auth = string(decoded)
			if f.options.rejectAuth {
				fmt.Fprint(conn, "535 provider-private-auth-detail\r\n")
			} else if arg == "" || !strings.HasPrefix(arg, "PLAIN ") || delivery.auth != "\x00sender@qq.com\x00fixture-auth-code" {
				fmt.Fprint(conn, "535 wrong credentials\r\n")
			} else {
				fmt.Fprint(conn, "235 authenticated\r\n")
			}
		case "MAIL":
			delivery.from = arg
			fmt.Fprint(conn, "250 sender accepted\r\n")
		case "RCPT":
			delivery.to = arg
			if f.options.stall == "recipient" {
				f.reached <- "recipient"
				io.Copy(io.Discard, conn)
				return
			}
			if f.options.rejectRecipient {
				fmt.Fprint(conn, "550 provider-private-recipient-detail\r\n")
			} else {
				fmt.Fprint(conn, "250 recipient accepted\r\n")
			}
		case "DATA":
			fmt.Fprint(conn, "354 send data\r\n")
			data, err := reader.ReadDotBytes()
			if err != nil {
				return
			}
			delivery.data = string(data)
			if f.options.stall == "data" {
				f.reached <- "data"
				io.Copy(io.Discard, conn)
				return
			}
			if f.options.rejectData {
				fmt.Fprint(conn, "554 provider-private-data-detail\r\n")
				continue
			}
			f.deliveries <- delivery
			fmt.Fprint(conn, "250 accepted\r\n")
			if f.options.dropAfterData {
				return
			}
		case "QUIT":
			fmt.Fprint(conn, "221 goodbye\r\n")
			return
		default:
			fmt.Fprint(conn, "500 unsupported\r\n")
		}
	}
}

func TestSendCodeRejectsInvalidInputBeforeConnecting(t *testing.T) {
	f := newSMTPFixture(t, fixtureOptions{})
	sender, err := New(f.config())
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	cases := []struct {
		name, to, code, purpose string
		expiry                  time.Time
	}{
		{"empty email", "", "123456", "register", future},
		{"multiple recipients", "a@example.com,b@example.com", "123456", "register", future},
		{"display name", "Person <a@example.com>", "123456", "register", future},
		{"header injection", "a@example.com\r\nBcc: b@example.com", "123456", "register", future},
		{"leading newline", "\na@example.com", "123456", "register", future},
		{"unicode email", "旅行者@example.com", "123456", "register", future},
		{"missing domain", "a@", "123456", "register", future},
		{"code too short", "a@example.com", "12345", "register", future},
		{"code too long", "a@example.com", "1234567", "register", future},
		{"nonnumeric code", "a@example.com", "a23456", "register", future},
		{"unicode code", "a@example.com", "１２３４５６", "register", future},
		{"code injection", "a@example.com", "123456\r\n", "register", future},
		{"unknown purpose", "a@example.com", "123456", "reset", future},
		{"purpose injection", "a@example.com", "123456", "register\n", future},
		{"expired", "a@example.com", "123456", "register", time.Now().Add(-time.Second)},
		{"zero expiry", "a@example.com", "123456", "register", time.Time{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := sender.SendCode(context.Background(), tc.to, tc.code, tc.purpose, tc.expiry); err != ErrUnavailable {
				t.Fatalf("invalid message must return sanitized error, got %v", err)
			}
		})
	}
	if got := f.connections.Load(); got != 0 {
		t.Fatalf("invalid messages opened %d SMTP connections", got)
	}
}

func TestSendCodeProviderFailuresAreSanitizedWithoutRetries(t *testing.T) {
	cases := []struct {
		name           string
		options        fixtureOptions
		badCertificate bool
	}{
		{"untrusted certificate", fixtureOptions{}, true},
		{"auth rejected", fixtureOptions{rejectAuth: true}, false},
		{"recipient rejected", fixtureOptions{rejectRecipient: true}, false},
		{"data rejected", fixtureOptions{rejectData: true}, false},
		{"greeting timeout", fixtureOptions{stall: "greeting"}, false},
		{"recipient timeout", fixtureOptions{stall: "recipient"}, false},
		{"ambiguous data timeout", fixtureOptions{stall: "data"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSMTPFixture(t, tc.options)
			cfg := f.config()
			cfg.Timeout = 120 * time.Millisecond
			if tc.badCertificate {
				cfg.TLSConfig.RootCAs = x509.NewCertPool()
			}
			sender, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			started := time.Now()
			err = sender.SendCode(context.Background(), "a@example.com", "123456", "register", time.Now().Add(time.Minute))
			if err != ErrUnavailable {
				t.Fatalf("error must not contain provider details: %v", err)
			}
			if elapsed := time.Since(started); elapsed > time.Second {
				t.Fatalf("send exceeded timeout bound: %v", elapsed)
			}
			if got := f.connections.Load(); got != 1 {
				t.Fatalf("send must attempt one connection, got %d", got)
			}
		})
	}
}

func TestSendCodeCancellationInterruptsSMTPRead(t *testing.T) {
	f := newSMTPFixture(t, fixtureOptions{stall: "recipient"})
	cfg := f.config()
	cfg.Timeout = 10 * time.Second
	sender, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- sender.SendCode(ctx, "a@example.com", "123456", "register", time.Now().Add(time.Minute))
	}()
	select {
	case <-f.reached:
	case <-time.After(time.Second):
		t.Fatal("SMTP send never reached blocked read")
	}
	cancel()
	select {
	case err := <-result:
		if err != ErrUnavailable {
			t.Fatalf("unexpected cancellation error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cancellation failed to interrupt SMTP read promptly")
	}
}

func TestSendCodeHonorsEarlierContextDeadline(t *testing.T) {
	f := newSMTPFixture(t, fixtureOptions{stall: "greeting"})
	sender, err := New(f.config())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := sender.SendCode(ctx, "a@example.com", "123456", "register", time.Now().Add(time.Minute)); err != ErrUnavailable {
		t.Fatalf("got %v", err)
	}
	if time.Since(started) > 500*time.Millisecond {
		t.Fatal("send ignored caller deadline")
	}
}

func TestSendCodeBindingUsesBindingPurposeAndTrimsAddress(t *testing.T) {
	f := newSMTPFixture(t, fixtureOptions{})
	sender, err := New(f.config())
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.SendCode(context.Background(), " Traveler@EXAMPLE.COM ", "012345", "bind", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	delivered := <-f.deliveries
	if delivered.to != "TO:<Traveler@example.com>" {
		t.Fatalf("wrong normalized recipient: %q", delivered.to)
	}
	message, err := mail.ReadMessage(strings.NewReader(delivered.data))
	if err != nil {
		t.Fatal(err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
	if err != nil || !strings.Contains(subject, "绑定") {
		t.Fatalf("wrong binding subject: %q (%v)", subject, err)
	}
	body, err := io.ReadAll(quotedprintable.NewReader(message.Body))
	if err != nil || !strings.Contains(string(body), "绑定") || !strings.Contains(string(body), "012345") {
		t.Fatalf("wrong binding body: %s (%v)", body, err)
	}
}

func TestNewRejectsUnsafeOrIncompleteConfiguration(t *testing.T) {
	valid := Config{Host: "smtp.qq.com", Port: 465, Username: "sender@qq.com", AuthCode: "fixture-auth-code", From: "sender@qq.com", Timeout: time.Second}
	cases := []struct {
		name   string
		change func(*Config)
	}{
		{"no username", func(c *Config) { c.Username = "" }},
		{"no auth code", func(c *Config) { c.AuthCode = "" }},
		{"no sender", func(c *Config) { c.From = "" }},
		{"sender injection", func(c *Config) { c.From = "sender@qq.com\r\nBcc: a@example.com" }},
		{"sender display name", func(c *Config) { c.From = "Someone <sender@qq.com>" }},
		{"username control", func(c *Config) { c.Username = "sender\n@qq.com" }},
		{"credential control", func(c *Config) { c.AuthCode = "fixture\x00secret" }},
		{"bad host", func(c *Config) { c.Host = "smtp.qq.com/path" }},
		{"bad port", func(c *Config) { c.Port = -1 }},
		{"oversized port", func(c *Config) { c.Port = 65536 }},
		{"STARTTLS port unsupported", func(c *Config) { c.Port = 587 }},
		{"negative timeout", func(c *Config) { c.Timeout = -time.Second }},
		{"timeout exceeds endpoint budget", func(c *Config) { c.Timeout = 11 * time.Second }},
		{"insecure TLS", func(c *Config) { c.TLSConfig = &tls.Config{InsecureSkipVerify: true} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.change(&cfg)
			if sender, err := New(cfg); sender != nil || err != ErrUnavailable {
				t.Fatalf("unsafe config accepted: sender=%v err=%v", sender != nil, err)
			}
		})
	}
}

var smtpEnvNames = []string{"SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_AUTH_CODE", "MAIL_FROM", "SMTP_TIMEOUT_SECONDS", "SMTP_CA_FILE"}

func clearSMTPEnv(t *testing.T) {
	t.Helper()
	for _, name := range append(append([]string{}, smtpEnvNames...), "ALLOW_LOCAL_PROVIDERS") {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

func setRequiredSMTPEnv(t *testing.T) {
	t.Setenv("SMTP_USERNAME", "sender@qq.com")
	t.Setenv("SMTP_AUTH_CODE", "fixture-auth-code")
	t.Setenv("MAIL_FROM", "sender@qq.com")
}

func TestLoadFromEnvAbsentAndDefaults(t *testing.T) {
	clearSMTPEnv(t)
	if sender, err := LoadFromEnv(); sender != nil || err != nil {
		t.Fatalf("absent config should disable email: %v %v", sender != nil, err)
	}
	setRequiredSMTPEnv(t)
	sender, err := LoadFromEnv()
	if err != nil || sender == nil {
		t.Fatalf("default QQ config failed: %v", err)
	}
	if sender.config.Host != "smtp.qq.com" || sender.config.Port != 465 || sender.config.Timeout != 10*time.Second {
		t.Fatalf("unexpected SMTP defaults: host=%q port=%d timeout=%s", sender.config.Host, sender.config.Port, sender.config.Timeout)
	}
}

func TestLoadFromEnvRejectsPartialOrMalformedConfiguration(t *testing.T) {
	cases := []struct {
		name     string
		values   map[string]string
		required bool
	}{
		{"empty host still configured", map[string]string{"SMTP_HOST": ""}, false},
		{"only host", map[string]string{"SMTP_HOST": "smtp.qq.com"}, false},
		{"only credential", map[string]string{"SMTP_AUTH_CODE": "fixture-auth-code"}, false},
		{"invalid port", map[string]string{"SMTP_PORT": "abc"}, true},
		{"zero port", map[string]string{"SMTP_PORT": "0"}, true},
		{"oversized port", map[string]string{"SMTP_PORT": "65536"}, true},
		{"invalid timeout", map[string]string{"SMTP_TIMEOUT_SECONDS": "abc"}, true},
		{"zero timeout", map[string]string{"SMTP_TIMEOUT_SECONDS": "0"}, true},
		{"negative timeout", map[string]string{"SMTP_TIMEOUT_SECONDS": "-1"}, true},
		{"timeout exceeds endpoint budget", map[string]string{"SMTP_TIMEOUT_SECONDS": "11"}, true},
		{"overflow timeout", map[string]string{"SMTP_TIMEOUT_SECONDS": "9223372036854775807"}, true},
		{"missing CA", map[string]string{"SMTP_HOST": "127.0.0.1", "ALLOW_LOCAL_PROVIDERS": "true", "SMTP_CA_FILE": "/fixture-does-not-exist"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearSMTPEnv(t)
			if tc.required {
				setRequiredSMTPEnv(t)
			}
			for name, value := range tc.values {
				t.Setenv(name, value)
			}
			if sender, err := LoadFromEnv(); sender != nil || err != ErrUnavailable {
				t.Fatalf("invalid environment accepted: sender=%v err=%v", sender != nil, err)
			}
		})
	}
}

func TestLoadFromEnvLocalCARequiresExplicitLoopbackOptIn(t *testing.T) {
	f := newSMTPFixture(t, fixtureOptions{})
	caPath := filepath.Join(t.TempDir(), "smtp-ca.pem")
	if err := os.WriteFile(caPath, f.caPEM, 0600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		host, optIn string
		accepted    bool
	}{
		{"127.0.0.1", "true", true}, {"localhost", "true", true},
		{"127.0.0.1", "", false}, {"127.0.0.1", "1", false},
		{"smtp.qq.com", "true", false}, {"127.0.0.2", "true", false},
		{"localhost.example.com", "true", false}, {"::1", "true", false},
	}
	for _, tc := range cases {
		t.Run(tc.host+"/"+tc.optIn, func(t *testing.T) {
			clearSMTPEnv(t)
			setRequiredSMTPEnv(t)
			t.Setenv("SMTP_HOST", tc.host)
			t.Setenv("SMTP_PORT", strconv.Itoa(f.config().Port))
			t.Setenv("SMTP_CA_FILE", caPath)
			t.Setenv("ALLOW_LOCAL_PROVIDERS", tc.optIn)
			sender, err := LoadFromEnv()
			if !tc.accepted {
				if sender != nil || err != ErrUnavailable {
					t.Fatalf("unsafe local CA configuration accepted: %v", err)
				}
				return
			}
			if err != nil || sender == nil {
				t.Fatalf("loopback CA config rejected: %v", err)
			}
			if err := sender.SendCode(context.Background(), "a@example.com", "123456", "register", time.Now().Add(time.Minute)); err != nil {
				t.Fatalf("trusted loopback SMTP failed: %v", err)
			}
		})
	}
}

func TestSendCodeDeliversAuthenticatedUTF8MIME(t *testing.T) {
	f := newSMTPFixture(t, fixtureOptions{dropAfterData: true})
	sender, err := New(f.config())
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(10 * time.Minute).Truncate(time.Second)
	if err := sender.SendCode(context.Background(), "Traveler@EXAMPLE.COM", "042619", "register", expiresAt); err != nil {
		t.Fatalf("SMTP server should accept verification email: %v", err)
	}
	select {
	case delivered := <-f.deliveries:
		if delivered.from != "FROM:<sender@qq.com>" || delivered.to != "TO:<Traveler@example.com>" {
			t.Fatalf("wrong envelope: from=%q to=%q", delivered.from, delivered.to)
		}
		message, err := mail.ReadMessage(strings.NewReader(delivered.data))
		if err != nil {
			t.Fatal(err)
		}
		subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(subject, "注册") || !strings.Contains(subject, "验证码") {
			t.Fatalf("wrong subject: %q", subject)
		}
		mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
		if err != nil || mediaType != "text/plain" || strings.ToLower(params["charset"]) != "utf-8" {
			t.Fatalf("wrong MIME type: %q", message.Header.Get("Content-Type"))
		}
		if message.Header.Get("MIME-Version") != "1.0" {
			t.Fatal("missing MIME version")
		}
		body, err := io.ReadAll(quotedprintable.NewReader(message.Body))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"042619", expiresAt.Format(time.RFC3339), "过期", "登录", "密码"} {
			if !strings.Contains(string(body), want) {
				t.Fatalf("body missing %q: %s", want, body)
			}
		}
		if strings.Contains(delivered.data, "fixture-auth-code") {
			t.Fatal("SMTP credential leaked into message")
		}
	case <-time.After(time.Second):
		t.Fatal("server received no email")
	}
}
