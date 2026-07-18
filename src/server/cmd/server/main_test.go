package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
	sharedconfig "oblivious/server/pkg/config"
)

func TestStandaloneProfilePreflightGroupOneContract(t *testing.T) {
	profile := releasecontract.DeploymentProfile{
		ID: "monolith", Commitment: releasecontract.CommitmentCommitted,
		RefreshIntervalSeconds: 30, MaxAgeSeconds: 120, AllowedFutureSkewSeconds: 30,
		Topology:    releasecontract.Topology{Kind: releasecontract.TopologyMonolith, Components: []string{"server"}},
		Entrypoints: []string{"server", "agent", "billing", "channel", "chat"},
	}
	contract := releasecontract.AuthoredContractV1{SchemaVersion: releasecontract.SchemaVersionV1, Profiles: []releasecontract.DeploymentProfile{profile}}
	digest, err := releasecontract.Digest(contract)
	if err != nil {
		t.Fatal(err)
	}
	identity := testIdentity()
	identity.ContractDigest = digest

	for _, root := range []string{"server", "agent", "billing", "channel", "chat"} {
		t.Run(root+" success freezes one exact handoff", func(t *testing.T) {
			spies := &entrypointPreflightSpies{contract: contract, profile: profile, identity: identity}
			err := sharedconfig.RunEntrypoint(context.Background(), releasecontract.EntrypointID(root), spies.options("monolith"), func(_ context.Context, inputs sharedconfig.ResolvedEntrypointInputs) error {
				spies.continuations++
				spies.databaseOpens++
				spies.listeners++
				spies.providerCalls++
				if !reflect.DeepEqual(inputs.Contract(), contract) || !reflect.DeepEqual(inputs.Profile(), profile) || inputs.Identity() != identity {
					t.Fatalf("continuation received a different preflight handoff")
				}
				contractCopy := inputs.Contract()
				contractCopy.Profiles[0].Entrypoints[0] = "mutated"
				profileCopy := inputs.Profile()
				profileCopy.Entrypoints[0] = "mutated"
				if inputs.Contract().Profiles[0].Entrypoints[0] == "mutated" || inputs.Profile().Entrypoints[0] == "mutated" {
					t.Fatal("entrypoint handoff accessors retained mutable aliases")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			spies.assertCounts(t, 1, 1)
		})
	}

	for _, test := range []struct {
		name      string
		profileID string
		mutate    func(*entrypointPreflightSpies)
	}{
		{name: "omitted profile", profileID: ""},
		{name: "unknown profile", profileID: "unknown", mutate: func(s *entrypointPreflightSpies) { s.profileErr = errors.New("profile_unknown") }},
		{name: "conditional profile", profileID: "monolith", mutate: func(s *entrypointPreflightSpies) {
			s.profile.Commitment = releasecontract.CommitmentConditional
			s.contract.Profiles[0] = s.profile
		}},
		{name: "excluded profile", profileID: "monolith", mutate: func(s *entrypointPreflightSpies) {
			s.profile.Commitment = releasecontract.CommitmentExcluded
			s.contract.Profiles[0] = s.profile
		}},
		{name: "entrypoint missing", profileID: "monolith", mutate: func(s *entrypointPreflightSpies) {
			s.profile.Entrypoints = []string{"server"}
			s.contract.Profiles[0] = s.profile
		}},
		{name: "identity mismatched", profileID: "monolith", mutate: func(s *entrypointPreflightSpies) {
			s.identity.ContractDigest = "sha256:" + strings.Repeat("d", 64)
		}},
	} {
		for _, root := range []string{"server", "agent", "billing", "channel", "chat"} {
			t.Run(root+" "+test.name+" stops before mutable dependencies", func(t *testing.T) {
				spies := &entrypointPreflightSpies{contract: contract, profile: profile, identity: identity}
				if test.mutate != nil {
					test.mutate(spies)
				}
				err := sharedconfig.RunEntrypoint(context.Background(), releasecontract.EntrypointID(root), spies.options(test.profileID), func(context.Context, sharedconfig.ResolvedEntrypointInputs) error {
					spies.continuations++
					spies.databaseOpens++
					spies.listeners++
					spies.providerCalls++
					return nil
				})
				if err == nil {
					t.Fatal("preflight unexpectedly authorized startup")
				}
				if spies.continuations != 0 || spies.databaseOpens != 0 || spies.listeners != 0 || spies.providerCalls != 0 {
					t.Fatalf("startup effects after denial: continuation=%d db=%d listener=%d provider=%d", spies.continuations, spies.databaseOpens, spies.listeners, spies.providerCalls)
				}
			})
		}
	}

	for root, source := range map[string]string{
		"server": "main.go", "agent": "../agent/main.go", "billing": "../billing/main.go",
		"channel": "../channel/main.go", "chat": "../chat/main.go",
	} {
		t.Run(root+" source-authored first call", func(t *testing.T) {
			assertEntrypointFirstCall(t, source, root)
		})
	}
}

type entrypointPreflightSpies struct {
	contract releasecontract.AuthoredContractV1
	profile  releasecontract.DeploymentProfile
	identity buildinfo.BuildIdentityV1

	profileErr                                             error
	contractLoads, profileResolves, identityResolves       int
	continuations, databaseOpens, listeners, providerCalls int
}

func (s *entrypointPreflightSpies) options(profileID string) sharedconfig.EntrypointPreflightOptions {
	return sharedconfig.EntrypointPreflightOptions{
		RepoRoot: "/app", ContractPath: "contract.json", SchemaPath: "schema.json", ProfileID: profileID,
		Contracts: entrypointContractLoader{s}, Profiles: entrypointProfileResolver{s}, Identities: entrypointIdentityProvider{s},
	}
}

func (s *entrypointPreflightSpies) assertCounts(t *testing.T, resolverCalls, continuationCalls int) {
	t.Helper()
	if s.contractLoads != resolverCalls || s.profileResolves != resolverCalls || s.identityResolves != resolverCalls || s.continuations != continuationCalls {
		t.Fatalf("calls contract/profile/identity/continuation = %d/%d/%d/%d, want %d/%d/%d/%d", s.contractLoads, s.profileResolves, s.identityResolves, s.continuations, resolverCalls, resolverCalls, resolverCalls, continuationCalls)
	}
}

type entrypointContractLoader struct{ spies *entrypointPreflightSpies }

func (l entrypointContractLoader) Load(context.Context, string, string, string) (releasecontract.AuthoredContractV1, error) {
	l.spies.contractLoads++
	return l.spies.contract, nil
}

type entrypointProfileResolver struct{ spies *entrypointPreflightSpies }

func (r entrypointProfileResolver) ResolveCommittedProfile(context.Context, string, string, string, string) (releasecontract.DeploymentProfile, error) {
	r.spies.profileResolves++
	return r.spies.profile, r.spies.profileErr
}

type entrypointIdentityProvider struct{ spies *entrypointPreflightSpies }

func (p entrypointIdentityProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	p.spies.identityResolves++
	return p.spies.identity, nil
}

func assertEntrypointFirstCall(t *testing.T, source, entrypoint string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), source, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var mainDecl *ast.FuncDecl
	for _, declaration := range file.Decls {
		if candidate, ok := declaration.(*ast.FuncDecl); ok && candidate.Name.Name == "main" {
			mainDecl = candidate
			break
		}
	}
	if mainDecl == nil || len(mainDecl.Body.List) == 0 {
		t.Fatal("main function has no first statement")
	}
	statement, ok := mainDecl.Body.List[0].(*ast.IfStmt)
	if !ok || statement.Init == nil {
		t.Fatalf("first statement is %T, want preflight if statement", mainDecl.Body.List[0])
	}
	assignment, ok := statement.Init.(*ast.AssignStmt)
	if !ok || len(assignment.Rhs) != 1 {
		t.Fatal("first statement does not assign RunEntrypoint result")
	}
	call, ok := assignment.Rhs[0].(*ast.CallExpr)
	if !ok || len(call.Args) < 2 {
		t.Fatal("first statement is not a RunEntrypoint call")
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "RunEntrypoint" {
		t.Fatal("first fallible call is not RunEntrypoint")
	}
	idCall, ok := call.Args[1].(*ast.CallExpr)
	if !ok || len(idCall.Args) != 1 {
		t.Fatal("entrypoint ID is not an explicit source conversion")
	}
	literal, ok := idCall.Args[0].(*ast.BasicLit)
	if !ok || literal.Value != `"`+entrypoint+`"` {
		t.Fatalf("entrypoint literal = %v, want %q", call.Args[1], entrypoint)
	}
}

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
