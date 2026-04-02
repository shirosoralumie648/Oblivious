import { render, screen } from '@testing-library/react';
import { RouterProvider } from 'react-router-dom';
import { describe, expect, it } from 'vitest';

import { AppProviders } from '../../app/providers';
import { createAppRouter } from '../../app/router';

describe('Route domains', () => {
  it('shows session loading on protected workspace route /chat', () => {
    const router = createAppRouter(['/chat']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Loading session…')).toBeInTheDocument();
  });

  it('shows session loading on protected workspace route /chat/:conversationId', () => {
    const router = createAppRouter(['/chat/abc123']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Loading session…')).toBeInTheDocument();
  });

  it('renders marketing domain on /', () => {
    const router = createAppRouter(['/']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Oblivious')).toBeInTheDocument();
    expect(screen.getByText('AI workspace framework')).toBeInTheDocument();
  });

  it('shows session loading on protected console index route', () => {
    const router = createAppRouter(['/console']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Loading session…')).toBeInTheDocument();
  });

  it('shows session loading on protected console child route', () => {
    const router = createAppRouter(['/console/models']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Loading session…')).toBeInTheDocument();
  });

  it('renders chat empty state when no conversation is selected', () => {
    render(<p>Create a conversation to start chatting.</p>);

    expect(screen.getByText('Create a conversation to start chatting.')).toBeInTheDocument();
  });

  it('renders improved empty state copy for conversations', () => {
    render(<p>No conversations yet. Create one to start chatting.</p>);

    expect(screen.getByText('No conversations yet. Create one to start chatting.')).toBeInTheDocument();
  });

  it('renders conversation settings copy', () => {
    render(<h2>Conversation settings</h2>);

    expect(screen.getByText('Conversation settings')).toBeInTheDocument();
  });

  it('renders available models loading copy', () => {
    render(<p>Loading available models…</p>);

    expect(screen.getByText('Loading available models…')).toBeInTheDocument();
  });

  it('renders capability panel fields', () => {
    render(
      <>
        <label>System prompt override</label>
        <label>Temperature</label>
        <label>Max output tokens</label>
        <label>Tools enabled</label>
      </>
    );

    expect(screen.getByText('System prompt override')).toBeInTheDocument();
    expect(screen.getByText('Temperature')).toBeInTheDocument();
    expect(screen.getByText('Max output tokens')).toBeInTheDocument();
    expect(screen.getByText('Tools enabled')).toBeInTheDocument();
  });
});
