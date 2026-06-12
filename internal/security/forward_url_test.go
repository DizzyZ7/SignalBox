package security

import "testing"

func TestValidateForwardURLAllowsPublicHTTPSTarget(t *testing.T) {
	if err := ValidateForwardURL("https://example.com/webhooks/signalbox", false); err != nil {
		t.Fatalf("expected public https target to be allowed: %v", err)
	}
}

func TestValidateForwardURLRejectsPrivateAndLocalTargets(t *testing.T) {
	tests := []string{
		"http://localhost:8080/webhook",
		"http://127.0.0.1:8080/webhook",
		"http://10.0.0.10/webhook",
		"http://172.16.0.10/webhook",
		"http://192.168.1.10/webhook",
		"http://[::1]/webhook",
		"http://[fe80::1]/webhook",
		"http://internal.local/webhook",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if err := ValidateForwardURL(raw, false); err == nil {
				t.Fatalf("expected %s to be rejected", raw)
			}
		})
	}
}

func TestValidateForwardURLAllowsPrivateTargetsWhenConfigured(t *testing.T) {
	if err := ValidateForwardURL("http://127.0.0.1:8080/webhook", true); err != nil {
		t.Fatalf("expected private target to be allowed when configured: %v", err)
	}
}

func TestValidateForwardURLRejectsUnsafeShapes(t *testing.T) {
	tests := []string{
		"ftp://example.com/webhook",
		"https://user:pass@example.com/webhook",
		"https:///missing-host",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if err := ValidateForwardURL(raw, false); err == nil {
				t.Fatalf("expected %s to be rejected", raw)
			}
		})
	}
}
