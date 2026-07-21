import {
  createPublishingChannelOperationContract,
  deletePublishingChannelOperationContract,
  listPublishingChannelFailedMessagesOperationContract,
  listPublishingChannelMessagesOperationContract,
  listPublishingChannelsOperationContract,
  retryPublishingChannelFailedMessagesOperationContract,
  sendPublishingChannelMessageOperationContract,
  testPublishingChannelOperationContract,
  updatePublishingChannelOperationContract,
  updatePublishingChannelStatusOperationContract,
  type OperationContractMetadataV1
} from '@/generated/operation-contracts.generated';
import {
  jsonEnvelopeDecoder,
  jsonRequestEncoder,
  noneRequestEncoder,
  type HttpClient,
  type OperationTransportContract
} from '../../services/http/client';

export type PublishingChannelType = 'api' | 'webhook' | 'feishu' | 'wechat' | 'discord' | 'slack' | 'telegram' | 'web_embed';
export type PublishingChannelStatus = 'active' | 'degraded' | 'disabled';
export type PublishingMessageRole = 'user' | 'assistant' | 'system';

export type PublishingChannel = {
  config: Record<string, unknown>;
  created_at?: string;
  id: string;
  name: string;
  organization_id?: string;
  status: PublishingChannelStatus;
  type: PublishingChannelType;
  updated_at?: string;
};

export type CreatePublishingChannelRequest = {
  config: Record<string, unknown>;
  name: string;
  type: PublishingChannelType;
};

export type UpdatePublishingChannelRequest = {
  config?: Record<string, unknown>;
  name: string;
  status?: PublishingChannelStatus;
  type: PublishingChannelType;
};

export type SendPublishingChannelMessageRequest = {
  conversation_id: string;
  role: PublishingMessageRole;
  text: string;
};

export type PublishingChannelTestResult = {
  channel_id?: string;
  channelId?: string;
  message: string;
  status: string;
  type?: string;
};

export type PublishingChannelMessageLog = {
  created_at?: string;
  direction?: 'inbound' | 'outbound';
  failure_reason?: string;
  id: string;
  next_retry_at?: string;
  raw_message?: unknown;
  retry_count?: number;
  status: string;
  transformed_message?: unknown;
  transform_error?: string;
  transform_success?: boolean;
};

export type RetryFailedChannelMessagesRequest = {
  fallback_channel_id?: string;
  force?: boolean;
  limit?: number;
};

export type RetryProcessResult = {
  claimed: number;
  failed: number;
  permanentFailures: number;
  succeeded: number;
};

type RetryProcessResultResponse = Omit<RetryProcessResult, 'permanentFailures'> & {
  permanent_failures?: number;
  permanentFailures?: number;
};

export type PublishingChannelsApi = {
  createChannel: (payload: CreatePublishingChannelRequest) => Promise<PublishingChannel>;
  deleteChannel: (id: string) => Promise<PublishingChannel>;
  listChannelMessages: (id: string) => Promise<PublishingChannelMessageLog[]>;
  listChannels: () => Promise<PublishingChannel[]>;
  listFailedChannelMessages: (id: string) => Promise<PublishingChannelMessageLog[]>;
  retryFailedChannelMessages: (id: string, payload: RetryFailedChannelMessagesRequest) => Promise<RetryProcessResult>;
  sendChannelMessage: (id: string, message: SendPublishingChannelMessageRequest) => Promise<PublishingChannelMessageLog>;
  testChannel: (id: string) => Promise<PublishingChannelTestResult>;
  updateChannel: (id: string, payload: UpdatePublishingChannelRequest) => Promise<PublishingChannel>;
  updateChannelStatus: (id: string, status: PublishingChannelStatus) => Promise<PublishingChannel>;
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

const createChannelTransport = jsonTransport<PublishingChannel>(createPublishingChannelOperationContract, 201);
const deleteChannelTransport = jsonTransport<PublishingChannel>(deletePublishingChannelOperationContract);
const listChannelMessagesTransport = jsonTransport<PublishingChannelMessageLog[]>(listPublishingChannelMessagesOperationContract);
const listChannelsTransport = jsonTransport<PublishingChannel[]>(listPublishingChannelsOperationContract);
const listFailedChannelMessagesTransport = jsonTransport<PublishingChannelMessageLog[]>(listPublishingChannelFailedMessagesOperationContract);
const retryFailedChannelMessagesTransport = jsonTransport<RetryProcessResultResponse>(retryPublishingChannelFailedMessagesOperationContract);
const sendChannelMessageTransport = jsonTransport<PublishingChannelMessageLog>(sendPublishingChannelMessageOperationContract);
const testChannelTransport = jsonTransport<PublishingChannelTestResult>(testPublishingChannelOperationContract);
const updateChannelTransport = jsonTransport<PublishingChannel>(updatePublishingChannelOperationContract);
const updateChannelStatusTransport = jsonTransport<PublishingChannel>(updatePublishingChannelStatusOperationContract);

export function createPublishingChannelsApi(client: HttpClient): PublishingChannelsApi {
  const path = '/api/v1/channels';

  return {
    createChannel: (payload) => client.post<PublishingChannel>(path, payload, undefined, createChannelTransport),
    deleteChannel: (id) => client.delete<PublishingChannel>(`${path}/${encodeURIComponent(id)}`, undefined, deleteChannelTransport),
    listChannelMessages: (id) => client.get<PublishingChannelMessageLog[]>(`${path}/${encodeURIComponent(id)}/messages`, undefined, listChannelMessagesTransport),
    listChannels: () => client.get<PublishingChannel[]>(path, undefined, listChannelsTransport),
    listFailedChannelMessages: (id) => client.get<PublishingChannelMessageLog[]>(`${path}/${encodeURIComponent(id)}/failed-messages`, undefined, listFailedChannelMessagesTransport),
    retryFailedChannelMessages: async (id, payload) => {
      const result = await client.post<RetryProcessResultResponse>(`${path}/${encodeURIComponent(id)}/retry-failed-messages`, payload, undefined, retryFailedChannelMessagesTransport);
      return {
        claimed: result.claimed,
        failed: result.failed,
        permanentFailures: result.permanentFailures ?? result.permanent_failures ?? 0,
        succeeded: result.succeeded
      };
    },
    sendChannelMessage: (id, message) => client.post<PublishingChannelMessageLog>(`${path}/${encodeURIComponent(id)}/send`, { message }, undefined, sendChannelMessageTransport),
    testChannel: (id) => client.post<PublishingChannelTestResult>(`${path}/${encodeURIComponent(id)}/test`, undefined, undefined, testChannelTransport),
    updateChannel: (id, payload) => client.put<PublishingChannel>(`${path}/${encodeURIComponent(id)}`, payload, undefined, updateChannelTransport),
    updateChannelStatus: (id, status) =>
      client.request<PublishingChannel>(`${path}/${encodeURIComponent(id)}/status`, {
        body: JSON.stringify({ status }),
        method: 'PATCH'
      }, updateChannelStatusTransport)
  };
}
