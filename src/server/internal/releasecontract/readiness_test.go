package releasecontract

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestReadinessEvaluatorContract(t *testing.T) {
	contract, profile, identity := loadReadinessTestAuthority(t)
	evaluator := NewEvaluator()
	now := time.Date(2026, time.July, 17, 12, 0, 0, 123, time.FixedZone("caller", 8*60*60))
	observations := readinessTestObservations(t, contract, profile, now)

	t.Run("trusted authority produces complete immutable evaluation", func(t *testing.T) {
		evaluation, err := evaluator.Evaluate(contract, identity, profile, 1, observations, now)
		if err != nil {
			t.Fatalf("evaluate trusted authority: %v", err)
		}
		if evaluation.Generation != 1 || evaluation.Profile != "monolith" {
			t.Fatalf("unexpected evaluation identity: generation=%d profile=%q", evaluation.Generation, evaluation.Profile)
		}
		if evaluation.CheckedAt.Location() != time.UTC || evaluation.ValidUntil.Location() != time.UTC {
			t.Fatalf("evaluation times are not UTC: checkedAt=%v validUntil=%v", evaluation.CheckedAt, evaluation.ValidUntil)
		}
		if got := evaluation.Capabilities["identity.account_session"].Availability; got != AvailabilityEnabled {
			t.Fatalf("committed capability availability = %q, want enabled", got)
		}
		if got := evaluation.Capabilities["sandbox.code_execution"].Availability; got != AvailabilityDisabled {
			t.Fatalf("excluded capability availability = %q, want disabled", got)
		}
		if _, ok := evaluation.Capabilities["caller.supplied"]; ok {
			t.Fatal("evaluation exposed an unknown caller capability")
		}

		evaluation.Capabilities["identity.account_session"] = CapabilityEvaluation{Availability: AvailabilityBlocked}
		again, err := evaluator.Evaluate(contract, identity, profile, 1, observations, now)
		if err != nil {
			t.Fatalf("re-evaluate after caller mutation: %v", err)
		}
		if got := again.Capabilities["identity.account_session"].Availability; got != AvailabilityEnabled {
			t.Fatalf("caller mutation changed evaluator output: %q", got)
		}
	})

	t.Run("identity profile and generation are mandatory", func(t *testing.T) {
		tests := []struct {
			name string
			id   BuildIdentityV1
			prof DeploymentProfile
			gen  uint64
			code ReadinessCode
		}{
			{name: "missing identity", id: BuildIdentityV1{}, prof: profile, gen: 1, code: CodeBuildIdentityMissing},
			{name: "identity digest mismatch", id: withContractDigest(identity, "sha256:"+repeatHex("0", 64)), prof: profile, gen: 1, code: CodeBuildIdentityMismatch},
			{name: "missing profile", id: identity, prof: DeploymentProfile{}, gen: 1, code: CodeProfileRequired},
			{name: "excluded profile", id: identity, prof: withProfileCommitment(profile, CommitmentExcluded), gen: 1, code: CodeProfileExcluded},
			{name: "profile splice", id: identity, prof: withProfileEntrypoints(profile, []string{"server", "unknown"}), gen: 1, code: CodeBuildIdentityMismatch},
			{name: "zero generation", id: identity, prof: profile, gen: 0, code: CodeReadinessUnavailable},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := evaluator.Evaluate(contract, tt.id, tt.prof, tt.gen, observations, now)
				assertReadinessCode(t, err, tt.code)
			})
		}
	})

	t.Run("availability and reason codes fail closed", func(t *testing.T) {
		for _, availability := range []Availability{AvailabilityBlocked, AvailabilityDisabled} {
			mutated := cloneObservations(observations)
			for i := range mutated {
				if mutated[i].DependencyID == "postgres" {
					mutated[i].Availability = availability
					mutated[i].ReasonCode = "dependency_unproven"
				}
			}
			evaluation, err := evaluator.Evaluate(contract, identity, profile, 2, mutated, now)
			if err != nil {
				t.Fatalf("evaluate %s observation: %v", availability, err)
			}
			if got := evaluation.Capabilities["identity.account_session"].Availability; got != availability {
				t.Fatalf("capability availability = %q, want %q", got, availability)
			}
		}

		unknownReason := cloneObservations(observations)
		unknownReason[0].Availability = AvailabilityBlocked
		unknownReason[0].ReasonCode = "caller_reason"
		_, err := evaluator.Evaluate(contract, identity, profile, 2, unknownReason, now)
		assertReadinessCode(t, err, CodeReadinessUnavailable)

		unknownCapability := cloneObservations(observations)
		unknownCapability[0].CapabilityIDs = append(unknownCapability[0].CapabilityIDs, "caller.supplied")
		_, err = evaluator.Evaluate(contract, identity, profile, 2, unknownCapability, now)
		assertReadinessCode(t, err, CodeReadinessUnavailable)
	})

	t.Run("nanosecond freshness and future skew boundaries are exact", func(t *testing.T) {
		tests := []struct {
			name       string
			observedAt time.Time
			code       ReadinessCode
		}{
			{name: "max age boundary", observedAt: now.Add(-120 * time.Second)},
			{name: "one nanosecond stale", observedAt: now.Add(-120*time.Second - time.Nanosecond), code: CodeReadinessStale},
			{name: "future skew boundary", observedAt: now.Add(30 * time.Second)},
			{name: "one nanosecond beyond future skew", observedAt: now.Add(30*time.Second + time.Nanosecond), code: CodeReadinessUnavailable},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				candidate := cloneObservations(observations)
				for i := range candidate {
					candidate[i].ObservedAt = tt.observedAt
				}
				evaluation, err := evaluator.Evaluate(contract, identity, profile, 3, candidate, now)
				if tt.code == "" {
					if err != nil {
						t.Fatalf("boundary rejected: %v", err)
					}
					if tt.name == "max age boundary" && evaluation.ValidUntil.UnixNano() != now.UTC().UnixNano() {
						t.Fatalf("validUntil = %s, want exact now %s", evaluation.ValidUntil, now.UTC())
					}
					return
				}
				assertReadinessCode(t, err, tt.code)
			})
		}
	})

	t.Run("only authored timing is accepted", func(t *testing.T) {
		if got := reflect.TypeOf((*Evaluator)(nil)).Elem().Method(0).Type.NumIn(); got != 6 {
			t.Fatalf("Evaluator.Evaluate input count = %d, want six authority inputs and no duration override", got)
		}
		for _, mutate := range []func(*DeploymentProfile){
			func(p *DeploymentProfile) { p.RefreshIntervalSeconds = 31 },
			func(p *DeploymentProfile) { p.MaxAgeSeconds = 121 },
			func(p *DeploymentProfile) { p.AllowedFutureSkewSeconds = 29 },
		} {
			mutated := profile
			mutate(&mutated)
			_, err := evaluator.Evaluate(contract, identity, mutated, 1, observations, now)
			assertReadinessCode(t, err, CodeBuildIdentityMismatch)
		}
		t.Setenv("OBLIVIOUS_READINESS_MAX_AGE_SECONDS", "999999")
		stale := cloneObservations(observations)
		for i := range stale {
			stale[i].ObservedAt = now.Add(-120*time.Second - time.Nanosecond)
		}
		_, err := evaluator.Evaluate(contract, identity, profile, 1, stale, now)
		assertReadinessCode(t, err, CodeReadinessStale)
	})

	t.Run("entrypoint requires an exact source-authored identifier", func(t *testing.T) {
		if err := RequireProfileEntrypoint(profile, EntrypointID("server")); err != nil {
			t.Fatalf("require authored server entrypoint: %v", err)
		}
		assertReadinessCode(t, RequireProfileEntrypoint(profile, ""), CodeProfileRequired)
		assertReadinessCode(t, RequireProfileEntrypoint(profile, "unknown"), CodeProfileExcluded)
		assertReadinessCode(t, RequireProfileEntrypoint(withProfileCommitment(profile, CommitmentExcluded), "server"), CodeProfileExcluded)
		assertReadinessCode(t, RequireProfileEntrypoint(withProfileEntrypoints(profile, []string{"migrate"}), "server"), CodeProfileExcluded)
		t.Setenv("OBLIVIOUS_ENTRYPOINT_ID", "server")
		originalArgs := os.Args
		os.Args = []string{"server"}
		t.Cleanup(func() { os.Args = originalArgs })
		assertReadinessCode(t, RequireProfileEntrypoint(profile, "caller"), CodeProfileExcluded)
	})
}

func loadReadinessTestAuthority(t *testing.T) (AuthoredContractV1, DeploymentProfile, BuildIdentityV1) {
	t.Helper()
	repoRoot := testRepoRoot(t)
	contract, err := Load(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json")
	if err != nil {
		t.Fatalf("load checked-in contract: %v", err)
	}
	var profile DeploymentProfile
	for _, candidate := range contract.Profiles {
		if candidate.ID == "monolith" {
			profile = candidate
			break
		}
	}
	digest, err := Digest(contract)
	if err != nil {
		t.Fatalf("digest checked-in contract: %v", err)
	}
	identity := BuildIdentityV1{
		SchemaVersion:  buildIdentitySchemaV1,
		ReleaseCommit:  repeatHex("a", 40),
		SourceTree:     repeatHex("b", 40),
		ContractDigest: digest,
		EvidenceClass:  repositoryLocalEvidence,
	}
	return contract, profile, identity
}

func readinessTestObservations(t *testing.T, contract AuthoredContractV1, profile DeploymentProfile, observedAt time.Time) []Observation {
	t.Helper()
	_, dependencies, err := applicableCapabilityPolicy(contract, profile)
	if err != nil {
		t.Fatalf("derive applicable readiness policy: %v", err)
	}
	observations := make([]Observation, 0, len(dependencies))
	for dependencyID, capabilityIDs := range dependencies {
		observations = append(observations, Observation{
			ProbeID:       "probe." + dependencyID,
			DependencyID:  dependencyID,
			CapabilityIDs: append([]string(nil), capabilityIDs...),
			Availability:  AvailabilityEnabled,
			ObservedAt:    observedAt,
		})
	}
	return observations
}

func cloneObservations(source []Observation) []Observation {
	result := make([]Observation, len(source))
	for i := range source {
		result[i] = cloneObservation(source[i])
	}
	return result
}

func assertReadinessCode(t *testing.T, err error, code ReadinessCode) {
	t.Helper()
	if !IsReadinessCode(err, code) {
		t.Fatalf("error = %T %v, want readiness code %q", err, err, code)
	}
}

func withContractDigest(identity BuildIdentityV1, digest string) BuildIdentityV1 {
	identity.ContractDigest = digest
	return identity
}

func withProfileCommitment(profile DeploymentProfile, commitment Commitment) DeploymentProfile {
	profile.Commitment = commitment
	return profile
}

func withProfileEntrypoints(profile DeploymentProfile, entrypoints []string) DeploymentProfile {
	profile.Entrypoints = append([]string(nil), entrypoints...)
	return profile
}

func repeatHex(value string, count int) string {
	result := ""
	for len(result) < count {
		result += value
	}
	return result[:count]
}
