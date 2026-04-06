import { render, screen } from '@testing-library/react';
import { RouterProvider } from 'react-router-dom';
import { describe, expect, it } from 'vitest';

import { createAppRouter } from './router';

describe('app router', () => {
  it('renders home content on /', () => {
    const router = createAppRouter(['/']);

    render(<RouterProvider router={router} />);

    expect(screen.getByText('Oblivious')).toBeInTheDocument();
    expect(screen.getByText('AI workspace framework')).toBeInTheDocument();
  });

  it('renders knowledge route inside the workspace shell', () => {
    const router = createAppRouter(['/knowledge']);

    render(<RouterProvider router={router} />);

    expect(screen.getByText('Workspace')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Knowledge' })).toBeInTheDocument();
  });

  it('renders onboarding inside the workspace shell', () => {
    const router = createAppRouter(['/onboarding']);

    render(<RouterProvider router={router} />);

    expect(screen.getByText('Workspace')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Onboarding' })).toBeInTheDocument();
  });

  it('renders solo route inside the workspace shell', () => {
    const router = createAppRouter(['/solo']);

    render(<RouterProvider router={router} />);

    expect(screen.getByText('Workspace')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'SOLO' })).toBeInTheDocument();
  });
});
