import {
  createScheduledTaskOperationContract,
  listScheduledTaskRunsOperationContract,
  listScheduledTasksOperationContract,
  runScheduledTaskNowOperationContract,
  updateScheduledTaskStatusOperationContract,
  type OperationContractMetadataV1
} from '@/generated/operation-contracts.generated';
import {
  jsonEnvelopeDecoder,
  jsonRequestEncoder,
  noneRequestEncoder,
  type HttpClient,
  type OperationTransportContract
} from '../../services/http/client';
import type { ScheduledTaskRun } from '../../types/api';

export type ScheduledTaskTargetType = 'workflow' | 'agent';

export type ScheduledTask = {
  id: string;
  organizationId?: string;
  name: string;
  targetType: ScheduledTaskTargetType;
  targetId: string;
  workflowTriggerId?: string;
  cronExpression: string;
  enabled: boolean;
  lastRunAt?: string | null;
  nextRunAt?: string | null;
  createdAt?: string;
  updatedAt?: string;
};

export type CreateScheduledTaskRequest = {
  name: string;
  targetType: ScheduledTaskTargetType;
  targetId: string;
  cronExpression: string;
  enabled: boolean;
};

export type ScheduledTasksApi = {
  createScheduledTask: (payload: CreateScheduledTaskRequest) => Promise<ScheduledTask>;
  listRuns: (taskId: string) => Promise<ScheduledTaskRun[]>;
  listScheduledTasks: () => Promise<ScheduledTask[]>;
  runScheduledTaskNow: (taskId: string) => Promise<ScheduledTaskRun>;
  updateScheduledTaskEnabled: (taskId: string, enabled: boolean) => Promise<ScheduledTask>;
};

function jsonTransport<T>(
  operation: OperationContractMetadataV1,
  status = 200
): OperationTransportContract<T> {
  return {
    operation,
    requestEncoder: operation.request.mediaType === null
      ? noneRequestEncoder(operation)
      : jsonRequestEncoder(operation),
    responseDecoder: jsonEnvelopeDecoder<T>(operation, status)
  };
}

const createScheduledTaskTransport = jsonTransport<ScheduledTask>(createScheduledTaskOperationContract, 201);
const listRunsTransport = jsonTransport<ScheduledTaskRun[]>(listScheduledTaskRunsOperationContract);
const listScheduledTasksTransport = jsonTransport<ScheduledTask[]>(listScheduledTasksOperationContract);
const runScheduledTaskNowTransport = jsonTransport<ScheduledTaskRun>(runScheduledTaskNowOperationContract, 202);
const updateScheduledTaskEnabledTransport = jsonTransport<ScheduledTask>(updateScheduledTaskStatusOperationContract);

export function createScheduledTasksApi(client: HttpClient): ScheduledTasksApi {
  const path = '/api/v1/scheduled-tasks';

  return {
    createScheduledTask: (payload) => client.post<ScheduledTask>(path, payload, undefined, createScheduledTaskTransport),
    listRuns: (taskId) => client.get<ScheduledTaskRun[]>(`${path}/${encodeURIComponent(taskId)}/runs`, undefined, listRunsTransport),
    listScheduledTasks: () => client.get<ScheduledTask[]>(path, undefined, listScheduledTasksTransport),
    runScheduledTaskNow: (taskId) => client.post<ScheduledTaskRun>(`${path}/${encodeURIComponent(taskId)}/run`, undefined, undefined, runScheduledTaskNowTransport),
    updateScheduledTaskEnabled: (taskId, enabled) =>
      client.request<ScheduledTask>(`${path}/${encodeURIComponent(taskId)}/status`, {
        body: JSON.stringify({ enabled }),
        method: 'PATCH'
      }, updateScheduledTaskEnabledTransport)
  };
}
