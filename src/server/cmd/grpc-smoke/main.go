package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"oblivious/server/internal/buildinfo"
	agentv1 "oblivious/server/internal/grpc/agentv1"
	taskv1 "oblivious/server/internal/grpc/taskv1"
	workflowv1 "oblivious/server/internal/grpc/workflowv1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	packagedRepoRoot     = "/app"
	packagedContractPath = "config/release/contract.v1.json"
	packagedSchemaPath   = "config/release/contract.schema.json"
)

type inspectionDependencies struct {
	provider buildinfo.IdentityProvider
	stdout   io.Writer
	stderr   io.Writer
	repoRoot string
	contract string
	schema   string
}

type config struct {
	AgentAddr    string
	WorkflowAddr string
	TaskAddr     string
	Timeout      time.Duration
}

type serviceResult struct {
	Service         string `json:"service"`
	Address         string `json:"address"`
	GeneratedClient string `json:"generatedClient"`
	Status          string `json:"status"`
	Detail          string `json:"detail,omitempty"`
}

type report struct {
	RecordedAt time.Time       `json:"recordedAt"`
	Timeout    string          `json:"timeout"`
	Results    []serviceResult `json:"results"`
}

func main() {
	exitCode := runMain(context.Background(), os.Args[1:], inspectionDependencies{
		provider: buildinfo.NewEmbeddedProvider(), stdout: os.Stdout, stderr: os.Stderr,
		repoRoot: packagedRepoRoot, contract: packagedContractPath, schema: packagedSchemaPath,
	}, runSmokeMain)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func runMain(ctx context.Context, args []string, deps inspectionDependencies, normalSmoke func(context.Context, []string) int) int {
	handled, exitCode := buildinfo.HandleInspection(ctx, args, deps.stdout, deps.stderr, deps.provider, deps.repoRoot, deps.contract, deps.schema)
	if handled {
		return exitCode
	}
	if normalSmoke == nil {
		return 0
	}
	return normalSmoke(ctx, args)
}

func runSmokeMain(ctx context.Context, args []string) int {
	cfg := config{}
	flags := flag.NewFlagSet("oblivious-grpc-smoke", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&cfg.AgentAddr, "agent-addr", firstNonEmpty(os.Getenv("AGENT_GRPC_ADDR"), os.Getenv("OBLIVIOUS_AGENT_GRPC_ADDR")), "Agent gRPC address")
	flags.StringVar(&cfg.WorkflowAddr, "workflow-addr", firstNonEmpty(os.Getenv("WORKFLOW_GRPC_ADDR"), os.Getenv("OBLIVIOUS_WORKFLOW_GRPC_ADDR")), "Workflow gRPC address")
	flags.StringVar(&cfg.TaskAddr, "task-addr", firstNonEmpty(os.Getenv("TASK_GRPC_ADDR"), os.Getenv("OBLIVIOUS_TASK_GRPC_ADDR")), "Task gRPC address")
	timeout := flags.Duration("timeout", envDuration("GRPC_SMOKE_TIMEOUT", 10*time.Second), "per-service timeout")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return 2
	}
	cfg.Timeout = *timeout

	if err := run(ctx, cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func run(ctx context.Context, cfg config, output io.Writer) error {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	missing := missingAddresses(cfg)
	if len(missing) > 0 {
		return fmt.Errorf("missing gRPC smoke addresses: %s", strings.Join(missing, ", "))
	}

	results := []serviceResult{
		smokeAgent(ctx, cfg.AgentAddr, cfg.Timeout),
		smokeWorkflow(ctx, cfg.WorkflowAddr, cfg.Timeout),
		smokeTask(ctx, cfg.TaskAddr, cfg.Timeout),
	}
	rep := report{RecordedAt: time.Now().UTC(), Timeout: cfg.Timeout.String(), Results: results}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(rep); err != nil {
		return fmt.Errorf("write smoke report: %w", err)
	}
	var failures []string
	for _, result := range results {
		if result.GeneratedClient != "pass" {
			failures = append(failures, fmt.Sprintf("%s at %s failed: %s", result.Service, result.Address, result.Detail))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("gRPC smoke failed: %s", strings.Join(failures, "; "))
	}
	return nil
}

func smokeAgent(ctx context.Context, address string, timeout time.Duration) serviceResult {
	result := serviceResult{Service: "agent", Address: address, GeneratedClient: "fail"}
	serviceCtx, serviceCancel := context.WithTimeout(ctx, timeout)
	defer serviceCancel()
	conn, cancel, err := dial(serviceCtx, address, timeout)
	if err != nil {
		result.Status = "transport_error"
		result.Detail = err.Error()
		return result
	}
	defer cancel()
	client := agentv1.NewAgentServiceClient(conn)
	_, err = client.ContinuePlan(serviceCtx, &agentv1.PlanRunRequest{})
	if status.Code(err) != codes.InvalidArgument {
		result.Status = status.Code(err).String()
		result.Detail = errorDetail(err)
		return result
	}
	result.GeneratedClient = "pass"
	result.Status = "validation_error"
	result.Detail = "generated Agent client reached service and received expected InvalidArgument"
	return result
}

func smokeWorkflow(ctx context.Context, address string, timeout time.Duration) serviceResult {
	result := serviceResult{Service: "workflow", Address: address, GeneratedClient: "fail"}
	serviceCtx, serviceCancel := context.WithTimeout(ctx, timeout)
	defer serviceCancel()
	conn, cancel, err := dial(serviceCtx, address, timeout)
	if err != nil {
		result.Status = "transport_error"
		result.Detail = err.Error()
		return result
	}
	defer cancel()
	client := workflowv1.NewWorkflowServiceClient(conn)
	resp, err := client.TestNode(serviceCtx, &workflowv1.TestNodeRequest{
		NodeId:         "grpc-smoke-node",
		OrganizationId: "grpc-smoke-org",
	})
	if err != nil {
		result.Status = status.Code(err).String()
		result.Detail = errorDetail(err)
		return result
	}
	if resp.GetStatus() != "failed" || !strings.Contains(resp.GetError(), "workflow ID is required") {
		result.Status = "unexpected_response"
		result.Detail = fmt.Sprintf("status=%q error=%q", resp.GetStatus(), resp.GetError())
		return result
	}
	result.GeneratedClient = "pass"
	result.Status = "validation_response"
	result.Detail = "generated Workflow client reached configured service and received expected validation response"
	return result
}

func smokeTask(ctx context.Context, address string, timeout time.Duration) serviceResult {
	result := serviceResult{Service: "task", Address: address, GeneratedClient: "fail"}
	serviceCtx, serviceCancel := context.WithTimeout(ctx, timeout)
	defer serviceCancel()
	conn, cancel, err := dial(serviceCtx, address, timeout)
	if err != nil {
		result.Status = "transport_error"
		result.Detail = err.Error()
		return result
	}
	defer cancel()
	client := taskv1.NewTaskServiceClient(conn)
	resp, err := client.Cancel(serviceCtx, &taskv1.CancelRequest{TaskId: "grpc-smoke-missing"})
	if err != nil {
		result.Status = status.Code(err).String()
		result.Detail = errorDetail(err)
		return result
	}
	if resp.GetSuccess() || resp.GetMessage() != "task not found" {
		result.Status = "unexpected_response"
		result.Detail = fmt.Sprintf("success=%v message=%q", resp.GetSuccess(), resp.GetMessage())
		return result
	}
	result.GeneratedClient = "pass"
	result.Status = "validation_response"
	result.Detail = "generated Task client reached configured scheduler and received expected validation response"
	return result
}

func dial(ctx context.Context, address string, timeout time.Duration) (*grpc.ClientConn, func(), error) {
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	conn, err := grpc.DialContext(
		dialCtx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	return conn, func() {
		_ = conn.Close()
		cancel()
	}, nil
}

func missingAddresses(cfg config) []string {
	var missing []string
	if strings.TrimSpace(cfg.AgentAddr) == "" {
		missing = append(missing, "agent")
	}
	if strings.TrimSpace(cfg.WorkflowAddr) == "" {
		missing = append(missing, "workflow")
	}
	if strings.TrimSpace(cfg.TaskAddr) == "" {
		missing = append(missing, "task")
	}
	return missing
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func envDuration(name string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func errorDetail(err error) string {
	if err == nil {
		return ""
	}
	if st, ok := status.FromError(err); ok {
		return st.Message()
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return err.Error()
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Error()
	}
	return err.Error()
}
