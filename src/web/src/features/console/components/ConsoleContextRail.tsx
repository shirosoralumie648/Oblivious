import { Link } from 'react-router-dom';

import type { AccessSummary } from '../../../types/api';

type ConsoleContextRailProps = {
  accessSummary: AccessSummary | null;
};

export function ConsoleContextRail({ accessSummary }: ConsoleContextRailProps) {
  return (
    <aside>
      <h2>Current workspace scope</h2>
      {accessSummary ? (
        <>
          <p>{`Workspace: ${accessSummary.workspaceId}`}</p>
          <p>{`Session: ${accessSummary.sessionId}`}</p>
          <p>{`Default mode: ${accessSummary.defaultMode}`}</p>
          <p>{`Model strategy: ${accessSummary.modelStrategy}`}</p>
        </>
      ) : (
        <p>Access context unavailable.</p>
      )}
      <nav aria-label="Console workbench shortcuts">
        <Link to="/console">Back to overview</Link>
        <Link to="/settings">Workspace settings</Link>
        <Link to="/chat">Return to workspace</Link>
      </nav>
    </aside>
  );
}
