'use client';

import { useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
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

          <div className="flex justify-end">
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
