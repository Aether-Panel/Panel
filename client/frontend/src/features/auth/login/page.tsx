'use client';

import { useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { useAuth } from '@/contexts/providers';
import { useConfig } from '@/contexts/config-context';
import { useTranslations } from '@/contexts/translations-context';
import { Loader2, Mail, Lock } from 'lucide-react';
import Turnstile from '@/components/Turnstile';
import { AuthFrame, AuthInput, authLabelClass, authLinkClass, authMessageClass } from '@/components/AuthShell';

/** Same rules as before; only the messages now follow the active language. */
const createFormSchema = (t: (key: string) => string) =>
  z.object({
    email: z.string().email({ message: t('auth.validation.email') }),
    password: z.string().min(1, { message: t('auth.validation.passwordRequired') }),
  });

type LoginValues = z.infer<ReturnType<typeof createFormSchema>>;

function DiscordIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028 14.09 14.09 0 0 0 1.226-1.994.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z" />
    </svg>
  );
}

function SteamIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="M11.979 0C5.678 0 .511 4.86.022 11.037l6.432 2.658a3.387 3.387 0 0 1 1.913-.59c.064 0 .125.002.187.006l2.861-4.142V8.91a3.527 3.527 0 0 1 3.521-3.521 3.527 3.527 0 0 1 3.521 3.521 3.527 3.527 0 0 1-3.521 3.521h-.105l-4.076 2.911c0 .052.004.105.004.159 0 1.875-1.515 3.396-3.39 3.396a3.406 3.406 0 0 1-3.362-2.898L.436 15.27C1.862 20.307 6.486 24 11.979 24c6.627 0 12.001-5.373 12.001-12S18.606 0 11.979 0zM7.54 18.21l-1.473-.61a2.032 2.032 0 0 0 1.8-3.375l.002.001.138.085a4.17 4.17 0 0 0-.467 1.9zm4.439-6.794a2.017 2.017 0 1 0 0 4.035 2.017 2.017 0 0 0 0-4.035zm5.86 2.55a2.528 2.528 0 0 0-2.522-2.521 2.528 2.528 0 0 0-2.521 2.521 2.528 2.528 0 0 0 2.521 2.522 2.528 2.528 0 0 0 2.522-2.522z" />
    </svg>
  );
}

export default function LoginPage() {
  const { t } = useTranslations();
  const { login, loading } = useAuth();
  const { config } = useConfig();
  const panelName = config?.branding?.name || 'Aether Panel';
  const [turnstileToken, setTurnstileToken] = useState<string | null>(null);

  const formSchema = useMemo(() => createFormSchema(t), [t]);
  const form = useForm<LoginValues>({
    resolver: zodResolver(formSchema as any),
    defaultValues: {
      email: '',
      password: '',
    },
  });

  function onSubmit(values: LoginValues) {
    void login({ ...values, turnstileToken: turnstileToken || undefined });
  }

  const demoEmail = `admin@${panelName.toLowerCase().replaceAll(/\s+/g, '')}.com`;

  return (
    <AuthFrame
      title={t('auth.login.title')}
      description={t('auth.login.description')}
      artTitle={t('auth.login.artTitle', { name: panelName })}
      artDescription={t('auth.login.artDescription')}
      footer={
        config?.registrationEnabled !== false ? (
          <>
            {t('auth.login.noAccount')}{' '}
            <a href="/register" className={authLinkClass}>
              {t('auth.login.signUp')}
            </a>
          </>
        ) : null
      }
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} noValidate className="space-y-5">
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
                  <AuthInput {...field} icon={Lock} type="password" autoComplete="current-password" placeholder="••••••••" />
                </FormControl>
                <FormMessage className={authMessageClass} />
              </FormItem>
            )}
          />

          <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
            <div className="flex items-center gap-2">
              <Checkbox id="remember" />
              <Label htmlFor="remember" className="text-sm font-normal text-muted-foreground">
                {t('auth.login.remember')}
              </Label>
            </div>
            <a href="/forgot-password" className={`text-sm ${authLinkClass}`}>
              {t('auth.login.forgot')}
            </a>
          </div>

          <Button type="submit" className="h-11 w-full font-semibold" disabled={loading}>
            {loading && <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />}
            {t('auth.login.submit')}
          </Button>
        </form>
      </Form>

      <div className="my-6 flex items-center gap-3">
        <div className="h-px flex-1 bg-border/60" />
        <span className="text-xs text-muted-foreground">{t('auth.login.or')}</span>
        <div className="h-px flex-1 bg-border/60" />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <Button
          type="button"
          variant="outline"
          className="h-11 hover:border-[#5865F2]/50 hover:bg-[#5865F2]/10 hover:text-[#5865F2]"
        >
          <DiscordIcon className="h-4 w-4" />
          Discord
        </Button>
        <Button
          type="button"
          variant="outline"
          className="h-11 hover:border-[#66c0f4]/50 hover:bg-[#66c0f4]/10 hover:text-[#66c0f4]"
        >
          <SteamIcon className="h-4 w-4" />
          Steam
        </Button>
      </div>

      {config?.turnstileEnabled && (
        <div className="mt-6 flex justify-center">
          <Turnstile
            siteKey={config?.turnstileSiteKey || ''}
            onVerify={(token) => setTurnstileToken(token)}
            theme="dark"
          />
        </div>
      )}
    </AuthFrame>
  );
}
