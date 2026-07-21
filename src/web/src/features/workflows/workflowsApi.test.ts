import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import type { WorkflowExecution } from './workflowsApi';
import { createWorkflowsApi } from './workflowsApi';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    get: overrides.get
      ? ((path, init) => init === undefined ? overrides.get!(path) : overrides.get!(path, init)) as HttpClient['get']
      : vi.fn(),
    post: overrides.post
      ? ((path, body, init) => init === undefined
          ? body === undefined ? overrides.post!(path) : overrides.post!(path, body)
          : overrides.post!(path, body, init)) as HttpClient['post']
      : vi.fn(),
    put: overrides.put
      ? ((path, body, init) => init === undefined
          ? body === undefined ? overrides.put!(path) : overrides.put!(path, body)
          : overrides.put!(path, body, init)) as HttpClient['put']
      : vi.fn(),
    delete: overrides.delete
      ? ((path, init) => init === undefined ? overrides.delete!(path) : overrides.delete!(path, init)) as HttpClient['delete']
      : vi.fn(),
    request: overrides.request
      ? ((path, init) => init === undefined ? overrides.request!(path) : overrides.request!(path, init)) as HttpClient['request']
      : vi.fn(),
  };
  return client;
}

describe('createWorkflowsApi', () => {
  it('accepts backend workflow execution completion statuses', () => {
    const executions = [
      {
        id: 'wexec_completed',
        status: 'completed',
        workflowId: 'workflow_1',
      },
      {
        id: 'wexec_partial',
        status: 'partial_success',
        workflowId: 'workflow_1',
      },
    ] satisfies WorkflowExecution[];

    expect(executions.map((execution) => execution.status)).toEqual(['completed', 'partial_success']);
  });

  it('lists workflows from the workflow collection endpoint', async () => {
    const get = vi.fn().mockResolvedValue([
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);
    const api = createWorkflowsApi(createClient({ get }));

    await expect(api.listWorkflows()).resolves.toEqual([
      expect.objectContaining({ id: 'workflow_1', name: 'Incident triage' }),
    ]);

    expect(get).toHaveBeenCalledWith('/api/v1/workflows');
  });

  it('creates workflows on the workflow collection endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
      id: 'workflow_2',
      name: 'Manual draft',
      status: 'draft',
      version: 1,
    });
    const api = createWorkflowsApi(createClient({ post }));
    const payload = {
      definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
      description: 'Created from the workspace',
      name: 'Manual draft',
      status: 'draft' as const,
      variables: {},
    };

    await expect(api.createWorkflow(payload)).resolves.toEqual(
      expect.objectContaining({ id: 'workflow_2', name: 'Manual draft' })
    );

    expect(post).toHaveBeenCalledWith('/api/v1/workflows', payload);
  });

  it('updates workflows on the workflow detail endpoint with variables', async () => {
    const put = vi.fn().mockResolvedValue({
      definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
      id: 'workflow_2',
      name: 'Manual draft',
      status: 'draft',
      variables: { severity: 'critical' },
      version: 2,
    });
    const api = createWorkflowsApi(createClient({ put }));
    const payload = {
      definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
      description: 'Created from the workspace',
      name: 'Manual draft',
      status: 'draft' as const,
      variables: { severity: 'critical' },
    };

    await expect(api.updateWorkflow('workflow_2', payload)).resolves.toEqual(
      expect.objectContaining({ id: 'workflow_2', variables: { severity: 'critical' }, version: 2 })
    );

    expect(put).toHaveBeenCalledWith('/api/v1/workflows/workflow_2', payload);
  });

  it('archives workflows through the workflow detail endpoint', async () => {
    const deleteRequest = vi.fn().mockResolvedValue({
      definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
      id: 'workflow_2',
      name: 'Manual draft',
      status: 'archived',
      version: 3,
    });
    const api = createWorkflowsApi(createClient({ delete: deleteRequest }));

    await expect(api.deleteWorkflow('workflow_2')).resolves.toEqual(
      expect.objectContaining({ id: 'workflow_2', status: 'archived', version: 3 })
    );

    expect(deleteRequest).toHaveBeenCalledWith('/api/v1/workflows/workflow_2');
  });

  it('executes a workflow from the workflow execution endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      id: 'wexec_1',
      status: 'running',
      workflowId: 'workflow_1',
    });
    const api = createWorkflowsApi(createClient({ post }));

    await expect(api.executeWorkflow('workflow_1', { input: { source: 'workspace' } })).resolves.toEqual({
      id: 'wexec_1',
      status: 'running',
      workflowId: 'workflow_1',
    });

    expect(post).toHaveBeenCalledWith('/api/v1/workflows/workflow_1/execute', {
      input: { source: 'workspace' },
    });
  });

  it('supports automatic workflow execution mode from the execution endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      id: 'wexec_auto',
      status: 'running',
      workflowId: 'workflow_1',
    });
    const api = createWorkflowsApi(createClient({ post }));

    await expect(api.executeWorkflow('workflow_1', { executionMode: 'auto', input: {} })).resolves.toEqual({
      id: 'wexec_auto',
      status: 'running',
      workflowId: 'workflow_1',
    });

    expect(post).toHaveBeenCalledWith('/api/v1/workflows/workflow_1/execute', {
      executionMode: 'auto',
      input: {},
    });
  });

  it('triggers a workflow webhook with the raw payload', async () => {
    const post = vi.fn().mockResolvedValue({
      id: 'wexec_webhook',
      input: { action: 'opened', source: 'github' },
      status: 'running',
      workflowId: 'workflow_1',
    });
    const api = createWorkflowsApi(createClient({ post }));

    await expect(api.triggerWorkflowWebhook('workflow_1', { action: 'opened', source: 'github' })).resolves.toEqual({
      id: 'wexec_webhook',
      input: { action: 'opened', source: 'github' },
      status: 'running',
      workflowId: 'workflow_1',
    });

    expect(post).toHaveBeenCalledWith('/api/v1/workflows/workflow_1/webhook', {
      action: 'opened',
      source: 'github',
    });
  });

  it('lists workflow versions from the version history endpoint', async () => {
    const get = vi.fn().mockResolvedValue([
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'published',
        version: 2,
      },
    ]);
    const api = createWorkflowsApi(createClient({ get }));

    await expect(api.listWorkflowVersions('workflow_1')).resolves.toEqual([
      expect.objectContaining({ status: 'draft', version: 1 }),
      expect.objectContaining({ status: 'published', version: 2 }),
    ]);

    expect(get).toHaveBeenCalledWith('/api/v1/workflows/workflow_1/versions');
  });

  it('rolls workflows back through the rollback endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      version: 3,
    });
    const api = createWorkflowsApi(createClient({ post }));

    await expect(api.rollbackWorkflow('workflow_1', { version: 1 })).resolves.toEqual(
      expect.objectContaining({ id: 'workflow_1', version: 3 })
    );

    expect(post).toHaveBeenCalledWith('/api/v1/workflows/workflow_1/rollback', { version: 1 });
  });

  it('creates workflow branches from a source workflow version', async () => {
    const post = vi.fn().mockResolvedValue({
      definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
      id: 'workflow_branch',
      name: 'Incident triage branch',
      status: 'draft',
      version: 1,
    });
    const api = createWorkflowsApi(createClient({ post }));
    const payload = {
      description: 'Experiment branch',
      experimentKey: 'routing-copy-v2',
      name: 'Incident triage branch',
      trafficPercent: 25,
      version: 2,
    };

    await expect(api.createWorkflowBranch('workflow_1', payload)).resolves.toEqual(
      expect.objectContaining({ id: 'workflow_branch', name: 'Incident triage branch' })
    );

    expect(post).toHaveBeenCalledWith('/api/v1/workflows/workflow_1/branches', payload);
  });

  it('publishes and merges workflow branches through branch action endpoints', async () => {
    const post = vi
      .fn()
      .mockResolvedValueOnce({
        definition: { nodes: [{ id: 'experiment-start', type: 'manual' }] },
        id: 'workflow_published_branch',
        name: 'Published branch',
        status: 'published',
        version: 1,
      })
      .mockResolvedValueOnce({
        definition: { nodes: [{ id: 'merged-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'published',
        version: 3,
      });
    const api = createWorkflowsApi(createClient({ post }));

    await expect(
      api.publishWorkflowBranch('workflow_1', 'workflow_branch', {
        description: 'Published experiment',
        name: 'Published branch',
      })
    ).resolves.toEqual(expect.objectContaining({ id: 'workflow_published_branch', status: 'published' }));
    await expect(api.mergeWorkflowBranch('workflow_1', 'workflow_branch')).resolves.toEqual(
      expect.objectContaining({ id: 'workflow_1', version: 3 })
    );

    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/workflows/workflow_1/branches/workflow_branch/publish', {
      description: 'Published experiment',
      name: 'Published branch',
    });
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/workflows/workflow_1/branches/workflow_branch/merge');
  });

  it('tests a workflow node from the node debug endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      nodeId: 'notify',
      output: { ok: true },
      status: 'succeeded',
      workflowId: 'workflow_1',
    });
    const api = createWorkflowsApi(createClient({ post }));

    await expect(api.testNode('workflow_1', { nodeId: 'notify', input: { ticket: 'INC-1' } })).resolves.toEqual({
      nodeId: 'notify',
      output: { ok: true },
      status: 'succeeded',
      workflowId: 'workflow_1',
    });

    expect(post).toHaveBeenCalledWith('/api/v1/workflows/workflow_1/test-node', {
      input: { ticket: 'INC-1' },
      nodeId: 'notify',
    });
  });

  it('returns structured failed workflow node test results', async () => {
    const post = vi.fn().mockResolvedValue({
      durationMs: 12,
      error: { message: 'upstream timeout' },
      input: { ticket: 'INC-1' },
      nodeId: 'notify',
      output: { statusCode: 500 },
      status: 'failed',
      trace: [{ nodeId: 'notify', status: 'failed' }],
      workflowId: 'workflow_1',
    });
    const api = createWorkflowsApi(createClient({ post }));

    await expect(api.testNode('workflow_1', { nodeId: 'notify', input: { ticket: 'INC-1' } })).resolves.toEqual({
      durationMs: 12,
      error: { message: 'upstream timeout' },
      input: { ticket: 'INC-1' },
      nodeId: 'notify',
      output: { statusCode: 500 },
      status: 'failed',
      trace: [{ nodeId: 'notify', status: 'failed' }],
      workflowId: 'workflow_1',
    });
  });

  it('reads workflow executions and execution detail endpoints', async () => {
    const get = vi
      .fn()
      .mockResolvedValueOnce([{ id: 'wexec_1', status: 'running', workflowId: 'workflow_1' }])
      .mockResolvedValueOnce({ id: 'wexec_1', status: 'paused', workflowId: 'workflow_1' });
    const api = createWorkflowsApi(createClient({ get }));

    await expect(api.listExecutions('workflow_1')).resolves.toEqual([
      { id: 'wexec_1', status: 'running', workflowId: 'workflow_1' },
    ]);
    await expect(api.getExecution('workflow_1', 'wexec_1')).resolves.toEqual({
      id: 'wexec_1',
      status: 'paused',
      workflowId: 'workflow_1',
    });

    expect(get).toHaveBeenNthCalledWith(1, '/api/v1/workflows/workflow_1/executions');
    expect(get).toHaveBeenNthCalledWith(2, '/api/v1/workflows/workflow_1/executions/wexec_1');
  });

  it('reads execution debug snapshots from the dedicated debug endpoint', async () => {
    const get = vi.fn().mockResolvedValue({
      executionId: 'wexec_1',
      workflowId: 'workflow_1',
      status: 'failed',
      variableSnapshot: {
        context: { traceId: 'trace-123' },
        input: { ticket: 'INC-1' },
        nodeOutputs: { classify: { severity: 'critical' } },
      },
      trace: [{ durationMs: 480, nodeId: 'classify', status: 'failed' }],
      events: [
        {
          id: 'wevt_1',
          executionId: 'wexec_1',
          organizationId: 'org_1',
          eventType: 'status_changed',
          fromStatus: 'running',
          toStatus: 'failed',
          createdAt: '2026-07-02T09:30:00Z',
        },
      ],
      stateReplay: {
        initialStatus: 'running',
        finalStatus: 'failed',
        valid: true,
        transitions: [
          {
            event: 'fail',
            fromStatus: 'running',
            toStatus: 'failed',
            createdAt: '2026-07-02T09:30:00Z',
            eventId: 'wevt_1',
          },
        ],
      },
      outputs: { classify: { severity: 'critical' } },
      performance: {
        bottleneckNodeId: 'classify',
        nodeDurationsMs: { classify: 480 },
        totalDurationMs: 520,
      },
      logs: [],
    });
    const api = createWorkflowsApi(createClient({ get }));

    await expect(api.getExecutionDebugSnapshot('workflow_1', 'wexec_1')).resolves.toEqual(
      expect.objectContaining({
        executionId: 'wexec_1',
        events: [expect.objectContaining({ eventType: 'status_changed', fromStatus: 'running', toStatus: 'failed' })],
        performance: expect.objectContaining({ bottleneckNodeId: 'classify' }),
        stateReplay: expect.objectContaining({
          finalStatus: 'failed',
          initialStatus: 'running',
          transitions: [expect.objectContaining({ event: 'fail', fromStatus: 'running', toStatus: 'failed' })],
          valid: true,
        }),
      })
    );

    expect(get).toHaveBeenCalledWith('/api/v1/workflows/workflow_1/executions/wexec_1/debug-snapshot');
  });

  it('posts workflow execution transition actions', async () => {
    const post = vi
      .fn()
      .mockResolvedValueOnce({ id: 'wexec_1', status: 'paused', workflowId: 'workflow_1' })
      .mockResolvedValueOnce({ id: 'wexec_1', status: 'running', workflowId: 'workflow_1' })
      .mockResolvedValueOnce({ id: 'wexec_1', status: 'cancelled', workflowId: 'workflow_1' });
    const api = createWorkflowsApi(createClient({ post }));

    await expect(api.pauseExecution('workflow_1', 'wexec_1')).resolves.toEqual(
      expect.objectContaining({ status: 'paused' })
    );
    await expect(api.resumeExecution('workflow_1', 'wexec_1')).resolves.toEqual(
      expect.objectContaining({ status: 'running' })
    );
    await expect(api.cancelExecution('workflow_1', 'wexec_1')).resolves.toEqual(
      expect.objectContaining({ status: 'cancelled' })
    );

    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/workflows/workflow_1/executions/wexec_1/pause');
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/workflows/workflow_1/executions/wexec_1/resume');
    expect(post).toHaveBeenNthCalledWith(3, '/api/v1/workflows/workflow_1/executions/wexec_1/cancel');
  });

  it('posts optional resume payloads for paused user input nodes', async () => {
    const post = vi.fn().mockResolvedValue({
      id: 'wexec_1',
      nodeExecutions: [{ nodeId: 'approval', output: { approved: true }, status: 'succeeded' }],
      status: 'running',
      workflowId: 'workflow_1',
    });
    const api = createWorkflowsApi(createClient({ post }));
    const payload = {
      input: { approved: true, approver: 'ops' },
      nodeId: 'approval',
    };

    await expect(api.resumeExecution('workflow_1', 'wexec_1', payload)).resolves.toEqual(
      expect.objectContaining({ status: 'running' })
    );

    expect(post).toHaveBeenCalledWith('/api/v1/workflows/workflow_1/executions/wexec_1/resume', payload);
  });

  it('resolves paused workflow failures through the execution decision endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      id: 'wexec_1',
      nodeExecutions: [{ attempt: 2, nodeId: 'classify-ticket', status: 'pending' }],
      status: 'running',
      workflowId: 'workflow_1',
    });
    const api = createWorkflowsApi(createClient({ post }));
    const payload = {
      action: 'retry' as const,
      input: { priority: 'urgent' },
      nodeId: 'classify-ticket',
    };

    await expect(api.resolvePausedFailure('workflow_1', 'wexec_1', payload)).resolves.toEqual({
      id: 'wexec_1',
      nodeExecutions: [{ attempt: 2, nodeId: 'classify-ticket', status: 'pending' }],
      status: 'running',
      workflowId: 'workflow_1',
    });

    expect(post).toHaveBeenCalledWith('/api/v1/workflows/workflow_1/executions/wexec_1/decision', payload);
  });

  it('checks workflow execution resource limits from the resource governance endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      id: 'wexec_1',
      status: 'max_iterations',
      workflowId: 'workflow_1',
    });
    const api = createWorkflowsApi(createClient({ post }));
    const payload = {
      nodeExecutionCount: 1001,
      now: '2026-06-04T10:30:00Z',
      totalTokens: 2048,
    };

    await expect(api.checkWorkflowResourceLimits('workflow_1', 'wexec_1', payload)).resolves.toEqual({
      id: 'wexec_1',
      status: 'max_iterations',
      workflowId: 'workflow_1',
    });

    expect(post).toHaveBeenCalledWith(
      '/api/v1/workflows/workflow_1/executions/wexec_1/resource-check',
      payload
    );
  });

  it('checks semantic trigger matches from the collection endpoint', async () => {
    const post = vi.fn().mockResolvedValue([
      {
        keyword: 'urgent outage',
        matchMethod: 'embedding',
        score: 0.91,
        semanticThreshold: 0.85,
        triggerId: 'urgent-alerts',
        workflowId: 'workflow_1',
        workflowName: 'Incident triage',
        workflowVersion: 2,
      },
    ]);
    const api = createWorkflowsApi(createClient({ post }));

    await expect(api.matchSemanticTriggers({ message: 'urgent outage in production' })).resolves.toEqual([
      expect.objectContaining({
        keyword: 'urgent outage',
        score: 0.91,
        triggerId: 'urgent-alerts',
        workflowId: 'workflow_1',
      }),
    ]);

    expect(post).toHaveBeenCalledWith('/api/v1/workflows/semantic-matches', {
      message: 'urgent outage in production',
    });
  });

  it('checks conversation trigger matches from the collection endpoint', async () => {
    const post = vi.fn().mockResolvedValue([
      {
        conversationId: 'conversation_incident',
        triggerId: 'conversation-main',
        workflowId: 'workflow_1',
        workflowName: 'Incident triage',
        workflowVersion: 2,
      },
    ]);
    const api = createWorkflowsApi(createClient({ post }));

    await expect(api.matchConversationTriggers({ conversationId: 'conversation_incident' })).resolves.toEqual([
      expect.objectContaining({
        conversationId: 'conversation_incident',
        triggerId: 'conversation-main',
        workflowId: 'workflow_1',
      }),
    ]);

    expect(post).toHaveBeenCalledWith('/api/v1/workflows/conversation-matches', {
      conversationId: 'conversation_incident',
    });
  });
});
