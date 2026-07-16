package buildinfo

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
)

const InspectionFlag = "--inspect-build-identity"

func HandleInspection(ctx context.Context, args []string, stdout, stderr io.Writer, provider IdentityProvider, repoRoot, defaultContractPath, defaultSchemaPath string) (bool, int) {
	if len(args) == 0 || args[0] != InspectionFlag {
		return false, 0
	}
	flags := flag.NewFlagSet("build-identity-inspection", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	contractPath := flags.String("contract", defaultContractPath, "packaged release contract")
	schemaPath := flags.String("schema", defaultSchemaPath, "packaged release contract schema")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *contractPath == "" || *schemaPath == "" || provider == nil {
		writeInspectionError(stderr, ErrorBuildIdentityMismatch, "arguments")
		return true, 2
	}
	identity, err := provider.Resolve(ctx, repoRoot, *contractPath, *schemaPath)
	if err != nil {
		code, field := identityErrorDetails(err)
		writeInspectionError(stderr, code, field)
		return true, 1
	}
	encoded, err := MarshalIdentity(identity)
	if err != nil {
		code, field := identityErrorDetails(err)
		writeInspectionError(stderr, code, field)
		return true, 1
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		writeInspectionError(stderr, ErrorBuildIdentityMismatch, "stdout")
		return true, 1
	}
	return true, 0
}

func identityErrorDetails(err error) (ErrorCode, string) {
	var identityErr *IdentityError
	if errors.As(err, &identityErr) {
		return identityErr.Code, identityErr.Field
	}
	return ErrorBuildIdentityMismatch, "identity"
}

func writeInspectionError(output io.Writer, code ErrorCode, field string) {
	_ = json.NewEncoder(output).Encode(struct {
		Error struct {
			Code  ErrorCode `json:"code"`
			Field string    `json:"field,omitempty"`
		} `json:"error"`
	}{Error: struct {
		Code  ErrorCode `json:"code"`
		Field string    `json:"field,omitempty"`
	}{Code: code, Field: field}})
}
