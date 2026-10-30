'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { AuthEmblem } from '@/components/AuthShell';
import { Logo } from '@/components/logo';
import { useConfig } from '@/contexts/config-context';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { LoginPanel } from '@/features/auth/login/page';
import { RegisterPanel } from '@/features/auth/register/page';

type Mode = 'login' | 'register';

const PATHS: Record<Mode, string> = { login: '/login', register: '/register' };

/** Same timing for the panel and both forms, so they move as one. */
const SLIDE = 'duration-700 ease-in-out motion-reduce:transition-none';

function modeFromPath(): Mode {
  return typeof window !== 'undefined' && window.location.pathname.startsWith('/register') ? 'register' : 'login';
}

/**
 * Sign-in and sign-up on one card. On desktop the brand panel (logo, the 3D emblem and an
 * invitation to the other form) covers one half and slides across to the other when the user
 * switches, while the forms slide with it and cross-fade, so changing forms never reloads the
 * page. The address follows (/login ↔ /register) and the browser's back button switches back.
 * Phones show one form at a time under the logo, with a link to the other.
 */
export function AuthCard({ initial }: Readonly<{ initial: Mode }>) {
  const { config } = useConfig();
  // until the panel's settings arrive sign-ups count as open, as the old pages did
  const canRegister = config?.registrationEnabled !== false;
  const [mode, setMode] = useState<Mode>(initial);
  // the sign-up form (and its captcha) only mounts once it is wanted
  const [registerReady, setRegisterReady] = useState(initial === 'register');
  const registerMounted = useRef(initial === 'register');

  const show = useCallback((next: Mode, push: boolean) => {
    if (next === 'register' && !registerMounted.current) {
      // first time: mount the sign-up form where it waits (under the panel), then slide
      registerMounted.current = true;
      setRegisterReady(true);
      requestAnimationFrame(() => requestAnimationFrame(() => setMode(next)));
    } else {
      setMode(next);
    }
    document.title = next === 'login' ? 'Login - Aether Panel' : 'Register - Aether Panel';
    if (push && window.location.pathname.replace(/\/$/, '') !== PATHS[next]) {
      window.history.pushState({ authMode: next }, '', PATHS[next] + window.location.search);
    }
  }, []);

  // closed sign-ups: the sign-up address shows the sign-in form instead
  useEffect(() => {
    if (!canRegister && mode === 'register') {
      setMode('login');
      window.history.replaceState(null, '', PATHS.login + window.location.search);
    }
  }, [canRegister, mode]);

  useEffect(() => {
    const onPop = () => show(modeFromPath(), false);
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, [show]);

  const toLogin = () => show('login', true);
  const toRegister = canRegister ? () => show('register', true) : null;
  const registering = mode === 'register' && canRegister;

  return (
    <div className="flex min-h-[100svh] items-center justify-center px-4 py-6 sm:py-10">
      <div className="auth-glow surface-panel relative w-full max-w-5xl overflow-hidden rounded-2xl border border-border/60 shadow-elevation-3">
        {/* phones: the brand is a strip on top */}
        <div className="auth-brand border-b border-border/60 p-5 sm:p-6 md:hidden">
          <Logo />
        </div>

        <div className="relative grid grid-cols-1 md:min-h-[640px] md:grid-cols-2">
          <FormSide active={!registering} hiddenShift="md:translate-x-full" className="md:col-start-1 md:row-start-1">
            <LoginPanel onSwitch={toRegister} />
          </FormSide>

          {registerReady && canRegister && (
            <FormSide active={registering} hiddenShift="md:-translate-x-full" className="md:col-start-2 md:row-start-1">
              <RegisterPanel onSwitch={toLogin} />
            </FormSide>
          )}

          <BrandPanel registering={registering} canRegister={canRegister} onLogin={toLogin} onRegister={toRegister} />
        </div>
      </div>
    </div>
  );
}

/**
 * One form's half. The hidden one slides under the brand panel while it fades, and is inert
 * so neither the keyboard nor a screen reader lands in it.
 */
function FormSide({ active, hiddenShift, className, children }: Readonly<{
  active: boolean;
  /** where it slides to while hidden: under the panel */
  hiddenShift: string;
  className?: string;
  children: ReactNode;
}>) {
  return (
    <section
      inert={!active}
      aria-hidden={!active}
      className={cn(
        'transition-[opacity,transform,visibility]',
        SLIDE,
        active ? 'visible opacity-100 md:translate-x-0' : cn('max-md:hidden invisible opacity-0', hiddenShift),
        className,
      )}
    >
      {children}
    </section>
  );
}

/**
 * The half that slides: our logo and 3D emblem over the brand colours, and an invitation to
 * the form it is covering, which it uncovers when clicked.
 */
function BrandPanel({ registering, canRegister, onLogin, onRegister }: Readonly<{
  registering: boolean;
  canRegister: boolean;
  onLogin: () => void;
  onRegister: (() => void) | null;
}>) {
  const { t } = useTranslations();
  const { config } = useConfig();
  const name = config?.branding?.name || 'Aether Panel';

  return (
    <div
      className={cn(
        'absolute inset-y-0 left-0 z-10 hidden w-1/2 p-2 transition-transform md:block',
        SLIDE,
        registering ? 'translate-x-0' : 'translate-x-full',
      )}
    >
      <div className="auth-brand relative flex h-full flex-col justify-between gap-6 overflow-hidden rounded-xl border border-border/60 p-8 lg:p-10">
        <Logo />
        <AuthEmblem />

        {/* both invitations share one place and cross-fade */}
        <div className="grid">
          <Invitation
            shown={!registering}
            title={canRegister ? t('auth.register.artTitle', { name }) : t('auth.login.artTitle', { name })}
            text={canRegister ? t('auth.register.artDescription') : t('auth.login.artDescription')}
            button={onRegister ? t('auth.register.submit') : null}
            onClick={onRegister ?? undefined}
          />
          {canRegister && (
            <Invitation
              shown={registering}
              title={t('auth.login.artTitle', { name })}
              text={t('auth.login.artDescription')}
              button={t('auth.login.submit')}
              onClick={onLogin}
            />
          )}
        </div>
      </div>
    </div>
  );
}

function Invitation({ shown, title, text, button, onClick }: Readonly<{
  shown: boolean;
  title: string;
  text: string;
  button: string | null;
  onClick?: () => void;
}>) {
  return (
    <div
      inert={!shown}
      aria-hidden={!shown}
      className={cn(
        'col-start-1 row-start-1 flex flex-col items-start gap-4 transition-[opacity,transform,visibility] duration-500 ease-panel motion-reduce:transition-none',
        shown ? 'visible translate-y-0 opacity-100 delay-150' : 'invisible translate-y-2 opacity-0',
      )}
    >
      <div>
        <h2 className="text-balance text-[1.625rem] font-semibold leading-tight tracking-[-0.03em] text-foreground">{title}</h2>
        <p className="mt-3 max-w-sm text-pretty text-sm leading-relaxed text-foreground/70">{text}</p>
      </div>
      {button && (
        <button
          type="button"
          onClick={onClick}
          className="rounded-full border border-foreground/15 bg-foreground/10 px-8 py-2.5 text-sm font-medium text-foreground backdrop-blur-sm transition-colors duration-300 ease-panel hover:bg-foreground hover:text-background focus-ring"
        >
          {button}
        </button>
      )}
    </div>
  );
}
