package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/workflow"
)

type capturedCommand struct {
	name        string
	args        []string
	hasDeadline bool
	deadline    time.Time
	capturedAt  time.Time
}

func fakeCommandFactory(captured *capturedCommand, helperEnv ...string) commandFactory {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		captured.name = name
		captured.args = args
		captured.capturedAt = time.Now()
		if deadline, ok := ctx.Deadline(); ok {
			captured.hasDeadline = true
			captured.deadline = deadline
		}
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestSandboxHelperProcess", "--")
		cmd.Env = append(os.Environ(), append([]string{"GO_SANDBOX_HELPER=1"}, helperEnv...)...)
		return cmd
	}
}

// TestSandboxHelperProcess is not a real test: it is the fake container
// process spawned by fakeCommandFactory.
func TestSandboxHelperProcess(t *testing.T) {
	if os.Getenv("GO_SANDBOX_HELPER") != "1" {
		return
	}
	switch os.Getenv("SANDBOX_HELPER_MODE") {
	case "stdout":
		fmt.Print("hello from sandbox")
	case "both":
		fmt.Print("partial result")
		fmt.Fprint(os.Stderr, "warning: deprecation\nsecond line")
	case "bigout":
		fmt.Print(strings.Repeat("x", 300_000))
	case "fail":
		fmt.Fprint(os.Stderr, "boom")
		os.Exit(3)
	case "sleep":
		time.Sleep(5 * time.Second)
	}
	os.Exit(0)
}

func enabledConfig() Config {
	return Config{Enabled: true, DefaultTimeoutMS: 30_000}
}

func runRequest(language, code string) workflow.WorkflowCodeRequest {
	return workflow.WorkflowCodeRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		Language:       language,
		Code:           code,
	}
}

func TestDisabledRunnerReturnsPolicyError(t *testing.T) {
	captured := &capturedCommand{}
	runner := NewDockerSandboxRunner(Config{}, WithCommandFactory(fakeCommandFactory(captured)))

	_, err := runner.RunWorkflowCode(context.Background(), runRequest("python", "print(1)"))
	if err != ErrSandboxDisabled {
		t.Fatalf("expected ErrSandboxDisabled, got %v", err)
	}
	if captured.name != "" {
		t.Fatalf("disabled runner must not launch containers, launched %q", captured.name)
	}
}

func TestUnsupportedLanguageRejected(t *testing.T) {
	runner := NewDockerSandboxRunner(enabledConfig(), WithCommandFactory(fakeCommandFactory(&capturedCommand{})))

	_, err := runner.RunWorkflowCode(context.Background(), runRequest("cobol", "DISPLAY 'HI'."))
	if err == nil || !strings.Contains(err.Error(), "unsupported sandbox language") {
		t.Fatalf("expected unsupported language error, got %v", err)
	}
}

func TestLanguageAllowlistEnforced(t *testing.T) {
	config := enabledConfig()
	config.AllowedLanguages = []string{"python"}
	runner := NewDockerSandboxRunner(config, WithCommandFactory(fakeCommandFactory(&capturedCommand{})))

	if _, err := runner.RunWorkflowCode(context.Background(), runRequest("ruby", "puts 1")); err == nil ||
		!strings.Contains(err.Error(), "not allowed by policy") {
		t.Fatalf("expected allowlist policy error, got %v", err)
	}
	if _, err := runner.RunWorkflowCode(context.Background(), runRequest("python", "print(1)")); err != nil {
		t.Fatalf("allowlisted language should run, got %v", err)
	}
}

func TestCodeSizeCap(t *testing.T) {
	config := enabledConfig()
	config.MaxCodeBytes = 16
	runner := NewDockerSandboxRunner(config, WithCommandFactory(fakeCommandFactory(&capturedCommand{})))

	_, err := runner.RunWorkflowCode(context.Background(), runRequest("python", strings.Repeat("a", 17)))
	if err == nil || !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("expected code size error, got %v", err)
	}
}

func TestDockerArgsSecurityProfile(t *testing.T) {
	captured := &capturedCommand{}
	runner := NewDockerSandboxRunner(enabledConfig(), WithCommandFactory(fakeCommandFactory(captured, "SANDBOX_HELPER_MODE=stdout")))

	result, err := runner.RunWorkflowCode(context.Background(), runRequest("python", "print('hi')"))
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if result.Output["stdout"] != "hello from sandbox" {
		t.Fatalf("stdout = %q", result.Output["stdout"])
	}
	if captured.name != "docker" {
		t.Fatalf("binary = %q, want docker", captured.name)
	}

	joined := strings.Join(captured.args, " ")
	for _, required := range []string{
		"--network=none",
		"--memory=256m",
		"--memory-swap=256m",
		"--cpus=1",
		"--pids-limit=128",
		"--read-only",
		"--tmpfs=/sandbox:rw,exec,size=128m",
		"--user=65534:65534",
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges",
		"--workdir=/sandbox",
		"python:3.12-alpine",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("docker args missing %q in: %s", required, joined)
		}
	}
	if !strings.Contains(joined, "OBLIVIOUS_INPUTS={}") {
		t.Fatalf("expected empty inputs env, got: %s", joined)
	}
}

func TestImageSelectionPerLanguage(t *testing.T) {
	expectations := map[string]struct {
		image          string
		scriptFragment string
	}{
		"python":     {"python:3.12-alpine", "python3 main.py"},
		"javascript": {"node:22-alpine", "node main.js"},
		"ruby":       {"ruby:3.3-alpine", "ruby main.rb"},
		"java":       {"eclipse-temurin:21", "javac Main.java && java Main"},
		"cpp":        {"gcc:14", "g++ -O2 -o main main.cpp"},
		"go":         {"golang:1.23-alpine", "go run main.go"},
		"rust":       {"rust:1.82-slim", "rustc -O -o main main.rs"},
		"php":        {"php:8.3-cli-alpine", "php main.php"},
	}

	for language, expected := range expectations {
		captured := &capturedCommand{}
		runner := NewDockerSandboxRunner(enabledConfig(), WithCommandFactory(fakeCommandFactory(captured)))
		if _, err := runner.RunWorkflowCode(context.Background(), runRequest(language, "code")); err != nil {
			t.Fatalf("%s run failed: %v", language, err)
		}
		joined := strings.Join(captured.args, " ")
		if !strings.Contains(joined, expected.image) {
			t.Fatalf("%s args missing image %q: %s", language, expected.image, joined)
		}
		if !strings.Contains(joined, expected.scriptFragment) {
			t.Fatalf("%s args missing script %q: %s", language, expected.scriptFragment, joined)
		}
	}
}

func TestLanguageAliasesNormalize(t *testing.T) {
	captured := &capturedCommand{}
	runner := NewDockerSandboxRunner(enabledConfig(), WithCommandFactory(fakeCommandFactory(captured)))

	result, err := runner.RunWorkflowCode(context.Background(), runRequest("Js", "console.log(1)"))
	if err != nil {
		t.Fatalf("alias run failed: %v", err)
	}
	if result.Raw["language"] != "javascript" {
		t.Fatalf("canonical language = %v, want javascript", result.Raw["language"])
	}
	if !strings.Contains(strings.Join(captured.args, " "), "node:22-alpine") {
		t.Fatalf("alias did not select node image")
	}
}

func TestImageOverride(t *testing.T) {
	captured := &capturedCommand{}
	config := enabledConfig()
	config.Images = map[string]string{"python": "registry.internal/python:custom"}
	runner := NewDockerSandboxRunner(config, WithCommandFactory(fakeCommandFactory(captured)))

	if _, err := runner.RunWorkflowCode(context.Background(), runRequest("python", "print(1)")); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(strings.Join(captured.args, " "), "registry.internal/python:custom") {
		t.Fatalf("image override not applied: %s", strings.Join(captured.args, " "))
	}
}

func TestTimeoutPropagation(t *testing.T) {
	captured := &capturedCommand{}
	runner := NewDockerSandboxRunner(enabledConfig(), WithCommandFactory(fakeCommandFactory(captured)))

	req := runRequest("python", "print(1)")
	req.TimeoutMS = 1500
	if _, err := runner.RunWorkflowCode(context.Background(), req); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !captured.hasDeadline {
		t.Fatal("expected context deadline to be set")
	}
	remaining := captured.deadline.Sub(captured.capturedAt)
	if remaining <= 0 || remaining > 1500*time.Millisecond+100*time.Millisecond {
		t.Fatalf("deadline %v not within requested 1500ms", remaining)
	}

	// Requests above the ceiling clamp to MaxTimeoutMS.
	config := enabledConfig()
	config.MaxTimeoutMS = 2000
	captured = &capturedCommand{}
	runner = NewDockerSandboxRunner(config, WithCommandFactory(fakeCommandFactory(captured)))
	req.TimeoutMS = 999_999
	if _, err := runner.RunWorkflowCode(context.Background(), req); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	remaining = captured.deadline.Sub(captured.capturedAt)
	if remaining > 2000*time.Millisecond+100*time.Millisecond {
		t.Fatalf("deadline %v exceeds MaxTimeoutMS clamp", remaining)
	}
}

func TestTimeoutKillsExecution(t *testing.T) {
	captured := &capturedCommand{}
	runner := NewDockerSandboxRunner(enabledConfig(), WithCommandFactory(fakeCommandFactory(captured, "SANDBOX_HELPER_MODE=sleep")))

	req := runRequest("python", "import time; time.sleep(60)")
	req.TimeoutMS = 200
	_, err := runner.RunWorkflowCode(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "timed out after 200ms") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestCancellationStopsExecution(t *testing.T) {
	captured := &capturedCommand{}
	runner := NewDockerSandboxRunner(enabledConfig(), WithCommandFactory(fakeCommandFactory(captured, "SANDBOX_HELPER_MODE=sleep")))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := runner.RunWorkflowCode(ctx, runRequest("python", "import time; time.sleep(60)"))
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "execution cancelled") {
			t.Fatalf("expected cancellation error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("sandbox cancellation did not stop execution")
	}
}

func TestNonZeroExitSurfacedInResult(t *testing.T) {
	captured := &capturedCommand{}
	runner := NewDockerSandboxRunner(enabledConfig(), WithCommandFactory(fakeCommandFactory(captured, "SANDBOX_HELPER_MODE=fail")))

	result, err := runner.RunWorkflowCode(context.Background(), runRequest("python", "raise SystemExit(3)"))
	if err != nil {
		t.Fatalf("user-code failure must not be an infra error, got %v", err)
	}
	if result.Output["exitCode"] != 3 {
		t.Fatalf("exitCode = %v, want 3", result.Output["exitCode"])
	}
	if result.Output["stderr"] != "boom" {
		t.Fatalf("stderr = %q, want boom", result.Output["stderr"])
	}
}

func TestStdoutStderrCapturedSeparatelyWithLogs(t *testing.T) {
	captured := &capturedCommand{}
	runner := NewDockerSandboxRunner(enabledConfig(), WithCommandFactory(fakeCommandFactory(captured, "SANDBOX_HELPER_MODE=both")))

	result, err := runner.RunWorkflowCode(context.Background(), runRequest("ruby", "puts 1"))
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if result.Output["stdout"] != "partial result" {
		t.Fatalf("stdout = %q", result.Output["stdout"])
	}
	if !strings.Contains(result.Output["stderr"].(string), "warning: deprecation") {
		t.Fatalf("stderr = %q", result.Output["stderr"])
	}
	if len(result.Logs) != 2 || result.Logs[0] != "warning: deprecation" || result.Logs[1] != "second line" {
		t.Fatalf("logs = %v", result.Logs)
	}
}

func TestOutputByteCapTruncates(t *testing.T) {
	captured := &capturedCommand{}
	config := enabledConfig()
	config.MaxOutputBytes = 1024
	runner := NewDockerSandboxRunner(config, WithCommandFactory(fakeCommandFactory(captured, "SANDBOX_HELPER_MODE=bigout")))

	result, err := runner.RunWorkflowCode(context.Background(), runRequest("python", "print('x'*300000)"))
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	stdout := result.Output["stdout"].(string)
	if len(stdout) != 1024 {
		t.Fatalf("stdout length = %d, want capped 1024", len(stdout))
	}
	if result.Raw["stdoutTruncated"] != true {
		t.Fatalf("expected stdoutTruncated=true, raw=%v", result.Raw)
	}
	foundLog := false
	for _, line := range result.Logs {
		if strings.Contains(line, "stdout truncated") {
			foundLog = true
		}
	}
	if !foundLog {
		t.Fatalf("expected truncation log, logs=%v", result.Logs)
	}
}

func TestInputsPassedAsJSONEnv(t *testing.T) {
	captured := &capturedCommand{}
	runner := NewDockerSandboxRunner(enabledConfig(), WithCommandFactory(fakeCommandFactory(captured)))

	req := runRequest("python", "print(1)")
	req.Inputs = map[string]any{"ticket": "T-42"}
	if _, err := runner.RunWorkflowCode(context.Background(), req); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(strings.Join(captured.args, " "), `OBLIVIOUS_INPUTS={"ticket":"T-42"}`) {
		t.Fatalf("inputs env missing: %s", strings.Join(captured.args, " "))
	}
}

func TestExecutionContextPassedAsJSONEnvAndRawEvidence(t *testing.T) {
	captured := &capturedCommand{}
	runner := NewDockerSandboxRunner(enabledConfig(), WithCommandFactory(fakeCommandFactory(captured)))

	req := runRequest("python", "print(1)")
	req.AgentID = "agent_1"
	req.RunID = "run_1"
	req.ToolRunID = "tool_run_1"
	req.ToolCallID = "tool_call_1"
	req.ToolName = "sum_order"
	req.RequestID = "req_1"
	result, err := runner.RunWorkflowCode(context.Background(), req)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	wantContextJSON := `OBLIVIOUS_EXECUTION_CONTEXT={"agentId":"agent_1","requestId":"req_1","runId":"run_1","toolCallId":"tool_call_1","toolName":"sum_order","toolRunId":"tool_run_1"}`
	if !strings.Contains(strings.Join(captured.args, " "), wantContextJSON) {
		t.Fatalf("execution context env missing: %s", strings.Join(captured.args, " "))
	}
	rawContext, ok := result.Raw["executionContext"].(map[string]string)
	if !ok {
		t.Fatalf("execution context raw type = %T, value=%+v", result.Raw["executionContext"], result.Raw["executionContext"])
	}
	if rawContext["agentId"] != "agent_1" ||
		rawContext["runId"] != "run_1" ||
		rawContext["toolRunId"] != "tool_run_1" ||
		rawContext["toolCallId"] != "tool_call_1" ||
		rawContext["toolName"] != "sum_order" ||
		rawContext["requestId"] != "req_1" {
		t.Fatalf("execution context raw = %+v", rawContext)
	}

	evidence, ok := result.Raw["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence raw type = %T, value=%+v", result.Raw["evidence"], result.Raw["evidence"])
	}
	evidenceContext, ok := evidence["executionContext"].(map[string]string)
	if !ok || evidenceContext["requestId"] != "req_1" || evidenceContext["toolRunId"] != "tool_run_1" {
		t.Fatalf("evidence execution context = %+v", evidence["executionContext"])
	}
	logRetention, ok := evidence["logRetention"].(map[string]any)
	if !ok || logRetention["maxStderrLines"] != maxSandboxLogLines || logRetention["maxOutputBytes"] != 64*1024 {
		t.Fatalf("log retention evidence = %+v", evidence["logRetention"])
	}
	if evidence["codeBytes"] != len(req.Code) || evidence["inputsBytes"] != len(`{}`) {
		t.Fatalf("size evidence = %+v", evidence)
	}
}

func TestEmptyCodeRejected(t *testing.T) {
	runner := NewDockerSandboxRunner(enabledConfig(), WithCommandFactory(fakeCommandFactory(&capturedCommand{})))

	if _, err := runner.RunWorkflowCode(context.Background(), runRequest("python", "   ")); err == nil {
		t.Fatal("expected empty-code error")
	}
}

// TestDockerSandboxIntegration exercises a real container. It is skipped
// unless docker is installed AND OBLIVIOUS_SANDBOX_IT=1, keeping CI hermetic.
func TestDockerSandboxIntegration(t *testing.T) {
	if os.Getenv("OBLIVIOUS_SANDBOX_IT") != "1" {
		t.Skip("set OBLIVIOUS_SANDBOX_IT=1 to run the docker integration test")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker binary not available")
	}

	runner := NewDockerSandboxRunner(enabledConfig())
	req := runRequest("python", "print('sandbox-integration-ok')")
	req.TimeoutMS = 60_000

	result, err := runner.RunWorkflowCode(context.Background(), req)
	if err != nil {
		t.Fatalf("integration run failed: %v", err)
	}
	if !strings.Contains(result.Output["stdout"].(string), "sandbox-integration-ok") {
		t.Fatalf("stdout = %q", result.Output["stdout"])
	}
	if result.Output["exitCode"] != 0 {
		t.Fatalf("exitCode = %v", result.Output["exitCode"])
	}
}
