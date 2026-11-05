import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type Dispatch,
  type ReactNode,
  type SetStateAction,
} from 'react';
import { setCsrfToken } from '../lib/auth';
import { apiFetch, setOnGlobalError } from '../lib/request';
import type { ConsoleUser } from './types';

type MeResponse = {
  user: ConsoleUser;
  csrf_token: string;
};

type AuthContextValue = {
  user: ConsoleUser | null;
  /** True until the bootstrap GET /auth/me settles; guards wait for this. */
  loading: boolean;
  refresh: () => Promise<void>;
  clearUser: () => void;
  /** Replaces the in-memory user (login response, post-change flag flip). */
  setUser: Dispatch<SetStateAction<ConsoleUser | null>>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<ConsoleUser | null>(null);
  const [loading, setLoading] = useState(true);

  const clearUser = useCallback(() => {
    setUser(null);
    setCsrfToken(null);
  }, []);

  const refresh = useCallback(async () => {
    const res = await apiFetch<MeResponse>('/api/v1/auth/me');
    setUser(res.user);
    setCsrfToken(res.csrf_token);
  }, []);

  useEffect(() => {
    let cancelled = false;
    void refresh()
      .catch(() => {
        if (!cancelled) {
          clearUser();
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [refresh, clearUser]);

  // Global auth-error handling: any request that fails with SESSION_EXPIRED or
  // PASSWORD_CHANGE_REQUIRED bounces the user out of the console. The bootstrap
  // /auth/me call is resolved here (user=null) and by the route guards instead.
  useEffect(() => {
    setOnGlobalError((error, path) => {
      if (path === '/api/v1/auth/me') {
        return;
      }
      if (error.code === 'SESSION_EXPIRED') {
        clearUser();
        window.location.assign('/login');
      } else if (error.code === 'PASSWORD_CHANGE_REQUIRED') {
        window.location.assign('/change-password');
      }
    });
    return () => setOnGlobalError(null);
  }, [clearUser]);

  const value = useMemo(
    () => ({ user, loading, refresh, clearUser, setUser }),
    [user, loading, refresh, clearUser, setUser],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (ctx === null) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return ctx;
}
