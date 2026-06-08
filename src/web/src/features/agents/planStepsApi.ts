import type { HttpClient } from '../../services/http/client';

export type AgentPlanStepStatus = 'pending' | 'approved' | 'running' | 'completed' | 'failed' | 'skipped' | string;

export type AgentPlanStep = {
  id: string;
  runId?: string;
  index?: number;
  title: string;
  status: AgentPlanStepStatus;
  approvalStatus?: string;
  toolName?: string;
  input?: Record<string, unknown>;
  resultContent?: string;
  error?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type AgentToolRun = {
  id: string;
  runId?: string;
  toolCallId?: string;
  toolName: string;
  toolType?: string;
  serverId?: string;
  riskLevel?: string;
  arguments?: Record<string, unknown>;
  status: string;
  approvalStatus?: string;
  approvalDecisionReason?: string;
  attemptCount?: number;
  resultContent?: string;
  error?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type AgentRunDetail = {
  error?: string;
  id: string;
  iterationCount?: number;
  mode?: string;
  status: string;
  toolCallCount?: number;
  planSteps: AgentPlanStep[];
  toolRuns: AgentToolRun[];
};

export type UpdateAgentPlanStepRequest = {
  input?: Record<string, unknown>;
  title?: string;
  toolName?: string;
};

export type MoveAgentPlanStepDirection = 'up' | 'down';

type AgentPlanStepsPayload = {
  data?: {
    planSteps?: AgentPlanStep[];
    toolRuns?: AgentToolRun[];
  };
  planSteps?: AgentPlanStep[];
  toolRuns?: AgentToolRun[];
};

type AgentRunRecord = {
  error?: string;
  id?: string;
  iterationCount?: number;
  mode?: string;
  status?: string;
  toolCallCount?: number;
};

type AgentRunDetailPayload = AgentPlanStepsPayload & {
  data?: AgentPlanStepsPayload['data'] & {
    error?: string;
    id?: string;
    iterationCount?: number;
    mode?: string;
    run?: AgentRunRecord | null;
    status?: string;
    toolCallCount?: number;
  };
  error?: string;
  id?: string;
  iterationCount?: number;
  mode?: string;
  run?: AgentRunRecord | null;
  status?: string;
  toolCallCount?: number;
};

function planStepsFromPayload(payload: AgentPlanStepsPayload): AgentPlanStep[] {
  return payload.planSteps ?? payload.data?.planSteps ?? [];
}

function toolRunsFromPayload(payload: AgentPlanStepsPayload): AgentToolRun[] {
  return payload.toolRuns ?? payload.data?.toolRuns ?? [];
}

function runDetailFromPayload(payload: AgentRunDetailPayload): AgentRunDetail {
  const detail = payload.data ?? payload;
  const run = detail.run ?? null;

  return {
    error: detail.error ?? run?.error,
    id: detail.id ?? run?.id ?? '',
    iterationCount: detail.iterationCount ?? run?.iterationCount,
    mode: detail.mode ?? run?.mode,
    planSteps: planStepsFromPayload(payload),
    status: detail.status ?? run?.status ?? '',
    toolCallCount: detail.toolCallCount ?? run?.toolCallCount,
    toolRuns: toolRunsFromPayload(payload)
  };
}

export type AgentPlanStepsApi = {
  getRunDetail: (runId: string) => Promise<AgentRunDetail>;
  approvePlanStep: (runId: string, planStepId: string, reason?: string) => Promise<AgentPlanStep[]>;
  updatePlanStep: (runId: string, planStepId: string, payload: UpdateAgentPlanStepRequest) => Promise<AgentPlanStep[]>;
  movePlanStep: (runId: string, planStepId: string, direction: MoveAgentPlanStepDirection) => Promise<AgentPlanStep[]>;
  executePlanStep: (runId: string, planStepId: string) => Promise<AgentPlanStep[]>;
  approveToolRun: (runId: string, toolRunId: string, reason?: string) => Promise<AgentToolRun[]>;
  rejectToolRun: (runId: string, toolRunId: string, reason?: string) => Promise<AgentToolRun[]>;
  retryToolRun: (runId: string, toolRunId: string) => Promise<AgentToolRun[]>;
};

export function createAgentPlanStepsApi(client: HttpClient): AgentPlanStepsApi {
  const runPath = (runId: string) => `/api/v1/agent/runs/${encodeURIComponent(runId)}`;

  return {
    getRunDetail: async (runId) => runDetailFromPayload(await client.get<AgentRunDetailPayload>(runPath(runId))),
    approvePlanStep: async (runId, planStepId, reason) =>
      planStepsFromPayload(
        await client.post<AgentPlanStepsPayload>(`${runPath(runId)}/approve-plan-step`, {
          planStepId,
          ...(reason ? { reason } : {})
        })
      ),
    updatePlanStep: async (runId, planStepId, payload) =>
      planStepsFromPayload(
        await client.request<AgentPlanStepsPayload>(`${runPath(runId)}/update-plan-step`, {
          body: JSON.stringify({
            planStepId,
            ...(payload.title !== undefined ? { title: payload.title } : {}),
            ...(payload.toolName !== undefined ? { toolName: payload.toolName } : {}),
            ...(payload.input !== undefined ? { input: payload.input } : {})
          }),
          headers: { 'Content-Type': 'application/json' },
          method: 'PATCH'
        })
      ),
    movePlanStep: async (runId, planStepId, direction) =>
      planStepsFromPayload(
        await client.post<AgentPlanStepsPayload>(`${runPath(runId)}/move-plan-step`, {
          direction,
          planStepId
        })
      ),
    executePlanStep: async (runId, planStepId) =>
      planStepsFromPayload(
        await client.post<AgentPlanStepsPayload>(`${runPath(runId)}/execute-plan-step`, {
          planStepId
        })
      ),
    approveToolRun: async (runId, toolRunId, reason) =>
      toolRunsFromPayload(
        await client.post<AgentPlanStepsPayload>(`${runPath(runId)}/approve-tool`, {
          toolRunId,
          ...(reason ? { reason } : {})
        })
      ),
    rejectToolRun: async (runId, toolRunId, reason) =>
      toolRunsFromPayload(
        await client.post<AgentPlanStepsPayload>(`${runPath(runId)}/reject-tool`, {
          toolRunId,
          ...(reason ? { reason } : {})
        })
      ),
    retryToolRun: async (runId, toolRunId) =>
      toolRunsFromPayload(
        await client.post<AgentPlanStepsPayload>(`${runPath(runId)}/retry-tool`, {
          toolRunId
        })
      )
  };
}
