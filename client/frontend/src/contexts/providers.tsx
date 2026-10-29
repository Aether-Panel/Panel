'use client';
import { useState, useEffect, createContext, useContext, useCallback, useMemo } from 'react';
import type { ReactNode } from 'react';
import { sileo } from "@/lib/toast";
import { TranslationsProvider, useTranslations } from '@/contexts/translations-context';
import { api, rememberPageForLogin, RETURN_TO_KEY } from '@/lib/api-client';
import { ConfigProvider, useConfig } from '@/contexts/config-context';

export type UserRole = 'admin' | 'user';

/** How a sign-in step ended: 'otp' means the account needs its 2FA code next. */
export type LoginResult = 'success' | 'otp' | 'expired' | 'limited' | 'failed';

interface LoginCredentials {
  email: string;
  password?: string;
  turnstileToken?: string;
  captchaToken?: string;
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
  captchaToken?: string;
}

interface AuthContextType {
  role: UserRole | null;
  user: UserData | null;
  scopes: string[];
  login: (credentials: LoginCredentials) => Promise<LoginResult>;
  /** Second step of a sign-in on an account with two-factor authentication. */
  verifyOtp: (token: string) => Promise<LoginResult>;
  register: (data: RegisterData) => Promise<void>;
  logout: () => void;
  hasScope: (scope: string) => boolean;
  loading: boolean;
  fetchSelf: (forceScopes?: string[]) => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

const normalizeScopes = (rawScopes: any[]): string[] => {
  if (!rawScopes) return [];
  const normalized = rawScopes.map(s => {
    if (typeof s === 'string') return s;
    if (s && typeof s === 'object') return s.value || s.scope || '';
    return '';
  }).filter(s => s !== '');
  return normalized;
};

function AuthProvider({ children }: Readonly<{ children: ReactNode }>) {
  const { t } = useTranslations();
  const [role, setRole] = useState<UserRole | null>(null);
  const [user, setUser] = useState<UserData | null>(null);
  const [scopes, setScopes] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchSelf = useCallback(async (forceScopes?: string[]) => {
    try {
      const data = await api.get('/api/self');
      setUser(data);

      // Scopes only ever come from the server; no scopes means no permissions
      const currentScopes = forceScopes ?? normalizeScopes(data.scopes || []);

      setScopes(currentScopes);
      setRole(currentScopes.includes('admin') ? 'admin' : 'user');
    } catch (e) {
      console.error('Failed to fetch self:', e);
      setRole(null);
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchSelf();
  }, [fetchSelf]);

  const normalizedPath = typeof window !== 'undefined' ? (window.location.pathname.replace(/\/$/, '') || '/') : '';
  const isAuthPage =
    normalizedPath === '/login' ||
    normalizedPath === '/register' ||
    normalizedPath === '/forgot-password' ||
    normalizedPath === '/reset-password';

  useEffect(() => {
    if (loading) return;

    if (!role && !isAuthPage) {
      // back to this page after signing in
      if (window.location.pathname !== '/') rememberPageForLogin();
      window.location.href = '/login/';
    }
    if (role && isAuthPage) {
      window.location.href = afterLoginPath();
    }
  }, [role, loading]);

  // Shared tail of a successful sign-in, with or without a second step
  const finishLogin = useCallback(async (data: { scopes?: unknown[] }) => {
    const scopesList = normalizeScopes(data.scopes || []);

    // Fetch user info and pass current scopes to avoid race conditions
    await fetchSelf(scopesList);

    sileo.success({
      title: t('auth.loginSuccess'),
      description: t('auth.redirecting'),
    });

    setTimeout(() => {
      window.location.href = afterLoginPath();
    }, 500);
  }, [t, fetchSelf]);

  const login = useCallback(async (credentials: LoginCredentials): Promise<LoginResult> => {
    setLoading(true);
    try {
      const data = await api.post('/auth/login', {
        email: credentials.email,
        password: credentials.password,
        turnstileToken: credentials.turnstileToken,
        captchaToken: credentials.captchaToken,
      });

      if (data.otpNeeded) {
        // the login page asks for the code; the server keeps the attempt for 5 minutes
        setLoading(false);
        return 'otp';
      }

      await finishLogin(data);
      return 'success';
    } catch (e: any) {
      console.error('Login error:', e);
      sileo.error({
        title: t('auth.loginFailed'),
        description: e?.status === 429 ? t('auth.tooManyAttempts') : (e.message || t('auth.checkCredentials')),
      });
      setLoading(false);
      return 'failed';
    }
  }, [t, finishLogin]);

  const verifyOtp = useCallback(async (token: string): Promise<LoginResult> => {
    setLoading(true);
    try {
      const data = await api.post('/auth/otp', { token });
      await finishLogin(data);
      return 'success';
    } catch (e: any) {
      const code = e?.data?.error?.code;
      setLoading(false);
      if (code === 'ErrSessionExpired' || code === 'ErrInvalidSession') {
        sileo.error({ title: t('auth.otp.expiredTitle'), description: t('auth.otp.expiredDescription') });
        return 'expired';
      }
      // a wrong code is shown under the boxes; only throttling needs a toast
      if (e?.status === 429) {
        sileo.error({ title: t('auth.loginFailed'), description: t('auth.tooManyAttempts') });
        return 'limited';
      }
      return 'failed';
    }
  }, [t, finishLogin]);

  const register = useCallback(async (data: RegisterData) => {
    setLoading(true);
    try {
      const res = await api.post('/auth/register', {
        username: data.username,
        email: data.email,
        password: data.password,
        turnstileToken: data.turnstileToken,
        captchaToken: data.captchaToken,
      });

      const scopesList = normalizeScopes(res.scopes || []);

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
  }, [t, fetchSelf]);

  const logout = useCallback(async () => {
    try {
      await api.post('/auth/logout', {});
    } catch (e) {
      console.error('Logout failed:', e);
    } finally {
      setRole(null);
      setUser(null);
      window.location.href = '/login/';
    }
  }, []);

  const hasScope = useCallback((scope: string) => {
    if (role === 'admin') return true;
    if (scopes.includes(scope)) return true;
    if (scope.startsWith('server.') && scope !== 'server.create' && scope !== 'server.admin') {
      if (scopes.includes('server.admin')) return true;
    }
    return false;
  }, [role, scopes]);

  const value = useMemo(
    () => ({ role, user, scopes, login, verifyOtp, register, logout, hasScope, loading, fetchSelf }),
    [role, user, scopes, login, verifyOtp, register, logout, hasScope, loading, fetchSelf],
  );

  const { config } = useConfig();
  const panelName = config?.branding?.name || "Aether Panel";

  if ((loading && !isAuthPage) || (!role && !isAuthPage)) {
    return (
      <div className="flex h-screen w-full items-center justify-center bg-background">
        <div className="flex flex-col items-center gap-4">
          <div className="h-10 w-10 animate-spin rounded-full border-[3px] border-border border-t-primary border-l-accent" />
          <p className="text-muted-foreground animate-pulse">{t('common.initializing', { name: panelName })}</p>
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

/**
 * Where to go after signing in: back to the page the session expired on, which api-client kept
 * in sessionStorage (never read from the address), when it is one of the panel's own pages.
 */
function afterLoginPath(): string {
  try {
    const saved = sessionStorage.getItem(RETURN_TO_KEY) || '';
    sessionStorage.removeItem(RETURN_TO_KEY);
    if (saved.startsWith('/') && !saved.startsWith('//') && !saved.includes('\\') && !/^\/(login|register)(\/|$)/.test(saved)) {
      return saved;
    }
  } catch {
    // storage blocked: the dashboard
  }
  return '/dashboard/';
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}

export function Providers({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <TranslationsProvider>
      <ConfigProvider>
        <AuthProvider>{children}</AuthProvider>
      </ConfigProvider>
    </TranslationsProvider>
  )
}
