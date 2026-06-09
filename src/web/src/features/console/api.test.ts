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

  it('starts a subscription package checkout session', async () => {
    const post = vi.fn().mockResolvedValue({
      checkoutSessionId: 'cs_subscription_1',
      url: 'https://checkout.stripe.test/session/cs_subscription_1',
    });
    const api = createConsoleApi(createClient({ post }));

    await expect(api.createBillingCheckout({ kind: 'subscription', packageId: 'pkg_pro', provider: 'stripe' })).resolves.toEqual({
      checkoutSessionId: 'cs_subscription_1',
      url: 'https://checkout.stripe.test/session/cs_subscription_1',
    });

    expect(post).toHaveBeenCalledWith('/api/v1/billing/checkout', {
      kind: 'subscription',
      packageId: 'pkg_pro',
      provider: 'stripe',
    });
  });

  it('lists active app packages for subscription checkout', async () => {
    const get = vi.fn().mockResolvedValue([
      {
        agentLimit: 20,
        createdAt: '2026-06-01T00:00:00Z',
        durationDays: 30,
        id: 'pkg_pro',
        isActive: true,
        isPublic: true,
        maxTokensPerRequest: 32000,
        modelAccess: ['gpt-4.1'],
        name: 'Pro',
        price: 29,
        quotaAmount: 100,
        sortOrder: 10,
        tokenQuota: 1000000,
        updatedAt: '2026-06-01T00:00:00Z',
      },
    ]);
    const api = createConsoleApi(createClient({ get }));

    await expect(api.listPackages()).resolves.toEqual([
      expect.objectContaining({
        id: 'pkg_pro',
        name: 'Pro',
        price: 29,
      }),
    ]);

    expect(get).toHaveBeenCalledWith('/api/v1/app/packages');
  });
});
