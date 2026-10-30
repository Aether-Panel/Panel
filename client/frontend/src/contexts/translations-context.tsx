'use client';

import React, { createContext, useState, useContext, useEffect, useCallback, useMemo } from 'react';
import type { ReactNode } from 'react';
import en from '@/lib/locales/en.json';
import es from '@/lib/locales/es.json';

type Locale = 'en' | 'es';

const translations: Record<Locale, any> = {
  en,
  es,
};

// Helper to get nested values from an object using a dot-separated string.
const getNestedTranslation = (language: Locale, key: string): string | undefined => {
  const langFile = translations[language];
  try {
    return key.split('.').reduce((obj, k) => (obj as any)?.[k], langFile);
  } catch (e) {
    return undefined;
  }
}

interface TranslationsContextType {
  language: Locale;
  setLanguage: (language: Locale) => void;
  t: (key: string, options?: { [key: string]: string | number }) => string;
}

const TranslationsContext = createContext<TranslationsContextType | undefined>(undefined);

export function TranslationsProvider({ children }: Readonly<{ children: ReactNode }>) {
  // English until the user picks a language in their profile (kept in localStorage)
  const [language, setLanguageState] = useState<Locale>('en');

  useEffect(() => {
    let storedLang: Locale | null = null;
    try {
      storedLang = localStorage.getItem('aether_panel_language') as Locale;
    } catch (e) {
      console.error('localStorage is not available');
    }

    if (storedLang && ['en', 'es'].includes(storedLang)) {
      setLanguageState(storedLang);
    }
  }, []);

  // keep <html lang> in step so screen readers and the browser use the right language
  useEffect(() => {
    document.documentElement.lang = language;
  }, [language]);

  const setLanguage = useCallback((lang: Locale) => {
    try {
      localStorage.setItem('aether_panel_language', lang);
    } catch (e) {
      console.error('localStorage is not available');
    }
    setLanguageState(lang);
  }, []);

  const t = useCallback((key: string, options?: { [key: string]: string | number }): string => {
    let translation = getNestedTranslation(language, key);

    // If translation is not found in the current language, fallback to English.
    translation ??= getNestedTranslation('en', key);

    let result = translation ?? key;

    if (options && typeof result === 'string') {
      Object.keys(options).forEach(k => {
        result = result.replace(new RegExp(`{{${k}}}`, 'g'), String(options[k]));
      });
    }

    // If still not found, return the key itself.
    return result;
  }, [language]);

  const value = useMemo(() => ({ language, setLanguage, t }), [language, setLanguage, t]);

  return (
    <TranslationsContext.Provider value={value}>
      {children}
    </TranslationsContext.Provider>
  );
}

export function useTranslations() {
  const context = useContext(TranslationsContext);
  if (context === undefined) {
    throw new Error('useTranslations must be used within a TranslationsProvider');
  }
  return context;
}
