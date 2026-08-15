'use client';

import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { Loader2 } from 'lucide-react';
import { useState } from 'react';

const formSchema = z
  .object({
    password: z.string().min(8, { message: 'Password must be at least 8 characters.' }),
    confirmPassword: z.string(),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: 'Passwords do not match.',
    path: ['confirmPassword'],
  });

const inputClass =
  'px-3 py-2.5 text-sm text-foreground rounded-md bg-background/60 border border-border/60 placeholder:text-muted-foreground/50 focus:border-primary focus:bg-background/80 focus:ring-0';

export default function ResetPasswordPage() {
  const [token] = useState(() => new URLSearchParams(window.location.search).get('token') || '');
  const [loading, setLoading] = useState(false);
  const [done, setDone] = useState(false);

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema as any),
    defaultValues: {
      password: '',
      confirmPassword: '',
    },
  });

  async function onSubmit(values: z.infer<typeof formSchema>) {
    setLoading(true);
    try {
      await api.post('/auth/reset-password', { token, password: values.password });
      setDone(true);
    } catch (e: any) {
      sileo.error({ title: 'Error', description: e.message || 'Unable to reset your password.' });
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="md:min-h-screen flex items-center justify-center py-4 px-4 md:px-8 bg-background">
      <div className="w-full max-w-5xl bg-card [box-shadow:0_2px_10px_-3px_rgba(0,0,0,0.3)] rounded-2xl overflow-hidden border border-border">
        <div className="grid items-center w-full gap-0 md:grid-cols-2">
          <div className="relative md:aspect-[8/10] w-full h-full overflow-hidden">
            <img
              src="/img/Fondos/minecraft-shaders-anime-hd-wallpaper-preview.jpg"
              className="h-full w-full object-cover"
              alt="reset password image"
            />
            <div className="absolute inset-0 flex items-end justify-center">
              <div className="w-full bg-gradient-to-t from-black/70 via-black/50 to-transparent absolute bottom-0 p-6 max-md:hidden">
                <h2 className="text-white text-2xl font-semibold">Create a new password</h2>
                <p className="text-slate-200 text-base font-medium mt-4 leading-relaxed">
                  Choose a strong password that you haven&apos;t used on this account before.
                </p>
              </div>
            </div>
          </div>

          <div className="py-6 px-6 lg:px-8">
            <div className="max-w-md mx-auto w-full">
              <h1 className="text-foreground text-3xl font-bold mb-4">Reset password</h1>

              {done ? (
                <div className="space-y-6">
                  <p className="text-sm text-muted-foreground leading-relaxed">
                    Your password has been reset successfully. You can now sign in with your new password.
                  </p>
                  <Button
                    type="button"
                    onClick={() => (window.location.href = '/login')}
                    className="w-full py-2 px-3.5 text-sm rounded-md font-semibold cursor-pointer text-primary-foreground bg-primary hover:bg-primary/90 transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-primary h-auto"
                  >
                    Sign in
                  </Button>
                </div>
              ) : !token ? (
                <div className="space-y-6">
                  <p className="text-sm text-muted-foreground leading-relaxed">
                    This link is missing the reset token. Please use the link you received in your email.
                  </p>
                  <Button
                    type="button"
                    onClick={() => (window.location.href = '/forgot-password')}
                    className="w-full py-2 px-3.5 text-sm rounded-md font-semibold cursor-pointer text-primary-foreground bg-primary hover:bg-primary/90 transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-primary h-auto"
                  >
                    Request a new link
                  </Button>
                </div>
              ) : (
                <>
                  <p className="text-sm text-muted-foreground mb-8 leading-relaxed">
                    Enter a new password for your account.
                  </p>

                  <Form {...form}>
                    <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
                      <FormField
                        control={form.control}
                        name="password"
                        render={({ field }) => (
                          <FormItem>
                            <label className="mb-2 text-foreground font-medium text-sm inline-block">
                              New Password
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
                              Confirm New Password
                            </label>
                            <FormControl>
                              <Input {...field} type="password" placeholder="••••••••" className={inputClass} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <Button
                        type="submit"
                        className="w-full py-2 px-3.5 text-sm rounded-md font-semibold cursor-pointer text-primary-foreground bg-primary hover:bg-primary/90 transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-primary h-auto"
                        disabled={loading}
                      >
                        {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        Reset password
                      </Button>
                    </form>
                  </Form>
                </>
              )}
            </div>
          </div>
        </div>
      </div>
    </main>
  );
}