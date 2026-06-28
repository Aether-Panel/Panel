'use client';

import React, { createContext, useState, useContext, useEffect, useCallback } from 'react';
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

export function TranslationsProvider({ children }: { children: ReactNode }) {
  const [language, setLanguageState] = useState<Locale>('es');

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

  const setLanguage = (lang: Locale) => {
    try {
      localStorage.setItem('aether_panel_language', lang);
    } catch (e) {
      console.error('localStorage is not available');
    }
    setLanguageState(lang);
  };

  const t = useCallback((key: string, options?: { [key: string]: string | number }): string => {
    let translation = getNestedTranslation(language, key);

    // If translation is not found in the current language, fallback to English.
    if (translation === undefined) {
      translation = getNestedTranslation('en', key);
    }

    let result = translation ?? key;

    if (options && typeof result === 'string') {
      Object.keys(options).forEach(k => {
        result = result.replace(new RegExp(`{{${k}}}`, 'g'), String(options[k]));
      });
    }

    // If still not found, return the key itself.
    return result;
  }, [language]);


  return (
    <TranslationsContext.Provider value={{ language, setLanguage, t }}>
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
