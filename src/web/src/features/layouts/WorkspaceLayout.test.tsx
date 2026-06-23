import { render, screen, within } from '@testing-library/react';
import { RouterProvider, createMemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';

import { routerFuture } from '../../app/routerFuture';

const appContext = vi.hoisted(() => ({
  authState: {
    preferences: {
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    },
    status: 'authenticated',
    user: { email: 'user@example.com', id: 'u1' }
  },
  authStore: {
    clearUser: vi.fn()
  },
  refreshAuthState: vi.fn()
}));

vi.mock('../../app/providers', () => ({
  useAppContext: () => appContext
}));

import { WorkspaceLayout } from './WorkspaceLayout';

describe('WorkspaceLayout', () => {
  it('renders workspace navigation with a console entry', async () => {
    const router = createMemoryRouter(
      [
        {
          path: '/knowledge/:knowledgeBaseId',
          element: <WorkspaceLayout />,
          children: [{ index: true, element: <h1>Knowledge detail child</h1> }]
        }
      ],
      { future: routerFuture, initialEntries: ['/knowledge/kb_router'] }
    );

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Chat' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'SOLO' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Knowledge' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Agent Memories' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Agents' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'MCP Servers' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Settings' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Workflows' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Scheduled Tasks' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Publishing' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Console' })).toBeInTheDocument();
    expect(await screen.findByText('Knowledge detail child')).toBeInTheDocument();
    const main = await screen.findByRole('main');
    expect(main).toContainElement(screen.getByRole('heading', { name: 'Knowledge detail child' }));
    expect(main).toHaveClass('min-w-0');
    expect(await screen.findByRole('link', { name: 'Knowledge' })).toHaveAttribute('aria-current', 'page');
    expect(await screen.findByRole('link', { name: 'Chat' })).not.toHaveAttribute('aria-current');
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(document.querySelectorAll('[data-gsap-item]').length).toBeGreaterThan(4);
  });

  it('renders workspace navigation in chat-first order', async () => {
    const router = createMemoryRouter(
      [
        {
          path: '/',
          element: <WorkspaceLayout />,
          children: [{ index: true, element: <p>Workspace child</p> }]
        }
      ],
      { future: routerFuture, initialEntries: ['/'] }
    );

    render(<RouterProvider future={routerFuture} router={router} />);

    await screen.findByText('Workspace');

    const links = (await screen.findAllByRole('link')).map((link) => link.textContent);

    expect(links).toEqual([
      'Chat',
      'Knowledge',
      'Agents',
      'MCP Servers',
      'Agent Memories',
      'SOLO',
      'Workflows',
      'Scheduled Tasks',
      'Publishing',
      'Settings',
      'Console',
      'Marketplace'
    ]);
  });

  it('keeps marketplace detail navigation discoverable and active in the workspace landmark', async () => {
    const router = createMemoryRouter(
      [
        {
          path: '/marketplace/agents/:agentId',
          element: <WorkspaceLayout />,
          children: [{ index: true, element: <h1>Marketplace agent child</h1> }]
        }
      ],
      { future: routerFuture, initialEntries: ['/marketplace/agents/agent_1'] }
    );

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    const marketplaceLink = within(workspaceNavigation).getByRole('link', { name: 'Marketplace' });

    expect(marketplaceLink).toHaveAttribute('href', '/marketplace');
    expect(marketplaceLink).toHaveAttribute('aria-current', 'page');
    expect(await screen.findByRole('main')).toContainElement(
      screen.getByRole('heading', { name: 'Marketplace agent child' })
    );
  });
});
