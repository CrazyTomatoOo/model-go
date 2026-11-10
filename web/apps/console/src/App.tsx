import { ConfigProvider } from 'antd';
import { Suspense, lazy } from 'react';
import { BrowserRouter, Route, Routes } from 'react-router';
import LocaleProvider from './antd/LocaleProvider';
import LoginPage from './pages/LoginPage';
import NotFoundPage from './pages/NotFoundPage';
import ForbiddenPage from './pages/ForbiddenPage';
import DocumentTitle from './router/DocumentTitle';
import GuardLoading from './router/GuardLoading';
import {
  ChangePasswordGate,
  LoginGate,
  RequireAuth,
  RequirePasswordChange,
} from './router/guards';
import { brandTheme } from './theme/brand';

// 重页面走 dynamic import，避免全量进入首屏 bundle（issue #22）
const ConsoleShellPage = lazy(() => import('./pages/ConsoleShellPage'));
const ChangePasswordPage = lazy(() => import('./pages/ChangePasswordPage'));

export default function App() {
  return (
    <ConfigProvider theme={brandTheme}>
      <LocaleProvider>
        <BrowserRouter>
          <DocumentTitle />
          <AppRoutes />
        </BrowserRouter>
      </LocaleProvider>
    </ConfigProvider>
  );
}

/** Route wiring, router-agnostic so tests can mount it under a MemoryRouter. */
export function AppRoutes() {
  return (
    <Suspense fallback={<GuardLoading />}>
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
      <Route path="/403" element={<ForbiddenPage />} />
      {/* Unknown routes render the 404 page instead of bouncing: deep links
          to future routes get a real explanation, and the guards on `/`
          still send unauthenticated visitors to /login. */}
      <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </Suspense>
  );
}
