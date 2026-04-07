import type { ReactNode } from 'react';

type ConsoleSnapshotPanelProps = {
  title: string;
  children: ReactNode;
};

export function ConsoleSnapshotPanel({ title, children }: ConsoleSnapshotPanelProps) {
  return (
    <section aria-label={title}>
      <h3>{title}</h3>
      {children}
    </section>
  );
}
