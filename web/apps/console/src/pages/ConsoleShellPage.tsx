import { Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import ChangePasswordModalHost from '../features/auth/ChangePasswordModalHost';
import AppShell from '../shell/AppShell';

/**
 * Console overview route (#36). The chrome (brand header, side menu,
 * responsive drawer, user menu) lives in the shared AppShell; the content
 * area itself is T5's scope — the placeholder copy stays until then.
 */
export default function ConsoleShellPage() {
  const { t } = useTranslation();

  return (
    <AppShell>
      <Typography.Paragraph>{t('shell.placeholder')}</Typography.Paragraph>
      <ChangePasswordModalHost />
    </AppShell>
  );
}
