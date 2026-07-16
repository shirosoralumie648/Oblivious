package buildinfo

var (
	linkedReleaseCommit  string
	linkedSourceTree     string
	linkedContractDigest string
	linkedDirty          string
	linkedEvidenceClass  string
)

type LinkerIdentity struct {
	ReleaseCommit  string
	SourceTree     string
	ContractDigest string
	Dirty          string
	EvidenceClass  string
}

func ParseLinkedIdentity() (BuildIdentityV1, error) {
	return parseLinkerIdentity(LinkerIdentity{
		ReleaseCommit:  linkedReleaseCommit,
		SourceTree:     linkedSourceTree,
		ContractDigest: linkedContractDigest,
		Dirty:          linkedDirty,
		EvidenceClass:  linkedEvidenceClass,
	})
}

func parseLinkerIdentity(linked LinkerIdentity) (BuildIdentityV1, error) {
	if linked.Dirty == "" {
		return BuildIdentityV1{}, identityError(ErrorBuildIdentityMissing, "dirty", nil)
	}
	dirty := false
	switch linked.Dirty {
	case "false":
	case "true":
		dirty = true
	default:
		return BuildIdentityV1{}, identityError(ErrorBuildIdentityMismatch, "dirty", nil)
	}
	identity := BuildIdentityV1{
		SchemaVersion:  BuildIdentitySchemaV1,
		ReleaseCommit:  linked.ReleaseCommit,
		SourceTree:     linked.SourceTree,
		ContractDigest: linked.ContractDigest,
		Dirty:          dirty,
		EvidenceClass:  linked.EvidenceClass,
	}
	if err := ValidateIdentity(identity); err != nil {
		return BuildIdentityV1{}, err
	}
	return identity, nil
}
