import { useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';

import { createAuthApi } from '../../features/auth/api';
import { createHttpClient } from '../../services/http/client';

export function LoginPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [password, setPassword] = useState('');

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    setIsSubmitting(true);

    try {
      const authApi = createAuthApi(createHttpClient());
      const session = await authApi.login({ email, password });

      const redirectPath = typeof location.state === 'object' && location.state && 'from' in location.state
        ? String(location.state.from)
        : session.preferences.onboardingCompleted
          ? '/chat'
          : '/onboarding';

      navigate(redirectPath, { replace: true });
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : 'Login failed');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <main>
      <h1>Login</h1>
      <form onSubmit={handleSubmit}>
        <label>
          Email
          <input onChange={(event) => setEmail(event.target.value)} type="email" value={email} />
        </label>
        <label>
          Password
          <input onChange={(event) => setPassword(event.target.value)} type="password" value={password} />
        </label>
        {error ? <p>{error}</p> : null}
        <button disabled={isSubmitting} type="submit">
          {isSubmitting ? 'Signing in…' : 'Sign in'}
        </button>
      </form>
    </main>
  );
}
