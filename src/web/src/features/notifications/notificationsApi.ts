import type { HttpClient } from '../../services/http/client';

export type NotificationSeverity = 'debug' | 'info' | 'warning' | 'critical' | 'error' | 'success';

export type AppNotification = {
  actionUrl?: string;
  category: string;
  createdAt: string;
  id: string;
  isRead: boolean;
  message: string;
  metadata?: Record<string, unknown>;
  readAt?: string;
  title: string;
  type: NotificationSeverity;
  userId: string;
};

export type ListNotificationsParams = {
  limit?: number;
  offset?: number;
  unreadOnly?: boolean;
};

export type NotificationsApi = {
  deleteNotification: (notificationId: string) => Promise<{ status: string }>;
  getUnreadCount: () => Promise<{ count: number }>;
  listNotifications: (params?: ListNotificationsParams) => Promise<AppNotification[]>;
  markAllRead: () => Promise<{ status: string }>;
  markRead: (notificationId: string) => Promise<{ status: string }>;
};

function buildListPath(params: ListNotificationsParams = {}) {
  const searchParams = new URLSearchParams();

  if (params.unreadOnly !== undefined) {
    searchParams.set('unread', String(params.unreadOnly));
  }
  if (params.limit !== undefined) {
    searchParams.set('limit', String(params.limit));
  }
  if (params.offset !== undefined) {
    searchParams.set('offset', String(params.offset));
  }

  const query = searchParams.toString();
  return query ? `/api/v1/app/notifications?${query}` : '/api/v1/app/notifications';
}

export function createNotificationsApi(client: HttpClient): NotificationsApi {
  return {
    deleteNotification: (notificationId) =>
      client.delete<{ status: string }>(`/api/v1/app/notifications/${notificationId}`),
    getUnreadCount: () => client.get<{ count: number }>('/api/v1/app/notifications/unread-count'),
    listNotifications: (params) => client.get<AppNotification[]>(buildListPath(params)),
    markAllRead: () => client.post<{ status: string }>('/api/v1/app/notifications/mark-all-read'),
    markRead: (notificationId) =>
      client.request<{ status: string }>(`/api/v1/app/notifications/${notificationId}`, { method: 'PATCH' })
  };
}
