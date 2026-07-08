package config

import "testing"

func TestValidateProductionRequiresOIDCAndSessionSecret(t *testing.T) {
	cfg := Config{DevAuth: false, PublicBase: "https://bws.example.com"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() err = nil, want missing production config error")
	}

	cfg.OIDCIssuerURL = "https://issuer.example.com"
	cfg.OIDCClientID = "client-id"
	cfg.OIDCClientSecret = "client-secret"
	cfg.SessionSecret = "production-secret"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with production config: %v", err)
	}
}
