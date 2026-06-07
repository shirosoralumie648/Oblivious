import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listAPITokens = vi.fn();
const revokeAPIToken = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    listAPITokens,
    revokeAPIToken,
  }),
}));

import { AdminAPITokensPage } from './AdminAPITokensPage';

describe('AdminAPITokensPage', () => {
  beforeEach(() => {
    listAPITokens.mockReset();
    revokeAPIToken.mockReset();
  });

  it('renders API token operational fields and revokes a token', async () => {
    listAPITokens.mockResolvedValue({
      data: [
        {
          id: 'tok_1',
          organizationId: 'org_1',
          userId: 'user_1',
          userEmail: 'user@example.com',
          name: 'Production key',
          tokenPrefix: 'sk-oblv',
          status: 'active',
          userGroup: 'vip',
          modelLimitsEnabled: true,
          modelLimits: ['gpt-4o', 'claude-3-5-sonnet'],
          quotaLimit: 50,
          usedQuota: 12.5,
          requestCount: 12,
          totalCost: 1.23,
          lastUsedAt: '2026-06-01T10:00:00Z',
          createdAt: '2026-05-31T10:00:00Z',
        },
      ],
      total: 1,
    });
    revokeAPIToken.mockResolvedValue(undefined);
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);

    render(<AdminAPITokensPage />);

    expect(await screen.findByRole('heading', { name: 'API Tokens' })).toBeInTheDocument();
    expect(await screen.findByText('Production key')).toBeInTheDocument();
    expect(screen.getByText('sk-oblv')).toBeInTheDocument();
    expect(screen.getByText('user@example.com')).toBeInTheDocument();
    expect(screen.getByText('vip')).toBeInTheDocument();
    expect(screen.getByText('gpt-4o, claude-3-5-sonnet')).toBeInTheDocument();
    expect(screen.getByText('$12.5000 / $50.0000')).toBeInTheDocument();
    expect(screen.getByText('12')).toBeInTheDocument();
    expect(screen.getByText('$1.2300')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Revoke Production key' }));

    await waitFor(() => expect(revokeAPIToken).toHaveBeenCalledWith('tok_1'));
    await waitFor(() => expect(listAPITokens).toHaveBeenCalledTimes(2));
    confirmSpy.mockRestore();
  });

  it('passes filters to listAPITokens', async () => {
    listAPITokens.mockResolvedValue({ data: [], total: 0 });

    render(<AdminAPITokensPage />);

    fireEvent.change(await screen.findByLabelText('Organization ID filter'), { target: { value: 'org_1' } });
    fireEvent.change(screen.getByLabelText('User ID filter'), { target: { value: 'user_1' } });
    fireEvent.change(screen.getByLabelText('Status filter'), { target: { value: 'active' } });
    fireEvent.change(screen.getByLabelText('User group filter'), { target: { value: 'vip' } });
    fireEvent.change(screen.getByLabelText('Search tokens'), { target: { value: 'Production' } });
    fireEvent.change(screen.getByLabelText('Model filter'), { target: { value: 'gpt-4o' } });

    await waitFor(() =>
      expect(listAPITokens).toHaveBeenLastCalledWith(
        expect.objectContaining({
          organizationID: 'org_1',
          userID: 'user_1',
          status: 'active',
          userGroup: 'vip',
          search: 'Production',
          model: 'gpt-4o',
          limit: 50,
        })
      )
    );
  });
});
