import { useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { createAuthApi } from '../../features/auth/api';
import { createHttpClient } from '../../services/http/client';

export function RegisterPage() {
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
      const session = await authApi.register({ email, password });
      navigate(session.preferences.onboardingCompleted ? '/chat' : '/onboarding', { replace: true });
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : 'Registration failed');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <main>
      <h1>Register</h1>
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
          {isSubmitting ? 'Creating account…' : 'Create account'}
        </button>
      </form>
    </main>
  );
}
