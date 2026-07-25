'use client';

import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';


import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useAuth } from '@/contexts/providers';
import { useConfig } from '@/contexts/config-context';
import { Logo } from '@/components/logo';
import { Loader2, Mail, Lock } from 'lucide-react';
import { Separator } from '@/components/ui/separator';

const formSchema = z.object({
  email: z.string().email({
    message: "Please enter a valid email address.",
  }),
  password: z.string().min(1, {
    message: "Password is required.",
  }),
});


export default function LoginPage() {
  const { login, loading } = useAuth();
  const { config } = useConfig();
  const panelName = config?.branding?.name || "Aether Panel";

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema as any),
    defaultValues: {
      email: "",
      password: "",
    },
  });

  function onSubmit(values: z.infer<typeof formSchema>) {
    login(values);
  }

  return (
    <div className="relative flex min-h-screen w-full flex-col items-center justify-center bg-background p-4">
      <div className="absolute inset-0 -z-10 bg-[radial-gradient(ellipse_at_top,hsl(var(--primary)/0.12),transparent_60%),radial-gradient(ellipse_at_bottom,hsl(var(--accent)/0.06),transparent_60%)]" />

      <div className="w-full max-w-sm">
        <Card className="border-border/60 shadow-lg">
          <CardHeader className="items-center text-center space-y-4">
            <Logo className="mb-2" />
            <CardTitle className="text-3xl font-bold">Welcome Back</CardTitle>
            <CardDescription>Sign in to your {panelName} account</CardDescription>
          </CardHeader>
          <CardContent>
            <Form {...form}>
              <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
                <FormField
                  control={form.control}
                  name="email"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Email</FormLabel>
                      <FormControl>
                        <div className="relative">
                          <Mail className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                          <Input placeholder={`admin@${panelName.toLowerCase().replace(/\s+/g, '')}.com`} {...field} className="pl-10" />
                        </div>
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
                      <FormLabel>Password</FormLabel>
                      <FormControl>
                        <div className="relative">
                          <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                          <Input type="password" placeholder="••••••••" {...field} className="pl-10" />
                        </div>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <Button
                  size="lg"
                  type="submit"
                  className="w-full text-base py-6 bg-gradient-to-r from-primary to-accent hover:from-primary/90 hover:to-accent/90 shadow-lg shadow-primary/20"
                  disabled={loading}
                >
                  {loading && <Loader2 className="mr-2 h-5 w-5 animate-spin" />}
                  Sign In
                </Button>
              </form>
            </Form>
          </CardContent>
          <CardFooter className="flex flex-col gap-4 text-center">
            {config?.registrationEnabled !== false && (
              <div className="w-full">
                <p className="text-sm text-muted-foreground">
                  Don&apos;t have an account?{' '}
                  <a href="/register" className="font-semibold text-primary hover:underline">
                    Sign Up
                  </a>
                </p>
              </div>
            )}
            <div className="w-full flex items-center gap-4 text-xs text-muted-foreground my-2">
              <Separator className="flex-1" />
              <span>For Demo</span>
              <Separator className="flex-1" />
            </div>
            <div className="text-xs text-muted-foreground">
              <p>Use <span className="font-semibold text-foreground">admin@aether.panel</span> for admin access.</p>
              <p>Use <span className="font-semibold text-foreground">devops@aether.panel</span> for user access.</p>
              <p>(Any password will work)</p>
            </div>
            <p className="text-xs text-muted-foreground pt-6 mt-4 border-t border-border/50 w-full">
              © 2024 {panelName}. All rights reserved.
            </p>
          </CardFooter>
        </Card>
      </div>
    </div>
  );
}
