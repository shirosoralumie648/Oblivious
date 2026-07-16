package surfacereport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
)

const BuildIdentitySurfaceID = "build-identity"

var activeBinaryNames = []string{"grpc-smoke", "migrate", "server"}

type BinaryInspection struct {
	Name     string                    `json:"name"`
	Path     string                    `json:"path"`
	Digest   string                    `json:"digest"`
	Identity buildinfo.BuildIdentityV1 `json:"identity"`
	Matches  bool                      `json:"matches"`
}

type OCIInspection struct {
	Image    string                    `json:"image"`
	Digest   string                    `json:"digest"`
	Identity buildinfo.BuildIdentityV1 `json:"identity"`
	Matches  bool                      `json:"matches"`
}

type PackagedContractInspection struct {
	Path     string                    `json:"path"`
	Digest   string                    `json:"digest"`
	Identity buildinfo.BuildIdentityV1 `json:"identity"`
	Matches  bool                      `json:"matches"`
}

type BuildIdentityDetails struct {
	Binaries         []BinaryInspection         `json:"binaries"`
	OCI              OCIInspection              `json:"oci"`
	PackagedContract PackagedContractInspection `json:"packagedContract"`
	ResidualRisks    []string                   `json:"residualRisks"`
}

func registerFoundationDetails(registry *DetailsRegistry) {
	if err := RegisterDetails(registry, BuildIdentitySurfaceID, validateBuildIdentityDetails); err != nil {
		panic("register foundation build identity details: " + err.Error())
	}
}

func NewBuildIdentityReport(
	ctx context.Context,
	identities buildinfo.IdentityProvider,
	profiles releasecontract.ProfileResolver,
	repoRoot, contractPath, schemaPath, profileID string,
	details BuildIdentityDetails,
	outcome Outcome,
) (SurfaceReportV1, error) {
	if ctx == nil || identities == nil || profiles == nil || strings.TrimSpace(profileID) == "" {
		return SurfaceReportV1{}, reportError("releaseIdentity", nil)
	}
	identity, err := identities.Resolve(ctx, repoRoot, contractPath, schemaPath)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	if err := buildinfo.ValidateIdentity(identity); err != nil {
		return SurfaceReportV1{}, err
	}
	profile, err := profiles.ResolveCommittedProfile(ctx, repoRoot, contractPath, schemaPath, profileID)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	if profile.Commitment != releasecontract.CommitmentCommitted || profile.ID == "" || profile.ID != profileID {
		return SurfaceReportV1{}, reportError("releaseIdentity.deploymentProfile", nil)
	}
	if err := validateBuildIdentityDetailsAgainst(details, identity); err != nil {
		return SurfaceReportV1{}, err
	}
	registry := NewDetailsRegistry()
	rawDetails, err := registry.MarshalDetails(BuildIdentitySurfaceID, details)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	consumerDigest, err := detailsDigest(rawDetails)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	report := NewReport(
		ReleaseIdentity{
			ReleaseCommit: identity.ReleaseCommit, SourceTree: identity.SourceTree,
			ContractDigest: identity.ContractDigest, DeploymentProfile: profile.ID,
			Dirty: identity.Dirty, EvidenceClass: identity.EvidenceClass,
		},
		SurfaceIdentity{
			Surface: BuildIdentitySurfaceID, CanonicalSource: "config/release/contract.v1.json",
			Consumer: "binary-oci-packaged-contract-inspector", Version: "v1",
			SourceDigest: identity.ContractDigest, ConsumerDigest: consumerDigest,
		},
		Drift{Missing: []string{}, Extra: []string{}, Incompatible: []string{}},
		Evidence{
			Class: identity.EvidenceClass, Environment: "repository", Mode: "inspection",
			CheckedAt: time.Now().UTC().Format(time.RFC3339Nano), ToolVersions: map[string]string{}, Details: rawDetails,
		},
		outcome,
	)
	if err := Validate(report, registry); err != nil {
		return SurfaceReportV1{}, err
	}
	return report, nil
}

func validateBuildIdentityDetails(details BuildIdentityDetails) error {
	if len(details.Binaries) != len(activeBinaryNames) {
		return fmt.Errorf("expected three active binaries")
	}
	for index, binary := range details.Binaries {
		if binary.Name != activeBinaryNames[index] || strings.TrimSpace(binary.Path) == "" || !validDigest(binary.Digest) || !binary.Matches {
			return fmt.Errorf("invalid binary inspection")
		}
		if err := buildinfo.ValidateIdentity(binary.Identity); err != nil {
			return err
		}
	}
	if strings.TrimSpace(details.OCI.Image) == "" || !validDigest(details.OCI.Digest) || !details.OCI.Matches {
		return fmt.Errorf("invalid OCI inspection")
	}
	if err := buildinfo.ValidateIdentity(details.OCI.Identity); err != nil {
		return err
	}
	if strings.TrimSpace(details.PackagedContract.Path) == "" || !validDigest(details.PackagedContract.Digest) || !details.PackagedContract.Matches {
		return fmt.Errorf("invalid packaged contract inspection")
	}
	if err := buildinfo.ValidateIdentity(details.PackagedContract.Identity); err != nil {
		return err
	}
	if len(details.ResidualRisks) == 0 || !sortedUniqueNonEmpty(details.ResidualRisks) {
		return fmt.Errorf("residual risks must be explicit and sorted")
	}
	for _, risk := range details.ResidualRisks {
		normalized := strings.ToLower(risk)
		for _, prohibited := range []string{"readiness", "parity", "e3", "e4", "rels-01"} {
			if strings.Contains(normalized, prohibited) {
				return fmt.Errorf("prohibited claim in residual risk")
			}
		}
	}
	return nil
}

func validateBuildIdentityDetailsAgainst(details BuildIdentityDetails, expected buildinfo.BuildIdentityV1) error {
	if err := validateBuildIdentityDetails(details); err != nil {
		return reportError("evidence.details", err)
	}
	for _, binary := range details.Binaries {
		if binary.Identity != expected {
			return reportError("evidence.details.binaries", nil)
		}
	}
	if details.OCI.Identity != expected || details.PackagedContract.Identity != expected || details.PackagedContract.Digest != expected.ContractDigest {
		return reportError("evidence.details.identity", nil)
	}
	return nil
}

func detailsDigest(details json.RawMessage) (string, error) {
	var value any
	if err := json.Unmarshal(details, &value); err != nil {
		return "", reportError("evidence.details", err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", reportError("evidence.details", err)
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func validDigest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && value == strings.ToLower(value)
}

func sortedBinaryNames(details BuildIdentityDetails) []string {
	names := make([]string, 0, len(details.Binaries))
	for _, binary := range details.Binaries {
		names = append(names, binary.Name)
	}
	sort.Strings(names)
	return names
}
