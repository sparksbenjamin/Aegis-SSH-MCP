package mcp

import "testing"

func TestSSEConfigValidateRequiresHTTPSByDefault(t *testing.T) {
	t.Parallel()

	cfg := SSEConfig{
		BaseURL:     "http://localhost:8443",
		TLSCertFile: "/certs/tls.crt",
		TLSKeyFile:  "/certs/tls.key",
	}

	if err := cfg.validate(); err == nil {
		t.Fatal("expected https validation error")
	}
}

func TestSSEConfigValidateAllowsHTTPWhenTLSDisabled(t *testing.T) {
	t.Parallel()

	cfg := SSEConfig{
		BaseURL:    "http://localhost:8443",
		DisableTLS: true,
	}

	if err := cfg.validate(); err != nil {
		t.Fatalf("expected http config to validate when TLS is disabled: %v", err)
	}
}

func TestSSEConfigValidateRejectsHTTPSDisabledTLSMismatch(t *testing.T) {
	t.Parallel()

	cfg := SSEConfig{
		BaseURL:    "https://localhost:8443",
		DisableTLS: true,
	}

	if err := cfg.validate(); err == nil {
		t.Fatal("expected scheme mismatch error when TLS is disabled")
	}
}
