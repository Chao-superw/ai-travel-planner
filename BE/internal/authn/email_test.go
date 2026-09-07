package authn

import "testing"

func TestNormalizeEmailRejectsAmbiguityAndHeaderInjection(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"  Alice+trip@EXAMPLE.COM  ", "Alice+trip@example.com"},
		{"traveler@example.test", "traveler@example.test"},
		{"a.b@example.com", "a.b@example.com"},
	} {
		got, err := NormalizeEmail(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("normalization %q: %q %v", tc.in, got, err)
		}
	}
	for _, in := range []string{"\ta@example.com", "a@example.com\v", "", "a", "Name <a@example.com>", "a@example.com,b@example.com", "a@example.com\r\nBcc:x@example.com", "a..b@example.com", ".a@example.com", "a@-bad.com", "中文@example.com", "a@localhost", "a@foo..com"} {
		if _, err := NormalizeEmail(in); err == nil {
			t.Errorf("accepted invalid address %q", in)
		}
	}
}

func TestVerificationCodeAndMACBindAllProofFields(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		code, err := GenerateCode()
		if err != nil || !ValidCode(code) {
			t.Fatal("invalid secure verification code")
		}
		seen[code] = true
	}
	if len(seen) < 95 {
		t.Fatal("codes are not varying")
	}
	base := CodeMAC(key, "challenge", "a@example.com", "register", "", "123456")
	for _, v := range []string{CodeMAC(key, "other", "a@example.com", "register", "", "123456"), CodeMAC(key, "challenge", "b@example.com", "register", "", "123456"), CodeMAC(key, "challenge", "a@example.com", "bind", "", "123456"), CodeMAC(key, "challenge", "a@example.com", "register", "user", "123456"), CodeMAC(key, "challenge", "a@example.com", "register", "", "123457")} {
		if base == v {
			t.Fatal("proof binding missing")
		}
	}
}

func TestPolicyRejectsPartialOrUnsafeConfiguration(t *testing.T) {
	t.Setenv("AUTH_CODE_HMAC_KEY", "too-short")
	if _, err := LoadPolicy(); err == nil {
		t.Fatal("short HMAC key accepted")
	}
	t.Setenv("AUTH_CODE_HMAC_KEY", "01234567890123456789012345678901")
	t.Setenv("AUTH_CODE_MAX_ATTEMPTS", "0")
	if _, err := LoadPolicy(); err == nil {
		t.Fatal("unbounded guesses accepted")
	}
}
