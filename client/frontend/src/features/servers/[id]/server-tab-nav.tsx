'use client';

import { useLayoutEffect, useRef, useState } from 'react';
import type { AppIcon } from '@/components/icons';
import { ChevronDown } from '@/components/icons';
import { TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import { BetaBadge } from '@/components/beta-badge';

/** `beta` marks a tab for an experimental feature. */
export type ServerTab = { value: string; label: string; icon: AppIcon; beta?: boolean };

/** Gap between triggers, in px (matches `gap-1`). */
const GAP = 4;

const triggerClass =
    "relative h-11 shrink-0 gap-2 rounded-none bg-transparent px-3 text-[13px] font-medium text-muted-foreground shadow-none transition-colors duration-200 hover:text-foreground focus-visible:ring-inset focus-visible:ring-offset-0 data-[state=active]:bg-transparent data-[state=active]:text-foreground data-[state=active]:shadow-none [&_svg]:size-4 [&_svg]:shrink-0 [&_svg]:transition-colors [&_svg]:duration-200 data-[state=active]:[&_svg]:text-primary";

/**
 * One-row tab bar for the server page ("priority+" navigation): as many tabs
 * as fit are shown in order, the rest move into a "More" menu at the end. The
 * split is measured from an invisible copy of the row, so it follows the
 * available width (window size, sidebar open or closed) without wrapping.
 * When the active tab lives in the menu it is still shown as a real tab, next
 * to the menu button, so assistive tech always finds the selected tab and the
 * panel it labels. The menu button sits outside the tablist, which may only
 * contain tabs.
 */
export function ServerTabNav({
    tabs,
    value,
    onValueChange,
    moreLabel,
}: Readonly<{
    tabs: ServerTab[];
    value: string;
    onValueChange: (value: string) => void;
    moreLabel: string;
}>) {
    const rowRef = useRef<HTMLDivElement>(null);
    const measureRef = useRef<HTMLDivElement>(null);
    const listRef = useRef<HTMLDivElement>(null);
    const [visibleCount, setVisibleCount] = useState(tabs.length);
    // The single underline slides to whichever tab is active, instead of each tab drawing its own.
    const [indicator, setIndicator] = useState<{ left: number; width: number } | null>(null);

    useLayoutEffect(() => {
        const row = rowRef.current;
        const measure = measureRef.current;
        if (!row || !measure) return;

        const compute = () => {
            const items = Array.from(measure.querySelectorAll<HTMLElement>('[data-measure="tab"]'));
            const more = measure.querySelector<HTMLElement>('[data-measure="more"]');
            const available = row.clientWidth;
            const widths = items.map((el) => el.offsetWidth);
            const total = widths.reduce((sum, w) => sum + w, 0) + GAP * Math.max(0, widths.length - 1);
            if (total <= available) {
                setVisibleCount(widths.length);
                return;
            }
            const room = available - (more?.offsetWidth ?? 0) - GAP;
            let used = 0;
            let count = 0;
            for (const w of widths) {
                const next = used + (count > 0 ? GAP : 0) + w;
                if (next > room) break;
                used = next;
                count += 1;
            }
            setVisibleCount(Math.max(1, count));
        };

        compute();
        const observer = new ResizeObserver(compute);
        observer.observe(row);
        // Web fonts change label widths once they load.
        void document.fonts?.ready.then(compute);
        return () => observer.disconnect();
    }, [tabs]);

    const visible = tabs.slice(0, visibleCount);
    const overflow = tabs.slice(visibleCount);
    const activeOverflow = overflow.find((tab) => tab.value === value);

    useLayoutEffect(() => {
        const list = listRef.current;
        if (!list) return;
        const place = () => {
            const active = list.querySelector<HTMLElement>('[role="tab"][data-state="active"]');
            if (!active) {
                setIndicator(null);
                return;
            }
            // 12px inset on each side matches the trigger's horizontal padding.
            setIndicator({ left: active.offsetLeft + 12, width: Math.max(0, active.offsetWidth - 24) });
        };
        place();
        const observer = new ResizeObserver(place);
        observer.observe(list);
        void document.fonts?.ready.then(place);
        return () => observer.disconnect();
    }, [value, visibleCount, tabs]);

    return (
        <div ref={rowRef} className="relative">
            {/* Invisible copy of the full row, used only for measuring. The
                zero-height clipped frame keeps it from widening the page. */}
            <div aria-hidden="true" className="pointer-events-none invisible absolute inset-x-0 top-0 h-0 overflow-hidden">
            <div ref={measureRef} className="flex w-max gap-1 whitespace-nowrap">
                {tabs.map((tab) => (
                    <span key={tab.value} data-measure="tab" className={cn(triggerClass, 'inline-flex items-center')}>
                        <tab.icon />
                        {tab.label}
                        {tab.beta && <BetaBadge />}
                    </span>
                ))}
                {/* Worst case after the visible tabs: the longest tab shown as the
                    active one, plus the menu button. */}
                <span data-measure="more" className="inline-flex gap-1">
                    <span className={cn(triggerClass, 'inline-flex items-center')}>
                        <ChevronDown />
                        {tabs.reduce((longest, tab) => (tab.label.length > longest.length ? tab.label : longest), '')}
                        {tabs.some((tab) => tab.beta) && <BetaBadge />}
                    </span>
                    <span className={cn(triggerClass, 'inline-flex items-center')}>
                        {moreLabel}
                        <ChevronDown />
                    </span>
                </span>
            </div>
            </div>

            <div className="flex items-stretch gap-1 border-b border-border/60">
                <TabsList ref={listRef} className="relative flex h-auto items-stretch justify-start gap-1 rounded-none bg-transparent p-0">
                    <span
                        aria-hidden="true"
                        className={cn(
                            'glide pointer-events-none absolute -bottom-px left-0 h-0.5 rounded-full bg-primary shadow-[0_0_10px_hsl(var(--primary)/0.7)]',
                            indicator ? 'opacity-100' : 'opacity-0',
                        )}
                        style={indicator ? { transform: `translateX(${indicator.left}px)`, width: indicator.width } : undefined}
                    />
                    {visible.map((tab) => (
                        <TabsTrigger key={tab.value} value={tab.value} className={triggerClass}>
                            <tab.icon aria-hidden="true" weight={tab.value === value ? 'duotone' : 'regular'} />
                            {tab.label}
                            {tab.beta && <BetaBadge />}
                        </TabsTrigger>
                    ))}
                    {activeOverflow && (
                        <TabsTrigger value={activeOverflow.value} className={triggerClass}>
                            <activeOverflow.icon aria-hidden="true" weight="duotone" />
                            {activeOverflow.label}
                            {activeOverflow.beta && <BetaBadge />}
                        </TabsTrigger>
                    )}
                </TabsList>

                {overflow.length > 0 && (
                    <DropdownMenu>
                        <DropdownMenuTrigger
                            className={cn(triggerClass, 'inline-flex items-center outline-none focus-visible:ring-2 focus-visible:ring-ring')}
                        >
                            {moreLabel}
                            <ChevronDown aria-hidden="true" className="!size-3.5 opacity-70" />
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="min-w-48">
                            {overflow.map((tab) => (
                                <DropdownMenuItem
                                    key={tab.value}
                                    onSelect={() => onValueChange(tab.value)}
                                    className={cn('gap-2 py-2', tab.value === value && 'bg-accent text-foreground')}
                                >
                                    <tab.icon aria-hidden="true" />
                                    {tab.label}
                                    {tab.beta && <BetaBadge className="ml-auto" />}
                                </DropdownMenuItem>
                            ))}
                        </DropdownMenuContent>
                    </DropdownMenu>
                )}
            </div>
        </div>
    );
}
