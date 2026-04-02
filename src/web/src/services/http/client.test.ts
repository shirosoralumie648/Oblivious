import { describe, expect, it, vi } from 'vitest';

import type { ApiEnvelope } from '../../types/api';

import { createHttpClient } from './client';
import { HttpError } from './errors';

describe('http client', () => {
  it('calls endpoint with GET, includes credentials, and unwraps data', async () => {
    const fetchFn = vi.fn(async () => {
      const response: ApiEnvelope<{ ok: boolean }> = {
        data: { ok: true },
        error: null,
        ok: true
      };

      return new Response(JSON.stringify(response), {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      });
    });

    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });
    const result = await client.get<{ ok: boolean }>('/api/v1/auth/me');

    expect(result).toEqual({ ok: true });
    expect(fetchFn).toHaveBeenCalledWith('/api/v1/auth/me', {
      credentials: 'include',
      method: 'GET',
      headers: { Accept: 'application/json' }
    });
  });

  it('sends PUT requests with JSON bodies', async () => {
    const fetchFn = vi.fn(async () => {
      const response: ApiEnvelope<{ updated: boolean }> = {
        data: { updated: true },
        error: null,
        ok: true
      };

      return new Response(JSON.stringify(response), {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      });
    });

    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });
    const result = await client.put('/api/v1/app/me/preferences', { onboardingCompleted: true });

    expect(result).toEqual({ updated: true });
    expect(fetchFn).toHaveBeenCalledWith('/api/v1/app/me/preferences', {
      body: JSON.stringify({ onboardingCompleted: true }),
      credentials: 'include',
      method: 'PUT',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json'
      }
    });
  });

  it('throws HttpError when API envelope reports failure', async () => {
    const fetchFn = vi.fn(async () => {
      const response: ApiEnvelope<{ ok: boolean }> = {
        data: null,
        error: { code: 'unauthorized', message: 'Unauthorized' },
        ok: false
      };

      return new Response(JSON.stringify(response), {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      });
    });

    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });

    await expect(client.get('/api/v1/auth/me')).rejects.toEqual(new HttpError(200, 'Unauthorized'));
  });

  it('throws HttpError on non-ok response', async () => {
    const fetchFn = vi.fn(async () => new Response('nope', { status: 500, statusText: 'Server Error' }));

    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });

    await expect(client.get('/api/v1/console/usage')).rejects.toBeInstanceOf(HttpError);
  });
});
