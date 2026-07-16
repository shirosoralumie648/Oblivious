package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"oblivious/server/internal/buildinfo"
)

func TestIdentityInspectionPrecedesRuntimeSideEffects(t *testing.T) {
	identity := testIdentity()
	for _, test := range []struct {
		name     string
		args     []string
		provider buildinfo.IdentityProvider
		wantCode int
		wantRuns int
	}{
		{name: "inspection success", args: []string{buildinfo.InspectionFlag}, provider: staticInspectionProvider{identity: identity}, wantCode: 0},
		{name: "inspection failure", args: []string{buildinfo.InspectionFlag}, provider: staticInspectionProvider{err: &buildinfo.IdentityError{Code: buildinfo.ErrorBuildIdentityMismatch, Field: "releaseCommit"}}, wantCode: 1},
		{name: "normal startup", args: []string{"--serve"}, provider: staticInspectionProvider{identity: identity}, wantCode: 0, wantRuns: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			configLoads, databaseOpens, migrations, listeners := 0, 0, 0, 0
			exitCode := runMain(context.Background(), test.args, inspectionDependencies{
				provider: test.provider, stdout: &stdout, stderr: &stderr,
				repoRoot: "/app", contract: packagedContractPath, schema: packagedSchemaPath,
			}, func() {
				configLoads++
				databaseOpens++
				migrations++
				listeners++
			})
			if exitCode != test.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr=%s", exitCode, test.wantCode, stderr.String())
			}
			for name, calls := range map[string]int{"config": configLoads, "database": databaseOpens, "migrations": migrations, "listener": listeners} {
				if calls != test.wantRuns {
					t.Fatalf("%s calls = %d, want %d", name, calls, test.wantRuns)
				}
			}
			switch test.name {
			case "inspection success":
				var got buildinfo.BuildIdentityV1
				if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || got != identity {
					t.Fatalf("inspection output = %q, identity=%#v, error=%v", stdout.String(), got, err)
				}
			case "inspection failure":
				if !strings.Contains(stderr.String(), string(buildinfo.ErrorBuildIdentityMismatch)) {
					t.Fatalf("inspection error = %q", stderr.String())
				}
			}
		})
	}
}

type staticInspectionProvider struct {
	identity buildinfo.BuildIdentityV1
	err      error
}

func (p staticInspectionProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	if p.err != nil {
		return buildinfo.BuildIdentityV1{}, p.err
	}
	return p.identity, nil
}

func testIdentity() buildinfo.BuildIdentityV1 {
	return buildinfo.BuildIdentityV1{
		SchemaVersion: buildinfo.BuildIdentitySchemaV1, ReleaseCommit: strings.Repeat("a", 40),
		SourceTree: strings.Repeat("b", 40), ContractDigest: "sha256:" + strings.Repeat("c", 64),
		Dirty: false, EvidenceClass: buildinfo.EvidenceRepositoryLocal,
	}
}
