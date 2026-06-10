import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { routerFuture } from '../../app/routerFuture';

const listNotifications = vi.fn();
const getUnreadCount = vi.fn();
const deleteNotification = vi.fn();
const markAllRead = vi.fn();
const markRead = vi.fn();

vi.mock('../../features/notifications/notificationsApi', () => ({
  createNotificationsApi: () => ({
    deleteNotification,
    getUnreadCount,
    listNotifications,
    markAllRead,
    markRead
  })
}));

import { NotificationsPage } from './NotificationsPage';

describe('NotificationsPage', () => {
  afterEach(() => {
    listNotifications.mockReset();
    getUnreadCount.mockReset();
    deleteNotification.mockReset();
    markAllRead.mockReset();
    markRead.mockReset();
  });

  it('renders unread notification severity and marks a notification read', async () => {
    listNotifications
      .mockResolvedValueOnce([
        {
          category: 'system',
          createdAt: '2026-06-06T08:00:00Z',
          id: 'notif_critical',
          isRead: false,
          message: 'Database connection failed',
          title: 'Database down',
          type: 'critical',
          userId: 'user_1'
        },
        {
          category: 'billing',
          createdAt: '2026-06-06T07:30:00Z',
          id: 'notif_info',
          isRead: true,
          message: 'Monthly usage summary is ready',
          title: 'Usage report ready',
          type: 'info',
          userId: 'user_1'
        }
      ])
      .mockResolvedValueOnce([
        {
          category: 'system',
          createdAt: '2026-06-06T08:00:00Z',
          id: 'notif_critical',
          isRead: true,
          message: 'Database connection failed',
          readAt: '2026-06-06T08:05:00Z',
          title: 'Database down',
          type: 'critical',
          userId: 'user_1'
        }
      ]);
    getUnreadCount.mockResolvedValueOnce({ count: 1 }).mockResolvedValueOnce({ count: 0 });
    markRead.mockResolvedValue({ id: 'notif_critical', isRead: true });

    render(
      <MemoryRouter future={routerFuture}>
        <NotificationsPage />
      </MemoryRouter>
    );

    expect(await screen.findByRole('heading', { name: 'Notifications' })).toBeInTheDocument();
    expect(await screen.findByText('2 total')).toBeInTheDocument();
    expect(screen.getByText('1 unread')).toBeInTheDocument();
    expect(screen.getByText('Database down')).toBeInTheDocument();
    expect(screen.getByText('critical')).toBeInTheDocument();
    expect(screen.getByText('Unread')).toBeInTheDocument();
    expect(screen.getByText('Usage report ready')).toBeInTheDocument();
    expect(screen.getByText('info')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Mark Database down as read' }));

    expect(await screen.findByText('0 unread')).toBeInTheDocument();
    expect(markRead).toHaveBeenCalledWith('notif_critical');
    expect(listNotifications).toHaveBeenNthCalledWith(1, { limit: 50 });
    expect(listNotifications).toHaveBeenNthCalledWith(2, { limit: 50 });
    expect(getUnreadCount).toHaveBeenCalledTimes(2);
  });

  it('marks all notifications read and refreshes the server unread count', async () => {
    listNotifications
      .mockResolvedValueOnce([
        {
          category: 'system',
          createdAt: '2026-06-06T08:00:00Z',
          id: 'notif_critical',
          isRead: false,
          message: 'Database connection failed',
          title: 'Database down',
          type: 'critical',
          userId: 'user_1'
        }
      ])
      .mockResolvedValueOnce([
        {
          category: 'system',
          createdAt: '2026-06-06T08:00:00Z',
          id: 'notif_critical',
          isRead: true,
          message: 'Database connection failed',
          readAt: '2026-06-06T08:05:00Z',
          title: 'Database down',
          type: 'critical',
          userId: 'user_1'
        }
      ]);
    getUnreadCount.mockResolvedValueOnce({ count: 3 }).mockResolvedValueOnce({ count: 0 });
    markAllRead.mockResolvedValue({ status: 'ok' });

    render(
      <MemoryRouter future={routerFuture}>
        <NotificationsPage />
      </MemoryRouter>
    );

    expect(await screen.findByText('3 unread')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Mark all read' }));

    expect(await screen.findByText('0 unread')).toBeInTheDocument();
    expect(markAllRead).toHaveBeenCalledTimes(1);
    expect(listNotifications).toHaveBeenNthCalledWith(1, { limit: 50 });
    expect(listNotifications).toHaveBeenNthCalledWith(2, { limit: 50 });
    expect(getUnreadCount).toHaveBeenCalledTimes(2);
  });

  it('deletes one notification and refreshes the server unread count', async () => {
    listNotifications
      .mockResolvedValueOnce([
        {
          category: 'system',
          createdAt: '2026-06-06T08:00:00Z',
          id: 'notif_critical',
          isRead: false,
          message: 'Database connection failed',
          title: 'Database down',
          type: 'critical',
          userId: 'user_1'
        },
        {
          category: 'billing',
          createdAt: '2026-06-06T07:30:00Z',
          id: 'notif_info',
          isRead: true,
          message: 'Monthly usage summary is ready',
          title: 'Usage report ready',
          type: 'info',
          userId: 'user_1'
        }
      ])
      .mockResolvedValueOnce([
        {
          category: 'billing',
          createdAt: '2026-06-06T07:30:00Z',
          id: 'notif_info',
          isRead: true,
          message: 'Monthly usage summary is ready',
          title: 'Usage report ready',
          type: 'info',
          userId: 'user_1'
        }
      ]);
    getUnreadCount.mockResolvedValueOnce({ count: 1 }).mockResolvedValueOnce({ count: 0 });
    deleteNotification.mockResolvedValue({ status: 'deleted' });

    render(
      <MemoryRouter future={routerFuture}>
        <NotificationsPage />
      </MemoryRouter>
    );

    expect(await screen.findByText('2 total')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Delete Database down' }));

    expect(await screen.findByText('1 total')).toBeInTheDocument();
    expect(screen.getByText('0 unread')).toBeInTheDocument();
    expect(screen.queryByText('Database down')).not.toBeInTheDocument();
    expect(deleteNotification).toHaveBeenCalledWith('notif_critical');
    expect(listNotifications).toHaveBeenNthCalledWith(1, { limit: 50 });
    expect(listNotifications).toHaveBeenNthCalledWith(2, { limit: 50 });
    expect(getUnreadCount).toHaveBeenCalledTimes(2);
  });
});
