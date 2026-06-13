import type { HttpClient } from '../../services/http/client';

export type AgentPlanStepStatus = 'pending' | 'approved' | 'running' | 'completed' | 'failed' | 'skipped' | string;

export type AgentPlanStep = {
  id: string;
  runId?: string;
  index?: number;
  title: string;
  description?: string;
  status: AgentPlanStepStatus;
  approvalStatus?: string;
  toolName?: string;
  input?: Record<string, unknown>;
  dependsOn?: number[];
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
  dependsOn?: number[];
  description?: string;
  input?: Record<string, unknown>;
  title?: string;
  toolName?: string;
};

export type CreateAgentPlanStepRequest = {
  afterPlanStepId?: string;
  dependsOn?: number[];
  description?: string;
  input?: Record<string, unknown>;
  title: string;
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
  continueRunWithBudget: (runId: string, tokenBudget: number) => Promise<AgentRunDetail>;
  continuePlan: (runId: string) => Promise<AgentRunDetail>;
  adjustPlan: (runId: string, reason: string) => Promise<AgentRunDetail>;
  approvePlanStep: (runId: string, planStepId: string, reason?: string) => Promise<AgentRunDetail>;
  createPlanStep: (runId: string, payload: CreateAgentPlanStepRequest) => Promise<AgentRunDetail>;
  updatePlanStep: (runId: string, planStepId: string, payload: UpdateAgentPlanStepRequest) => Promise<AgentRunDetail>;
  movePlanStep: (runId: string, planStepId: string, direction: MoveAgentPlanStepDirection) => Promise<AgentRunDetail>;
  deletePlanStep: (runId: string, planStepId: string) => Promise<AgentRunDetail>;
  executePlanStep: (runId: string, planStepId: string) => Promise<AgentRunDetail>;
  skipPlanStep: (runId: string, planStepId: string, reason?: string) => Promise<AgentRunDetail>;
  retryPlanStep: (runId: string, planStepId: string) => Promise<AgentRunDetail>;
  approveToolRun: (runId: string, toolRunId: string, reason?: string) => Promise<AgentRunDetail>;
  rejectToolRun: (runId: string, toolRunId: string, reason?: string) => Promise<AgentRunDetail>;
  retryToolRun: (runId: string, toolRunId: string) => Promise<AgentRunDetail>;
};

export function createAgentPlanStepsApi(client: HttpClient): AgentPlanStepsApi {
  const runPath = (runId: string) => `/api/v1/agent/runs/${encodeURIComponent(runId)}`;

  return {
    getRunDetail: async (runId) => runDetailFromPayload(await client.get<AgentRunDetailPayload>(runPath(runId))),
    continueRunWithBudget: async (runId, tokenBudget) =>
      runDetailFromPayload(
        await client.post<AgentRunDetailPayload>(`${runPath(runId)}/continue-budget`, {
          tokenBudget
        })
      ),
    continuePlan: async (runId) =>
      runDetailFromPayload(await client.post<AgentRunDetailPayload>(`${runPath(runId)}/continue-plan`, {})),
    adjustPlan: async (runId, reason) =>
      runDetailFromPayload(
        await client.post<AgentRunDetailPayload>(`${runPath(runId)}/adjust-plan`, {
          reason
        })
      ),
    approvePlanStep: async (runId, planStepId, reason) =>
      runDetailFromPayload(
        await client.post<AgentRunDetailPayload>(`${runPath(runId)}/approve-plan-step`, {
          planStepId,
          ...(reason ? { reason } : {})
        })
      ),
    createPlanStep: async (runId, payload) =>
      runDetailFromPayload(
        await client.post<AgentRunDetailPayload>(`${runPath(runId)}/create-plan-step`, {
          title: payload.title,
          ...(payload.afterPlanStepId !== undefined ? { afterPlanStepId: payload.afterPlanStepId } : {}),
          ...(payload.description !== undefined ? { description: payload.description } : {}),
          ...(payload.toolName !== undefined ? { toolName: payload.toolName } : {}),
          ...(payload.input !== undefined ? { input: payload.input } : {}),
          ...(payload.dependsOn !== undefined ? { dependsOn: payload.dependsOn } : {})
        })
      ),
    updatePlanStep: async (runId, planStepId, payload) =>
      runDetailFromPayload(
        await client.request<AgentRunDetailPayload>(`${runPath(runId)}/update-plan-step`, {
          body: JSON.stringify({
            planStepId,
            ...(payload.title !== undefined ? { title: payload.title } : {}),
            ...(payload.description !== undefined ? { description: payload.description } : {}),
            ...(payload.toolName !== undefined ? { toolName: payload.toolName } : {}),
            ...(payload.input !== undefined ? { input: payload.input } : {}),
            ...(payload.dependsOn !== undefined ? { dependsOn: payload.dependsOn } : {})
          }),
          headers: { 'Content-Type': 'application/json' },
          method: 'PATCH'
        })
      ),
    movePlanStep: async (runId, planStepId, direction) =>
      runDetailFromPayload(
        await client.post<AgentRunDetailPayload>(`${runPath(runId)}/move-plan-step`, {
          direction,
          planStepId
        })
      ),
    deletePlanStep: async (runId, planStepId) =>
      runDetailFromPayload(
        await client.post<AgentRunDetailPayload>(`${runPath(runId)}/delete-plan-step`, {
          planStepId
        })
      ),
    executePlanStep: async (runId, planStepId) =>
      runDetailFromPayload(
        await client.post<AgentRunDetailPayload>(`${runPath(runId)}/execute-plan-step`, {
          planStepId
        })
      ),
    skipPlanStep: async (runId, planStepId, reason) =>
      runDetailFromPayload(
        await client.post<AgentRunDetailPayload>(`${runPath(runId)}/skip-plan-step`, {
          planStepId,
          ...(reason ? { reason } : {})
        })
      ),
    retryPlanStep: async (runId, planStepId) =>
      runDetailFromPayload(
        await client.post<AgentRunDetailPayload>(`${runPath(runId)}/retry-plan-step`, {
          planStepId
        })
      ),
    approveToolRun: async (runId, toolRunId, reason) =>
      runDetailFromPayload(
        await client.post<AgentRunDetailPayload>(`${runPath(runId)}/approve-tool`, {
          toolRunId,
          ...(reason ? { reason } : {})
        })
      ),
    rejectToolRun: async (runId, toolRunId, reason) =>
      runDetailFromPayload(
        await client.post<AgentRunDetailPayload>(`${runPath(runId)}/reject-tool`, {
          toolRunId,
          ...(reason ? { reason } : {})
        })
      ),
    retryToolRun: async (runId, toolRunId) =>
      runDetailFromPayload(
        await client.post<AgentRunDetailPayload>(`${runPath(runId)}/retry-tool`, {
          toolRunId
        })
      )
  };
}
