import type { HttpClient } from '../../services/http/client';
import type { SessionResponse } from '../../types/api';

export type AuthApi = {
  me: () => Promise<SessionResponse>;
};

export function createAuthApi(client: HttpClient): AuthApi {
  return {
    me: () => client.get<SessionResponse>('/api/v1/auth/me')
  };
}
