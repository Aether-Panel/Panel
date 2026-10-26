'use client';

import React from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { useAuth } from '@/contexts/providers';
import { useConfig } from '@/contexts/config-context';
import { useTranslations } from '@/contexts/translations-context';
import { sileo } from "@/lib/toast";
import { Loader2, Lock, Mail, User } from 'lucide-react';
import { useCaptcha } from '@/components/Turnstile';
import { AuthFrame, AuthInput, PasswordStrength, AuthPasswordInput, authLabelClass, authLinkClass, authMessageClass } from '@/components/AuthShell';

/** Same rules as before; only the messages now follow the active language. */
const createFormSchema = (t: (key: string) => string) =>
  z.object({
    username: z.string().min(5, { message: t('auth.validation.usernameMin') }),
    email: z.string().email({ message: t('auth.validation.email') }),
    password: z.string().min(8, { message: t('auth.validation.passwordMin') }),
    confirmPassword: z.string(),
  }).refine((data) => data.password === data.confirmPassword, {
    message: t('auth.validation.passwordMismatch'),
    path: ['confirmPassword'],
  });

type RegisterValues = z.infer<ReturnType<typeof createFormSchema>>;

export default function RegisterPage() {
  const { t } = useTranslations();
  const { register } = useAuth();
  const { config } = useConfig();
  const panelName = config?.branding?.name || 'Aether Panel';
  const [loading, setLoading] = React.useState(false);
  const captcha = useCaptcha();

  React.useEffect(() => {
    if (config?.registrationEnabled === false) {
      window.location.href = '/login';
    }
  }, [config?.registrationEnabled]);

  const formSchema = React.useMemo(() => createFormSchema(t), [t]);
  const form = useForm<RegisterValues>({
    resolver: zodResolver(formSchema as any),
    defaultValues: { username: '', email: '', password: '', confirmPassword: '' },
  });

  async function onSubmit(values: RegisterValues) {
    setLoading(true);
    try {
      await register({
        username: values.username,
        email: values.email,
        password: values.password,
        turnstileToken: captcha.token,
      });
    } catch (e: any) {
      captcha.reset();
      sileo.error({ title: t('auth.registerFailed'), description: e.message || t('auth.checkCredentials') });
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthFrame
      title={t('auth.register.title')}
      description={t('auth.register.description')}
      artTitle={t('auth.register.artTitle', { name: panelName })}
      artDescription={t('auth.register.artDescription')}
      footer={
        <>
          {t('auth.register.haveAccount')}{' '}
          <a href="/login" className={authLinkClass}>
            {t('auth.links.signIn')}
          </a>
        </>
      }
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} noValidate className="space-y-5">
          <FormField
            control={form.control}
            name="username"
            render={({ field }) => (
              <FormItem>
                <FormLabel className={authLabelClass}>{t('auth.fields.username')}</FormLabel>
                <FormControl>
                  <AuthInput
                    {...field}
                    icon={User}
                    type="text"
                    autoComplete="username"
                    placeholder={t('auth.fields.usernamePlaceholder')}
                  />
                </FormControl>
                <FormMessage className={authMessageClass} />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="email"
            render={({ field }) => (
              <FormItem>
                <FormLabel className={authLabelClass}>{t('auth.fields.email')}</FormLabel>
                <FormControl>
                  <AuthInput
                    {...field}
                    icon={Mail}
                    type="email"
                    autoComplete="email"
                    placeholder={t('auth.fields.emailPlaceholder')}
                  />
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
                  <AuthPasswordInput {...field} icon={Lock} autoComplete="new-password" placeholder="••••••••" />
                </FormControl>
                <PasswordStrength password={field.value ?? ''} />
                <FormMessage className={authMessageClass} />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="confirmPassword"
            render={({ field }) => (
              <FormItem>
                <FormLabel className={authLabelClass}>{t('auth.fields.confirmPassword')}</FormLabel>
                <FormControl>
                  <AuthPasswordInput {...field} icon={Lock} autoComplete="new-password" placeholder="••••••••" />
                </FormControl>
                <FormMessage className={authMessageClass} />
              </FormItem>
            )}
          />

          {captcha.widget}

          <Button type="submit" className="h-11 w-full font-semibold" disabled={loading || !captcha.ready}>
            {loading && <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />}
            {t('auth.register.submit')}
          </Button>
        </form>
      </Form>
    </AuthFrame>
  );
}
