package secretbox

import (
	"errors"
	"strings"
	"testing"
)

func TestProtectOpenRoundTripAndLegacyPlaintext(t *testing.T) {
	t.Setenv("APP_ENV", "development")
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

func TestOpenRejectsLegacyPlaintextInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", "test-secretbox-key")

	opened, err := Open(DomainRelayChannelAPIKey, "sk-legacy-plaintext")
	if !errors.Is(err, ErrPlaintextSecretRejected) {
		t.Fatalf("expected ErrPlaintextSecretRejected, got opened=%q err=%v", opened, err)
	}
	if opened != "" {
		t.Fatalf("expected rejected plaintext not to be returned, got %q", opened)
	}
}

func TestOpenAllowsProtectedValuesInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", "test-secretbox-key")

	protected, err := Protect(DomainRelayChannelAPIKey, "sk-production-protected")
	if err != nil {
		t.Fatalf("Protect returned error: %v", err)
	}
	opened, err := Open(DomainRelayChannelAPIKey, protected)
	if err != nil {
		t.Fatalf("Open protected value in production returned error: %v", err)
	}
	if opened != "sk-production-protected" {
		t.Fatalf("opened value = %q, want protected plaintext", opened)
	}
}

func TestInspectStoredClassifiesLegacyPlaintextWithoutReturningSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", "test-secretbox-key")

	protected, err := Protect(DomainRelayChannelAPIKey, "sk-inspect-protected")
	if err != nil {
		t.Fatalf("Protect returned error: %v", err)
	}

	tests := []struct {
		name        string
		stored      string
		wantStatus  SecretStorageStatus
		wantRotate  bool
		wantMessage string
	}{
		{name: "empty", stored: "", wantStatus: SecretStorageStatusEmpty},
		{name: "protected", stored: protected, wantStatus: SecretStorageStatusProtected},
		{name: "legacy plaintext", stored: "sk-legacy-plaintext", wantStatus: SecretStorageStatusPlaintext, wantRotate: true, wantMessage: "plaintext"},
		{name: "invalid protected payload", stored: CodecPrefix + "not-valid-base64!", wantStatus: SecretStorageStatusInvalidProtected, wantRotate: true, wantMessage: "invalid protected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inspection := InspectStored(DomainRelayChannelAPIKey, tt.stored)
			if inspection.Status != tt.wantStatus {
				t.Fatalf("Status=%s, want %s", inspection.Status, tt.wantStatus)
			}
			if inspection.NeedsRotation != tt.wantRotate {
				t.Fatalf("NeedsRotation=%v, want %v", inspection.NeedsRotation, tt.wantRotate)
			}
			if tt.wantMessage != "" && !strings.Contains(inspection.Message, tt.wantMessage) {
				t.Fatalf("Message=%q, want it to contain %q", inspection.Message, tt.wantMessage)
			}
			if strings.Contains(inspection.Message, "sk-legacy-plaintext") || strings.Contains(inspection.Message, "sk-inspect-protected") {
				t.Fatalf("inspection leaked a secret value: %+v", inspection)
			}
		})
	}
}

func TestInspectStoredMapFindsNestedLegacySecretsWithoutLeakingValues(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", "test-secretbox-key")

	protected, err := Protect(DomainWorkflowDefinitionSecretValue, "whsec-protected")
	if err != nil {
		t.Fatalf("Protect returned error: %v", err)
	}

	results := InspectStoredMap(DomainWorkflowDefinitionSecretValue, map[string]any{
		"webhook_secret": "whsec-legacy",
		"nodes": []any{
			map[string]any{
				"id": "notify",
				"input": map[string]any{
					"secret": protected,
				},
			},
			map[string]any{
				"id": "broken",
				"input": map[string]any{
					"secret": CodecPrefix + "broken!",
				},
			},
		},
		"non_secret": "leave-me-alone",
	}, func(key string) bool {
		normalized := strings.ToLower(strings.ReplaceAll(key, "_", ""))
		return normalized == "secret" || normalized == "webhooksecret"
	})

	if len(results) != 3 {
		t.Fatalf("expected three inspected secret values, got %+v", results)
	}
	byPath := map[string]SecretStorageInspection{}
	for _, result := range results {
		byPath[result.Path] = result
		if strings.Contains(result.Message, "whsec-legacy") || strings.Contains(result.Message, "whsec-protected") {
			t.Fatalf("inspection leaked secret value: %+v", result)
		}
	}
	if byPath["webhook_secret"].Status != SecretStorageStatusPlaintext || !byPath["webhook_secret"].NeedsRotation {
		t.Fatalf("expected webhook_secret legacy plaintext rotation finding, got %+v", byPath["webhook_secret"])
	}
	if byPath["nodes[0].input.secret"].Status != SecretStorageStatusProtected || byPath["nodes[0].input.secret"].NeedsRotation {
		t.Fatalf("expected protected nested secret to be healthy, got %+v", byPath["nodes[0].input.secret"])
	}
	if byPath["nodes[1].input.secret"].Status != SecretStorageStatusInvalidProtected || !byPath["nodes[1].input.secret"].NeedsRotation {
		t.Fatalf("expected broken nested secret to need rotation, got %+v", byPath["nodes[1].input.secret"])
	}
	if _, ok := byPath["non_secret"]; ok {
		t.Fatalf("non-secret key should not be inspected, got %+v", byPath["non_secret"])
	}
}
