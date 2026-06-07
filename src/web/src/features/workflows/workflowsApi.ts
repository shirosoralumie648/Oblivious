import type { HttpClient } from '../../services/http/client';

export type WorkflowStatus = 'draft' | 'published' | 'archived';
export type WorkflowExecutionStatus =
  | 'cancelled'
  | 'completed'
  | 'failed'
  | 'max_iterations'
  | 'partial_success'
  | 'paused'
  | 'queued'
  | 'running'
  | 'succeeded'
  | 'timeout';

export type WorkflowDefinition = {
  id: string;
  organizationId?: string;
  name: string;
  description?: string;
  status: WorkflowStatus;
  version: number;
  definition: Record<string, unknown>;
  variables?: Record<string, unknown>;
  createdAt?: string;
  updatedAt?: string;
};

export type CreateWorkflowRequest = {
  name: string;
  description?: string;
  status?: WorkflowStatus;
  definition: Record<string, unknown>;
  variables?: Record<string, unknown>;
};

export type UpdateWorkflowRequest = {
  name?: string;
  description?: string;
  status?: WorkflowStatus;
  definition?: Record<string, unknown>;
  variables?: Record<string, unknown>;
};

export type ExecuteWorkflowRequest = {
  executionMode?: 'auto';
  input?: Record<string, unknown>;
};

export type TriggerWorkflowWebhookRequest = Record<string, unknown>;

export type ResumeWorkflowExecutionRequest = {
  input?: Record<string, unknown>;
  nodeId?: string;
};

export type RollbackWorkflowRequest = {
  version: number;
};

export type CreateWorkflowBranchRequest = {
  name: string;
  description?: string;
  version: number;
  experimentKey?: string;
  trafficPercent?: number;
};

export type CheckWorkflowResourceLimitsRequest = {
  totalTokens?: number;
  nodeExecutionCount?: number;
  now?: string;
};

export type MatchSemanticTriggersRequest = {
  message: string;
};

export type MatchConversationTriggersRequest = {
  conversationId: string;
};

export type ConversationTriggerMatch = {
  workflowId: string;
  workflowVersion?: number;
  workflowName: string;
  triggerId?: string;
  conversationId: string;
  triggerDefinition?: Record<string, unknown>;
  workflowDefinition?: Record<string, unknown>;
};

export type SemanticTriggerMatch = {
  workflowId: string;
  workflowVersion?: number;
  workflowName: string;
  triggerId?: string;
  keyword: string;
  semanticThreshold?: number;
  score?: number;
  matchMethod?: string;
  triggerDefinition?: Record<string, unknown>;
  workflowDefinition?: Record<string, unknown>;
};

export type ResolvePausedFailureAction = 'retry' | 'continue' | 'branch' | 'fail';

export type ResolvePausedFailureRequest = {
  action: ResolvePausedFailureAction;
  input?: Record<string, unknown>;
  nextNodeId?: string;
  nodeId?: string;
};

export type TestWorkflowNodeRequest = {
  nodeId: string;
  input?: Record<string, unknown>;
};

export type WorkflowNodeTestResult = {
  workflowId: string;
  nodeId: string;
  status: string;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: Record<string, unknown>;
  durationMs?: number;
  trace?: Array<Record<string, unknown>>;
};

export type WorkflowNodeExecution = {
  nodeId?: string;
  nodeType?: string;
  input?: unknown;
  status?: string;
  attempt?: number;
  durationMs?: number;
  tokens?: number;
  costUsd?: number;
  output?: unknown;
  error?: Record<string, unknown>;
  context?: Record<string, unknown>;
  startedAt?: string;
  completedAt?: string;
};

export type WorkflowExecution = {
  id: string;
  workflowId: string;
  organizationId?: string;
  status: WorkflowExecutionStatus;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  context?: Record<string, unknown>;
  nodeExecutions?: WorkflowNodeExecution[];
  error?: Record<string, unknown>;
  startedAt?: string;
  completedAt?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type WorkflowExecutionVariableSnapshot = {
  input: Record<string, unknown>;
  context: Record<string, unknown>;
  nodeOutputs: Record<string, Record<string, unknown>>;
};

export type WorkflowExecutionDebugTraceEntry = {
  nodeId: string;
  nodeType?: string;
  status?: string;
  attempt?: number;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: Record<string, unknown>;
  context?: Record<string, unknown>;
  startedAt?: string;
  completedAt?: string;
  durationMs?: number;
};

export type WorkflowExecutionDebugPerformance = {
  totalDurationMs: number;
  nodeDurationsMs: Record<string, number>;
  bottleneckNodeId?: string;
};

export type WorkflowExecutionDebugLogEntry = {
  level: string;
  message: string;
  timestamp: string;
  nodeId?: string;
};

export type WorkflowExecutionDebugSnapshot = {
  executionId: string;
  workflowId: string;
  status: WorkflowExecutionStatus;
  variableSnapshot: WorkflowExecutionVariableSnapshot;
  trace: WorkflowExecutionDebugTraceEntry[];
  outputs: Record<string, Record<string, unknown>>;
  performance: WorkflowExecutionDebugPerformance;
  logs: WorkflowExecutionDebugLogEntry[];
};

export type WorkflowsApi = {
  checkWorkflowResourceLimits: (
    workflowId: string,
    executionId: string,
    payload: CheckWorkflowResourceLimitsRequest
  ) => Promise<WorkflowExecution>;
  createWorkflowBranch: (workflowId: string, payload: CreateWorkflowBranchRequest) => Promise<WorkflowDefinition>;
  createWorkflow: (payload: CreateWorkflowRequest) => Promise<WorkflowDefinition>;
  cancelExecution: (workflowId: string, executionId: string) => Promise<WorkflowExecution>;
  deleteWorkflow: (workflowId: string) => Promise<WorkflowDefinition>;
  executeWorkflow: (workflowId: string, payload?: ExecuteWorkflowRequest) => Promise<WorkflowExecution>;
  getExecution: (workflowId: string, executionId: string) => Promise<WorkflowExecution>;
  getExecutionDebugSnapshot: (workflowId: string, executionId: string) => Promise<WorkflowExecutionDebugSnapshot>;
  listExecutions: (workflowId: string) => Promise<WorkflowExecution[]>;
  listWorkflowVersions: (workflowId: string) => Promise<WorkflowDefinition[]>;
  listWorkflows: () => Promise<WorkflowDefinition[]>;
  matchConversationTriggers: (payload: MatchConversationTriggersRequest) => Promise<ConversationTriggerMatch[]>;
  matchSemanticTriggers: (payload: MatchSemanticTriggersRequest) => Promise<SemanticTriggerMatch[]>;
  pauseExecution: (workflowId: string, executionId: string) => Promise<WorkflowExecution>;
  resumeExecution: (
    workflowId: string,
    executionId: string,
    payload?: ResumeWorkflowExecutionRequest
  ) => Promise<WorkflowExecution>;
  resolvePausedFailure: (
    workflowId: string,
    executionId: string,
    payload: ResolvePausedFailureRequest
  ) => Promise<WorkflowExecution>;
  rollbackWorkflow: (workflowId: string, payload: RollbackWorkflowRequest) => Promise<WorkflowDefinition>;
  testNode: (workflowId: string, payload: TestWorkflowNodeRequest) => Promise<WorkflowNodeTestResult>;
  triggerWorkflowWebhook: (
    workflowId: string,
    payload?: TriggerWorkflowWebhookRequest
  ) => Promise<WorkflowExecution>;
  updateWorkflow: (workflowId: string, payload: UpdateWorkflowRequest) => Promise<WorkflowDefinition>;
};

export function createWorkflowsApi(client: HttpClient): WorkflowsApi {
  const executionPath = (workflowId: string, executionId: string) =>
    `/api/v1/workflows/${workflowId}/executions/${executionId}`;

  return {
    createWorkflow: (payload) => client.post<WorkflowDefinition>('/api/v1/workflows', payload),
    createWorkflowBranch: (workflowId, payload) =>
      client.post<WorkflowDefinition>(`/api/v1/workflows/${workflowId}/branches`, payload),
    checkWorkflowResourceLimits: (workflowId, executionId, payload) =>
      client.post<WorkflowExecution>(`${executionPath(workflowId, executionId)}/resource-check`, payload),
    cancelExecution: (workflowId, executionId) =>
      client.post<WorkflowExecution>(`${executionPath(workflowId, executionId)}/cancel`),
    deleteWorkflow: (workflowId) => client.delete<WorkflowDefinition>(`/api/v1/workflows/${workflowId}`),
    executeWorkflow: (workflowId, payload = { input: {} }) =>
      client.post<WorkflowExecution>(`/api/v1/workflows/${workflowId}/execute`, payload),
    getExecution: (workflowId, executionId) => client.get<WorkflowExecution>(executionPath(workflowId, executionId)),
    getExecutionDebugSnapshot: (workflowId, executionId) =>
      client.get<WorkflowExecutionDebugSnapshot>(`${executionPath(workflowId, executionId)}/debug-snapshot`),
    listExecutions: (workflowId) => client.get<WorkflowExecution[]>(`/api/v1/workflows/${workflowId}/executions`),
    listWorkflowVersions: (workflowId) =>
      client.get<WorkflowDefinition[]>(`/api/v1/workflows/${workflowId}/versions`),
    listWorkflows: () => client.get<WorkflowDefinition[]>('/api/v1/workflows'),
    matchConversationTriggers: (payload) =>
      client.post<ConversationTriggerMatch[]>('/api/v1/workflows/conversation-matches', payload),
    matchSemanticTriggers: (payload) => client.post<SemanticTriggerMatch[]>('/api/v1/workflows/semantic-matches', payload),
    pauseExecution: (workflowId, executionId) =>
      client.post<WorkflowExecution>(`${executionPath(workflowId, executionId)}/pause`),
    resumeExecution: (workflowId, executionId, payload) =>
      payload === undefined
        ? client.post<WorkflowExecution>(`${executionPath(workflowId, executionId)}/resume`)
        : client.post<WorkflowExecution>(`${executionPath(workflowId, executionId)}/resume`, payload),
    resolvePausedFailure: (workflowId, executionId, payload) =>
      client.post<WorkflowExecution>(`${executionPath(workflowId, executionId)}/decision`, payload),
    rollbackWorkflow: (workflowId, payload) =>
      client.post<WorkflowDefinition>(`/api/v1/workflows/${workflowId}/rollback`, payload),
    testNode: (workflowId, payload) =>
      client.post<WorkflowNodeTestResult>(`/api/v1/workflows/${workflowId}/test-node`, payload),
    triggerWorkflowWebhook: (workflowId, payload = {}) =>
      client.post<WorkflowExecution>(`/api/v1/workflows/${workflowId}/webhook`, payload),
    updateWorkflow: (workflowId, payload) =>
      client.put<WorkflowDefinition>(`/api/v1/workflows/${workflowId}`, payload),
  };
}
