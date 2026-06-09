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

  it('preserves structured error code and data from non-ok envelopes', async () => {
    const fetchFn = vi.fn(async () =>
      new Response(
        JSON.stringify({
          ok: false,
          data: {
            automatedReview: {
              decision: 'rejected',
              findings: [
                {
                  type: 'prompt_injection',
                  severity: 'critical',
                  message: 'Prompt content attempts to override instructions.',
                },
              ],
            },
          },
          error: {
            code: 'automated_review_rejected',
            message: 'Automated review rejected marketplace publication.',
          },
        }),
        { status: 400, statusText: 'Bad Request', headers: { 'Content-Type': 'application/json' } }
      )
    );

    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });

    await expect(client.post('/api/v1/marketplace/agents', { name: 'Unsafe Agent' })).rejects.toMatchObject({
      status: 400,
      code: 'automated_review_rejected',
      message: 'Automated review rejected marketplace publication.',
      data: {
        automatedReview: {
          decision: 'rejected',
          findings: [
            expect.objectContaining({
              severity: 'critical',
              type: 'prompt_injection',
            }),
          ],
        },
      },
    });
  });

  it('sends multipart request bodies without JSON content type', async () => {
    const fetchFn = vi.fn(async () => new Response(JSON.stringify({ ok: true, data: { uploaded: true }, error: null }), { status: 200 }));
    const client = createHttpClient({ baseUrl: 'https://api.example.test', fetchFn: fetchFn as unknown as typeof fetch });
    const formData = new FormData();
    formData.append('file', new File(['body'], 'notes.md', { type: 'text/markdown' }));

    const result = await client.request<{ uploaded: boolean }>('/api/v1/app/knowledge-bases/kb_1/documents/upload', {
      body: formData,
      method: 'POST'
    });

    expect(result).toEqual({ uploaded: true });
    expect(fetchFn).toHaveBeenCalledWith('https://api.example.test/api/v1/app/knowledge-bases/kb_1/documents/upload', {
      body: formData,
      method: 'POST',
      headers: { Accept: 'application/json' }
    });
  });
});
