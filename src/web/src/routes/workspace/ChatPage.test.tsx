import { render, screen } from '@testing-library/react';
import { RouterProvider } from 'react-router-dom';
import { describe, expect, it } from 'vitest';

import { createAppRouter } from '../../app/router';

describe('Route domains', () => {
  it('renders workspace shell on /chat', () => {
    const router = createAppRouter(['/chat']);

    render(<RouterProvider router={router} />);

    expect(screen.getByText('Workspace')).toBeInTheDocument();
    expect(screen.getByText('Conversations')).toBeInTheDocument();
    expect(screen.getByText('Capability Panel')).toBeInTheDocument();
    expect(screen.getByText('Chat')).toBeInTheDocument();
  });

  it('renders workspace shell on /chat/:conversationId', () => {
    const router = createAppRouter(['/chat/abc123']);

    render(<RouterProvider router={router} />);

    expect(screen.getByText('Workspace')).toBeInTheDocument();
    expect(screen.getByText('Conversations')).toBeInTheDocument();
    expect(screen.getByText('Capability Panel')).toBeInTheDocument();
    expect(screen.getByText('Chat')).toBeInTheDocument();
  });

  it('renders marketing domain on /', () => {
    const router = createAppRouter(['/']);

    render(<RouterProvider router={router} />);

    expect(screen.getByText('Oblivious')).toBeInTheDocument();
    expect(screen.getByText('AI workspace framework')).toBeInTheDocument();
  });

  it('renders console index route on /console', () => {
    const router = createAppRouter(['/console']);

    render(<RouterProvider router={router} />);

    expect(screen.getByText('Console')).toBeInTheDocument();
    expect(screen.getByText('Console Home')).toBeInTheDocument();
  });

  it('renders console child route on /console/models', () => {
    const router = createAppRouter(['/console/models']);

    render(<RouterProvider router={router} />);

    expect(screen.getByText('Console')).toBeInTheDocument();
    expect(screen.getByText('Models')).toBeInTheDocument();
  });
});
