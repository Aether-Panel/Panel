'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useConfig } from '@/contexts/config-context';

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
 * Turnstile for an auth form, when the panel has it turned on. A token is good for one
 * request only, so `reset` puts up a fresh widget after every attempt that used it.
 */
export function useCaptcha() {
  const { config } = useConfig();
  const siteKey = config?.turnstileEnabled ? config?.turnstileSiteKey || '' : '';
  const [token, setToken] = useState<string | null>(null);
  const [round, setRound] = useState(0);

  const reset = useCallback(() => {
    setToken(null);
    setRound((r) => r + 1);
  }, []);

  return {
    token: token ?? undefined,
    // forms wait for the check before they can be sent
    ready: !siteKey || token !== null,
    reset,
    widget: siteKey ? <Turnstile key={round} siteKey={siteKey} onVerify={setToken} /> : null,
  };
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