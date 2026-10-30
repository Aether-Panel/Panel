'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { useAuth } from '@/contexts/providers';
import type { LoginResult } from '@/contexts/providers';
import { useConfig } from '@/contexts/config-context';
import { useTranslations } from '@/contexts/translations-context';
import { ArrowLeft, Clock, KeyRound, Loader2, Mail, Lock, ShieldCheck } from '@/components/icons';
import { useCaptcha } from '@/components/Turnstile';
import { AuthInput, AuthNotice, AuthPane, AuthPasswordInput, OtpInput, authLabelClass, authLinkClass, authMessageClass } from '@/components/AuthShell';
import { AuthCard } from '@/features/auth/auth-card';

/** Same rules as before; only the messages now follow the active language. */
const createFormSchema = (t: (key: string) => string) =>
  z.object({
    email: z.string().email({ message: t('auth.validation.email') }),
    password: z.string().min(1, { message: t('auth.validation.passwordRequired') }),
  });

type LoginValues = z.infer<ReturnType<typeof createFormSchema>>;

export default function LoginPage() {
  return <AuthCard initial="login" />;
}

/**
 * The sign-in side of the auth card: email and password, then the 2FA code when the account
 * has it. `onSwitch` goes to the sign-up form (null when sign-ups are closed).
 */
export function LoginPanel({ onSwitch }: Readonly<{ onSwitch: (() => void) | null }>) {
  const { t } = useTranslations();
  const { login, verifyOtp, loading } = useAuth();
  const { config } = useConfig();
  const panelName = config?.branding?.name || 'Aether Panel';
  const captcha = useCaptcha();
  // the billing system's sign-in link was used already, came too late, or is for an account
  // that may not sign in
  const [ssoNotice] = useState(() => (typeof window === 'undefined' ? null : new URLSearchParams(window.location.search).get('sso')));
  // the session ended while a page was open (api-client sends the browser here)
  const [sessionExpired] = useState(() => typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('expired') === '1');

  const formSchema = useMemo(() => createFormSchema(t), [t]);
  const form = useForm<LoginValues>({
    resolver: zodResolver(formSchema as any),
    defaultValues: {
      email: '',
      password: '',
    },
  });

  // Accounts with two-factor authentication get a second step on the same page
  const [step, setStep] = useState<'credentials' | 'otp'>('credentials');

  async function onSubmit(values: LoginValues) {
    const result = await login({ ...values, ...captcha.fields });
    if (result === 'otp') {
      setStep('otp');
    } else if (result !== 'success') {
      captcha.reset();
    }
  }

  function backToCredentials() {
    form.resetField('password');
    setStep('credentials');
  }

  const demoEmail = `admin@${panelName.toLowerCase().replaceAll(/\s+/g, '')}.com`;

  if (step === 'otp') {
    return (
      <AuthPane
        title={t('auth.otp.title')}
        description={t('auth.otp.description', { email: form.getValues('email') })}
      >
        <OtpStep loading={loading} verify={verifyOtp} onExpired={backToCredentials} onBack={backToCredentials} />
      </AuthPane>
    );
  }

  return (
    <AuthPane
      title={t('auth.login.title')}
      description={t('auth.login.description')}
      // on desktop the card's sliding panel has the sign-up button
      footerClassName="md:hidden"
      footer={
        onSwitch ? (
          <>
            {t('auth.login.noAccount')}{' '}
            <button type="button" onClick={onSwitch} className={authLinkClass}>
              {t('auth.login.signUp')}
            </button>
          </>
        ) : null
      }
    >
      {sessionExpired && !ssoNotice && (
        <div className="mb-5">
          <AuthNotice icon={Clock} tone="warning">{t('auth.login.sessionExpired')}</AuthNotice>
        </div>
      )}
      {(ssoNotice === 'expired' || ssoNotice === 'denied') && (
        <div className="mb-5">
          <AuthNotice icon={Clock} tone="warning">
            {t(ssoNotice === 'denied' ? 'auth.login.ssoDenied' : 'auth.login.ssoExpired')}
          </AuthNotice>
        </div>
      )}
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} noValidate className="stagger space-y-5">
          <FormField
            control={form.control}
            name="email"
            render={({ field }) => (
              <FormItem>
                <FormLabel className={authLabelClass}>{t('auth.fields.email')}</FormLabel>
                <FormControl>
                  <AuthInput {...field} icon={Mail} type="email" autoComplete="email" placeholder={demoEmail} />
                </FormControl>
                <FormMessage className={authMessageClass} />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="password"
            render={({ field }) => (
              <FormItem>
                <FormLabel className={authLabelClass}>{t('auth.fields.password')}</FormLabel>
                <FormControl>
                  <AuthPasswordInput {...field} icon={Lock} autoComplete="current-password" placeholder="••••••••" />
                </FormControl>
                <FormMessage className={authMessageClass} />
              </FormItem>
            )}
          />

          <div className="flex justify-end">
            <a href="/forgot-password" className={`text-sm ${authLinkClass}`}>
              {t('auth.login.forgot')}
            </a>
          </div>

          {captcha.widget}

          <Button type="submit" className="h-11 w-full font-semibold" disabled={loading || !captcha.ready}>
            {loading && <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />}
            {t('auth.login.submit')}
          </Button>
        </form>
      </Form>
    </AuthPane>
  );
}

type OtpStepProps = {
  loading: boolean;
  verify: (token: string) => Promise<LoginResult>;
  onExpired: () => void;
  onBack: () => void;
};

/** Second sign-in step: the 6-digit code from the app, or one of the recovery codes. */
function OtpStep({ loading, verify, onExpired, onBack }: Readonly<OtpStepProps>) {
  const { t } = useTranslations();
  const [mode, setMode] = useState<'app' | 'recovery'>('app');
  const [code, setCode] = useState('');
  const [error, setError] = useState<'wrong' | 'limited' | null>(null);
  const failed = error !== null;
  const codeRef = useRef<HTMLInputElement>(null);

  // back to the code after switching mode or after a failed try (the input is disabled while checking)
  useEffect(() => {
    if (!loading) {
      codeRef.current?.focus();
    }
  }, [mode, loading]);

  async function submit(token: string) {
    const trimmed = token.trim();
    if (!trimmed || loading) {
      return;
    }
    const result = await verify(trimmed);
    if (result === 'expired') {
      onExpired();
      return;
    }
    if (result === 'failed' || result === 'limited') {
      setError(result === 'failed' ? 'wrong' : 'limited');
      setCode('');
    }
  }

  function switchMode() {
    setMode((current) => (current === 'app' ? 'recovery' : 'app'));
    setCode('');
    setError(null);
  }

  const ready = mode === 'app' ? code.length === 6 : code.trim().length > 0;

  return (
    <form
      noValidate
      className="stagger space-y-5"
      onSubmit={(event) => {
        event.preventDefault();
        void submit(code);
      }}
    >
      <div className="flex items-center gap-3 rounded-xl border border-primary/20 bg-primary/[0.06] p-3.5 text-sm text-foreground/85">
        <ShieldCheck className="h-5 w-5 shrink-0 text-primary-bright" aria-hidden="true" />
        <p>{mode === 'app' ? t('auth.otp.appHint') : t('auth.otp.recoveryHint')}</p>
      </div>

      {mode === 'app' ? (
        <OtpInput
          ref={codeRef}
          value={code}
          onChange={(value) => {
            setCode(value);
            setError(null);
          }}
          onComplete={(value) => void submit(value)}
          disabled={loading}
          invalid={failed}
          aria-label={t('auth.otp.codeLabel')}
        />
      ) : (
        <div className="space-y-2">
          <label htmlFor="recovery-code" className={authLabelClass}>
            {t('auth.otp.recoveryLabel')}
          </label>
          <AuthInput
            id="recovery-code"
            ref={codeRef}
            icon={KeyRound}
            value={code}
            onChange={(event) => {
              setCode(event.target.value);
              setError(null);
            }}
            aria-invalid={failed || undefined}
            autoComplete="off"
            autoCapitalize="off"
            spellCheck={false}
            maxLength={32}
            className="font-mono tracking-wider"
            placeholder="A1b2C3d4E5f6"
          />
        </div>
      )}

      {failed ? (
        <p role="alert" className={authMessageClass}>
          {error === 'limited' ? t('auth.tooManyAttempts') : t('auth.otp.wrongCode')}
        </p>
      ) : null}

      <Button type="submit" className="h-11 w-full font-semibold" disabled={loading || !ready}>
        {loading && <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />}
        {t('auth.otp.submit')}
      </Button>

      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 text-sm">
        <button type="button" onClick={onBack} className="inline-flex items-center gap-1.5 rounded-sm font-medium text-muted-foreground transition-colors duration-200 ease-panel hover:text-foreground focus-ring">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          {t('auth.otp.back')}
        </button>
        <button type="button" onClick={switchMode} className={authLinkClass}>
          {mode === 'app' ? t('auth.otp.useRecovery') : t('auth.otp.useApp')}
        </button>
      </div>
    </form>
  );
}
