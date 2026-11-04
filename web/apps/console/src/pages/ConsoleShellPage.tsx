import { Button, Dropdown, Layout, Segmented, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { useAuth } from '../auth/AuthProvider';
import { STORAGE_KEY } from '../i18n/constants';
import { apiFetch } from '../lib/request';

const { Header, Content } = Layout;

export default function ConsoleShellPage() {
  const { t, i18n } = useTranslation();
  const { user, clearUser } = useAuth();
  const navigate = useNavigate();

  const handleLogout = async () => {
    try {
      await apiFetch('/auth/logout', { method: 'POST' });
    } finally {
      // The user's intent is to leave; complete it even if the call fails.
      clearUser();
      navigate('/login', { replace: true });
    }
  };

  const currentLang = i18n.language === 'zh' ? 'zh' : 'en';

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Typography.Title level={4} style={{ color: '#fff', margin: 0, whiteSpace: 'nowrap' }}>
          {t('app.name')}
        </Typography.Title>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <Segmented
            options={[
              { label: '中', value: 'zh' },
              { label: 'EN', value: 'en' },
            ]}
            value={currentLang}
            onChange={(value) => {
              const next = String(value);
              localStorage.setItem(STORAGE_KEY, next);
              void i18n.changeLanguage(next);
            }}
          />
          <Dropdown
            trigger={['click']}
            menu={{
              items: [
                {
                  key: 'logout',
                  label: t('shell.logout'),
                  onClick: () => void handleLogout(),
                },
              ],
            }}
          >
            <Button type="text" style={{ color: '#fff' }}>
              {user?.username}
            </Button>
          </Dropdown>
        </div>
      </Header>
      <Content style={{ padding: 24 }}>
        <Typography.Paragraph>{t('shell.placeholder')}</Typography.Paragraph>
      </Content>
    </Layout>
  );
}
