package buildinfo

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildIdentityV1Validation(t *testing.T) {
	valid := validIdentity()
	if err := ValidateIdentity(valid); err != nil {
		t.Fatalf("valid identity: %v", err)
	}
	encoded, err := MarshalIdentity(valid)
	if err != nil {
		t.Fatalf("marshal identity: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode identity JSON: %v", err)
	}
	if len(fields) != 6 {
		t.Fatalf("identity JSON fields = %v, want exactly 6", fields)
	}

	tests := []struct {
		name string
		edit func(*BuildIdentityV1)
		code ErrorCode
	}{
		{name: "missing schema", edit: func(v *BuildIdentityV1) { v.SchemaVersion = "" }, code: ErrorBuildIdentityMissing},
		{name: "unknown schema", edit: func(v *BuildIdentityV1) { v.SchemaVersion = "build-identity/v2" }, code: ErrorBuildIdentityMismatch},
		{name: "missing commit", edit: func(v *BuildIdentityV1) { v.ReleaseCommit = "" }, code: ErrorBuildIdentityMissing},
		{name: "uppercase commit", edit: func(v *BuildIdentityV1) { v.ReleaseCommit = strings.ToUpper(v.ReleaseCommit) }, code: ErrorBuildIdentityMismatch},
		{name: "short tree", edit: func(v *BuildIdentityV1) { v.SourceTree = "abc" }, code: ErrorBuildIdentityMismatch},
		{name: "missing digest", edit: func(v *BuildIdentityV1) { v.ContractDigest = "" }, code: ErrorBuildIdentityMissing},
		{name: "malformed digest", edit: func(v *BuildIdentityV1) { v.ContractDigest = "sha256:ABC" }, code: ErrorContractDigestMismatch},
		{name: "dirty", edit: func(v *BuildIdentityV1) { v.Dirty = true }, code: ErrorSourceWorktreeDirty},
		{name: "unknown evidence", edit: func(v *BuildIdentityV1) { v.EvidenceClass = "target" }, code: ErrorBuildIdentityMismatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.edit(&candidate)
			if err := ValidateIdentity(candidate); !IsCode(err, test.code) {
				t.Fatalf("error = %v, want code %s", err, test.code)
			}
		})
	}
}

func TestParseLinkedIdentityRejectsInvalidValues(t *testing.T) {
	valid := validLinkerIdentity()
	identity, err := parseLinkerIdentity(valid)
	if err != nil {
		t.Fatalf("valid linked identity: %v", err)
	}
	if identity != validIdentity() {
		t.Fatalf("identity = %#v, want %#v", identity, validIdentity())
	}

	tests := []struct {
		name string
		edit func(*LinkerIdentity)
		code ErrorCode
	}{
		{name: "missing dirty", edit: func(v *LinkerIdentity) { v.Dirty = "" }, code: ErrorBuildIdentityMissing},
		{name: "dirty true", edit: func(v *LinkerIdentity) { v.Dirty = "true" }, code: ErrorSourceWorktreeDirty},
		{name: "unknown dirty", edit: func(v *LinkerIdentity) { v.Dirty = "0" }, code: ErrorBuildIdentityMismatch},
		{name: "uppercase commit", edit: func(v *LinkerIdentity) { v.ReleaseCommit = strings.ToUpper(v.ReleaseCommit) }, code: ErrorBuildIdentityMismatch},
		{name: "contract mismatch", edit: func(v *LinkerIdentity) { v.ContractDigest = "sha256:" + strings.Repeat("z", 64) }, code: ErrorContractDigestMismatch},
		{name: "unknown evidence", edit: func(v *LinkerIdentity) { v.EvidenceClass = "live" }, code: ErrorBuildIdentityMismatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.edit(&candidate)
			if _, err := parseLinkerIdentity(candidate); !IsCode(err, test.code) {
				t.Fatalf("error = %v, want code %s", err, test.code)
			}
		})
	}
}

func validIdentity() BuildIdentityV1 {
	return BuildIdentityV1{
		SchemaVersion:  BuildIdentitySchemaV1,
		ReleaseCommit:  strings.Repeat("a", 40),
		SourceTree:     strings.Repeat("b", 40),
		ContractDigest: "sha256:" + strings.Repeat("c", 64),
		Dirty:          false,
		EvidenceClass:  EvidenceRepositoryLocal,
	}
}

func validLinkerIdentity() LinkerIdentity {
	identity := validIdentity()
	return LinkerIdentity{
		ReleaseCommit:  identity.ReleaseCommit,
		SourceTree:     identity.SourceTree,
		ContractDigest: identity.ContractDigest,
		Dirty:          "false",
		EvidenceClass:  identity.EvidenceClass,
	}
}
