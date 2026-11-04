import { Layout, Menu, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { BrowserRouter, Link, Navigate, Route, Routes } from 'react-router';
import LocaleProvider from './antd/LocaleProvider';
import ChangePasswordPage from './pages/ChangePasswordPage';
import ConsoleShellPage from './pages/ConsoleShellPage';
import LoginPage from './pages/LoginPage';

const { Header, Content } = Layout;

export default function App() {
  return (
    <LocaleProvider>
      <BrowserRouter>
        <Shell />
      </BrowserRouter>
    </LocaleProvider>
  );
}

function Shell() {
  const { t } = useTranslation();

  const items = [
    { key: 'login', label: <Link to="/login">{t('nav.login')}</Link> },
    {
      key: 'change-password',
      label: <Link to="/change-password">{t('nav.changePassword')}</Link>,
    },
    { key: 'console', label: <Link to="/console">{t('nav.console')}</Link> },
  ];

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ display: 'flex', alignItems: 'center', gap: 24 }}>
        <Typography.Title
          level={4}
          style={{ color: '#fff', margin: 0, whiteSpace: 'nowrap' }}
        >
          {t('app.name')}
        </Typography.Title>
        <Menu theme="dark" mode="horizontal" items={items} style={{ flex: 1, minWidth: 0 }} />
      </Header>
      <Content style={{ padding: 24 }}>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/change-password" element={<ChangePasswordPage />} />
          <Route path="/console" element={<ConsoleShellPage />} />
          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
      </Content>
    </Layout>
  );
}
