package authn

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"regexp"
	"strings"
)

var localPart = regexp.MustCompile("^[A-Za-z0-9.!#$%&'*+/=?^_`{|}~-]+$")
var label = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// NormalizeEmail deliberately preserves the local-part case and aliases.
// The same function is used for all identity comparisons.
func NormalizeEmail(input string) (string, error) {
	for _, c := range input {
		if c < 32 || c == 127 {
			return "", errors.New("invalid email")
		}
	}

	s := strings.TrimSpace(input)
	if len(s) > 254 {
		return "", errors.New("invalid email")
	}
	a, e := mail.ParseAddress(s)
	if e != nil || a.Name != "" || a.Address != s {
		return "", errors.New("invalid email")
	}
	parts := strings.Split(s, "@")
	if len(parts) != 2 || len(parts[0]) > 64 || !localPart.MatchString(parts[0]) || strings.HasPrefix(parts[0], ".") || strings.HasSuffix(parts[0], ".") || strings.Contains(parts[0], "..") {
		return "", errors.New("invalid email")
	}
	domain := strings.ToLower(parts[1])
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", errors.New("invalid email")
	}
	for _, v := range labels {
		if !label.MatchString(v) {
			return "", errors.New("invalid email")
		}
	}
	return parts[0] + "@" + domain, nil
}

func GenerateCode() (string, error) {
	n, e := rand.Int(rand.Reader, big.NewInt(1000000))
	if e != nil {
		return "", e
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
func ValidCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func CodeMAC(key []byte, id, email, purpose, userID, code string) string {
	b, _ := json.Marshal([]string{id, email, purpose, userID, code})
	h := hmac.New(sha256.New, key)
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
func RateKey(key []byte, kind, value string) string {
	return kind + ":" + CodeMAC(key, "rate", value, kind, "", "")
}
