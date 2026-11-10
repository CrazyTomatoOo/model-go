import { Alert, Button, Form, Input, Typography } from 'antd';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { LoginError, login, type LoginErrorCode } from '../features/auth/login';
import { useAuth } from '../auth/AuthProvider';
import './LoginPage.css';

type LoginFormValues = {
  username: string;
  password: string;
};

/**
 * Client-side cooldown after a 429 RATE_LIMITED. The backend contract
 * (spec 2026-11-02 §4.5) carries no Retry-After, so the login page enforces
 * a fixed retry window of its own and disables resubmission for it.
 */
const RATE_LIMIT_COOLDOWN_SECONDS = 60;

// Spec §1: copy is keyed by the §4.5 code, never the backend message.
const ERROR_KEY_BY_CODE: Record<LoginErrorCode, string> = {
  INVALID_CREDENTIALS: 'errors:INVALID_CREDENTIALS',
  ACCOUNT_PENDING: 'errors:ACCOUNT_PENDING',
  ACCOUNT_DISABLED: 'errors:ACCOUNT_DISABLED',
  ACCOUNT_LOCKED: 'errors:ACCOUNT_LOCKED',
  RATE_LIMITED: 'errors:RATE_LIMITED',
  VALIDATION_ERROR: 'errors:VALIDATION_ERROR',
  NETWORK_ERROR: 'errors:NETWORK_ERROR',
};

/** Codes worth shaking the form for; NETWORK_ERROR is not the user's typo. */
const SHAKEN_CODES: readonly LoginErrorCode[] = [
  'INVALID_CREDENTIALS',
  'ACCOUNT_PENDING',
  'ACCOUNT_DISABLED',
  'ACCOUNT_LOCKED',
  'RATE_LIMITED',
  'VALIDATION_ERROR',
];

export default function LoginPage() {
  const { t } = useTranslation(['common', 'errors']);
  const { setUser } = useAuth();
  const navigate = useNavigate();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<{ code: LoginErrorCode; stamp: number } | null>(null);
  const [cooldownLeft, setCooldownLeft] = useState(0);
  const cooldownActive = cooldownLeft > 0;

  useEffect(() => {
    if (!cooldownActive) {
      return;
    }
    const timer = window.setInterval(() => {
      setCooldownLeft((left) => Math.max(0, left - 1));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [cooldownActive]);

  const handleSubmit = async (values: LoginFormValues) => {
    // Enter key in a field still fires form submit; never hit the API while
    // the rate-limit cooldown is running.
    if (cooldownActive) {
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const res = await login(values.username, values.password);
      // Mount the returned user into AuthProvider so RequireAuth no longer
      // bounces back to /login after a successful sign-in.
      setUser(res.user);
      navigate(res.user.must_change_password ? '/change-password' : '/');
    } catch (err) {
      const code = err instanceof LoginError ? err.code : 'NETWORK_ERROR';
      setError({ code, stamp: Date.now() });
      if (code === 'RATE_LIMITED') {
        setCooldownLeft(RATE_LIMIT_COOLDOWN_SECONDS);
      }
    } finally {
      setSubmitting(false);
    }
  };

  const errorMessage = (code: LoginErrorCode): string =>
    code === 'RATE_LIMITED'
      ? t('login.rateLimited', { seconds: cooldownLeft })
      : t(ERROR_KEY_BY_CODE[code]);

  return (
    <div className="login-page">
      <aside className="login-brand">
        <div className="login-brand-inner">
          <Typography.Title level={2} className="login-brand-title">
            {t('login.welcome')}
          </Typography.Title>
          <Typography.Paragraph className="login-brand-tagline">
            {t('login.subtitle')}
          </Typography.Paragraph>
        </div>
      </aside>
      <main className="login-panel">
        <div className="login-form-wrap">
          <Typography.Title level={3}>{t('login.title')}</Typography.Title>
          <Form layout="vertical" onFinish={handleSubmit}>
            <Form.Item
              label={t('login.username')}
              name="username"
              rules={[{ required: true, message: t('login.usernameRequired') }]}
            >
              <Input autoComplete="username" />
            </Form.Item>
            <Form.Item
              label={t('login.password')}
              name="password"
              rules={[{ required: true, message: t('login.passwordRequired') }]}
            >
              <Input.Password autoComplete="current-password" />
            </Form.Item>
            {error !== null && (
              <div
                key={error.stamp}
                className={SHAKEN_CODES.includes(error.code) ? 'login-error-shake' : undefined}
              >
                <Alert type="error" showIcon message={errorMessage(error.code)} />
              </div>
            )}
            <Button
              type="primary"
              htmlType="submit"
              block
              loading={submitting}
              disabled={cooldownActive}
            >
              {t('login.submit')}
            </Button>
          </Form>
        </div>
        <footer className="login-footer">
          {/* The legal page ships with the website track; the footer reserves
              the slot so the URL lands without a console change. */}
          <a className="login-footer-link" href="#privacy" onClick={(e) => e.preventDefault()}>
            {t('login.footer.privacy')}
          </a>
          <span className="login-footer-divider" aria-hidden="true" />
          <Typography.Text type="secondary" className="login-footer-icp">
            {t('login.footer.icp')}
          </Typography.Text>
        </footer>
      </main>
    </div>
  );
}
