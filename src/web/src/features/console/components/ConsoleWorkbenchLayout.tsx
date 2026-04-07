import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';

import type { AccessSummary } from '../../../types/api';
import { ConsoleContextRail } from './ConsoleContextRail';

type ConsoleWorkbenchLayoutProps = {
  accessSummary: AccessSummary | null;
  children: ReactNode;
  description: string;
  errorMessage: string | null;
  siblingLinks: Array<{ label: string; to: string }>;
  title: string;
};

export function ConsoleWorkbenchLayout({
  accessSummary,
  children,
  description,
  errorMessage,
  siblingLinks,
  title
}: ConsoleWorkbenchLayoutProps) {
  return (
    <section>
      <h1>{title}</h1>
      <p>{description}</p>
      <nav aria-label={`${title} sibling navigation`}>
        {siblingLinks.map((link) => (
          <Link key={link.to} to={link.to}>
            {link.label}
          </Link>
        ))}
      </nav>
      <div>
        <ConsoleContextRail accessSummary={accessSummary} />
        <section>{errorMessage ? <p>{errorMessage}</p> : children}</section>
      </div>
    </section>
  );
}
