import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const getBillingSummary = vi.fn();
const listBillingSurface = vi.fn();
const markMarketplacePayoutPaid = vi.fn();
const refundTopup = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    getBillingSummary,
    listBillingSurface,
    markMarketplacePayoutPaid,
    refundTopup,
  }),
}));

import { AdminBillingPage } from './AdminBillingPage';

describe('AdminBillingPage', () => {
  beforeEach(() => {
    getBillingSummary.mockReset();
    listBillingSurface.mockReset();
    markMarketplacePayoutPaid.mockReset();
    refundTopup.mockReset();
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

  it('opens the failed webhook recovery queue with a failed status filter', async () => {
    getBillingSummary.mockResolvedValue({
      webhookEvents: { count: 4, failedCount: 2 },
    });
    listBillingSurface.mockResolvedValue({ data: [], total: 0 });

    render(<AdminBillingPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Review failed webhooks' }));

    await waitFor(() =>
      expect(listBillingSurface).toHaveBeenLastCalledWith(
        'webhookEvents',
        expect.objectContaining({ status: 'failed', limit: 50 })
      )
    );
  });

  it('renders refund recovery investigation fields in the refunds table', async () => {
    getBillingSummary.mockResolvedValue({});
    listBillingSurface
      .mockResolvedValueOnce({ data: [], total: 0 })
      .mockResolvedValueOnce({
        data: [
          {
            id: 'rf_1',
            providerRefundId: 'stripe_rf_1',
            paymentIntentId: 'pi_recover_1',
            topupOrderId: 'topup_order_1',
            reason: 'duplicate charge',
            amount: 12,
            status: 'processed',
            createdAt: '2026-06-04T00:00:00Z',
          },
        ],
        total: 1,
      });

    render(<AdminBillingPage />);

    fireEvent.click(await screen.findByRole('tab', { name: 'Refunds' }));

    expect(await screen.findByText('duplicate charge')).toBeInTheDocument();
    expect(screen.getByText('pi_recover_1')).toBeInTheDocument();
    expect(screen.getByText('topup_order_1')).toBeInTheDocument();
  });

  it('marks payout pending rows as paid from the payouts surface', async () => {
    getBillingSummary.mockResolvedValue({});
    markMarketplacePayoutPaid.mockResolvedValue({ id: 'payout_1', status: 'paid_out', providerPayoutId: 'provider-paid-1' });
    listBillingSurface
      .mockResolvedValueOnce({ data: [], total: 0 })
      .mockResolvedValueOnce({
        data: [
          {
            id: 'payout_1',
            provider: 'local',
            providerPayoutId: 'manual-batch-1',
            amount: 40,
            status: 'payout_pending',
            createdAt: '2026-06-04T00:00:00Z',
          },
        ],
        total: 1,
      })
      .mockResolvedValueOnce({
        data: [
          {
            id: 'payout_1',
            provider: 'local',
            providerPayoutId: 'provider-paid-1',
            amount: 40,
            status: 'paid_out',
            createdAt: '2026-06-04T00:00:00Z',
          },
        ],
        total: 1,
      });

    render(<AdminBillingPage />);

    fireEvent.click(await screen.findByRole('tab', { name: 'Payouts' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Mark payout payout_1 paid' }));
    expect(await screen.findByRole('heading', { name: 'Payout confirmation' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Provider payout ID'), { target: { value: 'provider-paid-operator-1' } });
    fireEvent.click(screen.getByRole('button', { name: 'Confirm paid payout' }));

    await waitFor(() => expect(markMarketplacePayoutPaid).toHaveBeenCalledWith('payout_1', 'provider-paid-operator-1'));
    expect(await screen.findByText('Paid Out')).toBeInTheDocument();
  });

  it('records a provider-confirmed top-up refund from the top-ups surface', async () => {
    getBillingSummary.mockResolvedValue({});
    refundTopup.mockResolvedValue({
      id: 'refund_1',
      providerRefundId: 'admin-topup_1',
      topupOrderId: 'topup_1',
      amount: 15,
      status: 'succeeded',
    });
    listBillingSurface
      .mockResolvedValueOnce({ data: [], total: 0 })
      .mockResolvedValueOnce({
        data: [
          {
            id: 'topup_1',
            amount: 25,
            money: 25,
            refundedAmount: 10,
            status: 'paid',
            paymentIntentId: 'pi_topup_1',
            providerPaymentIntentId: 'pi_provider_topup_1',
            currency: 'usd',
            createdAt: '2026-06-04T00:00:00Z',
          },
        ],
        total: 1,
      })
      .mockResolvedValueOnce({
        data: [
          {
            id: 'topup_1',
            amount: 25,
            money: 25,
            refundedAmount: 25,
            status: 'paid',
            createdAt: '2026-06-04T00:00:00Z',
          },
        ],
        total: 1,
      });

    render(<AdminBillingPage />);

    fireEvent.click(await screen.findByRole('tab', { name: 'Top-ups' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Record refund for top-up topup_1' }));
    expect(await screen.findByRole('heading', { name: 'Top-up refund' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Provider refund ID'), { target: { value: 're_provider_operator_1' } });
    fireEvent.change(screen.getByLabelText('Provider charge ID'), { target: { value: 'ch_provider_operator_1' } });
    fireEvent.change(screen.getByLabelText('Refund amount'), { target: { value: '12.5' } });
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'duplicate provider capture' } });
    fireEvent.click(screen.getByRole('button', { name: 'Confirm top-up refund' }));

    await waitFor(() =>
      expect(refundTopup).toHaveBeenCalledWith('topup_1', {
        provider: 'manual',
        providerRefundID: 're_provider_operator_1',
        providerChargeID: 'ch_provider_operator_1',
        providerPaymentIntentID: 'pi_provider_topup_1',
        amount: 12.5,
        currency: 'usd',
        reason: 'duplicate provider capture',
      })
    );
    await waitFor(() => expect(listBillingSurface).toHaveBeenLastCalledWith('topups', expect.objectContaining({ limit: 50 })));
  });

  it('requires provider evidence before submitting billing operator actions', async () => {
    getBillingSummary.mockResolvedValue({});
    listBillingSurface
      .mockResolvedValueOnce({ data: [], total: 0 })
      .mockResolvedValueOnce({
        data: [
          {
            id: 'topup_1',
            amount: 25,
            money: 25,
            refundedAmount: 10,
            status: 'paid',
            createdAt: '2026-06-04T00:00:00Z',
          },
        ],
        total: 1,
      });

    render(<AdminBillingPage />);

    fireEvent.click(await screen.findByRole('tab', { name: 'Top-ups' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Record refund for top-up topup_1' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Confirm top-up refund' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Provider refund ID is required.');
    expect(refundTopup).not.toHaveBeenCalled();
  });
});
