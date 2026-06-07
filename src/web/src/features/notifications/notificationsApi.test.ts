import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createNotificationsApi } from './notificationsApi';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    delete: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    request: vi.fn(),
    ...overrides
  };
  return client;
}

describe('createNotificationsApi', () => {
  it('lists notifications with filters and marks one notification read', async () => {
    const get = vi.fn().mockResolvedValue([
      {
        category: 'system',
        createdAt: '2026-06-06T08:00:00Z',
        id: 'notif_1',
        isRead: false,
        message: 'Database connection failed',
        title: 'Database down',
        type: 'critical',
        userId: 'user_1'
      }
    ]);
    const request = vi.fn().mockResolvedValue({ status: 'ok' });
    const api = createNotificationsApi(createClient({ get, request }));

    await expect(api.listNotifications({ limit: 10, offset: 20, unreadOnly: true })).resolves.toEqual([
      expect.objectContaining({ id: 'notif_1', type: 'critical', isRead: false })
    ]);
    await expect(api.markRead('notif_1')).resolves.toEqual({ status: 'ok' });

    expect(get).toHaveBeenCalledWith('/api/v1/app/notifications?unread=true&limit=10&offset=20');
    expect(request).toHaveBeenCalledWith('/api/v1/app/notifications/notif_1', { method: 'PATCH' });
  });
});
