import type { HttpClient } from '../../services/http/client';
import type { LoginRequest, RegisterRequest, SessionResponse, UserPreferences } from '../../types/api';

export interface AuthApi {
  login: (payload: LoginRequest) => Promise<SessionResponse>;
  logout: () => Promise<void>;
  me: () => Promise<SessionResponse>;
  register: (payload: RegisterRequest) => Promise<SessionResponse>;
  updatePreferences: (payload: UserPreferences) => Promise<UserPreferences>;
}

export function createAuthApi(client: HttpClient): AuthApi {
  return {
    login: (payload) => client.post<SessionResponse>('/api/v1/auth/login', payload),
    logout: () => client.post<void>('/api/v1/auth/logout'),
    me: () => client.get<SessionResponse>('/api/v1/auth/me'),
    register: (payload) => client.post<SessionResponse>('/api/v1/auth/register', payload),
    updatePreferences: (payload) => client.put<UserPreferences>('/api/v1/app/me/preferences', payload)
  };
}
