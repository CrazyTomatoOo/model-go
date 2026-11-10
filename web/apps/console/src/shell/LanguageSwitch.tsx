import { Segmented } from 'antd';
import { useTranslation } from 'react-i18next';
import { STORAGE_KEY } from '../i18n/constants';

/** zh/en language switch, persisted through the i18n detector's storage. */
export default function LanguageSwitch() {
  const { t, i18n } = useTranslation();
  const currentLang = i18n.language === 'zh' ? 'zh' : 'en';

  return (
    <Segmented
      aria-label={t('shell.languageSwitch')}
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
  );
}
