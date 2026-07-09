// Package sandbox provides a Docker-backed implementation of the workflow
// package's CodeRunner interface: an 8-language code interpreter that executes
// untrusted workflow code inside hard-sandboxed, network-isolated containers.
//
// The runner is DISABLED by default. Construct it with Config{Enabled: true}
// to allow execution; a disabled runner returns ErrSandboxDisabled, matching
// the project's default-disabled commercial policy for tools that reach
// external systems.
package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"oblivious/server/internal/workflow"
)

// ErrSandboxDisabled is returned when code execution is requested but the
// sandbox has not been explicitly enabled by an operator.
var ErrSandboxDisabled = errors.New(
	"code interpreter sandbox is disabled for default commercial use: an operator must enable it explicitly (sandbox.Config{Enabled: true}) and provide a Docker runtime",
)

// languageSpec describes how one supported language runs inside a container.
// Code always arrives on stdin; the script writes it to the tmpfs workdir,
// compiles when needed, and runs it in the same container invocation.
type languageSpec struct {
	image  string
	script string
}

var defaultLanguageSpecs = map[string]languageSpec{
	"python": {
		image:  "python:3.12-alpine",
		script: "cat - > main.py && python3 main.py",
	},
	"javascript": {
		image:  "node:22-alpine",
		script: "cat - > main.js && node main.js",
	},
	"ruby": {
		image:  "ruby:3.3-alpine",
		script: "cat - > main.rb && ruby main.rb",
	},
	"java": {
		image:  "eclipse-temurin:21",
		script: "cat - > Main.java && javac Main.java && java Main",
	},
	"cpp": {
		image:  "gcc:14",
		script: "cat - > main.cpp && g++ -O2 -o main main.cpp && ./main",
	},
	"go": {
		image:  "golang:1.23-alpine",
		script: "cat - > main.go && go run main.go",
	},
	"rust": {
		image:  "rust:1.82-slim",
		script: "cat - > main.rs && rustc -O -o main main.rs && ./main",
	},
	"php": {
		image:  "php:8.3-cli-alpine",
		script: "cat - > main.php && php main.php",
	},
}

var languageAliases = map[string]string{
	"py":      "python",
	"python3": "python",
	"js":      "javascript",
	"node":    "javascript",
	"nodejs":  "javascript",
	"golang":  "go",
	"c++":     "cpp",
	"rb":      "ruby",
}

// SupportedLanguages returns the canonical names of all sandbox languages.
func SupportedLanguages() []string {
	names := make([]string, 0, len(defaultLanguageSpecs))
	for name := range defaultLanguageSpecs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Config controls sandbox enablement, resource ceilings, and security caps.
type Config struct {
	// Enabled must be set explicitly; the zero value keeps the sandbox off.
	Enabled bool
	// Images overrides the container image per canonical language name.
	Images map[string]string
	// AllowedLanguages restricts execution to a subset of supported
	// languages. Empty means every supported language is allowed.
	AllowedLanguages []string
	// MemoryMB is the container memory + swap ceiling (default 256).
	MemoryMB int
	// CPUs is the container CPU quota (default 1).
	CPUs float64
	// PidsLimit caps container processes (default 128).
	PidsLimit int
	// WorkdirMB sizes the writable tmpfs workdir (default 128).
	WorkdirMB int
	// DefaultTimeoutMS bounds executions that do not request a timeout
	// (default 10000).
	DefaultTimeoutMS int
	// MaxTimeoutMS is the hard wall-clock ceiling (default 60000).
	MaxTimeoutMS int
	// MaxCodeBytes caps submitted code size (default 131072).
	MaxCodeBytes int
	// MaxOutputBytes caps captured stdout and stderr, each (default 65536).
	MaxOutputBytes int
	// DockerBinary names the container runtime binary (default "docker").
	DockerBinary string
}

func (c Config) withDefaults() Config {
	if c.MemoryMB <= 0 {
		c.MemoryMB = 256
	}
	if c.CPUs <= 0 {
		c.CPUs = 1
	}
	if c.PidsLimit <= 0 {
		c.PidsLimit = 128
	}
	if c.WorkdirMB <= 0 {
		c.WorkdirMB = 128
	}
	if c.DefaultTimeoutMS <= 0 {
		c.DefaultTimeoutMS = 10_000
	}
	if c.MaxTimeoutMS <= 0 {
		c.MaxTimeoutMS = 60_000
	}
	if c.MaxCodeBytes <= 0 {
		c.MaxCodeBytes = 128 * 1024
	}
	if c.MaxOutputBytes <= 0 {
		c.MaxOutputBytes = 64 * 1024
	}
	if c.DockerBinary == "" {
		c.DockerBinary = "docker"
	}
	return c
}

type commandFactory func(ctx context.Context, name string, args ...string) *exec.Cmd

// DockerSandboxRunner implements workflow.CodeRunner on top of `docker run`.
type DockerSandboxRunner struct {
	config  Config
	allowed map[string]bool
	command commandFactory
}

var _ workflow.CodeRunner = (*DockerSandboxRunner)(nil)

// Option customizes a DockerSandboxRunner.
type Option func(*DockerSandboxRunner)

// WithCommandFactory replaces the process launcher; used by tests to avoid a
// real Docker dependency.
func WithCommandFactory(factory commandFactory) Option {
	return func(r *DockerSandboxRunner) {
		if factory != nil {
			r.command = factory
		}
	}
}

// NewDockerSandboxRunner builds a runner from config; the runner refuses all
// work until the config sets Enabled.
func NewDockerSandboxRunner(config Config, options ...Option) *DockerSandboxRunner {
	runner := &DockerSandboxRunner{
		config:  config.withDefaults(),
		command: exec.CommandContext,
	}
	if len(config.AllowedLanguages) > 0 {
		runner.allowed = make(map[string]bool, len(config.AllowedLanguages))
		for _, language := range config.AllowedLanguages {
			runner.allowed[normalizeLanguage(language)] = true
		}
	}
	for _, option := range options {
		if option != nil {
			option(runner)
		}
	}
	return runner
}

func normalizeLanguage(language string) string {
	normalized := strings.ToLower(strings.TrimSpace(language))
	if canonical, ok := languageAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func (r *DockerSandboxRunner) languageSpec(language string) (string, languageSpec, error) {
	canonical := normalizeLanguage(language)
	spec, ok := defaultLanguageSpecs[canonical]
	if !ok {
		return "", languageSpec{}, fmt.Errorf(
			"unsupported sandbox language %q (supported: %s)",
			language, strings.Join(SupportedLanguages(), ", "),
		)
	}
	if r.allowed != nil && !r.allowed[canonical] {
		return "", languageSpec{}, fmt.Errorf("sandbox language %q is not allowed by policy", canonical)
	}
	if image := r.config.Images[canonical]; image != "" {
		spec.image = image
	}
	return canonical, spec, nil
}

func (r *DockerSandboxRunner) timeout(req workflow.WorkflowCodeRequest) time.Duration {
	timeoutMS := req.TimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = r.config.DefaultTimeoutMS
	}
	if timeoutMS > r.config.MaxTimeoutMS {
		timeoutMS = r.config.MaxTimeoutMS
	}
	return time.Duration(timeoutMS) * time.Millisecond
}

func (r *DockerSandboxRunner) dockerArgs(spec languageSpec, inputsJSON, executionContextJSON string) []string {
	return []string{
		"run", "--rm", "-i",
		"--network=none",
		fmt.Sprintf("--memory=%dm", r.config.MemoryMB),
		fmt.Sprintf("--memory-swap=%dm", r.config.MemoryMB),
		"--cpus=" + strconv.FormatFloat(r.config.CPUs, 'f', -1, 64),
		fmt.Sprintf("--pids-limit=%d", r.config.PidsLimit),
		"--read-only",
		fmt.Sprintf("--tmpfs=/sandbox:rw,exec,size=%dm", r.config.WorkdirMB),
		"--tmpfs=/tmp:rw,size=16m",
		"--user=65534:65534",
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges",
		"--workdir=/sandbox",
		"--env", "HOME=/sandbox",
		"--env", "GOCACHE=/sandbox/.gocache",
		"--env", "OBLIVIOUS_INPUTS=" + inputsJSON,
		"--env", "OBLIVIOUS_EXECUTION_CONTEXT=" + executionContextJSON,
		spec.image,
		"sh", "-c", spec.script,
	}
}

// RunWorkflowCode executes req.Code in a sandboxed container and returns its
// stdout/stderr/exit code. Infrastructure and policy failures return errors;
// non-zero exits from user code are reported in the result, not as errors.
func (r *DockerSandboxRunner) RunWorkflowCode(ctx context.Context, req workflow.WorkflowCodeRequest) (*workflow.WorkflowCodeResult, error) {
	if r == nil || !r.config.Enabled {
		return nil, ErrSandboxDisabled
	}
	if strings.TrimSpace(req.Code) == "" {
		return nil, errors.New("sandbox: code is required")
	}
	if len(req.Code) > r.config.MaxCodeBytes {
		return nil, fmt.Errorf("sandbox: code size %d exceeds limit of %d bytes", len(req.Code), r.config.MaxCodeBytes)
	}

	canonical, spec, err := r.languageSpec(req.Language)
	if err != nil {
		return nil, err
	}

	inputs := req.Inputs
	if inputs == nil {
		inputs = map[string]any{}
	}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return nil, fmt.Errorf("sandbox: cannot encode inputs: %w", err)
	}
	executionContext := sandboxExecutionContext(req)
	executionContextJSON, err := json.Marshal(executionContext)
	if err != nil {
		return nil, fmt.Errorf("sandbox: cannot encode execution context: %w", err)
	}

	timeout := r.timeout(req)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stdout := &limitWriter{limit: r.config.MaxOutputBytes}
	stderr := &limitWriter{limit: r.config.MaxOutputBytes}

	cmd := r.command(runCtx, r.config.DockerBinary, r.dockerArgs(spec, string(inputsJSON), string(executionContextJSON))...)
	cmd.Stdin = strings.NewReader(req.Code)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	started := time.Now()
	runErr := cmd.Run()
	finished := time.Now()
	durationMS := finished.Sub(started).Milliseconds()

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("sandbox: execution timed out after %dms", timeout.Milliseconds())
	}
	if errors.Is(runCtx.Err(), context.Canceled) {
		return nil, fmt.Errorf("sandbox: execution cancelled: %w", runCtx.Err())
	}

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("sandbox: container execution failed: %w", runErr)
		}
	}

	stdoutText := stdout.String()
	stderrText := stderr.String()
	logs := sandboxLogs(stderrText, stdout.truncated, stderr.truncated)
	evidence := sandboxExecutionEvidence(req, executionContext, len(inputsJSON), timeout, started, finished, durationMS, stdoutText, stderrText, logs, stdout.truncated, stderr.truncated, r.config.MaxOutputBytes)

	return &workflow.WorkflowCodeResult{
		Output: map[string]any{
			"stdout":   stdoutText,
			"stderr":   stderrText,
			"exitCode": exitCode,
		},
		Logs: logs,
		Raw: map[string]any{
			"language":         canonical,
			"image":            spec.image,
			"exitCode":         exitCode,
			"durationMs":       durationMS,
			"stdoutTruncated":  stdout.truncated,
			"stderrTruncated":  stderr.truncated,
			"executionContext": executionContext,
			"evidence":         evidence,
		},
	}, nil
}

func sandboxExecutionContext(req workflow.WorkflowCodeRequest) map[string]string {
	context := map[string]string{}
	add := func(key, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			context[key] = value
		}
	}
	add("agentId", req.AgentID)
	add("runId", req.RunID)
	add("toolRunId", req.ToolRunID)
	add("toolCallId", req.ToolCallID)
	add("toolName", req.ToolName)
	add("requestId", req.RequestID)
	return context
}

func sandboxExecutionEvidence(
	req workflow.WorkflowCodeRequest,
	executionContext map[string]string,
	inputBytes int,
	timeout time.Duration,
	started time.Time,
	finished time.Time,
	durationMS int64,
	stdoutText string,
	stderrText string,
	logs []string,
	stdoutTruncated bool,
	stderrTruncated bool,
	maxOutputBytes int,
) map[string]any {
	return map[string]any{
		"executionContext": executionContext,
		"startedAt":        started.UTC().Format(time.RFC3339Nano),
		"finishedAt":       finished.UTC().Format(time.RFC3339Nano),
		"durationMs":       durationMS,
		"timeoutMs":        timeout.Milliseconds(),
		"codeBytes":        len(req.Code),
		"inputsBytes":      inputBytes,
		"stdoutBytes":      len(stdoutText),
		"stderrBytes":      len(stderrText),
		"stdoutTruncated":  stdoutTruncated,
		"stderrTruncated":  stderrTruncated,
		"logLines":         len(logs),
		"logRetention": map[string]any{
			"maxStderrLines": maxSandboxLogLines,
			"maxOutputBytes": maxOutputBytes,
		},
	}
}

const maxSandboxLogLines = 50

func sandboxLogs(stderrText string, stdoutTruncated, stderrTruncated bool) []string {
	logs := []string{}
	for _, line := range strings.Split(stderrText, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		logs = append(logs, line)
		if len(logs) == maxSandboxLogLines {
			logs = append(logs, fmt.Sprintf("... stderr log truncated at %d lines", maxSandboxLogLines))
			break
		}
	}
	if stdoutTruncated {
		logs = append(logs, "stdout truncated at output byte limit")
	}
	if stderrTruncated {
		logs = append(logs, "stderr truncated at output byte limit")
	}
	return logs
}

// limitWriter captures at most limit bytes and records overflow.
type limitWriter struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (w *limitWriter) Write(p []byte) (int, error) {
	remaining := w.limit - w.buf.Len()
	if remaining <= 0 {
		w.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		w.buf.Write(p[:remaining])
		w.truncated = true
		return len(p), nil
	}
	w.buf.Write(p)
	return len(p), nil
}

func (w *limitWriter) String() string {
	return w.buf.String()
}
