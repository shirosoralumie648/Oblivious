import { describe, expect, it, vi } from 'vitest';

import { getCurrentSessionOperationContract } from '../../generated/operation-contracts.generated';
import type { HttpClient } from '../../services/http/client';
import type { SessionResponse } from '../../types/api';
import { createAuthApi } from './api';

function createClient(overrides: Partial<HttpClient> = {}): HttpClient {
  return {
    delete: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    request: vi.fn(),
    ...overrides
  };
}

describe('createAuthApi', () => {
  it('loads the current session with its generated operation contract', async () => {
    const session = {
      onboardingCompleted: true,
      preferences: {
        defaultMode: 'chat',
        modelStrategy: 'balanced',
        networkEnabledHint: false,
        onboardingCompleted: true
      },
      session: { expiresAt: '2026-07-22T00:00:00Z', id: 'session_1' },
      user: { email: 'user@example.com', id: 'user_1' },
      workspace: { id: 'workspace_1' }
    } satisfies SessionResponse;
    const get = vi.fn().mockResolvedValue(session);

    await expect(createAuthApi(createClient({ get })).me()).resolves.toEqual(session);

    expect(get).toHaveBeenCalledWith(
      '/api/v1/auth/me',
      undefined,
      expect.objectContaining({
        operation: getCurrentSessionOperationContract,
        requestEncoder: expect.objectContaining({ id: 'none' }),
        responseDecoder: expect.objectContaining({ id: 'json-envelope', status: 200 })
      })
    );
  });
});
