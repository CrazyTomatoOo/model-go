import { Button, Card, Form, Input, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { LoginError, login, type LoginErrorCode } from '../features/auth/login';
import { useAuth } from '../auth/AuthProvider';

type LoginFormValues = {
  username: string;
  password: string;
};

// Spec §1: copy is keyed by the §4.5 code, never the backend message.
const ERROR_KEY_BY_CODE: Record<LoginErrorCode, string> = {
  INVALID_CREDENTIALS: 'errors:INVALID_CREDENTIALS',
  ACCOUNT_PENDING: 'errors:ACCOUNT_PENDING',
  ACCOUNT_DISABLED: 'errors:ACCOUNT_DISABLED',
  ACCOUNT_LOCKED: 'errors:ACCOUNT_LOCKED',
  RATE_LIMITED: 'errors:RATE_LIMITED',
  NETWORK_ERROR: 'errors:NETWORK_ERROR',
};

export default function LoginPage() {
  const { t } = useTranslation(['common', 'errors']);
  const { setUser } = useAuth();
  const navigate = useNavigate();
  const [submitting, setSubmitting] = useState(false);
  const [errorCode, setErrorCode] = useState<LoginErrorCode | null>(null);

  const handleSubmit = async (values: LoginFormValues) => {
    setSubmitting(true);
    setErrorCode(null);
    try {
      const res = await login(values.username, values.password);
      // Mount the returned user into AuthProvider so RequireAuth no longer
      // bounces back to /login after a successful sign-in.
      setUser(res.user);
      navigate(res.user.must_change_password ? '/change-password' : '/');
    } catch (err) {
      setErrorCode(err instanceof LoginError ? err.code : 'NETWORK_ERROR');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Card style={{ maxWidth: 420, margin: '0 auto' }}>
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
        <Button type="primary" htmlType="submit" block loading={submitting}>
          {t('login.submit')}
        </Button>
        {errorCode !== null && (
          <div style={{ textAlign: 'center', marginTop: 12 }}>
            <Typography.Text type="danger">{t(ERROR_KEY_BY_CODE[errorCode])}</Typography.Text>
          </div>
        )}
      </Form>
    </Card>
  );
}
