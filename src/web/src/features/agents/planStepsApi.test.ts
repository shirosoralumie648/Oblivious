import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createAgentPlanStepsApi } from './planStepsApi';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    delete: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    request: vi.fn(),
    ...overrides
  };
  return client;
}

function agentRunPayload(
  planSteps: unknown[],
  {
    error,
    status = 'planning',
    toolRuns = []
  }: { error?: string; status?: string; toolRuns?: unknown[] } = {}
) {
  return {
    data: {
      planSteps,
      run: {
        ...(error !== undefined ? { error } : {}),
        id: 'run_1',
        iterationCount: 2,
        mode: 'planning',
        status,
        toolCallCount: toolRuns.length
      },
      toolRuns
    }
  };
}

describe('createAgentPlanStepsApi', () => {
  it('loads run detail and normalizes run status with plan steps', async () => {
    const get = vi.fn().mockResolvedValue({
      data: {
        planSteps: [{ id: 'step_1', runId: 'run_1', status: 'pending', title: 'Inspect workspace' }],
        run: {
          error: 'tool loop exceeded max iterations',
          id: 'run_1',
          iterationCount: 4,
          mode: 'planning',
          status: 'planning',
          toolCallCount: 3
        },
        status: 'planning',
        toolRuns: [
          {
            approvalStatus: 'pending',
            id: 'tool_run_1',
            riskLevel: 'dangerous',
            runId: 'run_1',
            status: 'pending_approval',
            toolName: 'execute_code',
            toolType: 'builtin'
          }
        ]
      }
    });
    const api = createAgentPlanStepsApi(createClient({ get }));

    await expect(api.getRunDetail('run_1')).resolves.toEqual({
      error: 'tool loop exceeded max iterations',
      id: 'run_1',
      iterationCount: 4,
      mode: 'planning',
      planSteps: [{ id: 'step_1', runId: 'run_1', status: 'pending', title: 'Inspect workspace' }],
      status: 'planning',
      toolCallCount: 3,
      toolRuns: [
        {
          approvalStatus: 'pending',
          id: 'tool_run_1',
          riskLevel: 'dangerous',
          runId: 'run_1',
          status: 'pending_approval',
          toolName: 'execute_code',
          toolType: 'builtin'
        }
      ]
    });

    expect(get).toHaveBeenCalledWith('/api/v1/agent/runs/run_1');
  });

  it('approves a run plan step and preserves run detail metadata', async () => {
    const post = vi.fn().mockResolvedValue(agentRunPayload(
      [{ id: 'step_1', runId: 'run_1', status: 'approved', title: 'Inspect workspace' }],
      {
        status: 'pending_approval',
        toolRuns: [{ approvalStatus: 'pending', id: 'tool_run_1', status: 'pending_approval', toolName: 'write_file' }]
      }
    ));
    const api = createAgentPlanStepsApi(createClient({ post }));

    await expect(api.approvePlanStep('run_1', 'step_1', 'Looks good')).resolves.toMatchObject({
      id: 'run_1',
      iterationCount: 2,
      mode: 'planning',
      planSteps: [{ id: 'step_1', runId: 'run_1', status: 'approved', title: 'Inspect workspace' }],
      status: 'pending_approval',
      toolCallCount: 1,
      toolRuns: [{ approvalStatus: 'pending', id: 'tool_run_1', status: 'pending_approval', toolName: 'write_file' }]
    });

    expect(post).toHaveBeenCalledWith('/api/v1/agent/runs/run_1/approve-plan-step', {
      planStepId: 'step_1',
      reason: 'Looks good'
    });
  });

  it('continues a token-budget-stopped run with an increased budget', async () => {
    const post = vi.fn().mockResolvedValue({
      data: {
        planSteps: [],
        run: {
          error: '',
          id: 'run_1',
          iterationCount: 3,
          mode: 'react',
          status: 'completed',
          toolCallCount: 1
        },
        toolRuns: []
      }
    });
    const api = createAgentPlanStepsApi(createClient({ post }));

    await expect(api.continueRunWithBudget('run_1', 2500)).resolves.toEqual({
      error: '',
      id: 'run_1',
      iterationCount: 3,
      mode: 'react',
      planSteps: [],
      status: 'completed',
      toolCallCount: 1,
      toolRuns: []
    });

    expect(post).toHaveBeenCalledWith('/api/v1/agent/runs/run_1/continue-budget', {
      tokenBudget: 2500
    });
  });

  it('executes a run plan step and returns refreshed run detail', async () => {
    const post = vi.fn().mockResolvedValue(agentRunPayload(
      [{ id: 'step_1', runId: 'run_1', resultContent: 'done', status: 'completed', title: 'Inspect workspace' }],
      { status: 'completed' }
    ));
    const api = createAgentPlanStepsApi(createClient({ post }));

    await expect(api.executePlanStep('run_1', 'step_1')).resolves.toMatchObject({
      id: 'run_1',
      planSteps: [{ id: 'step_1', runId: 'run_1', resultContent: 'done', status: 'completed', title: 'Inspect workspace' }],
      status: 'completed',
      toolRuns: []
    });

    expect(post).toHaveBeenCalledWith('/api/v1/agent/runs/run_1/execute-plan-step', {
      planStepId: 'step_1'
    });
  });

  it('skips a run plan step and returns refreshed run detail', async () => {
    const post = vi.fn().mockResolvedValue(agentRunPayload(
      [{ error: 'not needed', id: 'step_1', runId: 'run_1', status: 'skipped', title: 'Inspect workspace' }],
      { status: 'completed' }
    ));
    const api = createAgentPlanStepsApi(createClient({ post }));

    await expect(api.skipPlanStep('run_1', 'step_1', 'not needed')).resolves.toMatchObject({
      id: 'run_1',
      planSteps: [{ error: 'not needed', id: 'step_1', runId: 'run_1', status: 'skipped', title: 'Inspect workspace' }],
      status: 'completed',
      toolRuns: []
    });

    expect(post).toHaveBeenCalledWith('/api/v1/agent/runs/run_1/skip-plan-step', {
      planStepId: 'step_1',
      reason: 'not needed'
    });
  });

  it('retries a failed run plan step and returns refreshed run detail', async () => {
    const post = vi.fn().mockResolvedValue(agentRunPayload(
      [{ id: 'step_1', resultContent: 'done', runId: 'run_1', status: 'completed', title: 'Inspect workspace' }],
      { status: 'running' }
    ));
    const api = createAgentPlanStepsApi(createClient({ post }));

    await expect(api.retryPlanStep('run_1', 'step_1')).resolves.toMatchObject({
      id: 'run_1',
      planSteps: [{ id: 'step_1', resultContent: 'done', runId: 'run_1', status: 'completed', title: 'Inspect workspace' }],
      status: 'running',
      toolRuns: []
    });

    expect(post).toHaveBeenCalledWith('/api/v1/agent/runs/run_1/retry-plan-step', {
      planStepId: 'step_1'
    });
  });

  it('updates a run plan step draft through the agent run endpoint', async () => {
    const request = vi.fn().mockResolvedValue(agentRunPayload([
      {
        approvalStatus: 'pending',
        id: 'step_1',
        input: { path: 'new.go' },
        runId: 'run_1',
        status: 'pending',
        title: 'Read safer file',
        toolName: 'read_file'
      }
    ]));
    const api = createAgentPlanStepsApi(createClient({ request }));

    await expect(api.updatePlanStep('run_1', 'step_1', {
      input: { path: 'new.go' },
      title: 'Read safer file',
      toolName: 'read_file'
    })).resolves.toMatchObject({
      id: 'run_1',
      planSteps: [
        {
          approvalStatus: 'pending',
          id: 'step_1',
          input: { path: 'new.go' },
          runId: 'run_1',
          status: 'pending',
          title: 'Read safer file',
          toolName: 'read_file'
        }
      ],
      status: 'planning'
    });

    expect(request).toHaveBeenCalledWith('/api/v1/agent/runs/run_1/update-plan-step', expect.objectContaining({
      headers: { 'Content-Type': 'application/json' },
      method: 'PATCH'
    }));
    const [, init] = request.mock.calls[0];
    expect(JSON.parse(init.body)).toEqual({
      input: { path: 'new.go' },
      planStepId: 'step_1',
      title: 'Read safer file',
      toolName: 'read_file'
    });
  });

  it('creates a run plan step draft through the agent run endpoint', async () => {
    const post = vi.fn().mockResolvedValue(agentRunPayload([
      { id: 'step_1', index: 1, runId: 'run_1', status: 'pending', title: 'Draft patch' },
      { id: 'step_new', index: 2, runId: 'run_1', status: 'pending', title: 'Run checks', toolName: 'execute_code' }
    ]));
    const api = createAgentPlanStepsApi(createClient({ post }));

    await expect(api.createPlanStep('run_1', {
      afterPlanStepId: 'step_1',
      input: { command: 'go test ./internal/agent' },
      title: 'Run checks',
      toolName: 'execute_code'
    })).resolves.toMatchObject({
      id: 'run_1',
      planSteps: [
        { id: 'step_1', index: 1, runId: 'run_1', status: 'pending', title: 'Draft patch' },
        { id: 'step_new', index: 2, runId: 'run_1', status: 'pending', title: 'Run checks', toolName: 'execute_code' }
      ],
      status: 'planning'
    });

    expect(post).toHaveBeenCalledWith('/api/v1/agent/runs/run_1/create-plan-step', {
      afterPlanStepId: 'step_1',
      input: { command: 'go test ./internal/agent' },
      title: 'Run checks',
      toolName: 'execute_code'
    });
  });

  it('moves a run plan step through the agent run endpoint', async () => {
    const post = vi.fn().mockResolvedValue(agentRunPayload([
      { id: 'step_2', index: 1, runId: 'run_1', status: 'pending', title: 'Verify patch' },
      { id: 'step_1', index: 2, runId: 'run_1', status: 'pending', title: 'Draft patch' }
    ]));
    const api = createAgentPlanStepsApi(createClient({ post }));

    await expect(api.movePlanStep('run_1', 'step_2', 'up')).resolves.toMatchObject({
      id: 'run_1',
      planSteps: [
        { id: 'step_2', index: 1, runId: 'run_1', status: 'pending', title: 'Verify patch' },
        { id: 'step_1', index: 2, runId: 'run_1', status: 'pending', title: 'Draft patch' }
      ],
      status: 'planning'
    });

    expect(post).toHaveBeenCalledWith('/api/v1/agent/runs/run_1/move-plan-step', {
      direction: 'up',
      planStepId: 'step_2'
    });
  });

  it('deletes a run plan step draft through the agent run endpoint', async () => {
    const post = vi.fn().mockResolvedValue(agentRunPayload([
      { id: 'step_1', index: 1, runId: 'run_1', status: 'pending', title: 'Draft patch' },
      { id: 'step_3', index: 2, runId: 'run_1', status: 'pending', title: 'Verify patch' }
    ]));
    const api = createAgentPlanStepsApi(createClient({ post }));

    await expect(api.deletePlanStep('run_1', 'step_2')).resolves.toMatchObject({
      id: 'run_1',
      planSteps: [
        { id: 'step_1', index: 1, runId: 'run_1', status: 'pending', title: 'Draft patch' },
        { id: 'step_3', index: 2, runId: 'run_1', status: 'pending', title: 'Verify patch' }
      ],
      status: 'planning'
    });

    expect(post).toHaveBeenCalledWith('/api/v1/agent/runs/run_1/delete-plan-step', {
      planStepId: 'step_2'
    });
  });

  it('approves, rejects, and retries tool runs through the agent run endpoint', async () => {
    const post = vi.fn()
      .mockResolvedValueOnce({
        data: {
          planSteps: [{ id: 'step_1', runId: 'run_1', status: 'completed', title: 'Inspect workspace' }],
          run: {
            id: 'run_1',
            iterationCount: 3,
            mode: 'planning',
            status: 'completed',
            toolCallCount: 1
          },
          toolRuns: [{ approvalStatus: 'approved', id: 'tool_run_1', runId: 'run_1', status: 'completed', toolName: 'execute_code' }]
        }
      })
      .mockResolvedValueOnce({
        data: {
          planSteps: [],
          run: {
            error: 'Rejected by operator',
            id: 'run_1',
            iterationCount: 2,
            mode: 'planning',
            status: 'failed',
            toolCallCount: 1
          },
          toolRuns: [{ approvalStatus: 'rejected', id: 'tool_run_3', runId: 'run_1', status: 'rejected', toolName: 'write_file' }]
        }
      })
      .mockResolvedValueOnce({
        id: 'run_1',
        iterationCount: 4,
        mode: 'react',
        planSteps: [],
        status: 'running',
        toolCallCount: 2,
        toolRuns: [{ approvalStatus: 'not_required', id: 'tool_run_2', runId: 'run_1', status: 'running', toolName: 'web_search' }]
      });
    const api = createAgentPlanStepsApi(createClient({ post }));

    await expect(api.approveToolRun('run_1', 'tool_run_1', 'Reviewed command')).resolves.toEqual({
      error: undefined,
      id: 'run_1',
      iterationCount: 3,
      mode: 'planning',
      planSteps: [{ id: 'step_1', runId: 'run_1', status: 'completed', title: 'Inspect workspace' }],
      status: 'completed',
      toolCallCount: 1,
      toolRuns: [{ approvalStatus: 'approved', id: 'tool_run_1', runId: 'run_1', status: 'completed', toolName: 'execute_code' }]
    });
    await expect(api.rejectToolRun('run_1', 'tool_run_3', 'Unsafe command')).resolves.toEqual({
      error: 'Rejected by operator',
      id: 'run_1',
      iterationCount: 2,
      mode: 'planning',
      planSteps: [],
      status: 'failed',
      toolCallCount: 1,
      toolRuns: [{ approvalStatus: 'rejected', id: 'tool_run_3', runId: 'run_1', status: 'rejected', toolName: 'write_file' }]
    });
    await expect(api.retryToolRun('run_1', 'tool_run_2')).resolves.toEqual({
      error: undefined,
      id: 'run_1',
      iterationCount: 4,
      mode: 'react',
      planSteps: [],
      status: 'running',
      toolCallCount: 2,
      toolRuns: [{ approvalStatus: 'not_required', id: 'tool_run_2', runId: 'run_1', status: 'running', toolName: 'web_search' }]
    });

    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/agent/runs/run_1/approve-tool', {
      reason: 'Reviewed command',
      toolRunId: 'tool_run_1'
    });
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/agent/runs/run_1/reject-tool', {
      reason: 'Unsafe command',
      toolRunId: 'tool_run_3'
    });
    expect(post).toHaveBeenNthCalledWith(3, '/api/v1/agent/runs/run_1/retry-tool', {
      toolRunId: 'tool_run_2'
    });
  });
});
