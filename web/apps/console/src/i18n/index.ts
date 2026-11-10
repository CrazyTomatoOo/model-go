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
    ns: ['common', 'overview'],
    interpolation: { escapeValue: false },
    detection: {
      // The saved choice beats the OS/browser locale on reload (#37).
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
      lookupLocalStorage: STORAGE_KEY,
    },
  });

// <html lang> must follow the resolved language so assistive tech reads the
// page with the right voice (#37).
const syncDocumentLang = (lng: string) => {
  document.documentElement.lang = lng;
};
syncDocumentLang(i18n.resolvedLanguage ?? i18n.language);
i18n.on('languageChanged', syncDocumentLang);

export default i18n;
