'use client';
import { useState, useEffect, createContext, useContext } from 'react';
import type { ReactNode } from 'react';
import { useToast } from '@/hooks/use-toast';
import { TranslationsProvider } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { useConfig } from '@/contexts/config-context';

export type UserRole = 'admin' | 'user';

interface LoginCredentials {
  email: string;
  password?: string;
}

interface UserData {
  id: number;
  username: string;
  email: string;
  scopes?: string[];
}

interface RegisterData {
  username: string;
  email: string;
  password?: string;
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
  const [role, setRole] = useState<UserRole | null>(null);
  const [user, setUser] = useState<UserData | null>(null);
  const [scopes, setScopes] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const { toast } = useToast();

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
          localStorage.setItem('aether_panel_scopes', JSON.stringify(currentScopes));
        } else {
          console.warn('No scopes received from backend, falling back to local storage');
          currentScopes = JSON.parse(localStorage.getItem('aether_panel_scopes') || '[]');
        }
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

  useEffect(() => {
    if (loading) return;
    const pathname = window.location.pathname.replace(/\/$/, '') || '/';
    const isAuthPage = pathname === '/login' || pathname === '/register';

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
        password: credentials.password
      });

      if (data.otpNeeded) {
        toast({
          title: 'OTP Required',
          description: 'Two-factor authentication is enabled for this account.',
        });
        setLoading(false);
        return;
      }

      const scopesList = normalizeScopes(data.scopes || []);
      localStorage.setItem('aether_panel_scopes', JSON.stringify(scopesList));

      // Fetch user info and pass current scopes to avoid race conditions
      await fetchSelf(scopesList);

      toast({
        title: 'Login Successful',
        description: 'Redirecting to dashboard...',
      });

      setTimeout(() => {
        window.location.href = '/dashboard/';
      }, 500);
    } catch (e: any) {
      console.error('Login error:', e);
      toast({
        variant: 'destructive',
        title: 'Login Failed',
        description: e.message || 'Please check your credentials.',
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
        password: data.password
      });

      const scopesList = normalizeScopes(res.scopes || []);
      localStorage.setItem('aether_panel_scopes', JSON.stringify(scopesList));

      await fetchSelf(scopesList);

      toast({
        title: 'Registration Successful',
        description: 'Account created and logged in.',
      });

      setTimeout(() => {
        window.location.href = '/dashboard/';
      }, 500);
    } catch (e: any) {
      console.error('Registration error:', e);
      toast({
        variant: 'destructive',
        title: 'Registration Failed',
        description: e.message || 'Could not create account.',
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
    return scopes.includes(scope);
  };

  const value = { role, user, scopes, login, register, logout, hasScope, loading, fetchSelf };

  const normalizedPath = typeof window !== 'undefined' ? (window.location.pathname.replace(/\/$/, '') || '/') : '';
  const isAuthPage = normalizedPath === '/login' || normalizedPath === '/register';

  const { config } = useConfig();
  const panelName = config?.branding?.name || "Aether Panel";

  if ((loading && !isAuthPage) || (!role && !isAuthPage)) {
    return (
      <div className="flex h-screen w-full items-center justify-center bg-background">
        <div className="flex flex-col items-center gap-4">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
          <p className="text-muted-foreground">Initializing {panelName}...</p>
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
