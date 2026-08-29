import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';

import { HomePage } from './HomePage';

describe('HomePage', () => {
  it('presents commercial product entry points and route map', () => {
    render(
      <MemoryRouter >
        <HomePage />
      </MemoryRouter>
    );

    expect(screen.getByRole('heading', { name: 'Oblivious' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Create account' })).toHaveAttribute('href', '/register');
    expect(screen.getByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/login');
    expect(screen.getByRole('link', { name: 'Open console' })).toHaveAttribute('href', '/console');
    expect(screen.getByText('/chat')).toBeInTheDocument();
    expect(screen.getByText('/knowledge')).toBeInTheDocument();
    expect(screen.getByText('/solo')).toBeInTheDocument();
    expect(screen.getByText('/marketplace')).toBeInTheDocument();
    expect(screen.getByText('/admin/billing')).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="marketing"]')).toBeInTheDocument();
    expect(document.querySelectorAll('[data-gsap-item]').length).toBeGreaterThan(8);
    expect(document.querySelector('[data-gsap-magnetic]')).toBeInTheDocument();
  });
});
