package surfacereport

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicWriterCreatesParentAndReplaces(t *testing.T) {
	registry := testRegistry(t)
	writer := NewAtomicWriter(registry)
	destination := filepath.Join(t.TempDir(), "nested", "report.json")
	if err := writer.Write(context.Background(), destination, validTestReport(t, registry)); err != nil {
		t.Fatalf("write report: %v", err)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if _, err := Decode(content, registry); err != nil {
		t.Fatalf("decode written report: %v", err)
	}
	assertNoWriterResidue(t, filepath.Dir(destination))
}

func TestAtomicWriterPreservesDestinationOnInjectedFailures(t *testing.T) {
	for _, failpoint := range []string{"write", "file-sync", "close", "rename"} {
		t.Run(failpoint, func(t *testing.T) {
			registry := testRegistry(t)
			parent := t.TempDir()
			destination := filepath.Join(parent, "report.json")
			prior := []byte("prior-valid-report-bytes")
			if err := os.WriteFile(destination, prior, 0o644); err != nil {
				t.Fatalf("write prior destination: %v", err)
			}
			writer := NewAtomicWriter(registry)
			writer.failpoint = &writerFailpoint{name: failpoint}
			if err := writer.Write(context.Background(), destination, validTestReport(t, registry)); !IsCode(err, ErrorReportWriteFailed) {
				t.Fatalf("write error = %v", err)
			}
			assertDestination(t, destination, prior)
			assertNoWriterResidue(t, parent)
		})
	}
}

func TestAtomicWriterRollsBackPostRenameDirectorySyncFailure(t *testing.T) {
	for _, priorExists := range []bool{true, false} {
		t.Run(map[bool]string{true: "existing", false: "absent"}[priorExists], func(t *testing.T) {
			registry := testRegistry(t)
			parent := t.TempDir()
			destination := filepath.Join(parent, "report.json")
			prior := []byte("prior-valid-report-bytes")
			if priorExists {
				if err := os.WriteFile(destination, prior, 0o644); err != nil {
					t.Fatalf("write prior destination: %v", err)
				}
			}
			writer := NewAtomicWriter(registry)
			writer.failpoint = &writerFailpoint{name: "parent-sync"}
			if err := writer.Write(context.Background(), destination, validTestReport(t, registry)); !IsCode(err, ErrorReportWriteFailed) {
				t.Fatalf("write error = %v", err)
			}
			if priorExists {
				assertDestination(t, destination, prior)
			} else if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("initially absent destination exists or stat failed: %v", err)
			}
			assertNoWriterResidue(t, parent)
		})
	}
}

func TestPreserveProducerErrorKeepsPrimaryStatus(t *testing.T) {
	producerErr := errors.New("producer_primary_failure")
	writerErr := &ReportError{Code: ErrorReportOutputUnwritable, Field: "parent", Err: errors.New("sensitive path detail")}
	combined := PreserveProducerError(producerErr, writerErr)
	if !errors.Is(combined, producerErr) {
		t.Fatalf("producer error not preserved: %v", combined)
	}
	if !strings.Contains(combined.Error(), string(ErrorReportOutputUnwritable)) || strings.Contains(combined.Error(), "sensitive path detail") {
		t.Fatalf("secondary context is not sanitized: %v", combined)
	}
}

func assertDestination(t *testing.T, path string, expected []byte) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if !bytes.Equal(content, expected) {
		t.Fatalf("destination = %q, want %q", content, expected)
	}
}

func assertNoWriterResidue(t *testing.T, parent string) {
	t.Helper()
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("read output directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".surface-report-") {
			t.Fatalf("writer residue remains: %s", entry.Name())
		}
	}
}
