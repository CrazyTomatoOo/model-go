import { App as AntdApp, ConfigProvider } from 'antd';
import enUS from 'antd/locale/en_US';
import zhCN from 'antd/locale/zh_CN';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import { useEffect, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { DEFAULT_LOCALE, STORAGE_KEY } from '../i18n/constants';

export default function LocaleProvider({ children }: { children: ReactNode }) {
  const { i18n } = useTranslation();
  const [locale] = useState<string>(() => localStorage.getItem(STORAGE_KEY) ?? DEFAULT_LOCALE);

  useEffect(() => {
    const next = locale === 'zh' ? 'zh' : 'en';
    void i18n.changeLanguage(next);
    dayjs.locale(next === 'zh' ? 'zh-cn' : 'en');
    localStorage.setItem(STORAGE_KEY, next);
  }, [i18n, locale]);

  return (
    <ConfigProvider locale={locale === 'zh' ? zhCN : enUS}>
      <AntdApp>{children}</AntdApp>
    </ConfigProvider>
  );
}
