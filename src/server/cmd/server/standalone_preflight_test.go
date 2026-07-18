package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
	sharedconfig "oblivious/server/pkg/config"
)

func TestStandaloneProfilePreflightGroupTwoContract(t *testing.T) {
	assertStandalonePreflightContract(t,
		[]string{"admin", "gateway", "marketplace", "observability"},
		map[string]string{
			"admin":         "../admin/main.go",
			"gateway":       "../gateway/main.go",
			"marketplace":   "../marketplace/main.go",
			"observability": "../observability/main.go",
		},
	)
}

func TestStandaloneProfilePreflightEffectfulRootsContract(t *testing.T) {
	assertStandalonePreflightContract(t,
		[]string{"rag", "relay", "task", "workflow"},
		map[string]string{
			"rag":      "../rag/main.go",
			"relay":    "../relay/main.go",
			"task":     "../task/main.go",
			"workflow": "../workflow/main.go",
		},
	)
}

func assertStandalonePreflightContract(t *testing.T, roots []string, sources map[string]string) {
	t.Helper()
	for _, root := range roots {
		t.Run(root+" authorized immutable handoff", func(t *testing.T) {
			profile := standaloneProfile(root)
			contract := standaloneContract(profile)
			identity := standaloneIdentity(contract)
			spies := &standalonePreflightSpies{contract: contract, profile: profile, identity: identity}
			var captured sharedconfig.ResolvedEntrypointInputs
			err := sharedconfig.RunEntrypoint(context.Background(), releasecontract.EntrypointID(root), spies.options("monolith"), func(_ context.Context, inputs sharedconfig.ResolvedEntrypointInputs) error {
				spies.continuations++
				spies.configLoads++
				spies.databaseOpens++
				spies.listeners++
				spies.providerCalls++
				spies.workerStarts++
				spies.descriptorRegistrations++
				captured = inputs
				if !reflect.DeepEqual(inputs.Contract(), contract) || !reflect.DeepEqual(inputs.Profile(), profile) || inputs.Identity() != identity {
					t.Fatal("continuation received a different preflight handoff")
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
			if captured.Contract().Profiles[0].Entrypoints[0] != root || captured.Profile().Entrypoints[0] != root {
				t.Fatalf("captured handoff lost exact entrypoint: contract=%v profile=%v", captured.Contract().Profiles[0].Entrypoints, captured.Profile().Entrypoints)
			}
			spies.assertCounts(t, 1, 1)
			if spies.configLoads != 1 || spies.databaseOpens != 1 || spies.listeners != 1 || spies.providerCalls != 1 || spies.workerStarts != 1 || spies.descriptorRegistrations != 1 {
				t.Fatalf("authorized continuation effects = config=%d db=%d listener=%d provider=%d worker=%d descriptors=%d", spies.configLoads, spies.databaseOpens, spies.listeners, spies.providerCalls, spies.workerStarts, spies.descriptorRegistrations)
			}
		})

		t.Run(root+" committed monolith excludes standalone effects", func(t *testing.T) {
			profile := standaloneProfile("server")
			contract := standaloneContract(profile)
			identity := standaloneIdentity(contract)
			spies := &standalonePreflightSpies{contract: contract, profile: profile, identity: identity}
			err := sharedconfig.RunEntrypoint(context.Background(), releasecontract.EntrypointID(root), spies.options("monolith"), func(context.Context, sharedconfig.ResolvedEntrypointInputs) error {
				spies.continuations++
				spies.configLoads++
				spies.databaseOpens++
				spies.listeners++
				spies.providerCalls++
				spies.workerStarts++
				spies.descriptorRegistrations++
				return nil
			})
			if err == nil {
				t.Fatal("preflight unexpectedly authorized an excluded standalone root")
			}
			spies.assertCounts(t, 1, 0)
			if spies.configLoads != 0 || spies.databaseOpens != 0 || spies.listeners != 0 || spies.providerCalls != 0 || spies.workerStarts != 0 || spies.descriptorRegistrations != 0 {
				t.Fatalf("excluded root effects = config=%d db=%d listener=%d provider=%d worker=%d descriptors=%d", spies.configLoads, spies.databaseOpens, spies.listeners, spies.providerCalls, spies.workerStarts, spies.descriptorRegistrations)
			}
		})

		for _, test := range []struct {
			name   string
			mutate func(*standalonePreflightSpies)
		}{
			{name: "identity mismatch", mutate: func(s *standalonePreflightSpies) {
				s.identity.ContractDigest = "sha256:" + strings.Repeat("d", 64)
			}},
			{name: "profile resolver failure", mutate: func(s *standalonePreflightSpies) {
				s.profileErr = errors.New("profile_unknown")
			}},
		} {
			t.Run(root+" "+test.name+" stops before continuation", func(t *testing.T) {
				profile := standaloneProfile("server")
				contract := standaloneContract(profile)
				identity := standaloneIdentity(contract)
				spies := &standalonePreflightSpies{contract: contract, profile: profile, identity: identity}
				test.mutate(spies)
				err := sharedconfig.RunEntrypoint(context.Background(), releasecontract.EntrypointID(root), spies.options("monolith"), func(context.Context, sharedconfig.ResolvedEntrypointInputs) error {
					spies.continuations++
					spies.configLoads++
					spies.databaseOpens++
					spies.listeners++
					spies.providerCalls++
					spies.workerStarts++
					spies.descriptorRegistrations++
					return nil
				})
				if err == nil {
					t.Fatal("preflight unexpectedly succeeded")
				}
				if spies.continuations != 0 || spies.configLoads != 0 || spies.databaseOpens != 0 || spies.listeners != 0 || spies.providerCalls != 0 || spies.workerStarts != 0 || spies.descriptorRegistrations != 0 {
					t.Fatalf("failed preflight effects = %#v", spies)
				}
			})
		}

		t.Run(root+" source-authored first call", func(t *testing.T) {
			assertEntrypointFirstCall(t, sources[root], root)
		})
	}
}

type standalonePreflightSpies struct {
	contract releasecontract.AuthoredContractV1
	profile  releasecontract.DeploymentProfile
	identity buildinfo.BuildIdentityV1

	profileErr                                           error
	contractLoads, profileResolves, identityResolves     int
	continuations                                        int
	configLoads, databaseOpens, listeners                int
	providerCalls, workerStarts, descriptorRegistrations int
}

func (s *standalonePreflightSpies) options(profileID string) sharedconfig.EntrypointPreflightOptions {
	return sharedconfig.EntrypointPreflightOptions{
		RepoRoot: "/app", ContractPath: "contract.json", SchemaPath: "schema.json", ProfileID: profileID,
		Contracts: standaloneContractLoader{s}, Profiles: standaloneProfileResolver{s}, Identities: standaloneIdentityProvider{s},
	}
}

func (s *standalonePreflightSpies) assertCounts(t *testing.T, resolverCalls, continuationCalls int) {
	t.Helper()
	if s.contractLoads != resolverCalls || s.profileResolves != resolverCalls || s.identityResolves != resolverCalls || s.continuations != continuationCalls {
		t.Fatalf("calls contract/profile/identity/continuation = %d/%d/%d/%d, want %d/%d/%d/%d", s.contractLoads, s.profileResolves, s.identityResolves, s.continuations, resolverCalls, resolverCalls, resolverCalls, continuationCalls)
	}
}

type standaloneContractLoader struct{ spies *standalonePreflightSpies }

func (l standaloneContractLoader) Load(context.Context, string, string, string) (releasecontract.AuthoredContractV1, error) {
	l.spies.contractLoads++
	return l.spies.contract, nil
}

type standaloneProfileResolver struct{ spies *standalonePreflightSpies }

func (r standaloneProfileResolver) ResolveCommittedProfile(context.Context, string, string, string, string) (releasecontract.DeploymentProfile, error) {
	r.spies.profileResolves++
	return r.spies.profile, r.spies.profileErr
}

type standaloneIdentityProvider struct{ spies *standalonePreflightSpies }

func (p standaloneIdentityProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	p.spies.identityResolves++
	return p.spies.identity, nil
}

func standaloneProfile(entrypoint string) releasecontract.DeploymentProfile {
	return releasecontract.DeploymentProfile{
		ID: "monolith", Commitment: releasecontract.CommitmentCommitted,
		RefreshIntervalSeconds: 30, MaxAgeSeconds: 120, AllowedFutureSkewSeconds: 30,
		Topology:    releasecontract.Topology{Kind: releasecontract.TopologyMonolith, Components: []string{"server"}},
		Entrypoints: []string{entrypoint},
	}
}

func standaloneContract(profile releasecontract.DeploymentProfile) releasecontract.AuthoredContractV1 {
	return releasecontract.AuthoredContractV1{SchemaVersion: releasecontract.SchemaVersionV1, Profiles: []releasecontract.DeploymentProfile{profile}}
}

func standaloneIdentity(contract releasecontract.AuthoredContractV1) buildinfo.BuildIdentityV1 {
	identity := testIdentity()
	digest, err := releasecontract.Digest(contract)
	if err != nil {
		panic(err)
	}
	identity.ContractDigest = digest
	return identity
}
