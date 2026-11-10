import { useEffect } from 'react';
import { useLocation } from 'react-router';
import i18n from '../i18n';

/**
 * i18n copy keys per public route path. The 403 page has its own key; every
 * other unlisted path renders NotFoundPage, so its title follows suit (#34).
 */
const ROUTE_TITLE_KEYS: Record<string, string> = {
  '/login': 'login.title',
  '/change-password': 'changePassword.title',
  '/403': 'forbidden.title',
  '/': 'console.title',
};

/**
 * Keeps document.title in sync with the active route using i18n copy
 * (checklist A8, #33). The effect re-runs on pathname change, and the
 * languageChanged subscription refreshes the title in place when the
 * language toggles — no navigation required (#37).
 */
export default function DocumentTitle() {
  const { pathname } = useLocation();
  useEffect(() => {
    const applyTitle = () => {
      const key = ROUTE_TITLE_KEYS[pathname] ?? 'notFound.title';
      document.title = `${i18n.t(key)} · ${i18n.t('app.name')}`;
    };
    applyTitle();
    i18n.on('languageChanged', applyTitle);
    return () => {
      i18n.off('languageChanged', applyTitle);
    };
  }, [pathname]);
  return null;
}
