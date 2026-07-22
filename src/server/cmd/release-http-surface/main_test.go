package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"oblivious/server/internal/buildinfo"
	runtimehttp "oblivious/server/internal/http"
	"oblivious/server/internal/releasecontract"
	"oblivious/server/internal/surfacereport"
)

func TestReleaseHTTPRuntimeSurfaceCommandContract(t *testing.T) {
	repoRoot := httpSurfaceRepoRoot(t)
	manifestPath := filepath.Join(repoRoot, "docs/api/route-surface-manifest.json")

	t.Run("complete runtime registration writes one trusted report", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "nested", "http-runtime.json")
		identity := httpSurfaceIdentity()
		writer := &recordingHTTPReportWriter{delegate: surfacereport.NewAtomicWriter()}
		deps := httpSurfaceTestDependencies(identity, writer)

		stdout, stderr, exitCode := runHTTPCommand(repoRoot, manifestPath, output, deps)
		if exitCode != 0 {
			t.Fatalf("command exit=%d stdout=%s stderr=%s", exitCode, stdout, stderr)
		}
		if writer.calls != 1 || writer.path != output {
			t.Fatalf("writer calls/path = %d/%q", writer.calls, writer.path)
		}
		content, err := os.ReadFile(output)
		if err != nil {
			t.Fatalf("read report: %v", err)
		}
		report, err := surfacereport.Decode(content, surfacereport.NewDetailsRegistry())
		if err != nil {
			t.Fatalf("decode report: %v", err)
		}
		if report.ReleaseIdentity.ReleaseCommit != identity.ReleaseCommit || report.ReleaseIdentity.SourceTree != identity.SourceTree || report.ReleaseIdentity.ContractDigest != identity.ContractDigest {
			t.Fatalf("report identity was not resolver-owned: %+v", report.ReleaseIdentity)
		}
		if report.SurfaceIdentity.Surface != surfacereport.HTTPRuntimeSurfaceID || report.Outcome.Result != surfacereport.ResultPass || len(report.Outcome.SkippedChecks) != 0 {
			t.Fatalf("unexpected report surface/outcome: %s %+v", report.SurfaceIdentity.Surface, report.Outcome)
		}
		var details surfacereport.HTTPRuntimeDetails
		if err := json.Unmarshal(report.Evidence.Details, &details); err != nil {
			t.Fatalf("decode details: %v", err)
		}
		if details.OperationCount != expectedHTTPRuntimeOperationCount || details.MountedCount != expectedHTTPRuntimeOperationCount || details.DescriptorCount != expectedHTTPRuntimeOperationCount || details.MediaProbeCount <= 0 || details.ParityResult != "pass" || details.CoreDigest != details.RuntimeDigest {
			t.Fatalf("unexpected complete runtime details: %+v", details)
		}
		for _, prohibited := range []string{repoRoot, output, "releaseCommit", "sourceTree", "responseDecoder", "skippedChecks"} {
			if strings.Contains(stdout+stderr, prohibited) {
				t.Fatalf("diagnostics exposed prohibited value %q: stdout=%s stderr=%s", prohibited, stdout, stderr)
			}
		}
	})

	t.Run("runtime is constructed before manifest comparison", func(t *testing.T) {
		order := []string{}
		deps := httpSurfaceTestDependencies(httpSurfaceIdentity(), &recordingHTTPReportWriter{delegate: surfacereport.NewAtomicWriter()})
		defaultSnapshotBuilder := deps.snapshotBuilder
		defaultManifestLoader := deps.manifestLoader
		deps.snapshotBuilder = func() ([]runtimehttp.RouteSurfaceDescriptor, error) {
			order = append(order, "runtime")
			return defaultSnapshotBuilder()
		}
		deps.manifestLoader = func(path string) (httpSurfaceManifest, error) {
			order = append(order, "manifest")
			return defaultManifestLoader(path)
		}
		_, stderr, exitCode := runHTTPCommand(repoRoot, manifestPath, filepath.Join(t.TempDir(), "report.json"), deps)
		if exitCode != 0 || strings.Join(order, ",") != "runtime,manifest" {
			t.Fatalf("construction order=%v exit=%d stderr=%s", order, exitCode, stderr)
		}
	})

	t.Run("frontend handoff resolves the exact owner inventory", func(t *testing.T) {
		counts := verifyHTTPFrontendOwnerClosure(t, repoRoot)
		expected := httpFrontendUsageCounts{
			Owners: 24, Imports: 265, Uses: 267, Dispositions: 25,
			TransportCalls: 267, SharedTransports: 4,
		}
		if counts != expected {
			t.Fatalf("frontend owner closure counts=%+v want=%+v", counts, expected)
		}
	})

	t.Run("descriptor and dependency failures are nonzero without output", func(t *testing.T) {
		baseline, err := buildHTTPRuntimeSnapshot()
		if err != nil {
			t.Fatalf("build baseline snapshot: %v", err)
		}
		if len(baseline) != expectedHTTPRuntimeOperationCount {
			t.Fatalf("baseline descriptor count=%d", len(baseline))
		}
		cases := []struct {
			name   string
			mutate func(*dependencies)
		}{
			{name: "zero descriptors", mutate: func(deps *dependencies) {
				deps.snapshotBuilder = func() ([]runtimehttp.RouteSurfaceDescriptor, error) { return nil, nil }
			}},
			{name: "missing descriptor", mutate: func(deps *dependencies) {
				deps.snapshotBuilder = func() ([]runtimehttp.RouteSurfaceDescriptor, error) {
					return cloneHTTPDescriptors(baseline[:len(baseline)-1]), nil
				}
			}},
			{name: "extra descriptor", mutate: func(deps *dependencies) {
				deps.snapshotBuilder = func() ([]runtimehttp.RouteSurfaceDescriptor, error) {
					return append(cloneHTTPDescriptors(baseline), baseline[0]), nil
				}
			}},
			{name: "duplicate descriptor", mutate: func(deps *dependencies) {
				deps.snapshotBuilder = func() ([]runtimehttp.RouteSurfaceDescriptor, error) {
					value := cloneHTTPDescriptors(baseline)
					value[len(value)-1] = value[0]
					return value, nil
				}
			}},
			{name: "descriptor identity drift", mutate: func(deps *dependencies) {
				deps.snapshotBuilder = func() ([]runtimehttp.RouteSurfaceDescriptor, error) {
					value := cloneHTTPDescriptors(baseline)
					value[0].CapabilityID = "identity.splice"
					return value, nil
				}
			}},
			{name: "runtime unavailable", mutate: func(deps *dependencies) {
				deps.snapshotBuilder = func() ([]runtimehttp.RouteSurfaceDescriptor, error) { return nil, errors.New("do-not-print") }
			}},
			{name: "identity unavailable", mutate: func(deps *dependencies) {
				deps.identityProvider = &httpSurfaceIdentityProvider{err: &buildinfo.IdentityError{Code: buildinfo.ErrorBuildIdentityMissing, Field: "releaseCommit"}}
			}},
			{name: "output failure", mutate: func(deps *dependencies) {
				deps.reportWriter = &recordingHTTPReportWriter{err: &surfacereport.ReportError{Code: surfacereport.ErrorReportOutputUnwritable, Field: "destination"}}
			}},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				output := filepath.Join(t.TempDir(), "report.json")
				deps := httpSurfaceTestDependencies(httpSurfaceIdentity(), &recordingHTTPReportWriter{delegate: surfacereport.NewAtomicWriter()})
				test.mutate(&deps)
				stdout, stderr, exitCode := runHTTPCommand(repoRoot, manifestPath, output, deps)
				if exitCode == 0 || httpSurfaceFileExists(output) || strings.Contains(stdout+stderr, "do-not-print") {
					t.Fatalf("failure passed, wrote output, or leaked: exit=%d exists=%t stdout=%s stderr=%s", exitCode, httpSurfaceFileExists(output), stdout, stderr)
				}
			})
		}
	})

	t.Run("browser fields and caller authority flags are rejected", func(t *testing.T) {
		manifestBytes, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("read manifest: %v", err)
		}
		var value map[string]any
		if err := json.Unmarshal(manifestBytes, &value); err != nil {
			t.Fatalf("decode manifest fixture: %v", err)
		}
		operations := value["operations"].([]any)
		operations[0].(map[string]any)["responseDecoder"] = "browser"
		mutatedPath := filepath.Join(t.TempDir(), "manifest.json")
		mutatedBytes, _ := json.Marshal(value)
		if err := os.WriteFile(mutatedPath, mutatedBytes, 0o600); err != nil {
			t.Fatalf("write manifest fixture: %v", err)
		}
		output := filepath.Join(t.TempDir(), "report.json")
		_, stderr, exitCode := runHTTPCommand(repoRoot, mutatedPath, output, httpSurfaceTestDependencies(httpSurfaceIdentity(), &recordingHTTPReportWriter{delegate: surfacereport.NewAtomicWriter()}))
		if exitCode == 0 || httpSurfaceFileExists(output) || strings.Contains(stderr, "browser") {
			t.Fatalf("browser mutation passed, wrote output, or leaked: exit=%d stderr=%s", exitCode, stderr)
		}

		base := httpSurfaceArgs(repoRoot, manifestPath, filepath.Join(t.TempDir(), "report.json"))
		for _, flag := range []string{"--release-commit", "--source-tree", "--evidence-class", "--skipped-checks", "--response-decoder"} {
			args := append(append([]string(nil), base...), flag, "do-not-print")
			var stdout, stderr bytes.Buffer
			writer := &recordingHTTPReportWriter{delegate: surfacereport.NewAtomicWriter()}
			exitCode := runWithDependencies(context.Background(), args, &stdout, &stderr, httpSurfaceTestDependencies(httpSurfaceIdentity(), writer))
			if exitCode == 0 || writer.calls != 0 || strings.Contains(stdout.String()+stderr.String(), "do-not-print") {
				t.Fatalf("authority flag %s passed or leaked: exit=%d calls=%d stdout=%s stderr=%s", flag, exitCode, writer.calls, stdout.String(), stderr.String())
			}
		}
	})
}

func httpSurfaceTestDependencies(identity buildinfo.BuildIdentityV1, writer surfacereport.ReportWriter) dependencies {
	return dependencies{
		identityProvider: &httpSurfaceIdentityProvider{identity: identity},
		profileResolver:  &httpSurfaceProfileResolver{profile: releasecontract.DeploymentProfile{ID: "monolith", Commitment: releasecontract.CommitmentCommitted}},
		reportWriter:     writer,
		snapshotBuilder:  buildHTTPRuntimeSnapshot,
		manifestLoader:   loadHTTPRuntimeManifest,
	}
}

func runHTTPCommand(repoRoot, manifestPath, output string, deps dependencies) (string, string, int) {
	var stdout, stderr bytes.Buffer
	exitCode := runWithDependencies(context.Background(), httpSurfaceArgs(repoRoot, manifestPath, output), &stdout, &stderr, deps)
	return stdout.String(), stderr.String(), exitCode
}

func httpSurfaceArgs(repoRoot, manifestPath, output string) []string {
	return []string{
		"--repo", repoRoot,
		"--contract", "config/release/contract.v1.json",
		"--schema", "config/release/contract.schema.json",
		"--profile", "monolith",
		"--manifest", manifestPath,
		"--output", output,
	}
}

func httpSurfaceIdentity() buildinfo.BuildIdentityV1 {
	return buildinfo.BuildIdentityV1{
		SchemaVersion:  buildinfo.BuildIdentitySchemaV1,
		ReleaseCommit:  strings.Repeat("a", 40),
		SourceTree:     strings.Repeat("b", 40),
		ContractDigest: "sha256:" + strings.Repeat("c", 64),
		EvidenceClass:  buildinfo.EvidenceRepositoryLocal,
	}
}

type httpSurfaceIdentityProvider struct {
	identity buildinfo.BuildIdentityV1
	err      error
}

func (p *httpSurfaceIdentityProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	return p.identity, p.err
}

type httpSurfaceProfileResolver struct {
	profile releasecontract.DeploymentProfile
	err     error
}

func (r *httpSurfaceProfileResolver) ResolveCommittedProfile(context.Context, string, string, string, string) (releasecontract.DeploymentProfile, error) {
	return r.profile, r.err
}

type recordingHTTPReportWriter struct {
	delegate surfacereport.ReportWriter
	report   surfacereport.SurfaceReportV1
	path     string
	calls    int
	err      error
}

func (w *recordingHTTPReportWriter) Write(ctx context.Context, path string, report surfacereport.SurfaceReportV1) error {
	w.calls++
	w.path = path
	w.report = report
	if w.err != nil {
		return w.err
	}
	return w.delegate.Write(ctx, path, report)
}

func cloneHTTPDescriptors(source []runtimehttp.RouteSurfaceDescriptor) []runtimehttp.RouteSurfaceDescriptor {
	content, _ := json.Marshal(source)
	var cloned []runtimehttp.RouteSurfaceDescriptor
	_ = json.Unmarshal(content, &cloned)
	return cloned
}

func httpSurfaceRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve command source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
}

func httpSurfaceFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type httpFrontendUsageCounts struct {
	Owners           int `json:"owners"`
	Imports          int `json:"imports"`
	Uses             int `json:"uses"`
	Dispositions     int `json:"dispositions"`
	TransportCalls   int `json:"transportCalls"`
	SharedTransports int `json:"sharedTransports"`
}

func verifyHTTPFrontendOwnerClosure(t *testing.T, repoRoot string) httpFrontendUsageCounts {
	t.Helper()
	fixtureScript := filepath.Join(repoRoot, "scripts/verify-http-runtime-contract-fixtures.sh")
	listCommand := exec.Command("bash", fixtureScript, "--frontend-owner-list")
	listCommand.Dir = repoRoot
	inventory, err := listCommand.CombinedOutput()
	if err != nil {
		t.Fatalf("list frontend owner inventory: %v: %s", err, inventory)
	}
	lines := strings.Split(strings.TrimSpace(string(inventory)), "\n")
	if len(lines) != 25 {
		t.Fatalf("frontend owner inventory count=%d", len(lines))
	}

	arguments := []string{
		filepath.Join(repoRoot, "scripts/verify_frontend_operation_contract_usage.mjs"),
		"--tsconfig", filepath.Join(repoRoot, "src/web/tsconfig.json"),
	}
	seen := make(map[string]struct{}, len(lines))
	nonCallers := 0
	for _, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || len(fields) > 3 {
			t.Fatalf("frontend owner inventory row invalid: %q", line)
		}
		owner := fields[1]
		if filepath.IsAbs(owner) || filepath.Clean(owner) != owner || !httpSurfaceFileExists(filepath.Join(repoRoot, owner)) {
			t.Fatalf("frontend owner path invalid: %q", owner)
		}
		if _, exists := seen[owner]; exists {
			t.Fatalf("frontend owner duplicated: %q", owner)
		}
		seen[owner] = struct{}{}
		switch fields[0] {
		case "owner":
			if len(fields) != 2 {
				t.Fatalf("frontend owner has disposition reason: %q", line)
			}
			arguments = append(arguments, "--expect-owner", owner)
		case "non-caller":
			if len(fields) != 3 || strings.TrimSpace(fields[2]) == "" {
				t.Fatalf("frontend non-caller reason invalid: %q", line)
			}
			nonCallers++
			arguments = append(arguments, "--expect-non-caller", owner+"="+fields[2])
		default:
			t.Fatalf("frontend owner disposition invalid: %q", fields[0])
		}
	}
	if len(seen) != 25 || nonCallers != 1 {
		t.Fatalf("frontend owner inventory unique/non-callers=%d/%d", len(seen), nonCallers)
	}
	arguments = append(arguments, "--require-all")
	verifyCommand := exec.Command("node", arguments...)
	verifyCommand.Dir = repoRoot
	verified, err := verifyCommand.CombinedOutput()
	if err != nil {
		t.Fatalf("verify frontend owner closure: %v: %s", err, verified)
	}
	var evidence struct {
		SchemaVersion string                  `json:"schemaVersion"`
		EvidenceClass string                  `json:"evidenceClass"`
		Counts        httpFrontendUsageCounts `json:"counts"`
	}
	if err := json.Unmarshal(verified, &evidence); err != nil {
		t.Fatalf("decode frontend owner evidence: %v", err)
	}
	if evidence.SchemaVersion != "operation-contract-usage/v1" || evidence.EvidenceClass != "E1" {
		t.Fatalf("frontend owner evidence identity=%q/%q", evidence.SchemaVersion, evidence.EvidenceClass)
	}
	return evidence.Counts
}
