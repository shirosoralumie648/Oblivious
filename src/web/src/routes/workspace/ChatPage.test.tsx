import { render, screen } from '@testing-library/react';
import { RouterProvider } from 'react-router-dom';
import { describe, expect, it } from 'vitest';

import { createAppRouter } from '../../app/router';

describe('Chat route', () => {
  it('renders workspace shell on /chat', () => {
    const router = createAppRouter(['/chat']);

    render(<RouterProvider router={router} />);

    expect(screen.getByText('Workspace')).toBeInTheDocument();
    expect(screen.getByText('Conversations')).toBeInTheDocument();
    expect(screen.getByText('Capability Panel')).toBeInTheDocument();
    expect(screen.getByText('Chat')).toBeInTheDocument();
  });
});
