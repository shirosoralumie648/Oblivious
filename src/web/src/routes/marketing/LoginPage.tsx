import { FormEvent, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { RiArrowRightLine, RiShieldKeyholeLine } from '@remixicon/react';

import { useAppContext } from '../../app/providers';
import { createHttpClient } from '../../services/http/client';

function errorMessage(error: unknown) {
  return error instanceof Error && error.message ? error.message : 'Unable to sign in. Check the credentials and try again.';
}

export function LoginPage() {
  const navigate = useNavigate();
  const { bootstrapAuth } = useAppContext();
  const client = useMemo(() => createHttpClient(), []);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    setIsSubmitting(true);

    try {
      await client.post('/api/v1/auth/login', { email, password });
      await bootstrapAuth();
      navigate('/chat');
    } catch (caughtError) {
      setError(errorMessage(caughtError));
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <main className="grid min-h-screen bg-[#11100d] text-[#f7f4ea] lg:grid-cols-[0.9fr_1.1fr]" data-gsap-scope="marketing-auth">
      <section className="flex min-h-[320px] flex-col justify-between border-b border-white/10 bg-[#19160f] p-6 lg:border-b-0 lg:border-r lg:p-8" data-gsap-item>
        <Link to="/" className="inline-flex w-fit items-center gap-3 text-sm font-semibold" data-gsap-item>
          <span className="flex size-9 items-center justify-center rounded-lg border border-amber-300/30 bg-amber-300/10 text-amber-200">O</span>
          Oblivious
        </Link>
        <div className="max-w-md space-y-5" data-gsap-item>
          <div className="inline-flex items-center gap-2 rounded-lg border border-cyan-200/20 bg-cyan-200/10 px-3 py-2 text-sm text-cyan-100" data-gsap-item>
            <RiShieldKeyholeLine className="size-4" aria-hidden="true" />
            Session, tenant, and CSRF-aware workspace access
          </div>
          <h1 className="font-heading text-4xl font-semibold leading-tight text-white">Sign in</h1>
          <p className="text-sm leading-6 text-[#c9c0ad]">Enter the commercial workspace to manage Relay usage, Knowledge, SOLO runs, Marketplace work, and Admin evidence.</p>
        </div>
      </section>
      <section className="flex items-center justify-center bg-[#f4f3ee] px-6 py-10 text-[#181611]" data-gsap-item>
        <form onSubmit={handleSubmit} className="w-full max-w-md space-y-5 rounded-lg border border-[#d7d2c4] bg-white p-6 shadow-xl" data-gsap-item>
          <div>
            <h2 className="font-heading text-2xl font-semibold">Workspace login</h2>
            <p className="mt-2 text-sm leading-6 text-[#625b4f]">Uses `POST /api/v1/auth/login` and refreshes the current session before entering Chat.</p>
          </div>
          {error ? (
            <div className="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700" role="alert">
              {error}
            </div>
          ) : null}
          <label className="block space-y-2 text-sm font-medium" htmlFor="login-email">
            <span>Email</span>
            <input
              autoComplete="email"
              className="min-h-[44px] w-full rounded-lg border border-[#cfc8b7] bg-white px-3 text-[#181611] outline-none transition focus:border-cyan-600 focus:ring-2 focus:ring-cyan-600/20"
              id="login-email"
              onChange={(event) => setEmail(event.target.value)}
              required
              type="email"
              value={email}
            />
          </label>
          <label className="block space-y-2 text-sm font-medium" htmlFor="login-password">
            <span>Password</span>
            <input
              autoComplete="current-password"
              className="min-h-[44px] w-full rounded-lg border border-[#cfc8b7] bg-white px-3 text-[#181611] outline-none transition focus:border-cyan-600 focus:ring-2 focus:ring-cyan-600/20"
              id="login-password"
              onChange={(event) => setPassword(event.target.value)}
              required
              type="password"
              value={password}
            />
          </label>
          <button
            className="inline-flex min-h-[44px] w-full items-center justify-center gap-2 rounded-lg bg-[#1a614f] px-4 font-semibold text-white transition hover:bg-[#207a63] disabled:cursor-not-allowed disabled:opacity-60"
            data-gsap-magnetic
            disabled={isSubmitting}
            type="submit"
          >
            Sign in
            <RiArrowRightLine className="size-4" aria-hidden="true" />
          </button>
          <p className="text-center text-sm text-[#625b4f]">
            New workspace?{' '}
            <Link className="font-semibold text-[#1a614f] underline-offset-4 hover:underline" to="/register">
              Create account
            </Link>
          </p>
        </form>
      </section>
    </main>
  );
}
