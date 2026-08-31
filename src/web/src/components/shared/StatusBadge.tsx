import React from 'react';

import { cn } from '@/lib/utils';

export type StatusBadgeStatus =
  | 'online'
  | 'degraded'
  | 'offline'
  | 'active'
  | 'disabled'
  | 'pending'
  | 'pending_review'
  | 'appeal_pending'
  | 'needs_changes'
  | 'open'
  | 'resolved'
  | 'dismissed'
  | 'approved'
  | 'rejected'
  | 'public'
  | 'private'
  | 'unlisted';

export type StatusBadgeProps = {
  status: StatusBadgeStatus;
  label?: string;
  showDot?: boolean;
  className?: string;
};

const statusTone: Record<StatusBadgeStatus, string> = {
  online: 'bg-[oklch(0.723_0.219_149.579)]',
  active: 'bg-[oklch(0.723_0.219_149.579)]',
  approved: 'bg-[oklch(0.723_0.219_149.579)]',
  public: 'bg-[oklch(0.723_0.219_149.579)]',
  resolved: 'bg-[oklch(0.723_0.219_149.579)]',
  degraded: 'bg-[oklch(0.795_0.184_86.047)]',
  pending: 'bg-[oklch(0.795_0.184_86.047)]',
  pending_review: 'bg-[oklch(0.795_0.184_86.047)]',
  appeal_pending: 'bg-[oklch(0.795_0.184_86.047)]',
  needs_changes: 'bg-[oklch(0.795_0.184_86.047)]',
  open: 'bg-[oklch(0.795_0.184_86.047)]',
  offline: 'bg-[oklch(0.704_0.191_22.216)]',
  disabled: 'bg-[oklch(0.704_0.191_22.216)]',
  rejected: 'bg-[oklch(0.704_0.191_22.216)]',
  private: 'bg-[oklch(0.704_0.191_22.216)]',
  dismissed: 'bg-[oklch(0.704_0.191_22.216)]',
  unlisted: 'bg-[oklch(0.795_0.184_86.047)]',
};

function statusLabel(status: StatusBadgeStatus) {
  return status
    .split('_')
    .map((part) => `${part.charAt(0).toUpperCase()}${part.slice(1)}`)
    .join(' ');
}

// Optimization: Wrap StatusBadge in React.memo to prevent unnecessary re-renders in large lists/tables
// Measurement: Reduces React commit times when data tables re-render
export const StatusBadge = React.memo(function StatusBadge({ status, label, showDot = true, className }: StatusBadgeProps) {
  const display = label ?? statusLabel(status);

  return (
    <span
      aria-label={display}
      className={cn('inline-flex min-h-6 items-center gap-2 text-sm text-foreground', className)}
    >
      {showDot ? (
        <span
          aria-hidden="true"
          className={cn('size-2 rounded-full transition-colors', statusTone[status], status === 'online' && 'animate-pulse')}
        />
      ) : null}
      <span>{display}</span>
    </span>
  );
});
