import { describe, expect, it } from 'vitest';

import { createHttpClient } from './client';
import { HttpError } from './errors';

describe('http client', () => {
  it('calls endpoint with GET and parses JSON', async () => {
    const fetchFn = vi.fn(async () =>
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      })
    );

    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });
    const result = await client.get<{ ok: boolean }>('/api/v1/auth/me');

    expect(result).toEqual({ ok: true });
    expect(fetchFn).toHaveBeenCalledWith('/api/v1/auth/me', {
      method: 'GET',
      headers: { Accept: 'application/json' }
    });
  });

  it('throws HttpError on non-ok response', async () => {
    const fetchFn = vi.fn(async () => new Response('nope', { status: 500, statusText: 'Server Error' }));

    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });

    await expect(client.get('/api/v1/console/usage')).rejects.toBeInstanceOf(HttpError);
  });
});
