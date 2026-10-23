'use client';

import { useLayoutEffect, useRef, useState } from 'react';
import type { LucideIcon } from 'lucide-react';
import { ChevronDown } from 'lucide-react';
import { TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';

export type ServerTab = { value: string; label: string; icon: LucideIcon };

/** Gap between triggers, in px (matches `gap-1`). */
const GAP = 4;

const triggerClass =
    "relative h-11 shrink-0 gap-2 rounded-none bg-transparent px-3 text-[13px] font-medium text-muted-foreground shadow-none transition-colors duration-200 hover:text-foreground focus-visible:ring-inset focus-visible:ring-offset-0 data-[state=active]:bg-transparent data-[state=active]:text-foreground data-[state=active]:shadow-none after:absolute after:inset-x-3 after:-bottom-px after:h-0.5 after:rounded-full after:bg-primary after:opacity-0 after:transition-opacity after:duration-200 data-[state=active]:after:opacity-100 [&_svg]:size-4 [&_svg]:shrink-0";

/**
 * One-row tab bar for the server page ("priority+" navigation): as many tabs
 * as fit are shown in order, the rest move into a "More" menu at the end. The
 * split is measured from an invisible copy of the row, so it follows the
 * available width (window size, sidebar open or closed) without wrapping.
 * When the active tab lives in the menu, the menu button carries its name.
 */
export function ServerTabNav({
    tabs,
    value,
    onValueChange,
    moreLabel,
}: {
    tabs: ServerTab[];
    value: string;
    onValueChange: (value: string) => void;
    moreLabel: string;
}) {
    const rowRef = useRef<HTMLDivElement>(null);
    const measureRef = useRef<HTMLDivElement>(null);
    const [visibleCount, setVisibleCount] = useState(tabs.length);

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
                    </span>
                ))}
                <span data-measure="more" className={cn(triggerClass, 'inline-flex items-center')}>
                    {moreLabel}: {tabs.reduce((longest, tab) => (tab.label.length > longest.length ? tab.label : longest), '')}
                    <ChevronDown />
                </span>
            </div>
            </div>

            <TabsList className="flex h-auto w-full items-stretch justify-start gap-1 rounded-none border-b border-border/60 bg-transparent p-0">
                {visible.map((tab) => (
                    <TabsTrigger key={tab.value} value={tab.value} className={triggerClass}>
                        <tab.icon aria-hidden="true" />
                        {tab.label}
                    </TabsTrigger>
                ))}

                {overflow.length > 0 && (
                    <DropdownMenu>
                        <DropdownMenuTrigger
                            data-active={activeOverflow ? 'true' : undefined}
                            className={cn(triggerClass, 'inline-flex items-center outline-none focus-visible:ring-2 focus-visible:ring-ring data-[active=true]:text-foreground data-[active=true]:after:opacity-100')}
                        >
                            {activeOverflow ? (
                                <>
                                    <activeOverflow.icon aria-hidden="true" />
                                    {activeOverflow.label}
                                </>
                            ) : (
                                moreLabel
                            )}
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
                                </DropdownMenuItem>
                            ))}
                        </DropdownMenuContent>
                    </DropdownMenu>
                )}
            </TabsList>
        </div>
    );
}
