import { useEffect } from 'react';
import { useLocation } from 'react-router';
import i18n from '../i18n';

/** i18n copy keys per public route path. Unlisted paths keep the static title. */
const ROUTE_TITLE_KEYS: Record<string, string> = {
  '/login': 'login.title',
  '/change-password': 'changePassword.title',
  '/': 'console.title',
};

/**
 * Keeps document.title in sync with the active route using i18n copy
 * (checklist A8, #33). Route-level mechanism: the effect re-runs on pathname
 * change only. Language-switch-triggered title refresh builds on top of this
 * in T5 (#37) — deliberately out of scope here.
 */
export default function DocumentTitle() {
  const { pathname } = useLocation();
  useEffect(() => {
    const key = ROUTE_TITLE_KEYS[pathname];
    if (key !== undefined) {
      document.title = `${i18n.t(key)} · ${i18n.t('app.name')}`;
    }
  }, [pathname]);
  return null;
}
