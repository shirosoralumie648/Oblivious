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
    expect(screen.getByRole('link', { name: /Review Queue/ })).toHaveAttribute('href', '/admin/reviews');
  });

  it('filters modules by label and keyword and supports collapsed mode', () => {
    render(
      <MemoryRouter initialEntries={['/admin']} future={routerFuture}>
        <AdminSidebar />
      </MemoryRouter>
    );

    fireEvent.change(screen.getByPlaceholderText('Search modules...'), { target: { value: 'pricing' } });

    expect(screen.getByRole('link', { name: /Plans/ })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Users/ })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Collapse admin sidebar' }));

    expect(screen.getByRole('button', { name: 'Expand admin sidebar' })).toBeInTheDocument();
    expect(screen.queryByPlaceholderText('Search modules...')).not.toBeInTheDocument();
  });
});
