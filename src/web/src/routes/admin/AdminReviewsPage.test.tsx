import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listReviews = vi.fn();
const approveAgent = vi.fn();
const rejectAgent = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    listReviews,
    approveAgent,
    rejectAgent,
  }),
}));

import { AdminReviewsPage } from './AdminReviewsPage';

const pendingAgent = {
  id: 'agent_1',
  name: 'Research Agent',
  description: 'Helps with research',
  ownerID: 'owner_1',
  ownerName: 'Publisher',
  status: 'pending_review' as const,
  visibility: 'public' as const,
  pricingType: 'one_time' as const,
  pricingAmount: 19,
  categoryID: 'cat_1',
  categoryName: 'Productivity',
  tags: ['research'],
  ratingAvg: 4.5,
  ratingCount: 8,
  installCount: 120,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
};

describe('AdminReviewsPage', () => {
  beforeEach(() => {
    listReviews.mockReset();
    approveAgent.mockReset();
    rejectAgent.mockReset();
  });

  it('renders pending agents with owner, category, and status', async () => {
    listReviews.mockResolvedValue({ data: [pendingAgent], total: 1 });

    render(<AdminReviewsPage />);

    expect(await screen.findByRole('heading', { name: 'Review Queue' })).toBeInTheDocument();
    expect(await screen.findByText('Research Agent')).toBeInTheDocument();
    expect(screen.getByText('Publisher')).toBeInTheDocument();
    expect(screen.getByText('Productivity')).toBeInTheDocument();
    expect(screen.getByLabelText('Pending Review')).toBeInTheDocument();
  });

  it('renders pricing and governance context before approval or rejection', async () => {
    listReviews.mockResolvedValue({ data: [pendingAgent], total: 1 });

    render(<AdminReviewsPage />);

    expect(await screen.findByText('Pricing: one_time $19.00')).toBeInTheDocument();
    expect(screen.getByText('Visibility: public')).toBeInTheDocument();
    expect(screen.getByText('Governance status: pending_review')).toBeInTheDocument();
  });

  it('approves agents through confirmation', async () => {
    listReviews.mockResolvedValue({ data: [pendingAgent], total: 1 });
    approveAgent.mockResolvedValue(undefined);

    render(<AdminReviewsPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Approve agent Research Agent' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Approve Agent' }));

    await waitFor(() => expect(approveAgent).toHaveBeenCalledWith('agent_1'));
  });

  it('requires a reason and rejects agents', async () => {
    listReviews.mockResolvedValue({ data: [pendingAgent], total: 1 });
    rejectAgent.mockResolvedValue(undefined);

    render(<AdminReviewsPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Reject agent Research Agent' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Reject Agent' }));
    expect(await screen.findByText('A rejection reason is required.')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Rejection Reason'), { target: { value: 'Missing tool description.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Reject Agent' }));

    await waitFor(() => expect(rejectAgent).toHaveBeenCalledWith('agent_1', 'Missing tool description.'));
  });
});
