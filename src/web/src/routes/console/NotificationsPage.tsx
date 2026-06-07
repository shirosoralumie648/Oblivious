import { useEffect, useMemo, useState } from 'react';

import { createNotificationsApi, type AppNotification } from '../../features/notifications/notificationsApi';
import { createHttpClient } from '../../services/http/client';

const severityLabels: Record<string, string> = {
  critical: 'critical',
  debug: 'debug',
  error: 'critical',
  info: 'info',
  success: 'info',
  warning: 'warning'
};

function severityClassName(type: string) {
  switch (type) {
    case 'critical':
    case 'error':
      return 'border-[#8e1f1f] bg-[#fff0ed] text-[#7c1717]';
    case 'warning':
      return 'border-[#9a6b14] bg-[#fff7dc] text-[#6d4a0d]';
    case 'debug':
      return 'border-[#6b7280] bg-[#f3f4f6] text-[#374151]';
    default:
      return 'border-[#1a614f] bg-[#e9f2ee] text-[#154c40]';
  }
}

export function NotificationsPage() {
  const notificationsApi = useMemo(() => createNotificationsApi(createHttpClient()), []);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [notifications, setNotifications] = useState<AppNotification[]>([]);
  const [updatingId, setUpdatingId] = useState<string | null>(null);

  const loadNotifications = async () => {
    const items = await notificationsApi.listNotifications({ limit: 50 });
    setNotifications(items);
    setErrorMessage(null);
  };

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      try {
        const items = await notificationsApi.listNotifications({ limit: 50 });
        if (!cancelled) {
          setNotifications(items);
          setErrorMessage(null);
        }
      } catch {
        if (!cancelled) {
          setErrorMessage('Unable to load notifications.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void load();

    return () => {
      cancelled = true;
    };
  }, [notificationsApi]);

  const unreadCount = notifications.filter((notification) => !notification.isRead).length;

  const markNotificationRead = async (notification: AppNotification) => {
    setUpdatingId(notification.id);
    try {
      await notificationsApi.markRead(notification.id);
      await loadNotifications();
    } catch {
      setErrorMessage('Unable to mark notification read.');
    } finally {
      setUpdatingId(null);
    }
  };

  return (
    <section>
      <header className="flex flex-col gap-3 border-b border-[#d7d2c4] pb-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1>Notifications</h1>
          <p className="mt-2 text-sm text-[#5c5548]">Review in-app alerts routed from workspace and system events.</p>
        </div>
        <div className="flex flex-wrap gap-2 text-sm">
          <span className="rounded-lg border border-[#d7d2c4] bg-[#f8f6ef] px-3 py-2">{`${notifications.length} total`}</span>
          <span className="rounded-lg border border-[#d7d2c4] bg-[#f8f6ef] px-3 py-2">{`${unreadCount} unread`}</span>
        </div>
      </header>

      {errorMessage ? <p className="mt-4 text-sm text-[#8e1f1f]">{errorMessage}</p> : null}

      {isLoading ? (
        <p className="mt-4">Loading notifications...</p>
      ) : notifications.length > 0 ? (
        <ul className="mt-5 divide-y divide-[#e5dfd2]">
          {notifications.map((notification) => (
            <li key={notification.id} className="grid gap-3 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-start">
              <div>
                <div className="flex flex-wrap items-center gap-2">
                  <h2 className="text-base font-semibold text-[#181611]">{notification.title}</h2>
                  <span className={`rounded-lg border px-2 py-1 text-xs font-semibold ${severityClassName(notification.type)}`}>
                    {severityLabels[notification.type] ?? notification.type}
                  </span>
                  {!notification.isRead ? (
                    <span className="rounded-lg border border-[#1a614f] bg-white px-2 py-1 text-xs font-semibold text-[#1a614f]">
                      Unread
                    </span>
                  ) : null}
                </div>
                <p className="mt-2 text-sm text-[#4b453b]">{notification.message}</p>
                <p className="mt-2 text-xs uppercase tracking-wide text-[#7a7163]">{notification.category}</p>
              </div>
              {!notification.isRead ? (
                <button
                  className="inline-flex min-h-[40px] items-center justify-center rounded-lg border border-[#1a614f] px-3 text-sm font-semibold text-[#1a614f] transition hover:bg-[#e9f2ee] disabled:opacity-60"
                  disabled={updatingId === notification.id}
                  onClick={() => void markNotificationRead(notification)}
                  type="button"
                >
                  {updatingId === notification.id ? 'Marking read...' : `Mark ${notification.title} as read`}
                </button>
              ) : null}
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-4">No notifications yet.</p>
      )}
    </section>
  );
}
