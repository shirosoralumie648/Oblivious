package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/buildinfo"
	agentv1 "oblivious/server/internal/grpc/agentv1"
	taskv1 "oblivious/server/internal/grpc/taskv1"
	workflowv1 "oblivious/server/internal/grpc/workflowv1"
	"oblivious/server/internal/task/scheduler"
	internalworkflow "oblivious/server/internal/workflow"
	pkgagent "oblivious/server/pkg/agent"
	pkgtask "oblivious/server/pkg/task"
	pkgworkflow "oblivious/server/pkg/workflow"

	"google.golang.org/grpc"
)

func TestIdentityInspectionPrecedesRuntimeSideEffects(t *testing.T) {
	identity := smokeTestIdentity()
	for _, test := range []struct {
		name     string
		args     []string
		provider buildinfo.IdentityProvider
		wantCode int
		wantRuns int
	}{
		{name: "inspection success", args: []string{buildinfo.InspectionFlag}, provider: smokeInspectionProvider{identity: identity}},
		{name: "inspection failure", args: []string{buildinfo.InspectionFlag}, provider: smokeInspectionProvider{err: &buildinfo.IdentityError{Code: buildinfo.ErrorBuildIdentityMissing, Field: "releaseCommit"}}, wantCode: 1},
		{name: "normal smoke", args: []string{"--agent-addr", "127.0.0.1:1"}, provider: smokeInspectionProvider{identity: identity}, wantRuns: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			parseCalls, dialCalls, rpcCalls := 0, 0, 0
			exitCode := runMain(context.Background(), test.args, inspectionDependencies{
				provider: test.provider, stdout: &stdout, stderr: &stderr,
				repoRoot: "/app", contract: packagedContractPath, schema: packagedSchemaPath,
			}, func(context.Context, []string) int {
				parseCalls++
				dialCalls++
				rpcCalls++
				return 0
			})
			if exitCode != test.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr=%s", exitCode, test.wantCode, stderr.String())
			}
			for name, calls := range map[string]int{"parse": parseCalls, "dial": dialCalls, "rpc": rpcCalls} {
				if calls != test.wantRuns {
					t.Fatalf("%s calls = %d, want %d", name, calls, test.wantRuns)
				}
			}
			switch test.name {
			case "inspection success":
				var got buildinfo.BuildIdentityV1
				if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || got != identity {
					t.Fatalf("inspection output = %q, identity=%#v, error=%v", stdout.String(), got, err)
				}
			case "inspection failure":
				if !strings.Contains(stderr.String(), string(buildinfo.ErrorBuildIdentityMissing)) {
					t.Fatalf("inspection error = %q", stderr.String())
				}
			}
		})
	}
}

type smokeInspectionProvider struct {
	identity buildinfo.BuildIdentityV1
	err      error
}

func (p smokeInspectionProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	return p.identity, p.err
}

func smokeTestIdentity() buildinfo.BuildIdentityV1 {
	return buildinfo.BuildIdentityV1{
		SchemaVersion: buildinfo.BuildIdentitySchemaV1, ReleaseCommit: strings.Repeat("a", 40),
		SourceTree: strings.Repeat("b", 40), ContractDigest: "sha256:" + strings.Repeat("c", 64),
		Dirty: false, EvidenceClass: buildinfo.EvidenceRepositoryLocal,
	}
}

func TestRunRequiresAllAddresses(t *testing.T) {
	var output bytes.Buffer
	err := run(context.Background(), config{AgentAddr: "127.0.0.1:1"}, &output)
	if err == nil {
		t.Fatal("expected missing-address error")
	}
	if !strings.Contains(err.Error(), "workflow") || !strings.Contains(err.Error(), "task") {
		t.Fatalf("missing-address error should name workflow and task, got %v", err)
	}
}

func TestRunDispatchesGeneratedClients(t *testing.T) {
	agentAddr, stopAgent := startSmokeServer(t, func(server *grpc.Server) {
		agentv1.RegisterAgentServiceServer(server, pkgagent.NewServer())
	})
	defer stopAgent()

	workflowAddr, stopWorkflow := startSmokeServer(t, func(server *grpc.Server) {
		workflowv1.RegisterWorkflowServiceServer(server, pkgworkflow.NewServer(internalworkflow.NewService(nil)))
	})
	defer stopWorkflow()

	taskAddr, stopTask := startSmokeServer(t, func(server *grpc.Server) {
		taskv1.RegisterTaskServiceServer(server, pkgtask.NewServer(scheduler.NewCronScheduler(scheduler.CronSchedulerConfig{}), nil))
	})
	defer stopTask()

	var output bytes.Buffer
	err := run(context.Background(), config{
		AgentAddr:    agentAddr,
		WorkflowAddr: workflowAddr,
		TaskAddr:     taskAddr,
	}, &output)
	if err != nil {
		t.Fatalf("run returned error: %v\noutput:\n%s", err, output.String())
	}

	var rep report
	if err := json.Unmarshal(output.Bytes(), &rep); err != nil {
		t.Fatalf("decode report: %v\n%s", err, output.String())
	}
	if len(rep.Results) != 3 {
		t.Fatalf("expected three service results, got %+v", rep.Results)
	}
	for _, result := range rep.Results {
		if result.GeneratedClient != "pass" {
			t.Fatalf("service %s generatedClient = %q, output:\n%s", result.Service, result.GeneratedClient, output.String())
		}
	}
}

func TestSmokeWorkflowAppliesTimeoutToRPC(t *testing.T) {
	workflowAddr, stopWorkflow := startSmokeServer(t, func(server *grpc.Server) {
		workflowv1.RegisterWorkflowServiceServer(server, blockingWorkflowServer{})
	})
	defer stopWorkflow()

	result := smokeWorkflow(context.Background(), workflowAddr, 10*time.Millisecond)
	if result.GeneratedClient != "fail" {
		t.Fatalf("expected blocked workflow smoke to fail, got %+v", result)
	}
	if result.Status != "DeadlineExceeded" {
		t.Fatalf("expected DeadlineExceeded status, got %+v", result)
	}
}

func startSmokeServer(t *testing.T, register func(*grpc.Server)) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	register(server)
	go func() {
		_ = server.Serve(listener)
	}()
	return listener.Addr().String(), server.Stop
}

type blockingWorkflowServer struct {
	workflowv1.UnimplementedWorkflowServiceServer
}

func (blockingWorkflowServer) TestNode(ctx context.Context, _ *workflowv1.TestNodeRequest) (*workflowv1.TestNodeResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
