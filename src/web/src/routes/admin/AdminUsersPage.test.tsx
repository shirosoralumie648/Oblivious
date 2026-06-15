import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listUsers = vi.fn();
const updateUser = vi.fn();
const updateUserQuota = vi.fn();
const disableUser = vi.fn();
const enableUser = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    listUsers,
    updateUser,
    updateUserQuota,
    disableUser,
    enableUser,
  }),
}));

import { AdminUsersPage } from './AdminUsersPage';

const activeUser = {
  id: 'user_1',
  email: 'admin@example.com',
  name: 'Admin User',
  role: 'admin',
  planID: 'plan_pro',
  planName: 'Pro',
  quotaBalance: 1250.5,
  status: 'active' as const,
  lastLoginAt: '2026-01-01T00:00:00Z',
  createdAt: '2025-01-01T00:00:00Z',
  usageStats: { totalTokens: 1000, totalAPICalls: 20, totalCost: 1.5 },
};

describe('AdminUsersPage', () => {
  beforeEach(() => {
    listUsers.mockReset();
    updateUser.mockReset();
    updateUserQuota.mockReset();
    disableUser.mockReset();
    enableUser.mockReset();
  });

  it('renders users with role, plan, status, and usage', async () => {
    listUsers.mockResolvedValue({ data: [activeUser], total: 1 });

    render(<AdminUsersPage />);

    expect(await screen.findByRole('heading', { name: 'Users' })).toBeInTheDocument();
    expect(await screen.findByText('admin@example.com')).toBeInTheDocument();
    expect(screen.getByText('Pro')).toBeInTheDocument();
    expect(screen.getByText('1,250.5 quota')).toBeInTheDocument();
    expect(screen.getByLabelText('Active')).toBeInTheDocument();
    expect(screen.getByText('1,000 tokens / 20 calls / $1.50')).toBeInTheDocument();
  });

  it('opens edit drawer and submits user updates', async () => {
    listUsers.mockResolvedValue({ data: [activeUser], total: 1 });
    updateUser.mockResolvedValue(activeUser);
    updateUserQuota.mockResolvedValue({ ...activeUser, quotaBalance: 2500 });

    render(<AdminUsersPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Edit user admin@example.com' }));
    expect(await screen.findByRole('heading', { name: 'Edit User: admin@example.com' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Role'), { target: { value: 'user' } });
    fireEvent.change(screen.getByLabelText('Quota Balance'), { target: { value: '2500' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save User' }));

    await waitFor(() => expect(updateUser).toHaveBeenCalledWith('user_1', expect.objectContaining({ role: 'user', planID: 'plan_pro', status: 'active' })));
    await waitFor(() => expect(updateUserQuota).toHaveBeenCalledWith('user_1', { balance: 2500 }));
  });

  it('rejects invalid quota allocation before submitting updates', async () => {
    listUsers.mockResolvedValue({ data: [activeUser], total: 1 });

    render(<AdminUsersPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Edit user admin@example.com' }));
    fireEvent.change(screen.getByLabelText('Quota Balance'), { target: { value: '-1' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save User' }));

    expect(await screen.findByText('Quota balance must be a non-negative number.')).toBeInTheDocument();
    expect(updateUser).not.toHaveBeenCalled();
    expect(updateUserQuota).not.toHaveBeenCalled();
  });

  it('disables and enables users through row actions', async () => {
    listUsers
      .mockResolvedValueOnce({ data: [activeUser], total: 1 })
      .mockResolvedValueOnce({ data: [{ ...activeUser, status: 'disabled' }], total: 1 })
      .mockResolvedValue({ data: [{ ...activeUser, status: 'disabled' }], total: 1 });
    disableUser.mockResolvedValue(undefined);
    enableUser.mockResolvedValue(undefined);

    render(<AdminUsersPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Disable user admin@example.com' }));
    await waitFor(() => expect(disableUser).toHaveBeenCalledWith('user_1'));
  });
});
