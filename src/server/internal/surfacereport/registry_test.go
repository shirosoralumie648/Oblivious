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
