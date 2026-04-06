import { describe, expect, it } from 'vitest';

import { createHttpClient } from './client';
import { HttpError } from './errors';

describe('http client', () => {
  it('unwraps successful envelope payloads', async () => {
    const fetchFn = vi.fn(async () =>
      new Response(JSON.stringify({ ok: true, data: { requests: 3 }, error: null }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      })
    );

    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });
    const result = await client.get<{ requests: number }>('/api/v1/console/usage');

    expect(result).toEqual({ requests: 3 });
    expect(fetchFn).toHaveBeenCalledWith('/api/v1/console/usage', {
      method: 'GET',
      headers: { Accept: 'application/json' }
    });
  });

  it('supports PUT and DELETE requests', async () => {
    const fetchFn = vi
      .fn(async () => new Response(JSON.stringify({ ok: true, data: { saved: true }, error: null }), { status: 200 }))
      .mockImplementationOnce(
        async () => new Response(JSON.stringify({ ok: true, data: { saved: true }, error: null }), { status: 200 })
      )
      .mockImplementationOnce(async () => new Response(JSON.stringify({ ok: true, data: null, error: null }), { status: 200 }));

    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });
    const putResult = await client.put<{ saved: boolean }>('/api/v1/app/me/preferences', {
      defaultMode: 'solo'
    });
    const deleteResult = await client.delete<void>('/api/v1/app/knowledge-bases/kb_1');

    expect(putResult).toEqual({ saved: true });
    expect(deleteResult).toBeUndefined();
    expect(fetchFn).toHaveBeenNthCalledWith(1, '/api/v1/app/me/preferences', {
      method: 'PUT',
      body: JSON.stringify({ defaultMode: 'solo' }),
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json'
      }
    });
    expect(fetchFn).toHaveBeenNthCalledWith(2, '/api/v1/app/knowledge-bases/kb_1', {
      method: 'DELETE',
      headers: {
        Accept: 'application/json'
      }
    });
  });

  it('throws HttpError on non-ok response', async () => {
    const fetchFn = vi.fn(async () => new Response('nope', { status: 500, statusText: 'Server Error' }));

    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });

    await expect(client.get('/api/v1/console/usage')).rejects.toBeInstanceOf(HttpError);
  });
});
