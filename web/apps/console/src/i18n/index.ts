import i18n from 'i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { initReactI18next } from 'react-i18next';
import { STORAGE_KEY } from './constants';
import { resources } from './resources';

void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    defaultNS: 'common',
    fallbackLng: 'en',
    supportedLngs: ['en', 'zh'],
    ns: ['common'],
    interpolation: { escapeValue: false },
    detection: {
      order: ['navigator', 'localStorage'],
      caches: ['localStorage'],
      lookupLocalStorage: STORAGE_KEY,
    },
  });

export default i18n;
