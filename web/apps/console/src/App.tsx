import { Suspense, lazy } from 'react';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router';
import LocaleProvider from './antd/LocaleProvider';
import LoginPage from './pages/LoginPage';
import {
  ChangePasswordGate,
  LoginGate,
  RequireAuth,
  RequirePasswordChange,
} from './router/guards';

// 重页面走 dynamic import，避免全量进入首屏 bundle（issue #22）
const ConsoleShellPage = lazy(() => import('./pages/ConsoleShellPage'));
const ChangePasswordPage = lazy(() => import('./pages/ChangePasswordPage'));

export default function App() {
  return (
    <LocaleProvider>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </LocaleProvider>
  );
}

/** Route wiring, router-agnostic so tests can mount it under a MemoryRouter. */
export function AppRoutes() {
  return (
    <Suspense fallback={null}>
      <Routes>
      <Route
        path="/login"
        element={
          <LoginGate>
            <LoginPage />
          </LoginGate>
        }
      />
      <Route
        path="/change-password"
        element={
          <RequireAuth>
            <ChangePasswordGate>
              <ChangePasswordPage />
            </ChangePasswordGate>
          </RequireAuth>
        }
      />
      <Route
        path="/"
        element={
          <RequireAuth>
            <RequirePasswordChange>
              <ConsoleShellPage />
            </RequirePasswordChange>
          </RequireAuth>
        }
      />
      <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    </Suspense>
  );
}
