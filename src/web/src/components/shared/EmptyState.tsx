import type { ReactNode } from 'react';
import { RiInboxLine } from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export type EmptyStateProps = {
  icon?: ReactNode;
  title: string;
  description?: string;
  action?: { label: string; onClick: () => void };
  className?: string;
};

export function EmptyState({ icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div className={cn('flex min-h-[200px] flex-col items-center justify-center gap-4 px-6 py-10 text-center', className)}>
      <div className="flex size-12 items-center justify-center rounded-lg bg-muted/40 text-muted-foreground">
        {icon ?? <RiInboxLine className="size-7" aria-hidden="true" />}
      </div>
      <div className="space-y-2">
        <h3 className="font-heading text-xl font-semibold text-foreground">{title}</h3>
        {description ? <p className="max-w-md text-base text-muted-foreground">{description}</p> : null}
      </div>
      {action ? (
        <Button type="button" className="min-h-[44px]" onClick={action.onClick}>
          {action.label}
        </Button>
      ) : null}
    </div>
  );
}
