import { Alert, Button, Card, Form, Input, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { useAuth } from '../auth/AuthProvider';
import {
  ChangePasswordError,
  changePassword,
  type ChangePasswordErrorCode,
} from '../features/auth/changePassword';

type ChangePasswordFormValues = {
  oldPassword: string;
  newPassword: string;
  confirmPassword: string;
};

const ERROR_KEYS: Record<ChangePasswordErrorCode, string> = {
  invalidInitialPassword: 'changePassword.errors.invalidInitialPassword',
  policyViolation: 'changePassword.errors.policyViolation',
  rateLimited: 'changePassword.errors.rateLimited',
  network: 'changePassword.errors.network',
  unknown: 'changePassword.errors.unknown',
};

export default function ChangePasswordPage() {
  const { t } = useTranslation();
  const { setUser } = useAuth();
  const navigate = useNavigate();
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const onFinish = async (values: ChangePasswordFormValues): Promise<void> => {
    setErrorKey(null);
    setSubmitting(true);
    try {
      await changePassword({ oldPassword: values.oldPassword, newPassword: values.newPassword });
      // The backend cleared must_change_password on success; mirror it in the
      // context so RequirePasswordChange no longer bounces back.
      setUser((prev) => (prev === null ? prev : { ...prev, must_change_password: false }));
      navigate('/');
    } catch (error) {
      setErrorKey(
        error instanceof ChangePasswordError
          ? ERROR_KEYS[error.code]
          : 'changePassword.errors.unknown',
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Card style={{ maxWidth: 420, margin: '0 auto' }}>
      <Typography.Title level={3}>{t('changePassword.title')}</Typography.Title>
      <Form layout="vertical" onFinish={onFinish}>
        <Form.Item
          label={t('changePassword.oldPassword')}
          name="oldPassword"
          rules={[{ required: true, message: t('changePassword.errors.required') }]}
        >
          <Input.Password autoComplete="current-password" />
        </Form.Item>
        <Form.Item
          label={t('changePassword.newPassword')}
          name="newPassword"
          extra={t('changePassword.policyHint')}
          rules={[{ required: true, message: t('changePassword.errors.required') }]}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Form.Item
          label={t('changePassword.confirmPassword')}
          name="confirmPassword"
          dependencies={['newPassword']}
          rules={[
            { required: true, message: t('changePassword.errors.required') },
            ({ getFieldValue }) => ({
              validator: (_rule, value) => {
                if (value === undefined || value === '' || value === getFieldValue('newPassword')) {
                  return Promise.resolve();
                }
                return Promise.reject(new Error(t('changePassword.errors.confirmMismatch')));
              },
            }),
          ]}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        {errorKey !== null && (
          <Alert type="error" showIcon message={t(errorKey)} style={{ marginBottom: 16 }} />
        )}
        <Button type="primary" htmlType="submit" block loading={submitting}>
          {t('changePassword.submit')}
        </Button>
      </Form>
    </Card>
  );
}
