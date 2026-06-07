import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';

import { routerFuture } from '../../app/routerFuture';
import { AdminSidebar } from './AdminSidebar';

describe('AdminSidebar', () => {
  it('renders grouped admin modules with active route highlighting', () => {
    render(
      <MemoryRouter initialEntries={['/admin/channels']} future={routerFuture}>
        <AdminSidebar />
      </MemoryRouter>
    );

    expect(screen.getByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Channels/ })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: /Models/ })).toHaveAttribute('href', '/admin/models');
    expect(screen.getByRole('link', { name: /Billing/ })).toHaveAttribute('href', '/admin/billing');
    expect(screen.getByRole('link', { name: /API Tokens/ })).toHaveAttribute('href', '/admin/api-tokens');
    expect(screen.getByRole('link', { name: /Usage Logs/ })).toHaveAttribute('href', '/admin/usage-logs');
    expect(screen.getByRole('link', { name: /Review Queue/ })).toHaveAttribute('href', '/admin/reviews');
  });

  it('filters modules by label and keyword and supports collapsed mode', () => {
    render(
      <MemoryRouter initialEntries={['/admin']} future={routerFuture}>
        <AdminSidebar />
      </MemoryRouter>
    );

    fireEvent.change(screen.getByPlaceholderText('Search modules...'), { target: { value: 'payment' } });

    expect(screen.getByRole('link', { name: /Billing/ })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Users/ })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Collapse admin sidebar' }));

    expect(screen.getByRole('button', { name: 'Expand admin sidebar' })).toBeInTheDocument();
    expect(screen.queryByPlaceholderText('Search modules...')).not.toBeInTheDocument();
  });

  it('filters API token management by token and key keywords', () => {
    render(
      <MemoryRouter initialEntries={['/admin']} future={routerFuture}>
        <AdminSidebar />
      </MemoryRouter>
    );

    fireEvent.change(screen.getByPlaceholderText('Search modules...'), { target: { value: 'key' } });

    expect(screen.getByRole('link', { name: /API Tokens/ })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Channels/ })).not.toBeInTheDocument();
  });

  it('filters model inventory by model and provider keywords', () => {
    render(
      <MemoryRouter initialEntries={['/admin']} future={routerFuture}>
        <AdminSidebar />
      </MemoryRouter>
    );

    fireEvent.change(screen.getByPlaceholderText('Search modules...'), { target: { value: 'provider' } });

    expect(screen.getByRole('link', { name: /Models/ })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Channels/ })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Users/ })).not.toBeInTheDocument();
  });
});
