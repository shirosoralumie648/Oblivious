package surfacereport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type ReportWriter interface {
	Write(context.Context, string, SurfaceReportV1) error
}

type AtomicWriter struct {
	registry  *DetailsRegistry
	failpoint *writerFailpoint
}

type writerFailpoint struct {
	name string
	used bool
}

func NewAtomicWriter(registries ...*DetailsRegistry) *AtomicWriter {
	registry := NewDetailsRegistry()
	if len(registries) == 1 && registries[0] != nil {
		registry = registries[0]
	}
	return &AtomicWriter{registry: registry}
}

func (w *AtomicWriter) Write(ctx context.Context, destination string, report SurfaceReportV1) error {
	if ctx == nil || destination == "" || w == nil || w.registry == nil {
		return &ReportError{Code: ErrorReportOutputUnwritable, Field: "destination"}
	}
	if err := ctx.Err(); err != nil {
		return &ReportError{Code: ErrorReportWriteFailed, Field: "context", Err: err}
	}
	content, err := Marshal(report, w.registry)
	if err != nil {
		return err
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return &ReportError{Code: ErrorReportOutputUnwritable, Field: "parent", Err: err}
	}
	prior, priorExists, err := readPriorDestination(destination)
	if err != nil {
		return &ReportError{Code: ErrorReportOutputUnwritable, Field: "destination", Err: err}
	}

	staging, err := os.CreateTemp(parent, ".surface-report-staging-*")
	if err != nil {
		return &ReportError{Code: ErrorReportOutputUnwritable, Field: "staging", Err: err}
	}
	stagingPath := staging.Name()
	rollbackPath := ""
	cleanup := func() {
		_ = staging.Close()
		_ = os.Remove(stagingPath)
		if rollbackPath != "" {
			_ = os.Remove(rollbackPath)
		}
	}

	err = w.inject("write")
	if err == nil {
		_, err = staging.Write(content)
	}
	if err != nil {
		cleanup()
		return writeFailure("write", err)
	}
	err = w.inject("file-sync")
	if err == nil {
		err = staging.Sync()
	}
	if err != nil {
		cleanup()
		return writeFailure("file-sync", err)
	}
	closeErr := staging.Close()
	if injected := w.inject("close"); injected != nil {
		closeErr = injected
	}
	if closeErr != nil {
		cleanup()
		return writeFailure("close", closeErr)
	}

	if priorExists {
		rollbackPath, err = writeRollbackSnapshot(parent, prior)
		if err != nil {
			cleanup()
			return writeFailure("rollback", err)
		}
		if err := syncDirectory(parent); err != nil {
			cleanup()
			return writeFailure("rollback-parent-sync", err)
		}
	}
	if err := ctx.Err(); err != nil {
		cleanup()
		return writeFailure("context", err)
	}
	err = w.inject("rename")
	if err == nil {
		err = os.Rename(stagingPath, destination)
	}
	if err != nil {
		cleanup()
		return writeFailure("rename", err)
	}
	stagingPath = ""

	err = w.inject("parent-sync")
	if err == nil {
		err = syncDirectory(parent)
	}
	if err != nil {
		original := writeFailure("parent-sync", err)
		if restoreErr := restorePriorDestination(parent, destination, rollbackPath, prior, priorExists); restoreErr != nil {
			return &ReportError{Code: ErrorReportWriteFailed, Field: "rollback-verification", Err: errors.Join(original, restoreErr)}
		}
		rollbackPath = ""
		cleanup()
		return original
	}

	if rollbackPath != "" {
		if err := os.Remove(rollbackPath); err != nil {
			original := writeFailure("rollback-cleanup", err)
			if restoreErr := restorePriorDestination(parent, destination, rollbackPath, prior, true); restoreErr != nil {
				return &ReportError{Code: ErrorReportWriteFailed, Field: "rollback-verification", Err: errors.Join(original, restoreErr)}
			}
			return original
		}
		rollbackPath = ""
		// The replacement was already durably synchronized. A second sync only
		// persists rollback-snapshot cleanup and cannot invalidate the commit.
		_ = syncDirectory(parent)
	}
	return nil
}

func (w *AtomicWriter) inject(name string) error {
	if w.failpoint != nil && !w.failpoint.used && w.failpoint.name == name {
		w.failpoint.used = true
		return errors.New("injected " + name + " failure")
	}
	return nil
}

func readPriorDestination(path string) ([]byte, bool, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return content, true, nil
}

func writeRollbackSnapshot(parent string, content []byte) (string, error) {
	file, err := os.CreateTemp(parent, ".surface-report-rollback-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(content); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	ok = true
	return path, nil
}

func restorePriorDestination(parent, destination, rollbackPath string, prior []byte, priorExists bool) error {
	if priorExists {
		if rollbackPath == "" {
			return errors.New("rollback snapshot missing")
		}
		if err := os.Rename(rollbackPath, destination); err != nil {
			return err
		}
	} else if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := syncDirectory(parent); err != nil {
		return err
	}
	restored, exists, err := readPriorDestination(destination)
	if err != nil {
		return err
	}
	if exists != priorExists || (priorExists && !bytes.Equal(restored, prior)) {
		return errors.New("restored destination differs from prior state")
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func writeFailure(field string, err error) error {
	return &ReportError{Code: ErrorReportWriteFailed, Field: field, Err: err}
}

type preservedProducerError struct {
	primary   error
	secondary ErrorCode
}

func (e *preservedProducerError) Error() string {
	return fmt.Sprintf("%s: secondary=%s", e.primary.Error(), e.secondary)
}

func (e *preservedProducerError) Unwrap() error { return e.primary }

func PreserveProducerError(producerErr, writerErr error) error {
	if producerErr == nil {
		return writerErr
	}
	if writerErr == nil {
		return producerErr
	}
	secondary := ErrorReportWriteFailed
	var reportErr *ReportError
	if errors.As(writerErr, &reportErr) {
		secondary = reportErr.Code
	}
	return &preservedProducerError{primary: producerErr, secondary: secondary}
}
