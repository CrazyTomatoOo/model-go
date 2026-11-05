import { ConfigProvider } from 'antd';
import type { Locale } from 'antd/es/locale';
import enUS from 'antd/locale/en_US';
import zhCN from 'antd/locale/zh_CN';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

const locales: Record<string, Locale> = { en: enUS, zh: zhCN };

export default function LocaleProvider({ children }: { children: ReactNode }) {
  const { i18n } = useTranslation();
  const locale = locales[i18n.language] ?? enUS;
  return <ConfigProvider locale={locale}>{children}</ConfigProvider>;
}
