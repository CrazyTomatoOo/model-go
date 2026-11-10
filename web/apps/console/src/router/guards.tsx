import type { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router';
import { useAuth } from '../auth/AuthProvider';
import GuardLoading from './GuardLoading';

/**
 * Renders children only for an authenticated user; logged-out visits go to
 * /login with the attempted location preserved for post-login return.
 */
export function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth();
  const location = useLocation();
  if (loading) {
    return <GuardLoading />;
  }
  if (user === null) {
    return <Navigate to="/login" replace state={{ from: location }} />;
  }
  return <>{children}</>;
}

/**
 * Forced password change: a user with must_change_password=true may only stay
 * on /change-password; every other route bounces there.
 */
export function RequirePasswordChange({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  const location = useLocation();
  if (user !== null && user.must_change_password && location.pathname !== '/change-password') {
    return <Navigate to="/change-password" replace />;
  }
  return <>{children}</>;
}

/** Reverse of RequirePasswordChange: an active user has no business on /change-password. */
export function ChangePasswordGate({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  if (user !== null && !user.must_change_password) {
    return <Navigate to="/" replace />;
  }
  return <>{children}</>;
}

/** Logged-in users never see the login page; they go home (or to forced change). */
export function LoginGate({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth();
  if (loading) {
    return <GuardLoading />;
  }
  if (user !== null) {
    return <Navigate to={user.must_change_password ? '/change-password' : '/'} replace />;
  }
  return <>{children}</>;
}
