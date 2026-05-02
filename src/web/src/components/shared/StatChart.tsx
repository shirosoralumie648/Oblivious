import { RiErrorWarningLine, RiRefreshLine } from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

export type StatChartProps = {
  title: string;
  data: { label: string; value: number }[];
  type?: 'bar' | 'area';
  loading?: boolean;
  error?: string | null;
  onRetry?: () => void;
  height?: number;
  className?: string;
};

export function StatChart({ title, data, type = 'bar', loading = false, error = null, onRetry, height = 200, className }: StatChartProps) {
  const max = Math.max(...data.map((item) => item.value), 1);

  return (
    <Card className={cn('rounded-lg', className)}>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="flex items-end gap-3" style={{ height }}>
            {Array.from({ length: 8 }, (_, index) => (
              <Skeleton key={index} className="w-full rounded-t-sm" style={{ height: `${30 + index * 7}%` }} />
            ))}
          </div>
        ) : error ? (
          <div className="flex min-h-[160px] flex-col justify-center gap-3 text-sm">
            <p className="flex items-center gap-2 text-destructive">
              <RiErrorWarningLine className="size-4" aria-hidden="true" />
              {error}
            </p>
            {onRetry ? (
              <Button type="button" variant="outline" className="min-h-[44px] w-fit" onClick={onRetry}>
                <RiRefreshLine className="size-4" aria-hidden="true" />
                Try Again
              </Button>
            ) : null}
          </div>
        ) : (
          <div className="flex items-end gap-3" style={{ height }}>
            {data.map((item) => {
              const percent = Math.max((item.value / max) * 100, 4);
              return (
                <div key={item.label} className="flex h-full min-w-0 flex-1 flex-col justify-end gap-2">
                  <div className="flex flex-1 items-end">
                    <div
                      title={`${item.label}: ${item.value.toLocaleString()}`}
                      className={cn(
                        'w-full rounded-t-sm bg-primary transition-all hover:bg-primary/80',
                        type === 'area' && 'bg-primary/60 shadow-[0_-24px_32px_rgba(59,130,246,0.16)_inset]'
                      )}
                      style={{ height: `${percent}%` }}
                    />
                  </div>
                  <span className="truncate text-center text-xs text-muted-foreground">{item.label}</span>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
