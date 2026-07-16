package surfacereport

import "testing"

func TestDetailsRegistryRejectsDuplicateUnknownAndWrongType(t *testing.T) {
	registry := NewDetailsRegistry()
	validate := func(details testDetails) error {
		if details.Observed == "" {
			return reportError("observed", nil)
		}
		return nil
	}
	if err := RegisterDetails(registry, "test-surface", validate); err != nil {
		t.Fatalf("register details: %v", err)
	}
	if err := RegisterDetails(registry, "test-surface", validate); !IsCode(err, ErrorSurfaceSchemaInvalid) {
		t.Fatalf("duplicate error = %v", err)
	}
	if _, err := registry.MarshalDetails("unknown", testDetails{Observed: "value"}); !IsCode(err, ErrorSurfaceSchemaInvalid) {
		t.Fatalf("unknown error = %v", err)
	}
	if _, err := registry.MarshalDetails("test-surface", struct{ Observed string }{Observed: "value"}); !IsCode(err, ErrorSurfaceSchemaInvalid) {
		t.Fatalf("wrong type error = %v", err)
	}
	if err := registry.ValidateDetails("test-surface", []byte(`{"observed":"value","arbitrary":true}`)); !IsCode(err, ErrorSurfaceSchemaInvalid) {
		t.Fatalf("unknown details field error = %v", err)
	}
}

func TestBuildIdentityDetailsRegistration(t *testing.T) {
	registry := NewDetailsRegistry()
	details := validBuildDetails(validBuildIdentity())
	raw, err := registry.MarshalDetails(BuildIdentitySurfaceID, details)
	if err != nil {
		t.Fatalf("marshal registered build details: %v", err)
	}
	if err := registry.ValidateDetails(BuildIdentitySurfaceID, raw); err != nil {
		t.Fatalf("validate registered build details: %v", err)
	}
	if err := RegisterDetails(registry, BuildIdentitySurfaceID, validateBuildIdentityDetails); !IsCode(err, ErrorSurfaceSchemaInvalid) {
		t.Fatalf("replace foundation registration error = %v", err)
	}
}
