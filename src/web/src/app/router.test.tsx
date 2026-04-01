import '@testing-library/jest-dom/vitest';
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
});
