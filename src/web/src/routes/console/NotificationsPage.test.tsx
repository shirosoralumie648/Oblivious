import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { routerFuture } from '../../app/routerFuture';

const listNotifications = vi.fn();
const markRead = vi.fn();

vi.mock('../../features/notifications/notificationsApi', () => ({
  createNotificationsApi: () => ({
    listNotifications,
    markRead
  })
}));

import { NotificationsPage } from './NotificationsPage';

describe('NotificationsPage', () => {
  afterEach(() => {
    listNotifications.mockReset();
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
  });
});
