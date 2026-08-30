'use client';

import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Form, FormField, FormItem, FormMessage } from '@/components/ui/form';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { useAuth } from '@/contexts/providers';
import { useConfig } from '@/contexts/config-context';
import { Loader2, Mail, Lock } from 'lucide-react';
import Turnstile from '@/components/Turnstile';
import { useState } from 'react';
import { Logo } from '@/components/logo';

const formSchema = z.object({
  email: z.string().email({
    message: 'Please enter a valid email address.',
  }),
  password: z.string().min(1, {
    message: 'Password is required.',
  }),
});

function DiscordIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <path d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028 14.09 14.09 0 0 0 1.226-1.994.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z" />
    </svg>
  );
}

function SteamIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <path d="M11.979 0C5.678 0 .511 4.86.022 11.037l6.432 2.658a3.387 3.387 0 0 1 1.913-.59c.064 0 .125.002.187.006l2.861-4.142V8.91a3.527 3.527 0 0 1 3.521-3.521 3.527 3.527 0 0 1 3.521 3.521 3.527 3.527 0 0 1-3.521 3.521h-.105l-4.076 2.911c0 .052.004.105.004.159 0 1.875-1.515 3.396-3.39 3.396a3.406 3.406 0 0 1-3.362-2.898L.436 15.27C1.862 20.307 6.486 24 11.979 24c6.627 0 12.001-5.373 12.001-12S18.606 0 11.979 0zM7.54 18.21l-1.473-.61a2.032 2.032 0 0 0 1.8-3.375l.002.001.138.085a4.17 4.17 0 0 0-.467 1.9zm4.439-6.794a2.017 2.017 0 1 0 0 4.035 2.017 2.017 0 0 0 0-4.035zm5.86 2.55a2.528 2.528 0 0 0-2.522-2.521 2.528 2.528 0 0 0-2.521 2.521 2.528 2.528 0 0 0 2.521 2.522 2.528 2.528 0 0 0 2.522-2.522z" />
    </svg>
  );
}

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
    <main className="flex min-h-screen items-center justify-center bg-[#0F141D] px-4 py-8">
      <div className="w-full max-w-5xl overflow-hidden rounded-2xl border border-[#252a36]/60 bg-[#121620] shadow-2xl shadow-black/40">
        <div className="grid grid-cols-1 md:grid-cols-2">

          {/* ── Left Column: Image + Welcome ── */}
          <div className="relative hidden md:block">
            <img
              src="/img/Fondos/minecraft-shaders-anime-hd-wallpaper-preview.jpg"
              alt=""
              className="absolute inset-0 h-full w-full object-cover"
            />

            {/* Logo */}
            <div className="absolute top-8 left-8">
              <Logo />
            </div>

            {/* Bottom glassmorphism strip */}
            <div className="absolute bottom-0 left-0 w-full bg-gradient-to-t from-[#121620] via-[#121620]/70 to-transparent p-10 backdrop-blur-sm">
              <h2 className="text-2xl font-bold tracking-tight text-white">
                Welcome Back to {panelName}
              </h2>
              <p className="mt-3 max-w-sm text-sm leading-relaxed text-white/70">
                Manage your game servers, monitor live resources and keep your services online — all in one place.
              </p>
            </div>
          </div>

          {/* ── Right Column: Form ── */}
          <div className="flex items-center px-8 py-10 sm:px-12">
            <div className="w-full max-w-[400px]">
              <h1 className="text-3xl font-bold tracking-tight text-[#f0f1f3]">Sign in</h1>
              <p className="mt-2 text-sm text-[#8a919c]">
                Enter your credentials to access your account
              </p>

              <Form {...form}>
                <form onSubmit={form.handleSubmit(onSubmit)} className="mt-8 space-y-5">
                  <FormField
                    control={form.control}
                    name="email"
                    render={({ field }) => (
                      <FormItem>
                        <label className="mb-2 inline-block text-xs font-semibold uppercase tracking-wider text-[#f0f1f3]">
                          Email
                        </label>
                        <div className="relative">
                          <Mail className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[#8a919c]" />
                          <input
                            {...field}
                            type="email"
                            placeholder={demoEmail}
                            className="w-full rounded-lg border border-[#252a36] bg-[#0a0e14] py-2.5 pl-10 pr-3 text-sm text-[#f0f1f3] transition-colors placeholder:text-[#8a919c]/60 focus:border-[#3b82f6] focus:outline-none focus:ring-1 focus:ring-[#3b82f6]/20"
                          />
                        </div>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="password"
                    render={({ field }) => (
                      <FormItem>
                        <label className="mb-2 inline-block text-xs font-semibold uppercase tracking-wider text-[#f0f1f3]">
                          Password
                        </label>
                        <div className="relative">
                          <Lock className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[#8a919c]" />
                          <input
                            {...field}
                            type="password"
                            placeholder="••••••••"
                            className="w-full rounded-lg border border-[#252a36] bg-[#0a0e14] py-2.5 pl-10 pr-3 text-sm text-[#f0f1f3] transition-colors placeholder:text-[#8a919c]/60 focus:border-[#3b82f6] focus:outline-none focus:ring-1 focus:ring-[#3b82f6]/20"
                          />
                        </div>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Checkbox id="remember" className="rounded" />
                      <Label htmlFor="remember" className="text-sm text-[#8a919c]">
                        Remember me
                      </Label>
                    </div>
                    <a
                      href="/forgot-password"
                      className="text-sm font-medium text-[#22c55e] transition-colors hover:text-[#22c55e]/80 focus:outline-none focus-visible:ring-2 focus-visible:ring-[#22c55e] rounded"
                    >
                      Forgot password?
                    </a>
                  </div>

                  <Button
                    type="submit"
                    className="h-auto w-full rounded-lg bg-[#3b82f6] py-2.5 px-3.5 text-sm font-semibold text-white shadow-sm shadow-[#3b82f6]/20 transition-all duration-200 hover:bg-[#3b82f6]/90 active:scale-[0.98] focus:outline-none focus-visible:ring-2 focus-visible:ring-[#3b82f6] cursor-pointer"
                    disabled={loading}
                  >
                    {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                    Sign in
                  </Button>
                </form>
              </Form>

              {/* ── OR Separator ── */}
              <div className="my-6 flex items-center gap-4">
                <div className="h-px flex-1 bg-[#252a36]" />
                <span className="text-xs font-semibold uppercase tracking-wider text-[#8a919c]">OR</span>
                <div className="h-px flex-1 bg-[#252a36]" />
              </div>

              {/* ── Social Buttons ── */}
              <div className="grid grid-cols-2 gap-4">
                <button
                  type="button"
                  className="flex items-center justify-center gap-2.5 rounded-lg border border-[#252a36] bg-transparent py-2.5 text-sm font-medium text-[#f0f1f3] transition-all duration-200 hover:border-[#5865F2]/50 hover:bg-[#5865F2]/10 hover:text-[#5865F2] active:scale-[0.98] cursor-pointer"
                >
                  <DiscordIcon className="h-4 w-4" />
                  Discord
                </button>
                <button
                  type="button"
                  className="flex items-center justify-center gap-2.5 rounded-lg border border-[#252a36] bg-transparent py-2.5 text-sm font-medium text-[#f0f1f3] transition-all duration-200 hover:border-[#1b2838]/50 hover:bg-[#66c0f4]/10 hover:text-[#66c0f4] active:scale-[0.98] cursor-pointer"
                >
                  <SteamIcon className="h-4 w-4" />
                  Steam
                </button>
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

              {config?.registrationEnabled !== false && (
                <div className="mt-8 text-center text-sm text-[#8a919c]">
                  Don&apos;t have an account?{' '}
                  <a
                    href="/register"
                    className="font-semibold text-[#22c55e] transition-colors hover:text-[#22c55e]/80 focus:outline-none focus-visible:ring-2 focus-visible:ring-[#22c55e] rounded"
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
