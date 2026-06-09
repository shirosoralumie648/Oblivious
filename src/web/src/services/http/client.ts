import { HttpError } from './errors';
import { unwrapEnvelope } from './envelope';

export type HttpClient = {
  request: <T>(path: string, init?: RequestInit) => Promise<T>;
  get: <T>(path: string, init?: RequestInit) => Promise<T>;
  post: <T>(path: string, body?: unknown, init?: RequestInit) => Promise<T>;
  put: <T>(path: string, body?: unknown, init?: RequestInit) => Promise<T>;
  delete: <T>(path: string, init?: RequestInit) => Promise<T>;
};

export type HttpClientOptions = {
  baseUrl?: string;
  fetchFn?: typeof fetch;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

export function createHttpClient(options: HttpClientOptions = {}): HttpClient {
  const baseUrl = options.baseUrl ?? '';
  const fetchFn = options.fetchFn ?? fetch;

  const request = async <T>(path: string, init: RequestInit = {}): Promise<T> => {
    const isFormDataBody = typeof FormData !== 'undefined' && init.body instanceof FormData;
    const response = await fetchFn(`${baseUrl}${path}`, {
      ...init,
      headers: {
        Accept: 'application/json',
        ...(init.body && !isFormDataBody ? { 'Content-Type': 'application/json' } : {}),
        ...(init.headers ?? {})
      }
    });

    if (!response.ok) {
      let message = response.statusText || 'HTTP request failed';
      let code: string | undefined;
      let data: unknown;

      try {
        const payload = await response.json();
        if (isRecord(payload) && 'data' in payload) {
          data = payload.data;
        }
        if (isRecord(payload) && 'error' in payload) {
          const error = payload.error;
          if (isRecord(error) && typeof error.message === 'string') {
            message = error.message;
          }
          if (isRecord(error) && typeof error.code === 'string') {
            code = error.code;
          }
        }
      } catch {
        // Keep the default message when the error body is not JSON.
      }

      throw new HttpError(response.status, message, { code, data });
    }

    if (response.status === 204) {
      return undefined as T;
    }

    return unwrapEnvelope<T>(await response.json());
  };

  return {
    request,
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
