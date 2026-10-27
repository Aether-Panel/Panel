'use client';

import { useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { useConfig } from '@/contexts/config-context';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { Loader2, Mail, MailCheck } from 'lucide-react';
import { AuthFrame, AuthInput, AuthNotice, authLabelClass, authLinkClass, authMessageClass } from '@/components/AuthShell';
import { useCaptcha } from '@/components/Turnstile';

/** Same rule as before; only the message now follows the active language. */
const createFormSchema = (t: (key: string) => string) =>
  z.object({
    email: z.string().email({ message: t('auth.validation.email') }),
  });

type ForgotValues = z.infer<ReturnType<typeof createFormSchema>>;

export default function ForgotPasswordPage() {
  const { t } = useTranslations();
  const { config } = useConfig();
  const panelName = config?.branding?.name || 'Aether Panel';
  const [loading, setLoading] = useState(false);
  const [sent, setSent] = useState(false);
  const captcha = useCaptcha();

  const formSchema = useMemo(() => createFormSchema(t), [t]);
  const form = useForm<ForgotValues>({
    resolver: zodResolver(formSchema as any),
    defaultValues: {
      email: '',
    },
  });

  async function onSubmit(values: ForgotValues) {
    setLoading(true);
    try {
      await api.post('/auth/forgot-password', { email: values.email, ...captcha.fields });
      setSent(true);
    } catch (e: any) {
      captcha.reset();
      sileo.error({ title: t('common.error'), description: e.message || t('auth.forgot.errorFallback') });
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthFrame
      title={t('auth.forgot.title')}
      description={sent ? undefined : t('auth.forgot.description')}
      artTitle={t('auth.forgot.artTitle', { name: panelName })}
      artDescription={t('auth.forgot.artDescription')}
      footer={
        <>
          {t('auth.forgot.remembered')}{' '}
          <a href="/login" className={authLinkClass}>
            {t('auth.links.signIn')}
          </a>
        </>
      }
    >
      {sent ? (
        <div className="space-y-6">
          <AuthNotice icon={MailCheck} tone="success">
            {t('auth.forgot.sent')}
          </AuthNotice>
          <Button type="button" variant="outline" onClick={() => { captcha.reset(); setSent(false); }} className="h-11 w-full">
            {t('auth.forgot.sendAnother')}
          </Button>
        </div>
      ) : (
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} noValidate className="space-y-6">
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

            {captcha.widget}

            <Button type="submit" className="h-11 w-full font-semibold" disabled={loading || !captcha.ready}>
              {loading && <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />}
              {t('auth.forgot.submit')}
            </Button>
          </form>
        </Form>
      )}
    </AuthFrame>
  );
}
