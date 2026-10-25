'use client';

import { useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { CircleCheck, Loader2, Lock, TriangleAlert } from 'lucide-react';
import { AuthFrame, AuthInput, AuthNotice, authLabelClass, authLinkClass, authMessageClass } from '@/components/AuthShell';

/** Same rules as before; only the messages now follow the active language. */
const createFormSchema = (t: (key: string) => string) =>
  z
    .object({
      password: z.string().min(8, { message: t('auth.validation.passwordMin') }),
      confirmPassword: z.string(),
    })
    .refine((data) => data.password === data.confirmPassword, {
      message: t('auth.validation.passwordMismatch'),
      path: ['confirmPassword'],
    });

type ResetValues = z.infer<ReturnType<typeof createFormSchema>>;

export default function ResetPasswordPage() {
  const { t } = useTranslations();
  const [token] = useState(() => new URLSearchParams(window.location.search).get('token') || '');
  const [loading, setLoading] = useState(false);
  const [done, setDone] = useState(false);

  const formSchema = useMemo(() => createFormSchema(t), [t]);
  const form = useForm<ResetValues>({
    resolver: zodResolver(formSchema as any),
    defaultValues: {
      password: '',
      confirmPassword: '',
    },
  });

  async function onSubmit(values: ResetValues) {
    setLoading(true);
    try {
      await api.post('/auth/reset-password', { token, password: values.password });
      setDone(true);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('auth.reset.errorFallback') });
    } finally {
      setLoading(false);
    }
  }

  const showForm = !done && Boolean(token);

  let body: ReactNode;
  if (done) {
    body = (
      <div className="space-y-6">
        <AuthNotice icon={CircleCheck} tone="success">
          {t('auth.reset.done')}
        </AuthNotice>
        <Button type="button" onClick={() => (window.location.href = '/login')} className="h-11 w-full font-semibold">
          {t('auth.links.goToSignIn')}
        </Button>
      </div>
    );
  } else if (!token) {
    body = (
      <div className="space-y-6">
        <AuthNotice icon={TriangleAlert} tone="warning">
          {t('auth.reset.missingToken')}
        </AuthNotice>
        <Button
          type="button"
          onClick={() => (window.location.href = '/forgot-password')}
          className="h-11 w-full font-semibold"
        >
          {t('auth.reset.requestNew')}
        </Button>
      </div>
    );
  } else {
    body = (
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} noValidate className="space-y-5">
          <FormField
            control={form.control}
            name="password"
            render={({ field }) => (
              <FormItem>
                <FormLabel className={authLabelClass}>{t('auth.fields.newPassword')}</FormLabel>
                <FormControl>
                  <AuthInput {...field} icon={Lock} type="password" autoComplete="new-password" placeholder="••••••••" />
                </FormControl>
                <FormMessage className={authMessageClass} />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="confirmPassword"
            render={({ field }) => (
              <FormItem>
                <FormLabel className={authLabelClass}>{t('auth.fields.confirmNewPassword')}</FormLabel>
                <FormControl>
                  <AuthInput {...field} icon={Lock} type="password" autoComplete="new-password" placeholder="••••••••" />
                </FormControl>
                <FormMessage className={authMessageClass} />
              </FormItem>
            )}
          />

          <Button type="submit" className="h-11 w-full font-semibold" disabled={loading}>
            {loading && <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />}
            {t('auth.reset.submit')}
          </Button>
        </form>
      </Form>
    );
  }

  return (
    <AuthFrame
      title={t('auth.reset.title')}
      description={showForm ? t('auth.reset.description') : undefined}
      artTitle={t('auth.reset.artTitle')}
      artDescription={t('auth.reset.artDescription')}
      footer={
        done ? null : (
          <>
            {t('auth.forgot.remembered')}{' '}
            <a href="/login" className={authLinkClass}>
              {t('auth.links.signIn')}
            </a>
          </>
        )
      }
    >
      {body}
    </AuthFrame>
  );
}
