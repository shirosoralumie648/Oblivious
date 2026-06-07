import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createConsoleApi } from './api';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    delete: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    request: vi.fn(),
    ...overrides,
  };
  return client;
}

describe('createConsoleApi', () => {
  it('starts a billing top-up checkout session', async () => {
    const post = vi.fn().mockResolvedValue({
      checkoutSessionId: 'cs_topup_1',
      url: 'https://checkout.stripe.test/session/cs_topup_1',
    });
    const api = createConsoleApi(createClient({ post }));

    await expect(api.createBillingCheckout({ amount: 25, kind: 'topup', provider: 'stripe' })).resolves.toEqual({
      checkoutSessionId: 'cs_topup_1',
      url: 'https://checkout.stripe.test/session/cs_topup_1',
    });

    expect(post).toHaveBeenCalledWith('/api/v1/billing/checkout', {
      amount: 25,
      kind: 'topup',
      provider: 'stripe',
    });
  });
});
