import { HttpError } from './errors';
import { unwrapEnvelope } from './envelope';

export type HttpClient = {
  get: <T>(path: string, init?: RequestInit) => Promise<T>;
  post: <T>(path: string, body?: unknown, init?: RequestInit) => Promise<T>;
  put: <T>(path: string, body?: unknown, init?: RequestInit) => Promise<T>;
  delete: <T>(path: string, init?: RequestInit) => Promise<T>;
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
      ...init,
      headers: {
        Accept: 'application/json',
        ...(init.body ? { 'Content-Type': 'application/json' } : {}),
        ...(init.headers ?? {})
      }
    });

    if (!response.ok) {
      let message = response.statusText || 'HTTP request failed';

      try {
        const payload = await response.json();
        if (typeof payload === 'object' && payload !== null && 'error' in payload) {
          const error = payload.error;
          if (typeof error === 'object' && error !== null && 'message' in error && typeof error.message === 'string') {
            message = error.message;
          }
        }
      } catch {
        // Keep the default message when the error body is not JSON.
      }

      throw new HttpError(response.status, message);
    }

    if (response.status === 204) {
      return undefined as T;
    }

    return unwrapEnvelope<T>(await response.json());
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
      }),
    delete: (path, init) =>
      request(path, {
        ...init,
        method: 'DELETE'
      })
  };
}
