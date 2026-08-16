'use client';
import { useState, useEffect, createContext, useContext } from 'react';
import type { ReactNode } from 'react';
import { sileo } from "@/lib/toast";
import { TranslationsProvider, useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { useConfig } from '@/contexts/config-context';

export type UserRole = 'admin' | 'user';

interface LoginCredentials {
  email: string;
  password?: string;
  turnstileToken?: string;
}

interface UserData {
  id: number;
  username: string;
  email: string;
  scopes?: string[];
  name?: string;
  avatar?: string;
}

interface RegisterData {
  username: string;
  email: string;
  password?: string;
  turnstileToken?: string;
}

interface AuthContextType {
  role: UserRole | null;
  user: UserData | null;
  scopes: string[];
  login: (credentials: LoginCredentials) => Promise<void>;
  register: (data: RegisterData) => Promise<void>;
  logout: () => void;
  hasScope: (scope: string) => boolean;
  loading: boolean;
  fetchSelf: (forceScopes?: string[]) => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

function AuthProvider({ children }: { children: ReactNode }) {
  const { t } = useTranslations();
  const [role, setRole] = useState<UserRole | null>(null);
  const [user, setUser] = useState<UserData | null>(null);
  const [scopes, setScopes] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  

  const normalizeScopes = (rawScopes: any[]): string[] => {
    if (!rawScopes) return [];
    console.log('Normalizing raw scopes:', rawScopes);
    const normalized = rawScopes.map(s => {
      if (typeof s === 'string') return s;
      if (s && typeof s === 'object') return s.value || s.scope || '';
      return '';
    }).filter(s => s !== '');
    console.log('Normalized scopes:', normalized);
    return normalized;
  };

  const fetchSelf = async (forceScopes?: string[]) => {
    try {
      const data = await api.get('/api/self');
      setUser(data);
      console.log('Fetched self data:', data);

      let currentScopes = forceScopes;
      if (!currentScopes) {
        if (data.scopes && data.scopes.length > 0) {
          currentScopes = normalizeScopes(data.scopes);
          // Always persist the latest scopes from the server
          localStorage.setItem('aether_panel_scopes', JSON.stringify(currentScopes));
        } else {
          // Backend returned no scopes — still try to use stored ones as fallback
          // but this usually means the user has no permissions at all
          const stored = JSON.parse(localStorage.getItem('aether_panel_scopes') || '[]');
          currentScopes = stored;
        }
      } else {
        localStorage.setItem('aether_panel_scopes', JSON.stringify(currentScopes));
      }

      setScopes(currentScopes || []);
      setRole(currentScopes?.includes('admin') ? 'admin' : 'user');
      console.log('Session initialized:', { user: data.username, scopes: currentScopes });
    } catch (e) {
      console.error('Failed to fetch self:', e);
      setRole(null);
      setUser(null);
      localStorage.removeItem('aether_panel_scopes');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSelf();
  }, []);

  const normalizedPath = typeof window !== 'undefined' ? (window.location.pathname.replace(/\/$/, '') || '/') : '';
  const isAuthPage =
    normalizedPath === '/login' ||
    normalizedPath === '/register' ||
    normalizedPath === '/forgot-password' ||
    normalizedPath === '/reset-password';

  useEffect(() => {
    if (loading) return;

    if (!role && !isAuthPage) {
      window.location.href = '/login/';
    }
    if (role && isAuthPage) {
      window.location.href = '/dashboard/';
    }
  }, [role, loading]);

  const login = async (credentials: LoginCredentials) => {
    setLoading(true);
    try {
      console.log('Attempting login for:', credentials.email);
      const data = await api.post('/auth/login', {
        email: credentials.email,
        password: credentials.password,
        turnstileToken: credentials.turnstileToken
      });

      if (data.otpNeeded) {
        sileo.success({
          title: t('auth.otpRequired'),
          description: t('auth.otpDescription'),
        });
        setLoading(false);
        return;
      }

      const scopesList = normalizeScopes(data.scopes || []);
      localStorage.setItem('aether_panel_scopes', JSON.stringify(scopesList));

      // Fetch user info and pass current scopes to avoid race conditions
      await fetchSelf(scopesList);

      sileo.success({
        title: t('auth.loginSuccess'),
        description: t('auth.redirecting'),
      });

      setTimeout(() => {
        window.location.href = '/dashboard/';
      }, 500);
    } catch (e: any) {
      console.error('Login error:', e);
      sileo.error({
        title: t('auth.loginFailed'),
        description: e.message || t('auth.checkCredentials'),
      });
      setLoading(false);
    }
  };

  const register = async (data: RegisterData) => {
    setLoading(true);
    try {
      console.log('Attempting registration for:', data.email);
      const res = await api.post('/auth/register', {
        username: data.username,
        email: data.email,
        password: data.password,
        turnstileToken: data.turnstileToken
      });

      const scopesList = normalizeScopes(res.scopes || []);
      localStorage.setItem('aether_panel_scopes', JSON.stringify(scopesList));

      await fetchSelf(scopesList);

      sileo.success({
        title: t('auth.registerSuccess'),
        description: t('auth.accountCreated'),
      });

      setTimeout(() => {
        window.location.href = '/dashboard/';
      }, 500);
    } catch (e: any) {
      console.error('Registration error:', e);
      sileo.error({
        title: t('auth.registerFailed'),
        description: e.message || t('auth.checkCredentials'),
      });
      setLoading(false);
    }
  };

  const logout = async () => {
    try {
      await api.post('/auth/logout', {});
    } catch (e) {
      console.error('Logout failed:', e);
    } finally {
      localStorage.removeItem('aether_panel_scopes');
      setRole(null);
      setUser(null);
      window.location.href = '/login/';
    }
  };

  const hasScope = (scope: string) => {
    if (role === 'admin') return true;
    if (scopes.includes(scope)) return true;
    if (scope.startsWith('server.') && scope !== 'server.create' && scope !== 'server.admin') {
      if (scopes.includes('server.admin')) return true;
    }
    return false;
  };

  const value = { role, user, scopes, login, register, logout, hasScope, loading, fetchSelf };

  const { config } = useConfig();
  const panelName = config?.branding?.name || "Aether Panel";

  if ((loading && !isAuthPage) || (!role && !isAuthPage)) {
    return (
      <div className="flex h-screen w-full items-center justify-center bg-background">
        <div className="flex flex-col items-center gap-4">
          <div className="h-10 w-10 animate-spin rounded-full border-[3px] border-border border-t-primary border-l-accent" />
          <p className="text-muted-foreground animate-pulse">Initializing {panelName}...</p>
        </div>
      </div>
    );
  }

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}

import { ConfigProvider } from '@/contexts/config-context';

export function Providers({ children }: { children: ReactNode }) {
  return (
    <TranslationsProvider>
      <ConfigProvider>
        <AuthProvider>{children}</AuthProvider>
      </ConfigProvider>
    </TranslationsProvider>
  )
}
