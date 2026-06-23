import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listAuditLogs = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    listAuditLogs,
  }),
}));

import { AdminAuditLogPage } from './AdminAuditLogPage';

describe('AdminAuditLogPage', () => {
  beforeEach(() => {
    listAuditLogs.mockReset();
  });

  it('renders audit entries with actor, action, and resource values', async () => {
    listAuditLogs.mockResolvedValue({
      data: [
        {
          id: 'audit_1',
          actorID: 'user_1',
          actorEmail: 'admin@example.com',
          action: 'channel.create',
          resourceType: 'channel',
          resourceID: 'ch_1',
          changes: '{"name":"OpenAI"}',
          ipAddress: '127.0.0.1',
          createdAt: '2026-01-01T00:00:00Z',
        },
      ],
      total: 1,
    });

    render(<AdminAuditLogPage />);

    expect(await screen.findByRole('heading', { name: 'Audit Log' })).toBeInTheDocument();
    expect(await screen.findByText('admin@example.com')).toBeInTheDocument();
    expect(screen.getByText('channel.create')).toBeInTheDocument();
    expect(screen.getByText('channel / ch_1')).toBeInTheDocument();
    expect(screen.getByText('name')).toBeInTheDocument();
  });

  it('passes filters to listAuditLogs', async () => {
    listAuditLogs.mockResolvedValue({ data: [], total: 0 });

    render(<AdminAuditLogPage />);

    fireEvent.change(await screen.findByLabelText('Organization ID filter'), { target: { value: 'org_audit' } });
    fireEvent.change(await screen.findByLabelText('Action filter'), { target: { value: 'agent.approve' } });
    fireEvent.change(screen.getByLabelText('Resource type filter'), { target: { value: 'agent' } });

    await waitFor(() =>
      expect(listAuditLogs).toHaveBeenCalledWith(expect.objectContaining({ organizationID: 'org_audit', action: 'agent.approve', resourceType: 'agent' }))
    );
  });

  it('renders empty state when there are no entries', async () => {
    listAuditLogs.mockResolvedValue({ data: [], total: 0 });

    render(<AdminAuditLogPage />);

    expect(await screen.findByText('No audit entries found -- Administrative actions will appear here.')).toBeInTheDocument();
  });
});
