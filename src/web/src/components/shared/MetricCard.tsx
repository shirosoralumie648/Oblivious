import type { ReactNode } from 'react';
import { RiArrowDownLine, RiArrowUpLine } from '@remixicon/react';

import { Card, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

export type MetricCardProps = {
  label: string;
  value: string | number;
  format?: 'number' | 'currency' | 'percentage' | 'duration';
  trend?: { direction: 'up' | 'down'; value: string };
  loading?: boolean;
  icon?: ReactNode;
  className?: string;
};

// ⚡ Bolt: Extracted Intl.NumberFormat to a module-level constant to prevent recreating it on every render,
// reducing overhead for formatting currency values.
const currencyFormatter = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' });

function formatValue(value: string | number, format: MetricCardProps['format'] = 'number') {
  if (typeof value === 'string') {
    return value;
  }

  switch (format) {
    case 'currency':
      return currencyFormatter.format(value);
    case 'percentage':
      return `${value}%`;
    case 'duration':
      return `${value}ms`;
    case 'number':
    default:
      return value.toLocaleString();
  }
}

export function MetricCard({ label, value, format = 'number', trend, loading = false, icon, className }: MetricCardProps) {
  return (
    <Card className={cn('rounded-lg', className)}>
      <CardContent className="space-y-5">
        {loading ? (
          <>
            <Skeleton className="h-4 w-28" />
            <Skeleton className="h-9 w-36" />
            <Skeleton className="h-4 w-24" />
          </>
        ) : (
          <>
            <div className="flex items-center justify-between gap-3">
              <p className="text-sm text-muted-foreground">{label}</p>
              {icon ? <div className="text-muted-foreground">{icon}</div> : null}
            </div>
            <p className="font-heading text-[28px] font-semibold leading-tight text-foreground">{formatValue(value, format)}</p>
            {trend ? (
              <p className={cn('inline-flex items-center gap-1 text-sm', trend.direction === 'up' ? 'text-[oklch(0.723_0.219_149.579)]' : 'text-destructive')}>
                {trend.direction === 'up' ? <RiArrowUpLine className="size-4" aria-hidden="true" /> : <RiArrowDownLine className="size-4" aria-hidden="true" />}
                {trend.value}
              </p>
            ) : null}
          </>
        )}
      </CardContent>
    </Card>
  );
}
