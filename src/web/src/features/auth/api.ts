import type { HttpClient } from '../../services/http/client';
import type { ApiUser } from '../../types/api';

export type AuthApi = {
  me: () => Promise<ApiUser>;
};

export function createAuthApi(client: HttpClient): AuthApi {
  return {
    me: () => client.get<ApiUser>('/api/v1/auth/me')
  };
}
