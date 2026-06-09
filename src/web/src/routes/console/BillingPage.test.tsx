import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { routerFuture } from '../../app/routerFuture';

const getAccess = vi.fn();
const getBilling = vi.fn();
const listInvoices = vi.fn();
const createBillingCheckout = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    createBillingCheckout,
    getAccess,
    getBilling,
    listInvoices
  })
}));

import { BillingPage } from './BillingPage';

describe('BillingPage', () => {
  afterEach(() => {
    getAccess.mockReset();
    getBilling.mockReset();
    listInvoices.mockReset();
    createBillingCheckout.mockReset();
  });

  it('starts a quota top-up checkout from the billing page', async () => {
    getAccess.mockResolvedValue({
      defaultMode: 'solo',
      modelStrategy: 'balanced',
      networkEnabledHint: true,
      onboardingCompleted: true,
      sessionExpiresAt: '2026-04-03T00:00:00Z',
      sessionId: 'session_1',
      userEmail: 'user@example.com',
      userId: 'user_1',
      workspaceId: 'workspace_1'
    });
    getBilling.mockResolvedValue({
      period: '30d',
      requests: 5,
      inputTokens: 120,
      outputTokens: 80,
      estimatedCostUsd: 0.0004,
      balanceUsd: 42.5,
      creditLimitUsd: 100,
      currentSpendUsd: 0.0004,
      paymentProviders: [{ name: 'stripe' }]
    });
    listInvoices.mockResolvedValue([]);
    createBillingCheckout.mockResolvedValue({
      checkoutSessionId: 'cs_topup_1',
      url: 'https://checkout.stripe.test/session/cs_topup_1'
    });

    render(
      <MemoryRouter future={routerFuture}>
        <BillingPage />
      </MemoryRouter>
    );

    fireEvent.change(await screen.findByLabelText('Top-up amount USD'), { target: { value: '25' } });
    fireEvent.click(screen.getByRole('button', { name: 'Start top-up checkout' }));

    await waitFor(() =>
      expect(createBillingCheckout).toHaveBeenCalledWith({
        amount: 25,
        kind: 'topup',
        provider: 'stripe'
      })
    );
    expect(await screen.findByRole('link', { name: 'Continue to checkout' })).toHaveAttribute(
      'href',
      'https://checkout.stripe.test/session/cs_topup_1'
    );
  });

  it('passes the selected domestic provider when starting top-up checkout', async () => {
    getAccess.mockResolvedValue({
      defaultMode: 'solo',
      modelStrategy: 'balanced',
      networkEnabledHint: true,
      onboardingCompleted: true,
      sessionExpiresAt: '2026-04-03T00:00:00Z',
      sessionId: 'session_1',
      userEmail: 'user@example.com',
      userId: 'user_1',
      workspaceId: 'workspace_1'
    });
    getBilling.mockResolvedValue({
      period: '30d',
      requests: 5,
      inputTokens: 120,
      outputTokens: 80,
      estimatedCostUsd: 0.0004,
      balanceUsd: 42.5,
      creditLimitUsd: 100,
      currentSpendUsd: 0.0004,
      paymentProviders: [{ name: 'stripe' }, { name: 'alipay' }]
    });
    listInvoices.mockResolvedValue([]);
    createBillingCheckout.mockResolvedValue({
      checkoutSessionId: 'cs_topup_alipay_1',
      url: 'https://checkout.alipay.test/session/cs_topup_alipay_1'
    });

    render(
      <MemoryRouter future={routerFuture}>
        <BillingPage />
      </MemoryRouter>
    );

    fireEvent.change(await screen.findByLabelText('Payment provider'), { target: { value: 'alipay' } });
    fireEvent.click(screen.getByRole('button', { name: 'Start top-up checkout' }));

    await waitFor(() =>
      expect(createBillingCheckout).toHaveBeenCalledWith({
        amount: 25,
        kind: 'topup',
        provider: 'alipay'
      })
    );
    expect(await screen.findByRole('link', { name: 'Continue Alipay checkout' })).toHaveAttribute(
      'href',
      'https://checkout.alipay.test/session/cs_topup_alipay_1'
    );
  });

  it('only renders configured checkout providers and defaults to the first available provider', async () => {
    getAccess.mockResolvedValue({
      defaultMode: 'solo',
      modelStrategy: 'balanced',
      networkEnabledHint: true,
      onboardingCompleted: true,
      sessionExpiresAt: '2026-04-03T00:00:00Z',
      sessionId: 'session_1',
      userEmail: 'user@example.com',
      userId: 'user_1',
      workspaceId: 'workspace_1'
    });
    getBilling.mockResolvedValue({
      period: '30d',
      requests: 5,
      inputTokens: 120,
      outputTokens: 80,
      estimatedCostUsd: 0.0004,
      balanceUsd: 42.5,
      creditLimitUsd: 100,
      currentSpendUsd: 0.0004,
      paymentProviders: [{ name: 'alipay' }]
    });
    listInvoices.mockResolvedValue([]);
    createBillingCheckout.mockResolvedValue({
      checkoutSessionId: 'cs_topup_alipay_1',
      url: 'https://checkout.alipay.test/session/cs_topup_alipay_1'
    });

    render(
      <MemoryRouter future={routerFuture}>
        <BillingPage />
      </MemoryRouter>
    );

    expect(await screen.findByRole('option', { name: 'Alipay' })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Stripe' })).not.toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'WeChat Pay' })).not.toBeInTheDocument();
    expect(screen.getByLabelText('Payment provider')).toHaveValue('alipay');

    fireEvent.click(screen.getByRole('button', { name: 'Start top-up checkout' }));

    await waitFor(() =>
      expect(createBillingCheckout).toHaveBeenCalledWith({
        amount: 25,
        kind: 'topup',
        provider: 'alipay'
      })
    );
  });

  it('disables top-up checkout when no provider is configured', async () => {
    getAccess.mockResolvedValue({
      defaultMode: 'solo',
      modelStrategy: 'balanced',
      networkEnabledHint: true,
      onboardingCompleted: true,
      sessionExpiresAt: '2026-04-03T00:00:00Z',
      sessionId: 'session_1',
      userEmail: 'user@example.com',
      userId: 'user_1',
      workspaceId: 'workspace_1'
    });
    getBilling.mockResolvedValue({
      period: '30d',
      requests: 5,
      inputTokens: 120,
      outputTokens: 80,
      estimatedCostUsd: 0.0004,
      balanceUsd: 42.5,
      creditLimitUsd: 100,
      currentSpendUsd: 0.0004,
      paymentProviders: []
    });
    listInvoices.mockResolvedValue([]);

    render(
      <MemoryRouter future={routerFuture}>
        <BillingPage />
      </MemoryRouter>
    );

    await screen.findByLabelText('Payment provider');
    expect(screen.getByLabelText('Payment provider')).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Start top-up checkout' })).toBeDisabled();
    expect(screen.queryByRole('option', { name: 'Stripe' })).not.toBeInTheDocument();
  });

  it('renders billing inside a workbench layout with context rail and sibling links', async () => {
    getAccess.mockResolvedValue({
      defaultMode: 'solo',
      modelStrategy: 'balanced',
      networkEnabledHint: true,
      onboardingCompleted: true,
      sessionExpiresAt: '2026-04-03T00:00:00Z',
      sessionId: 'session_1',
      userEmail: 'user@example.com',
      userId: 'user_1',
      workspaceId: 'workspace_1'
    });
    getBilling.mockResolvedValue({
      period: '30d',
      requests: 5,
      inputTokens: 120,
      outputTokens: 80,
      estimatedCostUsd: 0.0004,
      balanceUsd: 42.5,
      creditLimitUsd: 100,
      currentSpendUsd: 0.0004,
      paymentProviders: [{ name: 'stripe' }],
      nextInvoice: {
        id: 'draft-2026-06',
        status: 'draft',
        amountUsd: 0.0004,
        dueAt: '2026-06-30T00:00:00Z'
      }
    });
    listInvoices.mockResolvedValue([
      {
        id: 'inv_paid_1',
        status: 'paid',
        amountUsd: 29,
        dueAt: '2026-05-31T00:00:00Z'
      },
      {
        id: 'draft-2026-06',
        status: 'draft',
        amountUsd: 0.0004,
        dueAt: '2026-06-30T00:00:00Z'
      }
    ]);

    render(
      <MemoryRouter future={routerFuture}>
        <BillingPage />
      </MemoryRouter>
    );

    expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
    expect(await screen.findByText('Workspace: workspace_1')).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Back to overview' })).toHaveAttribute('href', '/console');
    expect(await screen.findByRole('link', { name: 'Open usage' })).toHaveAttribute('href', '/console/usage');
    expect(await screen.findByText('Estimated cost: $0.0004')).toBeInTheDocument();
    expect(await screen.findByText('Balance: $42.50')).toBeInTheDocument();
    expect(await screen.findByText('Credit limit: $100.00')).toBeInTheDocument();
    expect(await screen.findByText('Current spend: $0.0004')).toBeInTheDocument();
    expect(await screen.findByText('Next invoice: draft - $0.0004 - due Jun 30, 2026')).toBeInTheDocument();
    expect(await screen.findByText('Invoice history')).toBeInTheDocument();
    expect(await screen.findByText('inv_paid_1')).toBeInTheDocument();
    expect(await screen.findByText('paid')).toBeInTheDocument();
    expect(await screen.findByText('$29.0000')).toBeInTheDocument();
    expect(await screen.findByText('May 31, 2026')).toBeInTheDocument();
  });
});
