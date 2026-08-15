'use client';

import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useConfig } from '@/contexts/config-context';
import { api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { Loader2 } from 'lucide-react';
import { useState } from 'react';

const formSchema = z.object({
  email: z.string().email({
    message: 'Please enter a valid email address.',
  }),
});

const inputClass =
  'px-3 py-2.5 text-sm text-foreground rounded-md bg-background/60 border border-border/60 placeholder:text-muted-foreground/50 focus:border-primary focus:bg-background/80 focus:ring-0';

export default function ForgotPasswordPage() {
  const { config } = useConfig();
  const panelName = config?.branding?.name || 'Aether Panel';
  const [loading, setLoading] = useState(false);
  const [sent, setSent] = useState(false);

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema as any),
    defaultValues: {
      email: '',
    },
  });

  async function onSubmit(values: z.infer<typeof formSchema>) {
    setLoading(true);
    try {
      await api.post('/auth/forgot-password', { email: values.email });
      setSent(true);
    } catch (e: any) {
      sileo.error({ title: 'Error', description: e.message || 'Unable to send password reset email.' });
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
              alt="forgot password image"
            />
            <div className="absolute inset-0 flex items-end justify-center">
              <div className="w-full bg-gradient-to-t from-black/70 via-black/50 to-transparent absolute bottom-0 p-6 max-md:hidden">
                <h2 className="text-white text-2xl font-semibold">Reset your {panelName} password</h2>
                <p className="text-slate-200 text-base font-medium mt-4 leading-relaxed">
                  We&apos;ll email you a secure link so you can get back into your account in minutes.
                </p>
              </div>
            </div>
          </div>

          <div className="py-6 px-6 lg:px-8">
            <div className="max-w-md mx-auto w-full">
              <h1 className="text-foreground text-3xl font-bold mb-4">Forgot password</h1>

              {sent ? (
                <div className="space-y-6">
                  <p className="text-sm text-muted-foreground leading-relaxed">
                    If an account exists for that email address, we&apos;ve sent you a link to reset your password.
                    Please check your inbox and follow the instructions.
                  </p>
                  <Button
                    type="button"
                    onClick={() => setSent(false)}
                    className="w-full py-2 px-3.5 text-sm rounded-md font-semibold cursor-pointer text-primary-foreground bg-primary hover:bg-primary/90 transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-primary h-auto"
                  >
                    Send another email
                  </Button>
                </div>
              ) : (
                <>
                  <p className="text-sm text-muted-foreground mb-8 leading-relaxed">
                    Enter your account email and we&apos;ll send you a link to reset your password.
                  </p>

                  <Form {...form}>
                    <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
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

                      <Button
                        type="submit"
                        className="w-full py-2 px-3.5 text-sm rounded-md font-semibold cursor-pointer text-primary-foreground bg-primary hover:bg-primary/90 transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-primary h-auto"
                        disabled={loading}
                      >
                        {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        Send reset link
                      </Button>
                    </form>
                  </Form>
                </>
              )}

              <div className="mt-6 text-center text-sm text-muted-foreground">
                Remembered your password?{' '}
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