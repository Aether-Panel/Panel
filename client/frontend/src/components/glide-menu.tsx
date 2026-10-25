'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

type Rect = { top: number; height: number };

/**
 * A menu whose hover highlight is one pill that glides between items instead
 * of a background that switches on and off per item. The pill follows the
 * pointer (and keyboard focus); when the pointer leaves it returns to the
 * active item, or fades out if the menu has none. Items are found through
 * `[data-sidebar="menu-button"]`, so the sidebar primitives need no changes.
 */
export function GlideMenu({ className, children }: Readonly<{ className?: string; children: ReactNode }>) {
    const listRef = useRef<HTMLUListElement>(null);
    const [rect, setRect] = useState<Rect | null>(null);
    const [visible, setVisible] = useState(false);

    const measure = useCallback((el: HTMLElement | null) => {
        const list = listRef.current;
        if (!el || !list) return null;
        const listBox = list.getBoundingClientRect();
        const box = el.getBoundingClientRect();
        return { top: box.top - listBox.top, height: box.height };
    }, []);

    const activeButton = useCallback(
        () => listRef.current?.querySelector<HTMLElement>('[data-sidebar="menu-button"][data-active="true"]') ?? null,
        [],
    );

    const settle = useCallback(() => {
        const active = activeButton();
        const next = measure(active);
        if (next) {
            setRect(next);
            setVisible(true);
        } else {
            setVisible(false);
        }
    }, [activeButton, measure]);

    // Start on the active item, and keep up with it when the list reflows.
    useEffect(() => {
        settle();
        const list = listRef.current;
        if (!list) return;
        const observer = new ResizeObserver(settle);
        observer.observe(list);
        return () => observer.disconnect();
    }, [settle]);

    const moveTo = (target: EventTarget | null) => {
        const button = (target as Element | null)?.closest?.<HTMLElement>('[data-sidebar="menu-button"]');
        if (!button || !listRef.current?.contains(button)) return;
        const next = measure(button);
        if (!next) return;
        setRect(next);
        setVisible(true);
    };

    return (
        <ul
            ref={listRef}
            data-sidebar="menu"
            className={cn('relative flex w-full min-w-0 flex-col gap-1.5', className)}
            onPointerOver={(event) => moveTo(event.target)}
            onPointerLeave={settle}
            onFocusCapture={(event) => moveTo(event.target)}
            onBlurCapture={(event) => {
                // Only settle when focus leaves the whole menu.
                if (!listRef.current?.contains(event.relatedTarget as Node | null)) settle();
            }}
        >
            <span
                aria-hidden="true"
                className={cn(
                    'glide pointer-events-none absolute left-0 right-0 top-0 rounded-lg bg-sidebar-accent',
                    visible ? 'opacity-100' : 'opacity-0',
                )}
                style={rect ? { transform: `translateY(${rect.top}px)`, height: rect.height } : undefined}
            />
            {children}
        </ul>
    );
}
