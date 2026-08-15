'use client';

import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { useAuth } from '@/contexts/providers';
import { useConfig } from '@/contexts/config-context';
import { Loader2 } from 'lucide-react';
import { Separator } from '@/components/ui/separator';
import Turnstile from '@/components/Turnstile';
import { useState } from 'react';

const formSchema = z.object({
  email: z.string().email({
    message: 'Please enter a valid email address.',
  }),
  password: z.string().min(1, {
    message: 'Password is required.',
  }),
});

const inputClass =
  'px-3 py-2.5 text-sm text-foreground rounded-md bg-background/60 border border-border/60 placeholder:text-muted-foreground/50 focus:border-primary focus:bg-background/80 focus:ring-0';

export default function LoginPage() {
  const { login, loading } = useAuth();
  const { config } = useConfig();
  const panelName = config?.branding?.name || 'Aether Panel';
  const [turnstileToken, setTurnstileToken] = useState<string | null>(null);

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema as any),
    defaultValues: {
      email: '',
      password: '',
    },
  });

  function onSubmit(values: z.infer<typeof formSchema>) {
    login({ ...values, turnstileToken: turnstileToken || undefined });
  }

  const demoEmail = `admin@${panelName.toLowerCase().replace(/\s+/g, '')}.com`;

  return (
    <main className="md:min-h-screen flex items-center justify-center py-4 px-4 md:px-8 bg-background">
      <div className="w-full max-w-5xl bg-card [box-shadow:0_2px_10px_-3px_rgba(0,0,0,0.3)] rounded-2xl overflow-hidden border border-border">
        <div className="grid items-center w-full gap-0 md:grid-cols-2">
          <div className="relative md:aspect-[8/10] w-full h-full overflow-hidden">
            <img
              src="/img/Fondos/minecraft-shaders-anime-hd-wallpaper-preview.jpg"
              className="h-full w-full object-cover"
              alt="login image"
            />
            <div className="absolute inset-0 flex items-end justify-center">
              <div className="w-full bg-gradient-to-t from-black/70 via-black/50 to-transparent absolute bottom-0 p-6 max-md:hidden">
                <h2 className="text-white text-2xl font-semibold">Welcome Back to {panelName}</h2>
                <p className="text-slate-200 text-base font-medium mt-4 leading-relaxed">
                  Manage your game servers, monitor live resources and keep your services online — all in one place.
                </p>
              </div>
            </div>
          </div>

          <div className="py-6 px-6 lg:px-8">
            <div className="max-w-md mx-auto w-full">
              <h1 className="text-foreground text-3xl font-bold mb-10">Sign in</h1>

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
                          <Input {...field} type="email" placeholder={demoEmail} className={inputClass} />
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

                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-center gap-2">
                      <Checkbox id="remember" />
                      <Label htmlFor="remember" className="text-sm text-muted-foreground">
                        Remember me
                      </Label>
                    </div>
                    <a
                      href="/forgot-password"
                      className="text-sm font-medium text-primary hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary rounded"
                    >
                      Forgot password?
                    </a>
                  </div>

                  <Button
                    type="submit"
                    className="w-full py-2 px-3.5 text-sm rounded-md font-semibold cursor-pointer text-primary-foreground bg-primary hover:bg-primary/90 transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-primary h-auto"
                    disabled={loading}
                  >
                    {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                    Sign in
                  </Button>
                </form>
              </Form>

              <div className="my-8 flex items-center gap-4">
                <Separator className="flex-1" />
                <p className="text-sm text-muted-foreground text-center">or</p>
                <Separator className="flex-1" />
              </div>

<div>
                <div className="flex justify-center">
                  {config?.turnstileEnabled && (
                    <Turnstile
                      siteKey={config?.turnstileSiteKey || ''}
                      onVerify={(token) => setTurnstileToken(token)}
                      theme="dark"
                    />
                  )}
                </div>
              </div>

              {config?.registrationEnabled !== false && (
                <div className="mt-6 text-center text-sm text-muted-foreground">
                  Don&apos;t have an account?{' '}
                  <a
                    href="/register"
                    className="font-semibold text-primary hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary rounded"
                  >
                    Sign up
                  </a>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </main>
  );
}
