package secretbox

import (
	"strings"
	"testing"
)

func TestProtectOpenRoundTripAndLegacyPlaintext(t *testing.T) {
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", "test-secretbox-key")

	protected, err := Protect(DomainRelayChannelAPIKey, "sk-secretbox-roundtrip")
	if err != nil {
		t.Fatalf("Protect returned error: %v", err)
	}
	if !IsProtected(protected) {
		t.Fatalf("expected protected value prefix, got %q", protected)
	}
	if strings.Contains(protected, "sk-secretbox-roundtrip") {
		t.Fatalf("protected value contains plaintext: %q", protected)
	}

	opened, err := Open(DomainRelayChannelAPIKey, protected)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if opened != "sk-secretbox-roundtrip" {
		t.Fatalf("opened value = %q, want plaintext", opened)
	}

	legacy, err := Open(DomainRelayChannelAPIKey, "sk-legacy-plaintext")
	if err != nil {
		t.Fatalf("Open legacy plaintext returned error: %v", err)
	}
	if legacy != "sk-legacy-plaintext" {
		t.Fatalf("legacy plaintext = %q", legacy)
	}
}

func TestOpenRejectsWrongDomain(t *testing.T) {
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", "test-secretbox-key")

	protected, err := Protect(DomainRelayChannelAPIKey, "sk-domain-secret")
	if err != nil {
		t.Fatalf("Protect returned error: %v", err)
	}
	if _, err := Open("different-domain", protected); err == nil {
		t.Fatal("Open with a different domain should fail")
	}
}
