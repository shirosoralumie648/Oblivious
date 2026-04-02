import { HttpError } from './errors';

import type { ApiEnvelope } from '../../types/api';

export type HttpClient = {
  get: <T>(path: string, init?: RequestInit) => Promise<T>;
  post: <T>(path: string, body?: unknown, init?: RequestInit) => Promise<T>;
  put: <T>(path: string, body?: unknown, init?: RequestInit) => Promise<T>;
};

export type HttpClientOptions = {
  baseUrl?: string;
  fetchFn?: typeof fetch;
};

export function createHttpClient(options: HttpClientOptions = {}): HttpClient {
  const baseUrl = options.baseUrl ?? '';
  const fetchFn = options.fetchFn ?? fetch;

  const request = async <T>(path: string, init: RequestInit = {}): Promise<T> => {
    const response = await fetchFn(`${baseUrl}${path}`, {
      credentials: 'include',
      ...init,
      headers: {
        Accept: 'application/json',
        ...(init.body ? { 'Content-Type': 'application/json' } : {}),
        ...(init.headers ?? {})
      }
    });

    if (!response.ok) {
      throw new HttpError(response.status, response.statusText || 'HTTP request failed');
    }

    if (response.status === 204) {
      return undefined as T;
    }

    const payload = (await response.json()) as ApiEnvelope<T>;

    if (!payload.ok || payload.data === null) {
      throw new HttpError(response.status, payload.error?.message || 'API request failed');
    }

    return payload.data;
  };

  return {
    get: (path, init) => request(path, { ...init, method: 'GET' }),
    post: (path, body, init) =>
      request(path, {
        ...init,
        method: 'POST',
        body: body === undefined ? undefined : JSON.stringify(body)
      }),
    put: (path, body, init) =>
      request(path, {
        ...init,
        method: 'PUT',
        body: body === undefined ? undefined : JSON.stringify(body)
      })
  };
}
