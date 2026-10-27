'use client';
import { Providers } from '@/contexts/providers';
import SileoToaster from '@/components/SileoToaster';
import { AmbientBackdrop } from '@/components/ambient-backdrop';
import { Logo } from '@/components/logo';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { useTranslations } from '@/contexts/translations-context';
import { forwardRef, useEffect, useRef, useState } from 'react';
import type { ComponentProps, KeyboardEvent, ReactNode } from 'react';
import { ArrowBigUp, Eye, EyeOff } from 'lucide-react';
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
    /** Caption in the brand column (desktop only). */
    artTitle: string;
    artDescription: string;
    /** Line under the form, e.g. "Don't have an account? Sign up". */
    footer?: ReactNode;
    children: ReactNode;
};

/**
 * The one frame every auth screen shares: the brand column on the left
 * (a short strip with the logo on phones) and the form column on the right. Keeping it in
 * one place is what makes sign-in, sign-up and both password screens read as
 * the same product.
 */
export function AuthFrame({ title, description, artTitle, artDescription, footer, children }: Readonly<AuthFrameProps>) {
    return (
        <div className="flex min-h-[100svh] items-center justify-center px-4 py-6 sm:py-10">
            <div className="auth-glow surface-panel relative w-full max-w-5xl overflow-hidden rounded-2xl border border-border/60 shadow-elevation-3">
                <div className="grid grid-cols-1 md:min-h-[620px] md:grid-cols-2">
                    <div className="auth-brand relative overflow-hidden border-b border-border/60 md:border-b-0 md:border-r">
                        <div className="relative flex h-full flex-col justify-between gap-6 p-5 sm:p-6 md:p-10">
                            <Logo />

                            <AuthEmblem />

                            <div className="hidden md:block">
                                <h2 className="text-balance text-[1.625rem] font-semibold leading-tight tracking-[-0.03em] text-foreground">
                                    {artTitle}
                                </h2>
                                <p className="mt-3 max-w-sm text-pretty text-sm leading-relaxed text-foreground/70">
                                    {artDescription}
                                </p>
                            </div>
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

/**
 * The Aether emblem in 3D (desktop only). The flat emblem shows at once and
 * fades out when the relief is ready; three.js loads on demand, so the rest of
 * the panel never pays for it. Without WebGL the flat emblem simply stays.
 */
function AuthEmblem() {
    const canvasRef = useRef<HTMLCanvasElement>(null);
    const [state, setState] = useState<'flat' | 'ready'>('flat');

    useEffect(() => {
        const canvas = canvasRef.current;
        if (!canvas || !window.matchMedia('(min-width: 768px)').matches) {
            return;
        }
        let unmount: (() => void) | undefined;
        let cancelled = false;
        import('@/lib/emblem-3d')
            .then(({ mountEmblem }) => {
                if (cancelled) return;
                unmount = mountEmblem({ canvas, onReady: () => setState('ready'), onFail: () => setState('flat') });
            })
            .catch(() => setState('flat'));
        return () => {
            cancelled = true;
            unmount?.();
        };
    }, []);

    return (
        <div aria-hidden="true" className="relative hidden min-h-0 flex-1 md:block">
            <img
                src="/img/appicons/512.png"
                alt=""
                className={cn(
                    'absolute left-1/2 top-1/2 h-auto w-[min(75%,18rem)] -translate-x-1/2 -translate-y-1/2 drop-shadow-[0_10px_30px_hsl(var(--primary)/0.25)] transition-opacity duration-700 ease-panel',
                    state === 'ready' && 'opacity-0',
                )}
            />
            <canvas
                ref={canvasRef}
                className={cn('absolute inset-0 h-full w-full opacity-0 transition-opacity duration-700 ease-panel', state === 'ready' && 'opacity-100')}
            />
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

/**
 * Password field: `AuthInput` plus a show/hide toggle and a Caps Lock warning,
 * the two things that cause most "wrong password" attempts.
 */
export const AuthPasswordInput = forwardRef<HTMLInputElement, Omit<ComponentProps<'input'>, 'type'> & { icon: LucideIcon }>(
    ({ className, onKeyDown, onKeyUp, onBlur, ...props }, ref) => {
        const { t } = useTranslations();
        const [visible, setVisible] = useState(false);
        const [capsLock, setCapsLock] = useState(false);

        const readCapsLock = (event: KeyboardEvent<HTMLInputElement>) => {
            // getModifierState is missing on some synthetic events (autofill)
            if (typeof event.getModifierState === 'function') {
                setCapsLock(event.getModifierState('CapsLock'));
            }
        };

        return (
            <div className="space-y-1.5">
                <div className="relative">
                    <AuthInput
                        ref={ref}
                        type={visible ? 'text' : 'password'}
                        className={cn('pr-11', className)}
                        onKeyDown={(event) => {
                            readCapsLock(event);
                            onKeyDown?.(event);
                        }}
                        onKeyUp={(event) => {
                            readCapsLock(event);
                            onKeyUp?.(event);
                        }}
                        onBlur={(event) => {
                            setCapsLock(false);
                            onBlur?.(event);
                        }}
                        {...props}
                    />
                    <button
                        type="button"
                        onClick={() => setVisible((value) => !value)}
                        aria-label={visible ? t('auth.password.hide') : t('auth.password.show')}
                        aria-pressed={visible}
                        className="absolute right-1.5 top-1/2 flex h-8 w-8 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground transition-colors duration-200 ease-panel hover:bg-muted/60 hover:text-foreground focus-ring"
                    >
                        {visible ? <EyeOff className="h-4 w-4" aria-hidden="true" /> : <Eye className="h-4 w-4" aria-hidden="true" />}
                    </button>
                </div>
                <p aria-live="polite" className="min-h-0 text-xs font-medium text-warning empty:hidden">
                    {capsLock ? (
                        <span className="flex items-center gap-1.5">
                            <ArrowBigUp className="h-3.5 w-3.5" aria-hidden="true" />
                            {t('auth.password.capsLock')}
                        </span>
                    ) : null}
                </p>
            </div>
        );
    },
);
AuthPasswordInput.displayName = 'AuthPasswordInput';

/** 0 (empty) to 4 (strong), from length and character variety. */
export function passwordStrength(password: string): 0 | 1 | 2 | 3 | 4 {
    if (!password) {
        return 0;
    }
    const variety = [/[a-z]/, /[A-Z]/, /\d/, /[^A-Za-z0-9]/].filter((re) => re.test(password)).length;
    let score = 1;
    if (password.length >= 8 && variety >= 2) score = 2;
    if (password.length >= 10 && variety >= 3) score = 3;
    if (password.length >= 14 && variety >= 3) score = 4;
    return score as 1 | 2 | 3 | 4;
}

const STRENGTH_TONE = ['', 'bg-destructive-bright', 'bg-warning', 'bg-primary', 'bg-success'] as const;
const STRENGTH_TEXT = ['', 'text-destructive-bright', 'text-warning', 'text-primary-bright', 'text-success'] as const;
const STRENGTH_KEY = ['', 'weak', 'fair', 'good', 'strong'] as const;

/** Four-segment meter under a new-password field; hidden until something is typed. */
export function PasswordStrength({ password }: Readonly<{ password: string }>) {
    const { t } = useTranslations();
    const score = passwordStrength(password);
    if (score === 0) {
        return null;
    }
    return (
        <div className="flex items-center gap-3">
            <div className="grid flex-1 grid-cols-4 gap-1" aria-hidden="true">
                {[1, 2, 3, 4].map((segment) => (
                    <span
                        key={segment}
                        className={cn(
                            'h-1 rounded-full transition-colors duration-300 ease-panel',
                            segment <= score ? STRENGTH_TONE[score] : 'bg-muted',
                        )}
                    />
                ))}
            </div>
            <p aria-live="polite" className={cn('w-20 text-right text-xs font-medium', STRENGTH_TEXT[score])}>
                {t(`auth.password.strength.${STRENGTH_KEY[score]}`)}
            </p>
        </div>
    );
}

const OTP_LENGTH = 6;

type OtpInputProps = {
    value: string;
    onChange: (value: string) => void;
    /** Called once all six digits are in, so the code submits without a click. */
    onComplete?: (value: string) => void;
    disabled?: boolean;
    invalid?: boolean;
    'aria-label': string;
};

/**
 * Six boxes for a 2FA code. One real input sits on top of them, so paste,
 * the phone's "code from SMS/app" suggestion and password managers all work.
 */
export const OtpInput = forwardRef<HTMLInputElement, OtpInputProps>(
    ({ value, onChange, onComplete, disabled, invalid, 'aria-label': ariaLabel }, ref) => {
        const [focused, setFocused] = useState(false);
        const active = Math.min(value.length, OTP_LENGTH - 1);

        return (
            <div className="relative">
                <div className="grid grid-cols-6 gap-2 sm:gap-2.5" aria-hidden="true">
                    {Array.from({ length: OTP_LENGTH }, (_, slot) => {
                        const digit = value[slot];
                        const isActive = focused && slot === active;
                        return (
                            <div
                                key={slot}
                                className={cn(
                                    'relative flex h-12 items-center justify-center rounded-lg border bg-background/60 font-mono text-xl font-semibold tabular-nums text-foreground transition-[border-color,box-shadow] duration-200 ease-panel sm:h-14',
                                    invalid ? 'border-destructive-bright/70' : 'border-input',
                                    isActive && 'border-primary ring-2 ring-ring/60',
                                    digit && !invalid && !isActive && 'border-foreground/25',
                                )}
                            >
                                {digit}
                                {isActive && !digit ? (
                                    <span className="h-6 w-px animate-pulse bg-foreground motion-reduce:animate-none" />
                                ) : null}
                            </div>
                        );
                    })}
                </div>
                <input
                    ref={ref}
                    value={value}
                    onChange={(event) => {
                        const next = event.target.value.replace(/\D/g, '').slice(0, OTP_LENGTH);
                        onChange(next);
                        if (next.length === OTP_LENGTH && next !== value) {
                            onComplete?.(next);
                        }
                    }}
                    onFocus={() => setFocused(true)}
                    onBlur={() => setFocused(false)}
                    disabled={disabled}
                    aria-label={ariaLabel}
                    aria-invalid={invalid || undefined}
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    pattern="[0-9]*"
                    maxLength={OTP_LENGTH}
                    spellCheck={false}
                    className="absolute inset-0 h-full w-full cursor-text bg-transparent text-transparent caret-transparent outline-none selection:bg-transparent disabled:cursor-not-allowed"
                />
            </div>
        );
    },
);
OtpInput.displayName = 'OtpInput';

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
        <output
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
        </output>
    );
}
