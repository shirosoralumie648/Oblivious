package surfacereport

import (
	"encoding/json"
	"strings"
	"testing"
)

type testDetails struct {
	Observed string `json:"observed"`
}

func TestSurfaceReportV1NestedValidation(t *testing.T) {
	registry := testRegistry(t)
	report := validTestReport(t, registry)
	if err := Validate(report, registry); err != nil {
		t.Fatalf("validate report: %v", err)
	}
	encoded, err := Marshal(report, registry)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	decoded, err := Decode(encoded, registry)
	if err != nil {
		t.Fatalf("strict decode report: %v", err)
	}
	if decoded.ReleaseIdentity != report.ReleaseIdentity || decoded.SurfaceIdentity != report.SurfaceIdentity {
		t.Fatalf("decoded identities = %#v / %#v", decoded.ReleaseIdentity, decoded.SurfaceIdentity)
	}

	report.Outcome.SkippedChecks = []string{"database"}
	if err := Validate(report, registry); !IsCode(err, ErrorSurfaceSchemaInvalid) {
		t.Fatalf("pass with skip error = %v", err)
	}
}

func TestSurfaceReportV1RejectsFlatAndMisplacedFields(t *testing.T) {
	registry := testRegistry(t)
	base, err := Marshal(validTestReport(t, registry), registry)
	if err != nil {
		t.Fatalf("marshal baseline: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "flat identity", mutate: func(value map[string]any) { value["releaseCommit"] = strings.Repeat("a", 40) }},
		{name: "errors under evidence", mutate: func(value map[string]any) { value["evidence"].(map[string]any)["errorCodes"] = []any{} }},
		{name: "environment under drift", mutate: func(value map[string]any) { value["drift"].(map[string]any)["environment"] = "local" }},
		{name: "unknown nested", mutate: func(value map[string]any) { value["surfaceIdentity"].(map[string]any)["readiness"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(base, &value); err != nil {
				t.Fatalf("decode baseline: %v", err)
			}
			test.mutate(value)
			mutated, err := json.Marshal(value)
			if err != nil {
				t.Fatalf("marshal mutation: %v", err)
			}
			if _, err := Decode(mutated, registry); !IsCode(err, ErrorSurfaceSchemaInvalid) {
				t.Fatalf("mutation error = %v", err)
			}
		})
	}
}

func testRegistry(t *testing.T) *DetailsRegistry {
	t.Helper()
	registry := NewDetailsRegistry()
	if err := RegisterDetails(registry, "test-surface", func(details testDetails) error {
		if details.Observed == "" {
			return reportError("observed", nil)
		}
		return nil
	}); err != nil {
		t.Fatalf("register test details: %v", err)
	}
	return registry
}

func validTestReport(t *testing.T, registry *DetailsRegistry) SurfaceReportV1 {
	t.Helper()
	details, err := registry.MarshalDetails("test-surface", testDetails{Observed: "matched"})
	if err != nil {
		t.Fatalf("marshal details: %v", err)
	}
	return NewReport(
		ReleaseIdentity{
			ReleaseCommit: strings.Repeat("a", 40), SourceTree: strings.Repeat("b", 40),
			ContractDigest: "sha256:" + strings.Repeat("c", 64), DeploymentProfile: "monolith",
			Dirty: false, EvidenceClass: "repository-local",
		},
		SurfaceIdentity{
			Surface: "test-surface", CanonicalSource: "canonical.json", Consumer: "test-consumer",
			Version: "v1", SourceDigest: "sha256:" + strings.Repeat("d", 64), ConsumerDigest: "sha256:" + strings.Repeat("d", 64),
		},
		Drift{Missing: []string{}, Extra: []string{}, Incompatible: []string{}},
		Evidence{Class: "repository-local", Environment: "test", Mode: "fixture", CheckedAt: "2026-07-16T00:00:00Z", ToolVersions: map[string]string{"go": "1.25"}, Details: details},
		Outcome{Result: ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}},
	)
}
