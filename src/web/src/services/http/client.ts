import { HttpError } from './errors';

export type HttpClient = {
  get: <T>(path: string, init?: RequestInit) => Promise<T>;
  post: <T>(path: string, body?: unknown, init?: RequestInit) => Promise<T>;
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
      throw new HttpError(response.status, response.statusText || 'HTTP request failed');
    }

    if (response.status === 204) {
      return undefined as T;
    }

    return (await response.json()) as T;
  };

  return {
    get: (path, init) => request(path, { ...init, method: 'GET' }),
    post: (path, body, init) =>
      request(path, {
        ...init,
        method: 'POST',
        body: body === undefined ? undefined : JSON.stringify(body)
      })
  };
}
