import { Avatar, Dropdown, Modal } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { useAuth } from '../auth/AuthProvider';
import { apiFetch } from '../lib/request';
import { requestChangePassword } from './changePasswordBus';
import { KeyIcon, LogoutIcon } from './icons';
import { getInitials } from './initials';

/**
 * Header user menu (#36): initials avatar (derived from the /auth/me
 * display_name, falling back to username), a change-password entry wired to
 * the T3 bus, and logout behind a confirmation dialog.
 *
 * T3 (change-password modal, parallel ticket) preserves this component's
 * mount point and export: the modal subscribes via useChangePasswordRequest
 * and completes the 修改密码 entry. Until then the entry falls back to the
 * standalone /change-password route.
 */
export default function UserMenu() {
  const { t } = useTranslation();
  const { user, clearUser } = useAuth();
  const navigate = useNavigate();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [loggingOut, setLoggingOut] = useState(false);

  const name = user?.display_name?.trim() || user?.username || '';
  const initials = getInitials(name);

  const handleChangePassword = () => {
    if (!requestChangePassword()) {
      // T3's modal is not integrated yet; degrade to the standalone route.
      navigate('/change-password');
    }
  };

  const handleLogout = async () => {
    setLoggingOut(true);
    try {
      try {
        await apiFetch('/api/v1/auth/logout', { method: 'POST' });
      } catch {
        // Best effort: even when the request fails, the local session is
        // cleared below and the visitor lands on /login (same contract as
        // before #36) — without a floating unhandled rejection.
      } finally {
        clearUser();
        setConfirmOpen(false);
        navigate('/login', { replace: true });
      }
    } finally {
      setLoggingOut(false);
    }
  };

  return (
    <>
      <Dropdown
        trigger={['click']}
        menu={{
          items: [
            {
              key: 'change-password',
              icon: <KeyIcon />,
              label: t('shell.user.changePassword'),
              onClick: handleChangePassword,
            },
            { type: 'divider' },
            {
              key: 'logout',
              icon: <LogoutIcon />,
              label: t('shell.logout'),
              onClick: () => setConfirmOpen(true),
            },
          ],
        }}
      >
        <button type="button" className="shell-user-button">
          <Avatar className="shell-avatar" size={28}>
            {initials}
          </Avatar>
          <span className="shell-user-name">{name}</span>
        </button>
      </Dropdown>
      <Modal
        open={confirmOpen}
        title={t('shell.logoutConfirm.title')}
        okText={t('shell.logoutConfirm.ok')}
        cancelText={t('shell.logoutConfirm.cancel')}
        confirmLoading={loggingOut}
        cancelButtonProps={{ disabled: loggingOut }}
        onOk={() => void handleLogout()}
        onCancel={() => setConfirmOpen(false)}
      >
        {t('shell.logoutConfirm.content')}
      </Modal>
    </>
  );
}
