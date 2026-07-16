package buildinfo

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestHandleInspectionReturnsTrustedIdentityWithoutSideEffects(t *testing.T) {
	provider := &recordingIdentityProvider{identity: validIdentity()}
	var stdout, stderr bytes.Buffer
	handled, exitCode := HandleInspection(context.Background(), []string{InspectionFlag}, &stdout, &stderr, provider, "/explicit/root", "contract.json", "schema.json")
	if !handled || exitCode != 0 {
		t.Fatalf("handled=%v exit=%d stderr=%s", handled, exitCode, stderr.String())
	}
	if provider.calls != 1 || provider.repoRoot != "/explicit/root" || provider.contractPath != "contract.json" || provider.schemaPath != "schema.json" {
		t.Fatalf("provider call = %#v", provider)
	}
	var got BuildIdentityV1
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if got != validIdentity() || stderr.Len() != 0 {
		t.Fatalf("identity=%#v stderr=%q", got, stderr.String())
	}
}

func TestHandleInspectionRejectsMismatch(t *testing.T) {
	provider := &recordingIdentityProvider{err: identityError(ErrorContractDigestMismatch, "contractDigest", nil)}
	var stdout, stderr bytes.Buffer
	handled, exitCode := HandleInspection(context.Background(), []string{InspectionFlag}, &stdout, &stderr, provider, "/explicit/root", "contract.json", "schema.json")
	if !handled || exitCode == 0 || stdout.Len() != 0 {
		t.Fatalf("handled=%v exit=%d stdout=%q", handled, exitCode, stdout.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte(`"code":"contract_digest_mismatch"`)) {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestHandleInspectionUsesExplicitRepoRoot(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("change cwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	provider := &recordingIdentityProvider{identity: validIdentity()}
	var stdout, stderr bytes.Buffer
	handled, exitCode := HandleInspection(context.Background(), []string{InspectionFlag, "--contract", "packaged.json", "--schema", "packaged.schema.json"}, &stdout, &stderr, provider, "/trusted/root", "default.json", "default.schema.json")
	if !handled || exitCode != 0 {
		t.Fatalf("handled=%v exit=%d stderr=%s", handled, exitCode, stderr.String())
	}
	if provider.repoRoot != "/trusted/root" || provider.contractPath != "packaged.json" || provider.schemaPath != "packaged.schema.json" {
		t.Fatalf("provider used wrong inputs: %#v", provider)
	}
}

type recordingIdentityProvider struct {
	identity     BuildIdentityV1
	err          error
	calls        int
	repoRoot     string
	contractPath string
	schemaPath   string
}

func (p *recordingIdentityProvider) Resolve(_ context.Context, repoRoot, contractPath, schemaPath string) (BuildIdentityV1, error) {
	p.calls++
	p.repoRoot = repoRoot
	p.contractPath = contractPath
	p.schemaPath = schemaPath
	return p.identity, p.err
}
