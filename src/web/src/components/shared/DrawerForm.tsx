import type { ReactNode } from 'react';
import { RiLoader4Line } from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from '@/components/ui/sheet';

export type DrawerFormProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  children: ReactNode;
  onSubmit: () => void;
  submitLabel?: string;
  cancelLabel?: string;
  loading?: boolean;
  error?: string | null;
};

export function DrawerForm({
  open,
  onOpenChange,
  title,
  description,
  children,
  onSubmit,
  submitLabel = 'Save Changes',
  cancelLabel = 'Cancel',
  loading = false,
  error = null,
}: DrawerFormProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full rounded-none sm:max-w-lg">
        <SheetHeader>
          <SheetTitle className="text-xl font-semibold">{title}</SheetTitle>
          <SheetDescription>{description ?? 'Complete the form fields, then submit or cancel.'}</SheetDescription>
        </SheetHeader>
        <ScrollArea className="min-h-0 flex-1 px-6">
          <div className="space-y-4 pb-6">{children}</div>
        </ScrollArea>
        <SheetFooter className="border-t border-border">
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" className="min-h-[44px]" disabled={loading} onClick={() => onOpenChange(false)}>
              {cancelLabel}
            </Button>
            <Button type="button" className="min-h-[44px]" disabled={loading} onClick={onSubmit}>
              {loading ? <RiLoader4Line className="size-4 animate-spin" aria-hidden="true" /> : null}
              {submitLabel}
            </Button>
          </div>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
