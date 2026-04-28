import i18n from 'i18next';
import HttpBackend from 'i18next-http-backend';
import { initReactI18next } from 'react-i18next';

void i18n
  .use(HttpBackend)
  .use(initReactI18next)
  .init({
    fallbackLng: 'en',
    supportedLngs: ['en', 'de', 'it', 'fr'],
    ns: ['common'],
    defaultNS: 'common',
    keySeparator: false,
    interpolation: { escapeValue: false },
    backend: { loadPath: '/locales/{{lng}}/{{ns}}.json' },
    detection: { order: ['localStorage', 'navigator'] },
  });

export default i18n;
