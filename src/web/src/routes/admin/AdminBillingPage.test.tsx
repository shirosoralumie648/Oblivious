import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const getBillingSummary = vi.fn();
const listBillingSurface = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    getBillingSummary,
    listBillingSurface,
  }),
}));

import { AdminBillingPage } from './AdminBillingPage';

describe('AdminBillingPage', () => {
  beforeEach(() => {
    getBillingSummary.mockReset();
    listBillingSurface.mockReset();
  });

  it('renders billing summary and session inspection rows', async () => {
    getBillingSummary.mockResolvedValue({
      billingSessions: { count: 1, settledAmount: 4.5 },
      paymentIntents: { count: 2, totalAmount: 79, refundedAmount: 10 },
      webhookEvents: { count: 2, failedCount: 1 },
      settlements: { count: 1, grossAmount: 50, publisherNetAmount: 40 },
      payouts: { count: 1, totalAmount: 40 },
    });
    listBillingSurface.mockResolvedValue({
      data: [
        {
          id: 'bs_admin_phase20',
          organizationId: 'org_1',
          userId: 'user_1',
          model: 'gpt-4o',
          status: 'settled',
          settledAmount: 4.5,
          createdAt: '2026-05-28T00:00:00Z',
        },
      ],
      total: 1,
    });

    render(<AdminBillingPage />);

    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Billing Sessions' })).toBeInTheDocument();
    expect(screen.getByText('$79.00')).toBeInTheDocument();
    expect(await screen.findByText('bs_admin_phase20')).toBeInTheDocument();
    expect(screen.getByText('gpt-4o')).toBeInTheDocument();
  });

  it('switches billing surfaces and passes filters to the admin API', async () => {
    getBillingSummary.mockResolvedValue({});
    listBillingSurface.mockResolvedValue({ data: [], total: 0 });

    render(<AdminBillingPage />);

    fireEvent.change(await screen.findByLabelText('Organization ID filter'), { target: { value: 'org_1' } });
    fireEvent.change(screen.getByLabelText('Status filter'), { target: { value: 'paid' } });
    fireEvent.click(screen.getByRole('tab', { name: 'Payment Intents' }));

    await waitFor(() =>
      expect(listBillingSurface).toHaveBeenLastCalledWith(
        'paymentIntents',
        expect.objectContaining({ organizationID: 'org_1', status: 'paid', limit: 50 })
      )
    );
  });

  it('renders a commercial empty state for empty billing surfaces', async () => {
    getBillingSummary.mockResolvedValue({});
    listBillingSurface.mockResolvedValue({ data: [], total: 0 });

    render(<AdminBillingPage />);

    expect(await screen.findByText('No billing records found for this commercial surface.')).toBeInTheDocument();
  });
});
