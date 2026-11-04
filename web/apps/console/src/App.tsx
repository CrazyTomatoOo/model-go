import { BrowserRouter, Navigate, Route, Routes } from 'react-router';
import LocaleProvider from './antd/LocaleProvider';
import ChangePasswordPage from './pages/ChangePasswordPage';
import ConsoleShellPage from './pages/ConsoleShellPage';
import LoginPage from './pages/LoginPage';
import {
  ChangePasswordGate,
  LoginGate,
  RequireAuth,
  RequirePasswordChange,
} from './router/guards';

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
  );
}
