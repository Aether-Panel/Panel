'use client';

import { cn } from '@/lib/utils';

/**
 * Backdrop behind the workspace: the panel's own Minecraft artwork, blurred
 * and dimmed, drifting in a slow zoom under a navy veil so every surface on
 * top stays readable.
 *
 * Decorative only: it never receives events, animates transform alone, and
 * holds still under `prefers-reduced-motion` (see `.ambient` in `globals.css`).
 */
export function AmbientBackdrop({ className }: { className?: string }) {
    return (
        <div className={cn('ambient', className)} aria-hidden="true">
            <i className="backdrop-art" />
            <i className="backdrop-veil" />
        </div>
    );
}
