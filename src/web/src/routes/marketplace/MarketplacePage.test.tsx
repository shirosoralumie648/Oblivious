import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { RouterProvider, createMemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { routerFuture } from '../../app/routerFuture';

const searchAgents = vi.fn();
const getCategories = vi.fn();
const getAgent = vi.fn();
const getVersions = vi.fn();
const getReviews = vi.fn();
const installAgent = vi.fn();
const publishAgent = vi.fn();
const getMyAgents = vi.fn();
const getInstalledAgents = vi.fn();
const uninstallAgent = vi.fn();
const submitReview = vi.fn();

vi.mock('../../features/marketplace/api', () => ({
  createMarketplaceApi: () => ({
    searchAgents,
    getCategories,
    getAgent,
    getVersions,
    getReviews,
    installAgent,
    publishAgent,
    getMyAgents,
    getInstalledAgents,
    uninstallAgent,
    submitReview,
  }),
}));

import { MarketplaceAgentDetailPage } from './MarketplaceAgentDetailPage';
import { MarketplaceHomePage } from './MarketplaceHomePage';
import { MarketplaceMyAgentsPage } from './MarketplaceMyAgentsPage';
import { MarketplacePublishPage } from './MarketplacePublishPage';

const agent = {
  id: 'agent_1',
  ownerID: 'owner_1',
  ownerName: 'Publisher',
  name: 'Research Agent',
  description: 'Helps with research workflows',
  categoryID: 'cat_1',
  categoryName: 'Productivity',
  categorySlug: 'productivity',
  tags: ['research', 'writing'],
  tools: '[{"name":"search"}]',
  exampleConversations: 'User asks for a market scan.',
  visibility: 'public' as const,
  status: 'approved',
  pricingType: 'free' as const,
  pricingAmount: 0,
  currentVersion: '1.0.0',
  installCount: 120,
  ratingAvg: 4.5,
  ratingCount: 8,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
};

const paidAgent = {
  ...agent,
  id: 'agent_paid',
  name: 'Paid Research Agent',
  pricingType: 'one_time' as const,
  pricingAmount: 19,
};

function resetMarketplaceMocks() {
  searchAgents.mockReset();
  getCategories.mockReset();
  getAgent.mockReset();
  getVersions.mockReset();
  getReviews.mockReset();
  installAgent.mockReset();
  publishAgent.mockReset();
  getMyAgents.mockReset();
  getInstalledAgents.mockReset();
  uninstallAgent.mockReset();
  submitReview.mockReset();

  searchAgents.mockResolvedValue({ agents: [agent], total: 1 });
  getCategories.mockResolvedValue([{ id: 'cat_1', name: 'Productivity', slug: 'productivity', agentCount: 1 }]);
  getAgent.mockResolvedValue(agent);
  getVersions.mockResolvedValue([{ id: 'ver_1', version: '1.0.0', createdAt: '2026-01-01T00:00:00Z' }]);
  getReviews.mockResolvedValue([{ id: 'rev_1', agentID: 'agent_1', userID: 'user_1', rating: 5, body: 'Great', createdAt: '2026-01-03T00:00:00Z' }]);
  installAgent.mockResolvedValue({ id: 'install_1', agentID: 'agent_1', installedAt: '2026-01-04T00:00:00Z' });
  publishAgent.mockResolvedValue(agent);
  getMyAgents.mockResolvedValue([agent]);
  getInstalledAgents.mockResolvedValue([{ id: 'install_1', agentID: 'agent_1', agentName: 'Research Agent', version: '1.0.0', installedAt: '2026-01-04T00:00:00Z' }]);
  uninstallAgent.mockResolvedValue(undefined);
  submitReview.mockResolvedValue({ id: 'rev_2', agentID: 'agent_1', userID: 'user_1', rating: 5, body: 'Useful', createdAt: '2026-01-05T00:00:00Z' });
}

function renderRoute(element: ReactNode, path = '/', initialEntry = '/') {
  const router = createMemoryRouter([{ path, element }], { future: routerFuture, initialEntries: [initialEntry] });
  return render(<RouterProvider future={routerFuture} router={router} />);
}

describe('Marketplace pages', () => {
  beforeEach(() => {
    resetMarketplaceMocks();
  });

  it('renders agent search results and invokes filters', async () => {
    renderRoute(<MarketplaceHomePage />);

    expect(await screen.findByRole('heading', { name: 'Agent Marketplace' })).toBeInTheDocument();
    expect(await screen.findByText('Research Agent')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Marketplace sort'), { target: { value: 'rating' } });

    await waitFor(() => expect(searchAgents).toHaveBeenCalledWith(expect.objectContaining({ sort: 'rating' })));
  });

  it('loads agent detail and installs an agent', async () => {
    renderRoute(<MarketplaceAgentDetailPage />, '/marketplace/agents/:agentId', '/marketplace/agents/agent_1');

    expect(await screen.findByRole('heading', { name: 'Research Agent' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Install Agent' }));

    await waitFor(() => expect(installAgent).toHaveBeenCalledWith('agent_1', 'ver_1'));
  });

  it('describes paid install settlement boundaries on the detail page', async () => {
    getAgent.mockResolvedValue(paidAgent);

    renderRoute(<MarketplaceAgentDetailPage />, '/marketplace/agents/:agentId', '/marketplace/agents/agent_paid');

    expect(await screen.findByRole('heading', { name: 'Paid Research Agent' })).toBeInTheDocument();
    expect(screen.getByText('Paid installs create a checkout-backed marketplace order before workspace installation.')).toBeInTheDocument();
  });

  it('surfaces install failures without showing installed success', async () => {
    installAgent.mockRejectedValue(new Error('checkout session rejected'));

    renderRoute(<MarketplaceAgentDetailPage />, '/marketplace/agents/:agentId', '/marketplace/agents/agent_1');

    expect(await screen.findByRole('heading', { name: 'Research Agent' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Install Agent' }));

    expect(await screen.findByText('Unable to install agent.')).toBeInTheDocument();
    expect(screen.getByText('checkout session rejected')).toBeInTheDocument();
    expect(screen.queryByText('Agent installed.')).not.toBeInTheDocument();
  });

  it('surfaces review failures and keeps review text', async () => {
    submitReview.mockRejectedValue(new Error('review moderation queue unavailable'));

    renderRoute(<MarketplaceAgentDetailPage />, '/marketplace/agents/:agentId', '/marketplace/agents/agent_1');

    expect(await screen.findByRole('heading', { name: 'Research Agent' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Review text'), { target: { value: 'Useful for launch research.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Submit Review' }));

    expect(await screen.findByText('Unable to submit review.')).toBeInTheDocument();
    expect(screen.getByText('review moderation queue unavailable')).toBeInTheDocument();
    expect(screen.getByLabelText('Review text')).toHaveValue('Useful for launch research.');
  });

  it('submits a publish payload', async () => {
    const router = createMemoryRouter(
      [
        { path: '/', element: <MarketplacePublishPage /> },
        { path: '/marketplace/agents/:agentId', element: <div>Published</div> },
      ],
      { future: routerFuture, initialEntries: ['/'] }
    );
    render(<RouterProvider future={routerFuture} router={router} />);

    fireEvent.change(await screen.findByLabelText('Name'), { target: { value: 'Research Agent' } });
    fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'Helps with research workflows' } });
    fireEvent.change(screen.getByLabelText('Tools'), { target: { value: '[{"name":"search"}]' } });
    fireEvent.change(screen.getByLabelText('Example Conversations'), { target: { value: 'Example' } });
    fireEvent.click(screen.getByRole('button', { name: 'Publish Agent' }));

    await waitFor(() => expect(publishAgent).toHaveBeenCalledWith(expect.objectContaining({ name: 'Research Agent', version: '1.0.0' })));
  });

  it('describes review and settlement boundaries for paid publisher submissions', async () => {
    renderRoute(<MarketplacePublishPage />);

    fireEvent.change(await screen.findByLabelText('Name'), { target: { value: 'Paid Research Agent' } });
    fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'Helps with paid research workflows' } });
    fireEvent.change(screen.getByLabelText('Tools'), { target: { value: '[{"name":"search"}]' } });
    fireEvent.change(screen.getByLabelText('Example Conversations'), { target: { value: 'Example' } });
    fireEvent.change(screen.getByLabelText('Pricing'), { target: { value: 'one_time' } });
    fireEvent.change(screen.getByLabelText('Price'), { target: { value: '19' } });
    fireEvent.click(screen.getByRole('button', { name: 'Publish Agent' }));

    expect(
      await screen.findByText('Agent submitted for review. Paid installs remain checkout-backed until approval and settlement evidence exist.')
    ).toBeInTheDocument();
  });

  it('renders my agents and uninstalls installed agents', async () => {
    renderRoute(<MarketplaceMyAgentsPage />);

    expect(await screen.findByRole('heading', { name: 'My Agents' })).toBeInTheDocument();
    expect(await screen.findAllByText('Research Agent')).toHaveLength(2);
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall Research Agent' }));

    await waitFor(() => expect(uninstallAgent).toHaveBeenCalledWith('install_1'));
  });

  it('surfaces uninstall failures on my agents', async () => {
    uninstallAgent.mockRejectedValue(new Error('install settlement still pending'));

    renderRoute(<MarketplaceMyAgentsPage />);

    expect(await screen.findByRole('heading', { name: 'My Agents' })).toBeInTheDocument();
    expect(await screen.findAllByText('Research Agent')).toHaveLength(2);
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall Research Agent' }));

    expect(await screen.findByText('Unable to uninstall agent.')).toBeInTheDocument();
    expect(screen.getByText('install settlement still pending')).toBeInTheDocument();
    expect(screen.getAllByText('Research Agent')).toHaveLength(2);
  });
});
