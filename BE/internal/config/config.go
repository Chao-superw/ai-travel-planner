package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

func Env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func Require(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable: %s", key)
	}
	return v, nil
}
func ProviderURL(key, official string) (string, error) {
	v := Env(key, official)
	if strings.TrimRight(v, "/") == official {
		return official, nil
	}
	u, e := url.Parse(v)
	if e != nil {
		return "", fmt.Errorf("invalid %s", key)
	}
	if os.Getenv("ALLOW_LOCAL_PROVIDERS") == "true" && u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost") && u.User == nil && u.RawQuery == "" && u.Fragment == "" {
		return strings.TrimRight(v, "/"), nil
	}
	return "", fmt.Errorf("%s must use official HTTPS endpoint or explicitly enabled loopback test endpoint", key)
}
