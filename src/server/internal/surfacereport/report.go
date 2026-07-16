package surfacereport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"oblivious/server/internal/buildinfo"
)

const SurfaceReportSchemaV1 = "surface-report/v1"

type ErrorCode string

const (
	ErrorSurfaceSchemaInvalid   ErrorCode = "surface_schema_invalid"
	ErrorReportOutputUnwritable ErrorCode = "report_output_unwritable"
	ErrorReportWriteFailed      ErrorCode = "report_write_failed"
)

type ReportError struct {
	Code  ErrorCode
	Field string
	Err   error
}

func (e *ReportError) Error() string {
	if e.Field == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": field=" + e.Field
}

func (e *ReportError) Unwrap() error { return e.Err }

func IsCode(err error, code ErrorCode) bool {
	var reportErr *ReportError
	return errors.As(err, &reportErr) && reportErr.Code == code
}

type Result string

const (
	ResultPass Result = "pass"
	ResultFail Result = "fail"
)

type SurfaceReportV1 struct {
	SchemaVersion   string          `json:"schemaVersion"`
	ReleaseIdentity ReleaseIdentity `json:"releaseIdentity"`
	SurfaceIdentity SurfaceIdentity `json:"surfaceIdentity"`
	Drift           Drift           `json:"drift"`
	Evidence        Evidence        `json:"evidence"`
	Outcome         Outcome         `json:"outcome"`
}

type ReleaseIdentity struct {
	ReleaseCommit     string `json:"releaseCommit"`
	SourceTree        string `json:"sourceTree"`
	ContractDigest    string `json:"contractDigest"`
	DeploymentProfile string `json:"deploymentProfile"`
	Dirty             bool   `json:"dirty"`
	EvidenceClass     string `json:"evidenceClass"`
}

type SurfaceIdentity struct {
	Surface         string `json:"surface"`
	CanonicalSource string `json:"canonicalSource"`
	Consumer        string `json:"consumer"`
	Version         string `json:"version"`
	SourceDigest    string `json:"sourceDigest"`
	ConsumerDigest  string `json:"consumerDigest"`
}

type Drift struct {
	Missing      []string `json:"missing"`
	Extra        []string `json:"extra"`
	Incompatible []string `json:"incompatible"`
}

type Evidence struct {
	Class        string            `json:"class"`
	Environment  string            `json:"environment"`
	Mode         string            `json:"mode"`
	CheckedAt    string            `json:"checkedAt"`
	ToolVersions map[string]string `json:"toolVersions"`
	Details      json.RawMessage   `json:"details"`
}

type Outcome struct {
	Result        Result   `json:"result"`
	ErrorCodes    []string `json:"errorCodes"`
	SkippedChecks []string `json:"skippedChecks"`
}

func NewReport(release ReleaseIdentity, surface SurfaceIdentity, drift Drift, evidence Evidence, outcome Outcome) SurfaceReportV1 {
	return SurfaceReportV1{
		SchemaVersion:   SurfaceReportSchemaV1,
		ReleaseIdentity: release,
		SurfaceIdentity: surface,
		Drift:           drift,
		Evidence:        evidence,
		Outcome:         outcome,
	}
}

func Validate(report SurfaceReportV1, registry *DetailsRegistry) error {
	if report.SchemaVersion != SurfaceReportSchemaV1 {
		return reportError("schemaVersion", nil)
	}
	identity := buildinfo.BuildIdentityV1{
		SchemaVersion:  buildinfo.BuildIdentitySchemaV1,
		ReleaseCommit:  report.ReleaseIdentity.ReleaseCommit,
		SourceTree:     report.ReleaseIdentity.SourceTree,
		ContractDigest: report.ReleaseIdentity.ContractDigest,
		Dirty:          report.ReleaseIdentity.Dirty,
		EvidenceClass:  report.ReleaseIdentity.EvidenceClass,
	}
	if err := buildinfo.ValidateIdentity(identity); err != nil {
		return reportError("releaseIdentity", err)
	}
	if strings.TrimSpace(report.ReleaseIdentity.DeploymentProfile) == "" {
		return reportError("releaseIdentity.deploymentProfile", nil)
	}
	if report.Evidence.Class != report.ReleaseIdentity.EvidenceClass || report.Evidence.Class != buildinfo.EvidenceRepositoryLocal {
		return reportError("evidence.class", nil)
	}
	for field, value := range map[string]string{
		"surfaceIdentity.surface":         report.SurfaceIdentity.Surface,
		"surfaceIdentity.canonicalSource": report.SurfaceIdentity.CanonicalSource,
		"surfaceIdentity.consumer":        report.SurfaceIdentity.Consumer,
		"surfaceIdentity.version":         report.SurfaceIdentity.Version,
		"surfaceIdentity.sourceDigest":    report.SurfaceIdentity.SourceDigest,
		"surfaceIdentity.consumerDigest":  report.SurfaceIdentity.ConsumerDigest,
		"evidence.environment":            report.Evidence.Environment,
		"evidence.mode":                   report.Evidence.Mode,
	} {
		if strings.TrimSpace(value) == "" {
			return reportError(field, nil)
		}
	}
	if !validDigest(report.SurfaceIdentity.SourceDigest) || !validDigest(report.SurfaceIdentity.ConsumerDigest) {
		return reportError("surfaceIdentity.digest", nil)
	}
	if err := validateCheckedAt(report.Evidence.CheckedAt); err != nil {
		return reportError("evidence.checkedAt", err)
	}
	if report.Evidence.ToolVersions == nil || report.Drift.Missing == nil || report.Drift.Extra == nil || report.Drift.Incompatible == nil || report.Outcome.ErrorCodes == nil || report.Outcome.SkippedChecks == nil {
		return reportError("requiredCollections", nil)
	}
	for name, version := range report.Evidence.ToolVersions {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(version) == "" {
			return reportError("evidence.toolVersions", nil)
		}
	}
	for field, values := range map[string][]string{
		"drift.missing":         report.Drift.Missing,
		"drift.extra":           report.Drift.Extra,
		"drift.incompatible":    report.Drift.Incompatible,
		"outcome.errorCodes":    report.Outcome.ErrorCodes,
		"outcome.skippedChecks": report.Outcome.SkippedChecks,
	} {
		if !sortedUniqueNonEmpty(values) {
			return reportError(field, nil)
		}
	}
	switch report.Outcome.Result {
	case ResultPass:
		if len(report.Drift.Missing)+len(report.Drift.Extra)+len(report.Drift.Incompatible)+len(report.Outcome.ErrorCodes)+len(report.Outcome.SkippedChecks) != 0 {
			return reportError("outcome.result", nil)
		}
	case ResultFail:
		if len(report.Drift.Missing)+len(report.Drift.Extra)+len(report.Drift.Incompatible)+len(report.Outcome.ErrorCodes) == 0 {
			return reportError("outcome.result", nil)
		}
	default:
		return reportError("outcome.result", nil)
	}
	if registry == nil {
		return reportError("evidence.details", nil)
	}
	if err := registry.ValidateDetails(report.SurfaceIdentity.Surface, report.Evidence.Details); err != nil {
		return err
	}
	if err := validateDetailsAgainstRelease(report); err != nil {
		return err
	}
	return nil
}

func Decode(content []byte, registry *DetailsRegistry) (SurfaceReportV1, error) {
	var report SurfaceReportV1
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return SurfaceReportV1{}, reportError("report", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return SurfaceReportV1{}, reportError("report", err)
	}
	if err := Validate(report, registry); err != nil {
		return SurfaceReportV1{}, err
	}
	return report, nil
}

func Marshal(report SurfaceReportV1, registry *DetailsRegistry) ([]byte, error) {
	if err := Validate(report, registry); err != nil {
		return nil, err
	}
	return json.Marshal(report)
}

func validateCheckedAt(value string) error {
	checkedAt, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || !strings.HasSuffix(value, "Z") || checkedAt.UTC().Format(time.RFC3339Nano) != value {
		return fmt.Errorf("checkedAt must be canonical RFC3339Nano UTC")
	}
	return nil
}

func sortedUniqueNonEmpty(values []string) bool {
	for index, value := range values {
		if strings.TrimSpace(value) == "" || (index > 0 && values[index-1] >= value) {
			return false
		}
	}
	return sort.StringsAreSorted(values)
}

func reportError(field string, err error) error {
	return &ReportError{Code: ErrorSurfaceSchemaInvalid, Field: field, Err: err}
}
