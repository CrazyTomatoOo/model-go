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
import PasswordPolicyList from '../features/auth/PasswordPolicyList';
import GuardLoading from '../router/GuardLoading';
import './ChangePasswordPage.css';

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
  const [form] = Form.useForm();
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  // Success transition: keep a loading surface painted while the shell
  // chunk warms, so no empty frame lands between this page unmounting and
  // the shell mounting (#39 C13).
  const [leaving, setLeaving] = useState(false);

  const onFinish = async (values: ChangePasswordFormValues): Promise<void> => {
    setErrorKey(null);
    setSubmitting(true);
    try {
      await changePassword({ oldPassword: values.oldPassword, newPassword: values.newPassword });
      // The backend cleared must_change_password on success. Warm the shell
      // chunk (same module App.tsx's lazy() resolves) while a loading
      // surface stays painted. Order matters: navigate FIRST, then clear the
      // flag, both in this same microtask so React batches them into one
      // commit — /change-password unmounts outright instead of its
      // ChangePasswordGate re-rendering <Navigate> (which renders null and
      // paints one blank frame, #39 C13).
      setLeaving(true);
      import('./ConsoleShellPage')
        .catch(() => undefined)
        .finally(() => {
          navigate('/', { replace: true });
          setUser((prev) => (prev === null ? prev : { ...prev, must_change_password: false }));
        });
    } catch (error) {
      if (error instanceof ChangePasswordError) {
        // Field-level mapping like the shell modal (#35): server errors[]
        // attach to the offending field instead of a generic form alert.
        if (error.code === 'invalidInitialPassword') {
          form.setFields([
            { name: 'oldPassword', errors: [t('changePassword.errors.invalidInitialPassword')] },
          ]);
        } else if (error.code === 'policyViolation') {
          form.setFields([
            { name: 'newPassword', errors: [t('changePassword.errors.policyViolation')] },
          ]);
        } else {
          setErrorKey(ERROR_KEYS[error.code]);
        }
      } else {
        setErrorKey('changePassword.errors.unknown');
      }
    } finally {
      setSubmitting(false);
    }
  };

  if (leaving) {
    return <GuardLoading />;
  }

  return (
    <div className="change-password-page">
      <Card style={{ maxWidth: 420, width: '100%' }}>
        <Typography.Title level={1}>{t('changePassword.title')}</Typography.Title>
        <Form form={form} layout="vertical" onFinish={onFinish} onValuesChange={() => setErrorKey(null)}>
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
          extra={<PasswordPolicyList />}
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
          <Alert
            type="error"
            showIcon
            message={t(errorKey)}
            closable
            onClose={() => setErrorKey(null)}
            style={{ marginBottom: 16 }}
          />
        )}
        <Button type="primary" htmlType="submit" block loading={submitting}>
          {t('changePassword.submit')}
        </Button>
      </Form>
      </Card>
    </div>
  );
}
