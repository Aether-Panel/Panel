'use client';

import { useEffect, useId, useMemo, useRef, useState } from 'react';
import type { KeyboardEvent as ReactKeyboardEvent } from 'react';
import { Search } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { useIsMobile } from '@/hooks/use-mobile';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';

export interface QuickSearchItem {
    href: string;
    label: string;
    /** Section the destination belongs to, shown as a trailing hint. */
    group: string;
    icon: LucideIcon;
}

/** Case- and accent-insensitive so "configuracion" finds "Configuración". */
const normalize = (value: string) =>
    value.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();

/**
 * Header quick search: filters the navigation destinations the signed-in user
 * is allowed to open (the caller passes the already permission-filtered list)
 * and navigates on Enter or click. Desktop gets an inline field, mobile an
 * icon button that opens the same list inside a dialog.
 */
export function QuickSearch({ items, className }: { items: QuickSearchItem[]; className?: string }) {
    const { t } = useTranslations();
    const isMobile = useIsMobile();
    const [open, setOpen] = useState(false);
    const [query, setQuery] = useState('');
    const [activeIndex, setActiveIndex] = useState(0);
    const listId = useId();

    const containerRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLInputElement>(null);
    const dialogInputRef = useRef<HTMLInputElement>(null);

    const results = useMemo(() => {
        const needle = normalize(query.trim());
        if (!needle) return items;
        return items.filter(
            (item) => normalize(item.label).includes(needle) || normalize(item.href).includes(needle),
        );
    }, [items, query]);

    useEffect(() => {
        setActiveIndex(0);
    }, [query]);

    const activeId = results.length > 0 ? `${listId}-option-${activeIndex}` : undefined;

    const move = (delta: number) => {
        if (results.length === 0) return;
        setActiveIndex((current) => (current + delta + results.length) % results.length);
    };

    const navigate = (href: string) => {
        window.location.assign(href);
    };

    // Ctrl/⌘ + K focuses the field on desktop, opens the dialog on mobile.
    useEffect(() => {
        const onKeyDown = (event: globalThis.KeyboardEvent) => {
            if (event.key.toLowerCase() === 'k' && (event.metaKey || event.ctrlKey)) {
                event.preventDefault();
                setOpen(true);
                if (!isMobile) {
                    inputRef.current?.focus();
                }
            }
        };
        window.addEventListener('keydown', onKeyDown);
        return () => window.removeEventListener('keydown', onKeyDown);
    }, [isMobile]);

    // Reset when the field closes so the next open starts from the full list.
    useEffect(() => {
        if (!open) {
            setQuery('');
            setActiveIndex(0);
        }
    }, [open]);

    useEffect(() => {
        if (!open || isMobile) return;
        const onPointerDown = (event: PointerEvent) => {
            if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
                setOpen(false);
            }
        };
        document.addEventListener('pointerdown', onPointerDown);
        return () => document.removeEventListener('pointerdown', onPointerDown);
    }, [open, isMobile]);

    const onFieldKeyDown = (event: ReactKeyboardEvent<HTMLInputElement>) => {
        if (event.key === 'ArrowDown') {
            event.preventDefault();
            setOpen(true);
            move(1);
        } else if (event.key === 'ArrowUp') {
            event.preventDefault();
            setOpen(true);
            move(-1);
        } else if (event.key === 'Enter') {
            const target = results[activeIndex];
            if (open && target) {
                event.preventDefault();
                navigate(target.href);
            }
        } else if (event.key === 'Escape' && open) {
            event.preventDefault();
            setOpen(false);
        }
    };

    const resultsList = results.length === 0 ? (
        <p className="px-4 py-8 text-center text-sm text-muted-foreground">{t('header.search.noResults')}</p>
    ) : (
        <ul
            id={listId}
            role="listbox"
            aria-label={t('header.search.label')}
            className="custom-scrollbar max-h-[min(23rem,60vh)] space-y-0.5 overflow-auto overscroll-contain p-1.5"
        >
            {results.map((item, index) => {
                const selected = index === activeIndex;
                return (
                    <li key={item.href}>
                        <a
                            id={`${listId}-option-${index}`}
                            role="option"
                            aria-selected={selected}
                            href={item.href}
                            tabIndex={-1}
                            onMouseEnter={() => setActiveIndex(index)}
                            className={cn(
                                'flex min-h-9 items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm transition-colors duration-150',
                                selected ? 'bg-accent text-accent-foreground' : 'text-foreground',
                            )}
                        >
                            <item.icon
                                className={cn('h-4 w-4 shrink-0 transition-colors duration-150', selected ? 'text-foreground' : 'text-muted-foreground')}
                                aria-hidden="true"
                            />
                            <span className="min-w-0 flex-1 truncate">{item.label}</span>
                            <span className="shrink-0 text-xs font-medium text-muted-foreground">
                                {item.group}
                            </span>
                        </a>
                    </li>
                );
            })}
        </ul>
    );

    const fieldProps = {
        type: 'search' as const,
        role: 'combobox' as const,
        'aria-expanded': open,
        'aria-controls': listId,
        'aria-activedescendant': open ? activeId : undefined,
        'aria-label': t('header.search.label'),
        placeholder: t('header.search.placeholder'),
        autoComplete: 'off',
        value: query,
        onChange: (event: React.ChangeEvent<HTMLInputElement>) => {
            setQuery(event.target.value);
            setOpen(true);
        },
        onKeyDown: onFieldKeyDown,
    };

    return (
        <div className={cn('flex items-center', className)}>
            <div ref={containerRef} className="relative hidden md:block">
                <Search
                    className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
                    aria-hidden="true"
                />
                <Input
                    {...fieldProps}
                    ref={inputRef}
                    onFocus={() => setOpen(true)}
                    className="h-9 w-60 bg-background/40 pl-8 pr-3 text-sm hover:border-border lg:w-72 lg:pr-16 [&::-webkit-search-cancel-button]:hidden"
                />
                <kbd className="pointer-events-none absolute right-2 top-1/2 hidden -translate-y-1/2 rounded border border-border/60 bg-muted/70 px-1.5 py-0.5 font-mono text-[10px] leading-none text-muted-foreground lg:block">
                    {t('header.search.hint')}
                </kbd>

                {open && (
                    <div className="animate-in fade-in-0 zoom-in-95 absolute right-0 top-[calc(100%+8px)] z-50 w-80 overflow-hidden rounded-xl border border-border/60 bg-popover shadow-elevation-3 duration-150">
                        {resultsList}
                        <div aria-hidden="true" className="flex items-center gap-4 border-t border-border/50 px-3 py-2 text-[11px] text-muted-foreground">
                            <span className="inline-flex items-center gap-1.5">
                                <kbd className="rounded border border-border/60 bg-muted/70 px-1 font-mono text-[10px] leading-4">↑↓</kbd>
                                {t('header.search.navigate')}
                            </span>
                            <span className="inline-flex items-center gap-1.5">
                                <kbd className="rounded border border-border/60 bg-muted/70 px-1 font-mono text-[10px] leading-4">↵</kbd>
                                {t('header.search.select')}
                            </span>
                            <span className="inline-flex items-center gap-1.5">
                                <kbd className="rounded border border-border/60 bg-muted/70 px-1 font-mono text-[10px] leading-4">Esc</kbd>
                                {t('header.search.close')}
                            </span>
                        </div>
                    </div>
                )}
            </div>

            <Button
                variant="ghost"
                size="icon"
                aria-label={t('header.search.open')}
                onClick={() => setOpen(true)}
                className="h-9 w-9 shrink-0 rounded-lg border border-border/60 text-muted-foreground hover:border-border hover:bg-accent/60 hover:text-foreground md:hidden"
            >
                <Search aria-hidden="true" />
            </Button>

            <Dialog open={open && isMobile} onOpenChange={setOpen}>
                <DialogContent className="top-[12%] w-[calc(100%-2rem)] max-w-md translate-y-0 gap-3 rounded-xl p-4">
                    <DialogHeader className="text-left">
                        <DialogTitle className="text-base font-semibold">{t('header.search.label')}</DialogTitle>
                        <DialogDescription className="sr-only">{t('header.search.placeholder')}</DialogDescription>
                    </DialogHeader>
                    <div className="relative">
                        <Search
                            className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
                            aria-hidden="true"
                        />
                        <Input {...fieldProps} ref={dialogInputRef} autoFocus className="h-11 pl-8 text-base [&::-webkit-search-cancel-button]:hidden" />
                    </div>
                    <div className="overflow-hidden rounded-xl border border-border/60 bg-background/30">{resultsList}</div>
                </DialogContent>
            </Dialog>
        </div>
    );
}
