import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createMarketplaceApi, isAutomatedReviewRejection } from './api';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
    request: vi.fn(),
    ...overrides,
  };
  return client;
}

describe('createMarketplaceApi', () => {
  it('normalizes marketplace collection envelopes', async () => {
    const client = createClient({
      get: vi
        .fn()
        .mockResolvedValueOnce({ agents: [{ id: 'agent_1', name: 'Researcher' }], total: 1 })
        .mockResolvedValueOnce({ categories: [{ id: 'cat_1', slug: 'research' }], total: 1 })
        .mockResolvedValueOnce({ agent: { id: 'agent_1' }, versions: [{ version: '1.0.0' }] }),
    });

    const api = createMarketplaceApi(client);

    await expect(api.getFeatured()).resolves.toEqual([{ id: 'agent_1', name: 'Researcher' }]);
    await expect(api.getCategories()).resolves.toEqual([{ id: 'cat_1', slug: 'research' }]);
    await expect(api.getAgent('agent_1')).resolves.toEqual({ id: 'agent_1' });

    expect(client.get).toHaveBeenNthCalledWith(1, '/api/v1/marketplace/featured');
    expect(client.get).toHaveBeenNthCalledWith(2, '/api/v1/marketplace/categories');
    expect(client.get).toHaveBeenNthCalledWith(3, '/api/v1/marketplace/agents/agent_1');
  });

  it('preserves configured payment providers from agent detail envelopes', async () => {
    const client = createClient({
      get: vi.fn().mockResolvedValue({
        agent: { id: 'agent_paid', name: 'Paid Agent' },
        paymentProviders: [{ name: 'stripe' }],
      }),
    });

    const api = createMarketplaceApi(client);

    await expect(api.getAgent('agent_paid')).resolves.toEqual({
      id: 'agent_paid',
      name: 'Paid Agent',
      paymentProviders: [{ name: 'stripe' }],
    });
  });

  it('loads curated marketplace sections from the backend route', async () => {
    const client = createClient({
      get: vi.fn().mockResolvedValue({
        popular: [{ id: 'agent_popular', name: 'Popular Agent' }],
        topRated: [{ id: 'agent_rated', name: 'Top Rated Agent' }],
        recent: [{ id: 'agent_recent', name: 'Recent Agent' }],
      }),
    });

    const api = createMarketplaceApi(client);

    await expect(api.getCuratedSections()).resolves.toEqual({
      popular: [{ id: 'agent_popular', name: 'Popular Agent' }],
      topRated: [{ id: 'agent_rated', name: 'Top Rated Agent' }],
      recent: [{ id: 'agent_recent', name: 'Recent Agent' }],
    });
    expect(client.get).toHaveBeenCalledWith('/api/v1/marketplace/curated');
  });

  it('preserves explainable recommendation metadata from search results', async () => {
    const client = createClient({
      get: vi.fn().mockResolvedValue({
        agents: [
          {
            id: 'agent_recommended',
            name: 'Invoice Reconciliation Agent',
            recommendation: {
              score: 0.91,
              reason: 'Matches "invoice"; Finance category; 4.7 rating',
            },
          },
        ],
        total: 1,
      }),
    });
    const api = createMarketplaceApi(client);

    await expect(api.searchAgents({ query: 'invoice', sort: 'recommended' })).resolves.toEqual({
      agents: [
        {
          id: 'agent_recommended',
          name: 'Invoice Reconciliation Agent',
          recommendation: {
            score: 0.91,
            reason: 'Matches "invoice"; Finance category; 4.7 rating',
          },
        },
      ],
      total: 1,
    });
  });

  it('matches backend install and uninstall routes', async () => {
    const post = vi.fn().mockResolvedValue({ id: 'install_1' });
    const del = vi.fn().mockResolvedValue(undefined);
    const api = createMarketplaceApi(createClient({ post, delete: del }));

    await api.installAgent('agent_1', 'ver_1');
    await api.uninstallAgent('agent_1');

    expect(post).toHaveBeenCalledWith('/api/v1/marketplace/agents/agent_1/install?versionID=ver_1');
    expect(del).toHaveBeenCalledWith('/api/v1/marketplace/installs/agent_1');
  });

  it('returns paid install checkout sessions from the install route', async () => {
    const post = vi.fn().mockResolvedValue({
      checkoutSessionId: 'cs_marketplace_1',
      url: 'https://checkout.example.test/session/cs_marketplace_1',
    });
    const api = createMarketplaceApi(createClient({ post }));

    await expect(api.installAgent('agent_paid', 'ver_paid')).resolves.toEqual({
      checkoutSessionId: 'cs_marketplace_1',
      url: 'https://checkout.example.test/session/cs_marketplace_1',
    });

    expect(post).toHaveBeenCalledWith('/api/v1/marketplace/agents/agent_paid/install?versionID=ver_paid');
  });

  it('serializes an optional payment provider for paid installs', async () => {
    const post = vi.fn().mockResolvedValue({
      checkoutSessionId: 'cs_marketplace_alipay',
      url: 'https://checkout.example.test/session/cs_marketplace_alipay',
    });
    const api = createMarketplaceApi(createClient({ post }));

    await api.installAgent('agent_paid', 'ver_paid', 'alipay');

    expect(post).toHaveBeenCalledWith('/api/v1/marketplace/agents/agent_paid/install?versionID=ver_paid&provider=alipay');
  });

  it('supports publisher settlement preference routes', async () => {
    const get = vi
      .fn()
      .mockResolvedValueOnce({ cycle: 'monthly', payoutBusinessDays: 5, processingFeePercent: 1 })
      .mockResolvedValueOnce({
        totalAgents: 2,
        revenueTier: {
          currentTier: 'tier_3',
          platformFeePercent: 15,
          salesToNextTier: 85000,
        },
      });
    const put = vi.fn().mockResolvedValue({ cycle: 'weekly', payoutBusinessDays: 3, processingFeePercent: 2 });
    const api = createMarketplaceApi(createClient({ get, put }));

    await expect(api.getSettlementPreferences()).resolves.toEqual({ cycle: 'monthly', payoutBusinessDays: 5, processingFeePercent: 1 });
    await expect(api.getPublisherStats()).resolves.toEqual({
      totalAgents: 2,
      revenueTier: {
        currentTier: 'tier_3',
        platformFeePercent: 15,
        salesToNextTier: 85000,
      },
    });
    await expect(api.updateSettlementPreferences('weekly')).resolves.toEqual({ cycle: 'weekly', payoutBusinessDays: 3, processingFeePercent: 2 });

    expect(get).toHaveBeenNthCalledWith(1, '/api/v1/marketplace/publisher/settlement-preferences');
    expect(get).toHaveBeenNthCalledWith(2, '/api/v1/marketplace/publisher/stats');
    expect(put).toHaveBeenCalledWith('/api/v1/marketplace/publisher/settlement-preferences', { cycle: 'weekly' });
  });

  it('supports marketplace template list detail create and install routes', async () => {
    const get = vi
      .fn()
      .mockResolvedValueOnce({ templates: [{ id: 'tpl_1', name: 'Lead Intake' }], total: 1 })
      .mockResolvedValueOnce({ template: { id: 'tpl_1', name: 'Lead Intake' } });
    const post = vi.fn().mockResolvedValueOnce({ id: 'tpl_new' }).mockResolvedValueOnce({ id: 'install_1', templateID: 'tpl_1' });
    const api = createMarketplaceApi(createClient({ get, post }));

    await expect(api.listTemplates({ type: 'agent', query: 'lead' })).resolves.toEqual({
      templates: [{ id: 'tpl_1', name: 'Lead Intake' }],
      total: 1,
    });
    await expect(api.getTemplate('tpl_1')).resolves.toEqual({ id: 'tpl_1', name: 'Lead Intake' });
    await api.createTemplate({
      type: 'agent',
      name: 'Lead Intake',
      templateData: { nodes: [] },
      tags: ['crm'],
    });
    await api.installTemplate('tpl_1');

    expect(get).toHaveBeenNthCalledWith(1, '/api/v1/marketplace/templates?type=agent&query=lead');
    expect(get).toHaveBeenNthCalledWith(2, '/api/v1/marketplace/templates/tpl_1');
    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/marketplace/templates', {
      type: 'agent',
      name: 'Lead Intake',
      templateData: { nodes: [] },
      tags: ['crm'],
    });
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/marketplace/templates/tpl_1/install');
  });

  it('recognizes automated review rejection errors with structured findings', () => {
    const error = Object.assign(new Error('Automated review rejected marketplace publication.'), {
      code: 'automated_review_rejected',
      data: {
        automatedReview: {
          decision: 'rejected',
          findings: [
            {
              type: 'prompt_injection',
              severity: 'critical',
              field: 'system_prompt',
              message: 'Prompt content attempts to override instructions or reveal hidden prompts.',
            },
          ],
        },
      },
    });

    expect(isAutomatedReviewRejection(error)).toBe(true);
  });
});
