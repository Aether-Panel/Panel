'use client';
import { Providers } from '@/contexts/providers';
import SileoToaster from '@/components/SileoToaster';
import { AmbientBackdrop } from '@/components/ambient-backdrop';
import { Logo } from '@/components/logo';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { forwardRef } from 'react';
import type { ComponentProps, ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';

export default function AuthShell({ children }: Readonly<{ children: ReactNode }>) {
    return (
        <Providers>
            <AmbientBackdrop />
            <main className="page-enter relative z-[1]">
                {children}
            </main>
            <SileoToaster />
        </Providers>
    );
}

type AuthFrameProps = {
    /** Heading of the form column. */
    title: string;
    /** One sentence under the heading. */
    description?: ReactNode;
    /** Caption over the owner's artwork (desktop only). */
    artTitle: string;
    artDescription: string;
    /** Line under the form, e.g. "Don't have an account? Sign up". */
    footer?: ReactNode;
    children: ReactNode;
};

/**
 * The one frame every auth screen shares: the owner's artwork on the left
 * (a short banner on phones) and the form column on the right. Keeping it in
 * one place is what makes sign-in, sign-up and both password screens read as
 * the same product.
 */
export function AuthFrame({ title, description, artTitle, artDescription, footer, children }: Readonly<AuthFrameProps>) {
    return (
        <div className="flex min-h-[100svh] items-center justify-center px-4 py-6 sm:py-10">
            <div className="auth-glow surface-panel relative w-full max-w-5xl overflow-hidden rounded-2xl border border-border/60 shadow-elevation-3">
                <div className="grid grid-cols-1 md:min-h-[620px] md:grid-cols-2">
                    <div className="relative h-28 overflow-hidden sm:h-36 md:h-auto">
                        <img
                            src="/img/Fondos/minecraft-shaders-anime-hd-wallpaper-preview.jpg"
                            alt=""
                            className="auth-art absolute inset-0 h-full w-full object-cover"
                        />
                        {/* Legibility scrims: the logo sits on the top edge, the caption on the bottom. */}
                        <div aria-hidden="true" className="absolute inset-x-0 top-0 h-24 bg-gradient-to-b from-background/70 to-transparent" />
                        <div aria-hidden="true" className="absolute inset-x-0 bottom-0 h-full bg-gradient-to-t from-card via-card/10 to-transparent md:h-2/3 md:via-card/70" />

                        <div className="absolute left-5 top-5 md:left-8 md:top-8">
                            <Logo />
                        </div>

                        <div className="absolute inset-x-0 bottom-0 hidden p-10 md:block">
                            <h2 className="text-balance text-[1.625rem] font-semibold leading-tight tracking-[-0.03em] text-foreground">
                                {artTitle}
                            </h2>
                            <p className="mt-3 max-w-sm text-pretty text-sm leading-relaxed text-foreground/75">
                                {artDescription}
                            </p>
                        </div>
                    </div>

                    <div className="flex items-center px-6 pb-8 pt-6 sm:px-12 sm:pb-12 sm:pt-10 md:py-12">
                        <div className="mx-auto w-full max-w-[380px]">
                            <h1 className="text-[1.75rem] font-semibold leading-tight tracking-[-0.035em] text-foreground">
                                {title}
                            </h1>
                            {description ? (
                                <p className="mt-2 text-pretty text-sm leading-relaxed text-muted-foreground">{description}</p>
                            ) : null}

                            <div className="mt-8">{children}</div>

                            {footer ? (
                                <p className="mt-8 text-center text-sm text-muted-foreground">{footer}</p>
                            ) : null}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}

/** Text link used across the auth screens (forgot password, sign up, sign in). */
export const authLinkClass =
    'rounded-sm font-medium text-primary-bright underline-offset-4 transition-colors duration-200 ease-panel hover:underline focus-ring';

/**
 * Auth text field: the shared `Input` with a leading icon and a visible
 * invalid state. `FormControl` passes `id`, `aria-invalid` and
 * `aria-describedby` straight through to the input.
 */
export const AuthInput = forwardRef<HTMLInputElement, ComponentProps<'input'> & { icon: LucideIcon }>(
    ({ icon: Icon, className, ...props }, ref) => (
        <div className="relative">
            <Icon
                aria-hidden="true"
                className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
            />
            <Input
                ref={ref}
                className={cn(
                    'h-11 bg-background/60 pl-10 hover:border-border aria-[invalid=true]:border-destructive-bright/70 aria-[invalid=true]:focus-visible:ring-destructive/40',
                    className,
                )}
                {...props}
            />
        </div>
    ),
);
AuthInput.displayName = 'AuthInput';

/** Shared label / message styling so field errors look the same on every screen. */
export const authLabelClass = 'text-sm font-medium text-foreground';
export const authMessageClass = 'text-xs font-medium text-destructive-bright';

type AuthNoticeProps = {
    icon: LucideIcon;
    tone: 'success' | 'warning';
    children: ReactNode;
};

/** Result panel for the "email sent" / "password changed" / "link broken" states. */
export function AuthNotice({ icon: Icon, tone, children }: Readonly<AuthNoticeProps>) {
    return (
        <div
            role="status"
            className={cn(
                'flex gap-3 rounded-xl border p-4 text-sm leading-relaxed',
                tone === 'success' ? 'border-success/25 bg-success/10' : 'border-warning/25 bg-warning/10',
            )}
        >
            <Icon
                aria-hidden="true"
                className={cn('mt-0.5 h-4 w-4 shrink-0', tone === 'success' ? 'text-success' : 'text-warning')}
            />
            <p className="text-foreground/90">{children}</p>
        </div>
    );
}
