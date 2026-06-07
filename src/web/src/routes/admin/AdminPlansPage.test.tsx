import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listPlans = vi.fn();
const createPlan = vi.fn();
const updatePlan = vi.fn();
const deactivatePlan = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    listPlans,
    createPlan,
    updatePlan,
    deactivatePlan,
  }),
}));

import { AdminPlansPage } from './AdminPlansPage';

describe('AdminPlansPage', () => {
  beforeEach(() => {
    listPlans.mockReset();
    createPlan.mockReset();
    updatePlan.mockReset();
    deactivatePlan.mockReset();
  });

  it('renders plans with pricing, quota, and visibility status', async () => {
    listPlans.mockResolvedValue([
      {
        id: 'plan_pro',
        name: 'Pro',
        description: 'Production plan',
        quotaAmount: 500,
        tokenQuota: 1000000,
        price: 29,
        modelAccess: ['gpt-4o', 'claude-3.5'],
        agentLimit: 10,
        maxTokensPerRequest: 32000,
        durationDays: 30,
        isActive: true,
        isPublic: true,
        sortOrder: 1,
        subscriberCount: 42,
        createdAt: '2026-01-01T00:00:00Z',
      },
    ]);

    render(<AdminPlansPage />);

    expect(await screen.findByRole('heading', { name: 'Plans' })).toBeInTheDocument();
    expect(await screen.findByText('Pro')).toBeInTheDocument();
    expect(screen.getByText('$29.00')).toBeInTheDocument();
    expect(screen.getByText('500 credits / 1,000,000 tokens')).toBeInTheDocument();
    expect(screen.getByText('32,000 tokens')).toBeInTheDocument();
    expect(screen.getByLabelText('Public')).toBeInTheDocument();
  });

  it('creates plans with a request token cap', async () => {
    listPlans.mockResolvedValue([]);
    createPlan.mockResolvedValue({
      id: 'plan_pro',
      name: 'Pro',
      description: 'Production plan',
      quotaAmount: 500,
      tokenQuota: 1000000,
      price: 29,
      modelAccess: ['gpt-4o'],
      agentLimit: 10,
      maxTokensPerRequest: 32000,
      isActive: true,
      isPublic: true,
      sortOrder: 1,
      createdAt: '2026-01-01T00:00:00Z',
    });

    render(<AdminPlansPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Add Plan' }));

    expect(await screen.findByRole('heading', { name: 'Add Plan' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Pro' } });
    fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'Production plan' } });
    fireEvent.change(screen.getByLabelText('Price'), { target: { value: '29' } });
    fireEvent.change(screen.getByLabelText('Quota Amount'), { target: { value: '500' } });
    fireEvent.change(screen.getByLabelText('Token Quota'), { target: { value: '1000000' } });
    fireEvent.change(screen.getByLabelText('Agent Limit'), { target: { value: '10' } });
    fireEvent.change(screen.getByLabelText('Request Token Cap'), { target: { value: '32000' } });
    fireEvent.change(screen.getByLabelText('Model Access'), { target: { value: 'gpt-4o' } });
    fireEvent.change(screen.getByLabelText('Sort Order'), { target: { value: '1' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create Plan' }));

    await waitFor(() => expect(createPlan).toHaveBeenCalledWith({
      name: 'Pro',
      description: 'Production plan',
      quotaAmount: 500,
      tokenQuota: 1000000,
      price: 29,
      modelAccess: ['gpt-4o'],
      agentLimit: 10,
      maxTokensPerRequest: 32000,
      durationDays: null,
      isPublic: true,
      sortOrder: 1,
    }));
  });

  it('confirms plan deactivation', async () => {
    listPlans.mockResolvedValue([
      {
        id: 'plan_pro',
        name: 'Pro',
        description: 'Production plan',
        quotaAmount: 500,
        tokenQuota: 1000000,
        price: 29,
        modelAccess: ['gpt-4o'],
        agentLimit: 10,
        maxTokensPerRequest: 32000,
        isActive: true,
        isPublic: true,
        sortOrder: 1,
        createdAt: '2026-01-01T00:00:00Z',
      },
    ]);
    deactivatePlan.mockResolvedValue(undefined);

    render(<AdminPlansPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Deactivate plan Pro' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Deactivate Plan' }));

    await waitFor(() => expect(deactivatePlan).toHaveBeenCalledWith('plan_pro'));
  });
});
