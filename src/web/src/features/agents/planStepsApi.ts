import type { HttpClient } from '../../services/http/client';

export type AgentPlanStepStatus = 'pending' | 'approved' | 'running' | 'completed' | 'failed' | 'skipped' | string;

export type AgentPlanStep = {
  id: string;
  runId?: string;
  index?: number;
  title: string;
  status: AgentPlanStepStatus;
  approvalStatus?: string;
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
  id: string;
  status: string;
  planSteps: AgentPlanStep[];
  toolRuns: AgentToolRun[];
};

type AgentPlanStepsPayload = {
  data?: {
    planSteps?: AgentPlanStep[];
    toolRuns?: AgentToolRun[];
  };
  planSteps?: AgentPlanStep[];
  toolRuns?: AgentToolRun[];
};

type AgentRunRecord = {
  id?: string;
  status?: string;
};

type AgentRunDetailPayload = AgentPlanStepsPayload & {
  data?: AgentPlanStepsPayload['data'] & {
    id?: string;
    run?: AgentRunRecord | null;
    status?: string;
  };
  id?: string;
  run?: AgentRunRecord | null;
  status?: string;
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
    id: detail.id ?? run?.id ?? '',
    planSteps: planStepsFromPayload(payload),
    status: detail.status ?? run?.status ?? '',
    toolRuns: toolRunsFromPayload(payload)
  };
}

export type AgentPlanStepsApi = {
  getRunDetail: (runId: string) => Promise<AgentRunDetail>;
  approvePlanStep: (runId: string, planStepId: string, reason?: string) => Promise<AgentPlanStep[]>;
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
