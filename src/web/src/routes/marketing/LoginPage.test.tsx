import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';


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

import { LoginPage } from './LoginPage';

describe('LoginPage', () => {
  beforeEach(() => {
    navigate.mockReset();
    bootstrapAuth.mockReset();
    bootstrapAuth.mockResolvedValue(undefined);
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        headers: new Headers({ 'Content-Type': 'application/json' }),
        json: async () => ({ ok: true, data: { user: { id: 'u1', email: 'admin@example.com' } }, error: null }),
        ok: true,
        status: 200
      })
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('posts credentials to the login route and enters the workspace', async () => {
    render(
      <MemoryRouter >
        <LoginPage />
      </MemoryRouter>
    );

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'admin@example.com' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'StrongerPass1!' } });
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    await waitFor(() => {
      expect(fetch).toHaveBeenCalledWith('/api/v1/auth/login', expect.objectContaining({
        body: JSON.stringify({ email: 'admin@example.com', password: 'StrongerPass1!' }),
        method: 'POST'
      }));
    });

    expect(bootstrapAuth).toHaveBeenCalled();
    expect(navigate).toHaveBeenCalledWith('/chat');
  });
});
