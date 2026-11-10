import { ConfigProvider, Drawer, Layout, Menu, Typography } from 'antd';
import type { MenuProps } from 'antd';
import { useEffect, useState, useSyncExternalStore, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router';
import { chromeInkVars, chromeMenuTokens } from './chromeTokens';
import { requestChangePassword } from './changePasswordBus';
import { KeyIcon, MenuIcon, OverviewIcon } from './icons';
import LanguageSwitch from './LanguageSwitch';
import UserMenu from './UserMenu';
import './AppShell.css';

const { Header, Sider, Content } = Layout;

/**
 * Responsive tiers (#36): >=1200 full side menu, 768-1199 icon-collapsed side
 * menu (the antd collapsed-sider pattern), <768 hamburger + drawer with the
 * header actions moved inside. CSS handles presentational tweaks (content
 * padding); JS matchMedia decides structure, so the rendered tree matches
 * the tier deterministically.
 */
type ViewportTier = 'desktop' | 'tablet' | 'mobile';

const MOBILE_QUERY = '(max-width: 767px)';
const TABLET_QUERY = '(max-width: 1199px)';

function useMediaQuery(query: string): boolean {
  return useSyncExternalStore(
    (onChange) => {
      const mql = window.matchMedia(query);
      mql.addEventListener('change', onChange);
      return () => mql.removeEventListener('change', onChange);
    },
    () => window.matchMedia(query).matches,
  );
}

function useViewportTier(): ViewportTier {
  const mobile = useMediaQuery(MOBILE_QUERY);
  const tablet = useMediaQuery(TABLET_QUERY);
  if (mobile) {
    return 'mobile';
  }
  return tablet ? 'tablet' : 'desktop';
}

/**
 * Shared console chrome (#36): brand Header + side menu on every guarded
 * page, fixed in this one place. antd Layout landmarks come for free —
 * Header is a <header> (banner) and Content is a <main>.
 *
 * T3 preserves: UserMenu's mount inside this header/drawer, and the
 * change-password entries routing through requestChangePassword().
 */
export default function AppShell({ children }: { children: ReactNode }) {
  const { t } = useTranslation();
  const tier = useViewportTier();
  const navigate = useNavigate();
  const location = useLocation();
  const [drawerOpen, setDrawerOpen] = useState(false);

  // Leaving the mobile tier hides the drawer; never leave it dangling.
  useEffect(() => {
    if (tier !== 'mobile') {
      setDrawerOpen(false);
    }
  }, [tier]);

  const menuItems: NonNullable<MenuProps['items']> = [
    { key: '/', icon: <OverviewIcon />, label: t('nav.console') },
    { key: 'change-password', icon: <KeyIcon />, label: t('nav.changePassword') },
  ];

  const handleMenuClick: NonNullable<MenuProps['onClick']> = ({ key }) => {
    setDrawerOpen(false);
    if (key === '/') {
      navigate('/');
      return;
    }
    if (key === 'change-password' && !requestChangePassword()) {
      // T3's modal is not integrated yet; degrade to the standalone route.
      navigate('/change-password');
    }
  };

  const selectedKeys = location.pathname === '/' ? ['/'] : [];

  return (
    <ConfigProvider theme={{ components: { Menu: chromeMenuTokens } }}>
      <Layout style={{ minHeight: '100vh' }}>
        {tier !== 'mobile' && (
          <Sider
            className="shell-chrome"
            style={chromeInkVars as React.CSSProperties}
            width={200}
            collapsedWidth={80}
            collapsed={tier === 'tablet'}
            trigger={null}
          >
            <Menu
              mode="inline"
              items={menuItems}
              selectedKeys={selectedKeys}
              onClick={handleMenuClick}
            />
          </Sider>
        )}
        <Layout>
          <Header className="shell-chrome shell-header" style={chromeInkVars as React.CSSProperties}>
            {tier === 'mobile' && (
              <button
                type="button"
                className="shell-icon-button"
                aria-label={t('shell.menu.open')}
                onClick={() => setDrawerOpen(true)}
              >
                <MenuIcon />
              </button>
            )}
            <Typography.Title level={4} className="shell-brand">
              {t('app.name')}
            </Typography.Title>
            {tier !== 'mobile' && (
              <div className="shell-header-actions">
                <LanguageSwitch />
                <UserMenu />
              </div>
            )}
          </Header>
          <Content className="shell-content">{children}</Content>
        </Layout>
        {tier === 'mobile' && (
          <Drawer
            rootClassName="shell-drawer"
            style={chromeInkVars as React.CSSProperties}
            placement="left"
            width={280}
            open={drawerOpen}
            onClose={() => setDrawerOpen(false)}
          >
            <div className="shell-drawer-actions">
              <UserMenu />
              <LanguageSwitch />
            </div>
            <Menu
              mode="inline"
              items={menuItems}
              selectedKeys={selectedKeys}
              onClick={handleMenuClick}
            />
          </Drawer>
        )}
      </Layout>
    </ConfigProvider>
  );
}
