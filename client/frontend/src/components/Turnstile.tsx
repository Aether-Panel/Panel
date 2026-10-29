'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { Loader2, ShieldAlert, ShieldCheck } from '@/components/icons';
import { useConfig } from '@/contexts/config-context';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { solveChallenge, type Challenge } from '@/lib/proof-of-work';

declare global {
  interface Window {
    turnstile?: {
      render: (element: HTMLElement, options: any) => string;
      reset: (widgetId?: string) => void;
      remove: (widgetId?: string) => void;
    };
  }
}

const TURNSTILE_SCRIPT_SRC = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';

// one load shared by every widget; a failed load is forgotten so the next widget retries
let scriptPromise: Promise<void> | null = null;

function loadTurnstileScript(): Promise<void> {
  if (window.turnstile) return Promise.resolve();
  scriptPromise ??= new Promise<void>((resolve, reject) => {
    const script = document.createElement('script');
    script.src = TURNSTILE_SCRIPT_SRC;
    script.async = true;
    script.defer = true;
    script.onload = () => resolve();
    script.onerror = () => {
      scriptPromise = null;
      script.remove();
      reject(new Error('Failed to load Cloudflare Turnstile script'));
    };
    document.head.appendChild(script);
  });
  return scriptPromise;
}

/**
 * The anti-bot check of an auth form: the panel's built-in one (a proof of work solved in
 * the background), Turnstile when the administrator configured it, or none. Spread `fields`
 * into the request; a token is good for one request, so `reset` starts a new check after
 * every attempt that used it.
 */
export function useCaptcha() {
  const { config } = useConfig();
  const provider = config?.captcha ?? (config?.turnstileEnabled && config?.turnstileSiteKey ? 'turnstile' : '');
  const siteKey = provider === 'turnstile' ? config?.turnstileSiteKey || '' : '';
  const [token, setToken] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);
  const [round, setRound] = useState(0);

  const reset = useCallback(() => {
    setToken(null);
    setRound((r) => r + 1);
  }, []);

  useEffect(() => {
    if (provider !== 'builtin') return;
    const controller = new AbortController();
    setFailed(false);
    api.get<Challenge>('/auth/challenge')
      .then((challenge) => solveChallenge(challenge, controller.signal))
      .then((solved) => {
        if (!controller.signal.aborted) setToken(solved);
      })
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    return () => controller.abort();
  }, [provider, round]);

  let fields: { turnstileToken?: string; captchaToken?: string } = {};
  if (provider === 'turnstile') fields = { turnstileToken: token ?? undefined };
  else if (provider === 'builtin') fields = { captchaToken: token ?? undefined };

  let widget: ReactNode = null;
  if (siteKey) widget = <Turnstile key={round} siteKey={siteKey} onVerify={setToken} />;
  else if (provider === 'builtin') widget = <BuiltinCheck done={token !== null} failed={failed} onRetry={reset} />;

  return {
    fields,
    // forms wait for the check before they can be sent
    ready: !provider || token !== null,
    reset,
    widget,
  };
}

/** One quiet line under the form while the built-in check runs; nothing to click. */
function BuiltinCheck({ done, failed, onRetry }: Readonly<{ done: boolean; failed: boolean; onRetry: () => void }>) {
  const { t } = useTranslations();
  if (failed) {
    return (
      <p role="alert" className="flex items-center justify-center gap-2 text-xs text-destructive-bright">
        <ShieldAlert className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        {t('auth.captcha.failed')}
        <button type="button" onClick={onRetry} className="rounded font-medium underline underline-offset-2 focus-ring">
          {t('auth.captcha.retry')}
        </button>
      </p>
    );
  }
  return (
    <p role="status" className="flex items-center justify-center gap-2 text-xs text-muted-foreground">
      {done ? (
        <ShieldCheck className="h-3.5 w-3.5 shrink-0 text-success" aria-hidden="true" />
      ) : (
        <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" aria-hidden="true" />
      )}
      {done ? t('auth.captcha.done') : t('auth.captcha.checking')}
    </p>
  );
}

export default function Turnstile({
  siteKey,
  onVerify,
  theme = 'dark',
}: Readonly<{
  siteKey: string;
  onVerify: (token: string | null) => void;
  theme?: 'light' | 'dark' | 'auto';
}>) {
  const containerRef = useRef<HTMLDivElement>(null);
  const widgetIdRef = useRef<string | null>(null);

  useEffect(() => {
    if (!siteKey || !containerRef.current) return;
    let cancelled = false;

    loadTurnstileScript()
      .then(() => {
        if (cancelled || !containerRef.current || !window.turnstile) return;
        const el = containerRef.current;
        const widgetId = window.turnstile.render(el, {
          sitekey: siteKey,
          theme,
          callback: (token: string) => onVerify(token),
          'expired-callback': () => onVerify(null),
          'error-callback': () => onVerify(null),
        });
        widgetIdRef.current = widgetId;
      })
      .catch((err) => console.error('[Turnstile]', err));

    return () => {
      cancelled = true;
      if (widgetIdRef.current && window.turnstile) {
        try {
          window.turnstile.remove(widgetIdRef.current);
        } catch { /* noop */ }
        widgetIdRef.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [siteKey, theme]);

  if (!siteKey) return null;

  return <div className="flex w-full justify-center"><div ref={containerRef} /></div>;
}