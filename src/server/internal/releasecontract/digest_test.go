package releasecontract

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalBytesGolden(t *testing.T) {
	contract, _ := loadDigestGolden(t, "canonical-equivalent-a.json")
	_, expected := loadDigestGolden(t, "canonical-equivalent-b.json")
	expected = bytes.TrimSuffix(expected, []byte{'\n'})

	canonical, err := CanonicalBytes(contract)
	if err != nil {
		t.Fatalf("canonical bytes: %v", err)
	}
	if !bytes.Equal(canonical, expected) {
		t.Fatalf("canonical bytes mismatch\n got: %s\nwant: %s", canonical, expected)
	}
	if bytes.HasSuffix(canonical, []byte{'\n'}) {
		t.Fatal("canonical bytes must not have a trailing newline")
	}

	digest, err := Digest(contract)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	const expectedDigest = "sha256:2baeda420ab178420e0fcb64072ce3c91bc3b821ed8dd1eede6eac073864cbfa"
	if digest != expectedDigest {
		t.Fatalf("digest = %q, want %q", digest, expectedDigest)
	}
	checkedIn := readTestFile(t, "config/release/contract.v1.json")
	if strings.Contains(string(checkedIn), digest) {
		t.Fatal("authored contract must not contain its derived digest")
	}
}

func TestDigestEquivalentFormatting(t *testing.T) {
	contractA, _ := loadDigestGolden(t, "canonical-equivalent-a.json")
	contractB, _ := loadDigestGolden(t, "canonical-equivalent-b.json")

	// appliesTo is set-like. Canonicalization remains stable if a typed caller
	// presents the already-validated values in a different in-memory order.
	contractB.ReasonCodes[0].AppliesTo = []string{"report", "build"}
	digestA, err := Digest(contractA)
	if err != nil {
		t.Fatalf("digest A: %v", err)
	}
	digestB, err := Digest(contractB)
	if err != nil {
		t.Fatalf("digest B: %v", err)
	}
	if digestA != digestB {
		t.Fatalf("equivalent digests differ: %s != %s", digestA, digestB)
	}
}

func TestDigestSemanticMutation(t *testing.T) {
	baseline, _ := loadDigestGolden(t, "canonical-equivalent-a.json")
	mutated, _ := loadDigestGolden(t, "canonical-semantic-change.json")

	baselineDigest, err := Digest(baseline)
	if err != nil {
		t.Fatalf("baseline digest: %v", err)
	}
	mutatedDigest, err := Digest(mutated)
	if err != nil {
		t.Fatalf("mutated digest: %v", err)
	}
	if baselineDigest == mutatedDigest {
		t.Fatalf("semantic mutation retained digest %s", baselineDigest)
	}
}

func loadDigestGolden(t *testing.T, name string) (AuthoredContractV1, []byte) {
	t.Helper()
	contractBytes := readTestFile(t, filepath.Join("src/server/internal/releasecontract/testdata", name))
	schemaBytes := readTestFile(t, "config/release/contract.schema.json")
	contract, err := LoadBytes(context.Background(), testRepoRoot(t), contractBytes, schemaBytes)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return contract, contractBytes
}
