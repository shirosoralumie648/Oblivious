import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { routerFuture } from '../../app/routerFuture';

const navigate = vi.fn();
const bootstrapAuth = vi.fn();

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');

  return {
    ...actual,
    useNavigate: () => navigate
  };
});

vi.mock('../../app/providers', () => ({
  useAppContext: () => ({
    bootstrapAuth
  })
}));

import { RegisterPage } from './RegisterPage';

describe('RegisterPage', () => {
  beforeEach(() => {
    navigate.mockReset();
    bootstrapAuth.mockReset();
    bootstrapAuth.mockResolvedValue(undefined);
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        headers: new Headers({ 'Content-Type': 'application/json' }),
        json: async () => ({ ok: true, data: { user: { id: 'u1', email: 'founder@example.com' } }, error: null }),
        ok: true,
        status: 200
      })
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('posts founder credentials to the register route and starts onboarding', async () => {
    render(
      <MemoryRouter future={routerFuture}>
        <RegisterPage />
      </MemoryRouter>
    );

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'founder@example.com' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'StrongerPass1!' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create account' }));

    await waitFor(() => {
      expect(fetch).toHaveBeenCalledWith('/api/v1/auth/register', expect.objectContaining({
        body: JSON.stringify({ email: 'founder@example.com', password: 'StrongerPass1!' }),
        method: 'POST'
      }));
    });

    expect(bootstrapAuth).toHaveBeenCalled();
    expect(navigate).toHaveBeenCalledWith('/onboarding');
  });
});
