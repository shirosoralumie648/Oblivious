import { render, screen } from '@testing-library/react';
import { RouterProvider } from 'react-router-dom';
import { describe, expect, it } from 'vitest';

import { createAppRouter } from '../../app/router';
import { routerFuture } from '../../app/routerFuture';

describe('Route domains', () => {
  it('renders workspace shell on /chat', async () => {
    const router = createAppRouter(['/chat']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByText('Conversations')).toBeInTheDocument();
    expect(screen.getByText('Capability Panel')).toBeInTheDocument();
    expect(screen.getByText('Chat')).toBeInTheDocument();
  });

  it('renders workspace shell on /chat/:conversationId', async () => {
    const router = createAppRouter(['/chat/abc123']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByText('Conversations')).toBeInTheDocument();
    expect(screen.getByText('Capability Panel')).toBeInTheDocument();
    expect(screen.getByText('Chat')).toBeInTheDocument();
  });

  it('renders marketing domain on /', async () => {
    const router = createAppRouter(['/']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Oblivious')).toBeInTheDocument();
    expect(screen.getByText('AI workspace framework')).toBeInTheDocument();
  });

  it('renders console index route on /console', async () => {
    const router = createAppRouter(['/console']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Console')).toBeInTheDocument();
    expect(await screen.findByText('Console Home')).toBeInTheDocument();
  });

  it('renders console child route on /console/models', async () => {
    const router = createAppRouter(['/console/models']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Console')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Models' })).toBeInTheDocument();
  });
});
