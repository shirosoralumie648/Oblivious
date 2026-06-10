import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listReviews = vi.fn();
const approveAgent = vi.fn();
const rejectAgent = vi.fn();
const requestAgentChanges = vi.fn();
const listMarketplaceAbuseReports = vi.fn();
const resolveMarketplaceAbuseReport = vi.fn();
const dismissMarketplaceAbuseReport = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    listReviews,
    approveAgent,
    rejectAgent,
    requestAgentChanges,
    listMarketplaceAbuseReports,
    resolveMarketplaceAbuseReport,
    dismissMarketplaceAbuseReport,
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

const openAbuseReport = {
  id: 'report_1',
  reporterOrganizationId: 'org_1',
  reporterUserId: 'user_1',
  agentId: 'agent_1',
  reason: 'malware',
  details: 'attempted credential exfiltration',
  status: 'open' as const,
  createdAt: '2026-01-07T00:00:00Z',
  updatedAt: '2026-01-07T00:00:00Z',
};

describe('AdminReviewsPage', () => {
  beforeEach(() => {
    listReviews.mockReset();
    approveAgent.mockReset();
    rejectAgent.mockReset();
    requestAgentChanges.mockReset();
    listMarketplaceAbuseReports.mockReset();
    resolveMarketplaceAbuseReport.mockReset();
    dismissMarketplaceAbuseReport.mockReset();
    listMarketplaceAbuseReports.mockResolvedValue({ data: [], total: 0 });
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

  it('renders pending review SLA status, deadlines, and publisher tier', async () => {
    listReviews.mockResolvedValue({
      data: [
        {
          ...pendingAgent,
          reviewSLA: {
            submittedAt: '2026-06-02T13:00:00Z',
            manualDeadlineAt: '2026-06-05T13:00:00Z',
            manualSlaHours: 72,
            manualSlaStatus: 'due_soon',
            minutesUntilDeadline: 60,
            automatedReviewDeadlineAt: '2026-06-02T13:05:00Z',
            automatedReviewSlaMinutes: 5,
            automatedReviewSlaStatus: 'overdue',
            vipPublisher: true,
            publisherTier: 'vip',
            publisherTierSource: 'organization_metadata',
          },
        },
      ],
      total: 1,
    });

    render(<AdminReviewsPage />);

    expect(await screen.findByText('Manual SLA: Due soon by 2026-06-05 13:00 UTC')).toBeInTheDocument();
    expect(screen.getByText('Automated SLA: Overdue')).toBeInTheDocument();
    expect(screen.getByText('Publisher tier: vip')).toBeInTheDocument();
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

  it('requests publisher changes with a required reason', async () => {
    listReviews.mockResolvedValue({ data: [pendingAgent], total: 1 });
    requestAgentChanges.mockResolvedValue(undefined);

    render(<AdminReviewsPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Request changes for agent Research Agent' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Request Changes' }));
    expect(await screen.findByText('A change request reason is required.')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Change Request Reason'), { target: { value: 'Add screenshots and clarify pricing.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Request Changes' }));

    await waitFor(() => expect(requestAgentChanges).toHaveBeenCalledWith('agent_1', 'Add screenshots and clarify pricing.'));
  });

  it('renders marketplace abuse reports and reloads by status filter', async () => {
    listReviews.mockResolvedValue({ data: [], total: 0 });
    listMarketplaceAbuseReports.mockResolvedValue({ data: [openAbuseReport], total: 1 });

    render(<AdminReviewsPage />);

    expect(await screen.findByRole('heading', { name: 'Marketplace Abuse Reports' })).toBeInTheDocument();
    expect(await screen.findByText('agent_1')).toBeInTheDocument();
    expect(screen.getByText('malware')).toBeInTheDocument();
    expect(screen.getByText('attempted credential exfiltration')).toBeInTheDocument();
    expect(screen.getByLabelText('Open')).toBeInTheDocument();

    await waitFor(() => expect(listMarketplaceAbuseReports).toHaveBeenCalledWith({ status: 'open', limit: 50 }));

    fireEvent.change(screen.getByLabelText('Abuse report status filter'), { target: { value: 'resolved' } });

    await waitFor(() => expect(listMarketplaceAbuseReports).toHaveBeenCalledWith({ status: 'resolved', limit: 50 }));
  });

  it('requires a resolution and resolves marketplace abuse reports', async () => {
    listReviews.mockResolvedValue({ data: [], total: 0 });
    listMarketplaceAbuseReports.mockResolvedValue({ data: [openAbuseReport], total: 1 });
    resolveMarketplaceAbuseReport.mockResolvedValue({ status: 'resolved' });

    render(<AdminReviewsPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Resolve abuse report report_1' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Resolve Report' }));
    expect(await screen.findByText('Resolution is required.')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Resolution'), { target: { value: 'agent removed' } });
    fireEvent.click(screen.getByRole('button', { name: 'Resolve Report' }));

    await waitFor(() => expect(resolveMarketplaceAbuseReport).toHaveBeenCalledWith('report_1', 'agent removed'));
    expect(await screen.findByText('Abuse report resolved.')).toBeInTheDocument();
  });

  it('dismisses marketplace abuse reports with a resolution', async () => {
    listReviews.mockResolvedValue({ data: [], total: 0 });
    listMarketplaceAbuseReports.mockResolvedValue({ data: [openAbuseReport], total: 1 });
    dismissMarketplaceAbuseReport.mockResolvedValue({ status: 'dismissed' });

    render(<AdminReviewsPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Dismiss abuse report report_1' }));
    fireEvent.change(await screen.findByLabelText('Resolution'), { target: { value: 'not reproducible' } });
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss Report' }));

    await waitFor(() => expect(dismissMarketplaceAbuseReport).toHaveBeenCalledWith('report_1', 'not reproducible'));
    expect(await screen.findByText('Abuse report dismissed.')).toBeInTheDocument();
  });
});
