import type { ReactNode } from 'react';

type ConsoleSnapshotPanelProps = {
  title: string;
  children: ReactNode;
};

export function ConsoleSnapshotPanel({ title, children }: ConsoleSnapshotPanelProps) {
  return (
    <section aria-label={title} className="rounded-lg border border-[#d7d2c4] bg-white p-5 shadow-sm" data-gsap-item>
      <h3 className="font-heading text-lg font-semibold">{title}</h3>
      <div className="mt-3 grid gap-2 text-sm leading-6 text-[#625b4f]">{children}</div>
    </section>
  );
}
