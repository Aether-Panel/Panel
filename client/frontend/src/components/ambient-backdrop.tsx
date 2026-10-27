'use client';

import { cn } from '@/lib/utils';

/**
 * Backdrop behind the workspace: soft pools of the brand colours under a veil
 * so every surface on top stays readable.
 *
 * Decorative only and static: it never receives events (see `.ambient` in `globals.css`).
 */
export function AmbientBackdrop({ className }: Readonly<{ className?: string }>) {
    return (
        <div className={cn('ambient', className)} aria-hidden="true">
            <i className="backdrop-art" />
            <i className="backdrop-veil" />
        </div>
    );
}
