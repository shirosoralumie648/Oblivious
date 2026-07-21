import { getCurrentSessionOperationContract } from '@/generated/operation-contracts.generated';
import {
  jsonEnvelopeDecoder,
  noneRequestEncoder,
  type HttpClient,
  type OperationTransportContract
} from '../../services/http/client';
import type { SessionResponse } from '../../types/api';

export type AuthApi = {
  me: () => Promise<SessionResponse>;
};

const getCurrentSessionTransport: OperationTransportContract<SessionResponse> = {
  operation: getCurrentSessionOperationContract,
  requestEncoder: noneRequestEncoder(getCurrentSessionOperationContract),
  responseDecoder: jsonEnvelopeDecoder<SessionResponse>(getCurrentSessionOperationContract, 200)
};

export function createAuthApi(client: HttpClient): AuthApi {
  return {
    me: () => client.get<SessionResponse>('/api/v1/auth/me', undefined, getCurrentSessionTransport)
  };
}
