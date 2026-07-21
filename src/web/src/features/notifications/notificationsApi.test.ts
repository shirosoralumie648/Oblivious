import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createNotificationsApi } from './notificationsApi';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    delete: overrides.delete
      ? ((path, init) => init === undefined ? overrides.delete!(path) : overrides.delete!(path, init)) as HttpClient['delete']
      : vi.fn(),
    get: overrides.get
      ? ((path, init) => init === undefined ? overrides.get!(path) : overrides.get!(path, init)) as HttpClient['get']
      : vi.fn(),
    post: overrides.post
      ? ((path, body, init) => init === undefined
          ? body === undefined ? overrides.post!(path) : overrides.post!(path, body)
          : overrides.post!(path, body, init)) as HttpClient['post']
      : vi.fn(),
    put: overrides.put
      ? ((path, body, init) => init === undefined
          ? body === undefined ? overrides.put!(path) : overrides.put!(path, body)
          : overrides.put!(path, body, init)) as HttpClient['put']
      : vi.fn(),
    request: overrides.request
      ? ((path, init) => init === undefined ? overrides.request!(path) : overrides.request!(path, init)) as HttpClient['request']
      : vi.fn(),
  };
  return client;
}

describe('createNotificationsApi', () => {
  it('lists notifications, reads unread count, and marks notifications read', async () => {
    const notifications = [
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
    ];
    const getCalls: string[] = [];
    const get: HttpClient['get'] = async <T,>(path: string): Promise<T> => {
      getCalls.push(path);
      return (path === '/api/v1/app/notifications/unread-count' ? { count: 1 } : notifications) as T;
    };
    const deleteRequest = vi.fn().mockResolvedValue({ status: 'deleted' });
    const post = vi.fn().mockResolvedValue({ status: 'ok' });
    const request = vi.fn().mockResolvedValue({ status: 'ok' });
    const api = createNotificationsApi(createClient({ delete: deleteRequest, get, post, request }));

    await expect(api.listNotifications({ limit: 10, offset: 20, unreadOnly: true })).resolves.toEqual([
      expect.objectContaining({ id: 'notif_1', type: 'critical', isRead: false })
    ]);
    await expect(api.getUnreadCount()).resolves.toEqual({ count: 1 });
    await expect(api.markAllRead()).resolves.toEqual({ status: 'ok' });
    await expect(api.markRead('notif_1')).resolves.toEqual({ status: 'ok' });
    await expect(api.deleteNotification('notif_1')).resolves.toEqual({ status: 'deleted' });

    expect(getCalls).toContain('/api/v1/app/notifications?unread=true&limit=10&offset=20');
    expect(getCalls).toContain('/api/v1/app/notifications/unread-count');
    expect(deleteRequest).toHaveBeenCalledWith('/api/v1/app/notifications/notif_1');
    expect(post).toHaveBeenCalledWith('/api/v1/app/notifications/mark-all-read');
    expect(request).toHaveBeenCalledWith('/api/v1/app/notifications/notif_1', { method: 'PATCH' });
  });
});
