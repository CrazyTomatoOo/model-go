import { Alert, Button, Flex, Form, Input, Modal } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../../auth/AuthProvider';
import { ChangePasswordError, changePassword } from './changePassword';
import { passwordPolicyIssue } from './passwordPolicy';
import useCountdown from '../../lib/useCountdown';
import PasswordPolicyList from './PasswordPolicyList';

type ChangePasswordModalProps = {
  open: boolean;
  onClose: () => void;
};

type ChangePasswordFormValues = {
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
};

/** Fallback countdown when a 429 response carries no Retry-After header. */
const RATE_LIMIT_COUNTDOWN_SECONDS = 15;
/** How long the success state dwells before the modal auto-closes. */
const SUCCESS_CLOSE_DELAY_MS = 1500;

/**
 * Voluntary password change from the shell header (issue #35). Forced
 * first-sign-in changes keep using the /change-password page; this modal is
 * the logged-in, self-service path.
 *
 * Server errors map to fields per the §4.5 contract: a 400 field error on
 * old_password lands on the current-password field, on new_password on the
 * new-password field; 429 RATE_LIMITED starts a submit countdown. Success
 * shows a success state, auto-closes, and keeps the session (the API only
 * kicks the user's *other* sessions).
 */
export default function ChangePasswordModal({ open, onClose }: ChangePasswordModalProps) {
  const { t } = useTranslation();
  const { user, setUser } = useAuth();
  const [form] = Form.useForm<ChangePasswordFormValues>();
  const [submitting, setSubmitting] = useState(false);
  const [succeeded, setSucceeded] = useState(false);
  const [alert, setAlert] = useState<{ key: string; seconds: number | null } | null>(null);

  const openRef = useRef(open);
  openRef.current = open;
  const successTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // An in-flight submit may settle after the user closed the modal; those
  // state transitions are dropped (afterClose resets everything below).
  useEffect(() => {
    return () => {
      if (successTimer.current !== null) {
        clearTimeout(successTimer.current);
      }
    };
  }, []);

  const countdownLeft = useCountdown(alert?.seconds ?? null);
  const countdownActive = alert !== null && alert.seconds !== null && countdownLeft > 0;

  // At zero the rate-limit alert lifts itself so the submit button unlocks.
  useEffect(() => {
    if (alert !== null && alert.seconds !== null && countdownLeft === 0) {
      setAlert(null);
    }
  }, [alert, countdownLeft]);

  const resetState = () => {
    if (successTimer.current !== null) {
      clearTimeout(successTimer.current);
      successTimer.current = null;
    }
    setSubmitting(false);
    setSucceeded(false);
    setAlert(null);
    form.resetFields();
  };

  // Focus lands in the dialog once its open animation settles. Doing this on
  // open-completion (not mount) matters: the header Dropdown restores focus
  // to its trigger while the dialog is still animating in, which would
  // otherwise strand focus outside the dialog and break Esc-to-close.
  const handleAfterOpenChange = (visible: boolean) => {
    if (visible) {
      form.getFieldInstance('currentPassword')?.focus?.();
    }
  };

  const handleCancel = () => {
    if (submitting) {
      return;
    }
    onClose();
  };

  const handleSubmit = async (values: ChangePasswordFormValues): Promise<void> => {
    setSubmitting(true);
    setAlert(null);
    try {
      await changePassword({
        oldPassword: values.currentPassword,
        newPassword: values.newPassword,
      });
      if (!openRef.current) {
        return;
      }
      // The backend cleared must_change_password and kept this session;
      // mirror the flag so any pending gate re-checks see the new state.
      setUser((prev) => (prev === null ? prev : { ...prev, must_change_password: false }));
      setSucceeded(true);
      successTimer.current = setTimeout(() => {
        if (openRef.current) {
          onClose();
        }
      }, SUCCESS_CLOSE_DELAY_MS);
    } catch (error) {
      if (!openRef.current) {
        return;
      }
      if (error instanceof ChangePasswordError) {
        switch (error.code) {
          case 'invalidInitialPassword':
            form.setFields([
              {
                name: 'currentPassword',
                errors: [t('changePassword.modal.errors.wrongCurrent')],
              },
            ]);
            break;
          case 'policyViolation':
            form.setFields([
              { name: 'newPassword', errors: [t('changePassword.modal.errors.newRejected')] },
            ]);
            break;
          case 'rateLimited':
            setAlert({
              key: 'changePassword.modal.errors.rateLimited',
              seconds: error.retryAfterSeconds ?? RATE_LIMIT_COUNTDOWN_SECONDS,
            });
            break;
          case 'network':
            setAlert({ key: 'changePassword.errors.network', seconds: null });
            break;
          default:
            setAlert({ key: 'changePassword.errors.unknown', seconds: null });
        }
      } else {
        setAlert({ key: 'changePassword.errors.unknown', seconds: null });
      }
    } finally {
      if (openRef.current) {
        setSubmitting(false);
      }
    }
  };

  return (
    <Modal
      title={t('changePassword.modal.title')}
      open={open}
      onCancel={handleCancel}
      afterClose={resetState}
      afterOpenChange={handleAfterOpenChange}
      destroyOnHidden
      footer={null}
      width={400}
    >
      {succeeded ? (
        <Alert type="success" showIcon message={t('changePassword.modal.success')} />
      ) : (
        <Form form={form} layout="vertical" onFinish={handleSubmit}>
          <Form.Item
            label={t('changePassword.modal.currentPassword')}
            name="currentPassword"
            rules={[{ required: true, message: t('changePassword.errors.required') }]}
          >
            <Input.Password autoComplete="current-password" autoFocus />
          </Form.Item>
          <Form.Item
            label={t('changePassword.modal.newPassword')}
            name="newPassword"
            extra={<PasswordPolicyList />}
            rules={[
              { required: true, message: t('changePassword.errors.required') },
              {
                validator: (_rule, value) => {
                  const issue = passwordPolicyIssue(String(value ?? ''), user?.username ?? '');
                  if (issue === null) {
                    return Promise.resolve();
                  }
                  const key =
                    issue === 'tooShort'
                      ? 'changePassword.errors.policyViolation'
                      : 'changePassword.modal.errors.containsUsername';
                  return Promise.reject(new Error(t(key)));
                },
              },
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            label={t('changePassword.modal.confirmPassword')}
            name="confirmPassword"
            dependencies={['newPassword']}
            rules={[
              { required: true, message: t('changePassword.errors.required') },
              ({ getFieldValue }) => ({
                validator: (_rule, value) => {
                  if (
                    value === undefined ||
                    value === '' ||
                    value === getFieldValue('newPassword')
                  ) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error(t('changePassword.errors.confirmMismatch')));
                },
              }),
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          {alert !== null && (
            <Form.Item>
              <Alert
                type="error"
                showIcon
                message={t(alert.key, { seconds: countdownLeft })}
                closable={alert.seconds === null}
                onClose={() => setAlert(null)}
              />
            </Form.Item>
          )}
          <Form.Item style={{ marginBottom: 0 }}>
            <Flex justify="flex-end" gap="small">
              <Button onClick={handleCancel} disabled={submitting}>
                {t('changePassword.modal.cancel')}
              </Button>
              <Button
                type="primary"
                htmlType="submit"
                loading={submitting}
                disabled={countdownActive}
              >
                {t('changePassword.modal.submit')}
              </Button>
            </Flex>
          </Form.Item>
        </Form>
      )}
    </Modal>
  );
}
