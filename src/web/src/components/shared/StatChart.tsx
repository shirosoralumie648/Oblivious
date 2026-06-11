import { RiErrorWarningLine, RiRefreshLine } from '@remixicon/react';
import { ResponsiveContainer, BarChart, Bar, XAxis, YAxis, Tooltip, CartesianGrid, AreaChart, Area } from 'recharts';

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
          <ResponsiveContainer width="100%" height={height}>
            {type === 'area' ? (
              <AreaChart data={data}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="label" />
                <YAxis />
                <Tooltip />
                <Area type="monotone" dataKey="value" stroke="hsl(var(--primary))" fill="hsl(var(--primary))" fillOpacity={0.6} />
              </AreaChart>
            ) : (
              <BarChart data={data}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="label" />
                <YAxis />
                <Tooltip />
                <Bar dataKey="value" fill="hsl(var(--primary))" />
              </BarChart>
            )}
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}
