'use client';

import React from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useAuth } from '@/contexts/providers';
import { useConfig } from '@/contexts/config-context';
import { useTranslations } from '@/contexts/translations-context';
import { sileo } from "@/lib/toast";
import { Loader2 } from 'lucide-react';
import Turnstile from '@/components/Turnstile';

const formSchema = z.object({
  username: z.string().min(5, { message: 'Username must be at least 5 characters.' }),
  email: z.string().email({ message: 'Please enter a valid email address.' }),
  password: z.string().min(8, { message: 'Password must be at least 8 characters.' }),
  confirmPassword: z.string(),
}).refine((data) => data.password === data.confirmPassword, {
  message: 'Passwords do not match.',
  path: ['confirmPassword'],
});

const inputClass =
  'px-3 py-2.5 text-sm text-foreground rounded-md bg-background/60 border border-border/60 placeholder:text-muted-foreground/50 focus:border-primary focus:bg-background/80 focus:ring-0';

export default function RegisterPage() {
  const { t } = useTranslations();
  const { register } = useAuth();
  const { config } = useConfig();
  const panelName = config?.branding?.name || 'Aether Panel';
  const [loading, setLoading] = React.useState(false);
  const [turnstileToken, setTurnstileToken] = React.useState<string | null>(null);
  

  React.useEffect(() => {
    if (config?.registrationEnabled === false) {
      window.location.href = '/login';
    }
  }, [config?.registrationEnabled]);

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema as any),
    defaultValues: { username: '', email: '', password: '', confirmPassword: '' },
  });

  async function onSubmit(values: z.infer<typeof formSchema>) {
    setLoading(true);
    try {
      await register({
        username: values.username,
        email: values.email,
        password: values.password,
        turnstileToken: turnstileToken || undefined,
      });
    } catch (e: any) {
      sileo.error({ title: t('auth.registerFailed'), description: e.message || t('auth.checkCredentials') });
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="md:min-h-screen flex items-center justify-center py-4 px-4 md:px-8 bg-background">
      <div className="w-full max-w-5xl bg-card [box-shadow:0_2px_10px_-3px_rgba(0,0,0,0.3)] rounded-2xl overflow-hidden border border-border">
        <div className="grid items-center w-full gap-0 md:grid-cols-2">
          <div className="relative md:aspect-[8/10] w-full h-full overflow-hidden max-md:-order-1">
            <img
              src="/img/Fondos/minecraft-shaders-anime-hd-wallpaper-preview.jpg"
              className="h-full w-full object-cover"
              alt="register image"
            />
            <div className="absolute inset-0 flex items-end justify-center">
              <div className="w-full bg-gradient-to-t from-black/70 via-black/50 to-transparent absolute bottom-0 p-6 max-md:hidden">
                <h2 className="text-white text-2xl font-semibold">Join {panelName} today</h2>
                <p className="text-slate-200 text-base font-medium mt-4 leading-relaxed">
                  Spin up your account, assign your first server and start managing your infrastructure in seconds.
                </p>
              </div>
            </div>
          </div>

          <div className="py-6 px-6 lg:px-8">
            <div className="max-w-md mx-auto w-full">
              <h1 className="text-foreground text-3xl font-bold mb-10">Sign up</h1>

              <Form {...form}>
                <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
                  <FormField
                    control={form.control}
                    name="username"
                    render={({ field }) => (
                      <FormItem>
                        <label className="mb-2 text-foreground font-medium text-sm inline-block">
                          Username
                        </label>
                        <FormControl>
                          <Input {...field} type="text" placeholder="yourusername" className={inputClass} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="email"
                    render={({ field }) => (
                      <FormItem>
                        <label className="mb-2 text-foreground font-medium text-sm inline-block">
                          Email
                        </label>
                        <FormControl>
                          <Input {...field} type="email" placeholder="you@example.com" className={inputClass} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="password"
                    render={({ field }) => (
                      <FormItem>
                        <label className="mb-2 text-foreground font-medium text-sm inline-block">
                          Password
                        </label>
                        <FormControl>
                          <Input {...field} type="password" placeholder="••••••••" className={inputClass} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="confirmPassword"
                    render={({ field }) => (
                      <FormItem>
                        <label className="mb-2 text-foreground font-medium text-sm inline-block">
                          Confirm Password
                        </label>
                        <FormControl>
                          <Input {...field} type="password" placeholder="••••••••" className={inputClass} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <div className="flex justify-center mt-4">
                    {config?.turnstileEnabled && (
                      <Turnstile
                        siteKey={config?.turnstileSiteKey || ''}
                        onVerify={(token) => setTurnstileToken(token)}
                        theme="dark"
                      />
                    )}
                  </div>

                  <Button
                    type="submit"
                    className="w-full mt-4 py-2 px-3.5 text-sm rounded-md font-semibold cursor-pointer text-primary-foreground bg-primary hover:bg-primary/90 transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-primary h-auto"
                    disabled={loading}
                  >
                    {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                    Create Account
                  </Button>
                </form>
              </Form>

              <div className="mt-6 text-center text-sm text-muted-foreground">
                Already have an account?{' '}
                <a
                  href="/login"
                  className="font-semibold text-primary hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary rounded"
                >
                  Sign in
                </a>
              </div>
            </div>
          </div>
        </div>
      </div>
    </main>
  );
}
