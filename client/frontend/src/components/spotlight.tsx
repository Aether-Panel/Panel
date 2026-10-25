'use client';

import { useEffect } from 'react';

/**
 * Pointer tracker for `[data-spotlight]` panels: one passive listener on the
 * window writes the pointer position, relative to the panel, into `--mx` and
 * `--my`, and the CSS in `globals.css` draws the glow from there. Mounted once
 * in the app shell; renders nothing. Does nothing on touch devices, where
 * there is no hover to light up.
 */
export function SpotlightTracker() {
    useEffect(() => {
        if (!window.matchMedia('(hover: hover)').matches) return;

        const onMove = (event: PointerEvent) => {
            const target = event.target as Element | null;
            const panel = target?.closest?.<HTMLElement>('[data-spotlight]');
            if (!panel) return;
            const rect = panel.getBoundingClientRect();
            panel.style.setProperty('--mx', `${event.clientX - rect.left}px`);
            panel.style.setProperty('--my', `${event.clientY - rect.top}px`);
        };

        window.addEventListener('pointermove', onMove, { passive: true });
        return () => window.removeEventListener('pointermove', onMove);
    }, []);

    return null;
}
