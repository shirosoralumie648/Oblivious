import {
  approveTaskOperationContract,
  cancelTaskOperationContract,
  createTaskOperationContract,
  getTaskOperationContract,
  listTasksOperationContract,
  pauseTaskOperationContract,
  resumeTaskOperationContract,
  startTaskOperationContract,
  updateTaskBudgetOperationContract,
  type OperationContractMetadataV1
} from '@/generated/operation-contracts.generated';
import {
  jsonEnvelopeDecoder,
  jsonRequestEncoder,
  noneRequestEncoder,
  type HttpClient,
  type OperationTransportContract
} from '../../services/http/client';
import type { CreateTaskRequest, TaskDetail, TaskSummary, UpdateTaskBudgetRequest } from '../../types/api';

export interface TasksApi {
  approveTask: (taskId: string) => Promise<TaskDetail>;
  cancelTask: (taskId: string) => Promise<TaskDetail>;
  createTask: (payload: CreateTaskRequest) => Promise<TaskSummary>;
  getTask: (taskId: string) => Promise<TaskDetail>;
  listTasks: () => Promise<TaskSummary[]>;
  pauseTask: (taskId: string) => Promise<TaskDetail>;
  resumeTask: (taskId: string) => Promise<TaskDetail>;
  startTask: (taskId: string) => Promise<TaskDetail>;
  updateTaskBudget: (taskId: string, payload: UpdateTaskBudgetRequest) => Promise<TaskDetail>;
}

function jsonTransport<T>(operation: OperationContractMetadataV1): OperationTransportContract<T> {
  return {
    operation,
    requestEncoder: operation.request.mediaType === null
      ? noneRequestEncoder(operation)
      : jsonRequestEncoder(operation),
    responseDecoder: jsonEnvelopeDecoder<T>(operation, 200)
  };
}

const approveTaskTransport = jsonTransport<TaskDetail>(approveTaskOperationContract);
const cancelTaskTransport = jsonTransport<TaskDetail>(cancelTaskOperationContract);
const createTaskTransport = jsonTransport<TaskSummary>(createTaskOperationContract);
const getTaskTransport = jsonTransport<TaskDetail>(getTaskOperationContract);
const listTasksTransport = jsonTransport<TaskSummary[]>(listTasksOperationContract);
const pauseTaskTransport = jsonTransport<TaskDetail>(pauseTaskOperationContract);
const resumeTaskTransport = jsonTransport<TaskDetail>(resumeTaskOperationContract);
const startTaskTransport = jsonTransport<TaskDetail>(startTaskOperationContract);
const updateTaskBudgetTransport = jsonTransport<TaskDetail>(updateTaskBudgetOperationContract);

export function createTasksApi(client: HttpClient): TasksApi {
  return {
    approveTask: (taskId) => client.post<TaskDetail>(`/api/v1/app/tasks/${taskId}/approve`, undefined, undefined, approveTaskTransport),
    cancelTask: (taskId) => client.post<TaskDetail>(`/api/v1/app/tasks/${taskId}/cancel`, undefined, undefined, cancelTaskTransport),
    createTask: (payload) => client.post<TaskSummary>('/api/v1/app/tasks', payload, undefined, createTaskTransport),
    getTask: (taskId) => client.get<TaskDetail>(`/api/v1/app/tasks/${taskId}`, undefined, getTaskTransport),
    listTasks: () => client.get<TaskSummary[]>('/api/v1/app/tasks', undefined, listTasksTransport),
    pauseTask: (taskId) => client.post<TaskDetail>(`/api/v1/app/tasks/${taskId}/pause`, undefined, undefined, pauseTaskTransport),
    resumeTask: (taskId) => client.post<TaskDetail>(`/api/v1/app/tasks/${taskId}/resume`, undefined, undefined, resumeTaskTransport),
    startTask: (taskId) => client.post<TaskDetail>(`/api/v1/app/tasks/${taskId}/start`, undefined, undefined, startTaskTransport),
    updateTaskBudget: (taskId, payload) => client.post<TaskDetail>(`/api/v1/app/tasks/${taskId}/budget`, payload, undefined, updateTaskBudgetTransport)
  };
}
