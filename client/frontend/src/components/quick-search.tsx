'use client';

import { Fragment, useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import type { KeyboardEvent as ReactKeyboardEvent, ReactNode } from 'react';
import { Archive, CalendarClock, CornerDownLeft, Folder, Loader2, Play, RotateCw, Search, Server as ServerIcon, Settings as SettingsIcon, Square, Terminal } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { useIsMobile } from '@/hooks/use-mobile';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';

export interface QuickSearchItem {
    href: string;
    label: string;
    /** Section the destination belongs to, shown as a trailing hint. */
    group: string;
    icon: LucideIcon;
}

/** One row of the palette: somewhere to go, or something to do. */
type PaletteItem = {
    id: string;
    label: string;
    /** Heading the row is listed under. */
    section: string;
    /** Short trailing hint (the page's sidebar group, a server's address...). */
    hint?: string;
    icon: LucideIcon;
    href?: string;
    run?: () => Promise<void>;
    /** Extra words that should find the row (e.g. "restart" for "Reiniciar"). */
    keywords?: string;
    /** Rows that only make sense once you start typing (actions, a server's tabs). */
    onlyWhenSearching?: boolean;
};

type ServerSummary = { id: string; name: string; ip?: string; port?: number };

/** Case- and accent-insensitive so "configuracion" finds "Configuración". */
const normalize = (value: string) =>
    value.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();

/** How well a row matches: a match at the start of the label beats one in the middle. */
function score(item: PaletteItem, needle: string) {
    const label = normalize(item.label);
    if (label.startsWith(needle)) return 3;
    if (label.split(/[\s›·-]+/).some((word) => word.startsWith(needle))) return 2;
    if (label.includes(needle)) return 1;
    if (item.keywords && normalize(item.keywords).includes(needle)) return 0.5;
    return 0;
}

/** The label with the typed text picked out. */
function Highlight({ text, needle }: { text: string; needle: string }): ReactNode {
    if (!needle) return text;
    const index = normalize(text).indexOf(needle);
    if (index < 0) return text;
    return (
        <>
            {text.slice(0, index)}
            <mark className="bg-transparent font-semibold text-primary-bright">{text.slice(index, index + needle.length)}</mark>
            {text.slice(index + needle.length)}
        </>
    );
}

/** The server tabs worth jumping to straight from search. */
const SERVER_TABS: { tab: string; icon: LucideIcon }[] = [
    { tab: 'console', icon: Terminal },
    { tab: 'files', icon: Folder },
    { tab: 'backups', icon: Archive },
    { tab: 'tasks', icon: CalendarClock },
    { tab: 'settings', icon: SettingsIcon },
];

const POWER_ACTIONS: { action: 'start' | 'restart' | 'stop'; icon: LucideIcon; keywords: string }[] = [
    { action: 'start', icon: Play, keywords: 'start iniciar encender arrancar' },
    { action: 'restart', icon: RotateCw, keywords: 'restart reiniciar reboot' },
    { action: 'stop', icon: Square, keywords: 'stop detener apagar parar' },
];

/**
 * Header command palette (Ctrl/⌘ + K): pages the user may open, their servers (and a
 * server's tabs), and quick power actions on a server. Desktop gets an inline field with a
 * dropdown, mobile a dialog with the same list.
 */
export function QuickSearch({ items, className }: { items: QuickSearchItem[]; className?: string }) {
    const { t } = useTranslations();
    const isMobile = useIsMobile();
    const [open, setOpen] = useState(false);
    const [query, setQuery] = useState('');
    const [activeIndex, setActiveIndex] = useState(0);
    const [servers, setServers] = useState<ServerSummary[] | null>(null);
    const [running, setRunning] = useState<string | null>(null);
    const listId = useId();

    const containerRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLInputElement>(null);
    const dialogInputRef = useRef<HTMLInputElement>(null);
    const listRef = useRef<HTMLUListElement>(null);

    // servers are loaded the first time the palette opens, not on every page
    useEffect(() => {
        if (!open || servers !== null) return;
        api.get<{ servers?: ServerSummary[] }>('/api/servers?limit=100')
            .then((data) => setServers(Array.isArray(data?.servers) ? data.servers : []))
            .catch(() => setServers([]));
    }, [open, servers]);

    const sections = {
        servers: t('header.search.sections.servers'),
        actions: t('header.search.sections.actions'),
        pages: t('header.search.sections.pages'),
    };

    const allItems = useMemo<PaletteItem[]>(() => {
        const pages: PaletteItem[] = items.map((item) => ({
            id: `page:${item.href}`,
            label: item.label,
            section: sections.pages,
            hint: item.group,
            icon: item.icon,
            href: item.href,
        }));

        const serverRows: PaletteItem[] = [];
        const actionRows: PaletteItem[] = [];
        for (const server of servers ?? []) {
            const href = `/servers/view/?id=${encodeURIComponent(server.id)}`;
            serverRows.push({
                id: `server:${server.id}`,
                label: server.name,
                section: sections.servers,
                // a listen-everywhere address says nothing; the port is what players type
                hint: server.ip && server.ip !== '0.0.0.0' ? `${server.ip}${server.port ? `:${server.port}` : ''}` : server.port ? `:${server.port}` : undefined,
                icon: ServerIcon,
                href,
                keywords: server.id,
            });
            for (const { tab, icon } of SERVER_TABS) {
                serverRows.push({
                    id: `server:${server.id}:${tab}`,
                    label: `${server.name} › ${t(`servers.detail.tabs.${tab}`)}`,
                    section: sections.servers,
                    icon,
                    href: `${href}&tab=${tab}`,
                    onlyWhenSearching: true,
                });
            }
            for (const { action, icon, keywords } of POWER_ACTIONS) {
                actionRows.push({
                    id: `action:${server.id}:${action}`,
                    label: t(`header.search.actions.${action}`, { name: server.name }),
                    section: sections.actions,
                    icon,
                    keywords: `${keywords} ${server.name}`,
                    onlyWhenSearching: true,
                    run: async () => {
                        await api.post(`/api/servers/${encodeURIComponent(server.id)}/${action}`, {});
                        sileo.success({ title: t('common.success'), description: t(`header.search.actions.${action}Done`, { name: server.name }) });
                    },
                });
            }
        }
        return [...serverRows, ...actionRows, ...pages];
        // the section names follow the language through t
    }, [items, servers, t]);

    const needle = normalize(query.trim());
    const results = useMemo(() => {
        if (!needle) {
            // nothing typed: a handful of servers, then every page
            const serverRows = allItems.filter((item) => item.section === sections.servers && !item.onlyWhenSearching).slice(0, 5);
            return [...serverRows, ...allItems.filter((item) => item.section === sections.pages)];
        }
        const ranked = allItems
            .map((item, order) => ({ item, order, score: score(item, needle) }))
            .filter((entry) => entry.score > 0)
            .sort((a, b) => b.score - a.score || a.order - b.order);
        // keep sections together, in the order of their best match
        const sectionOrder = [...new Set(ranked.map((entry) => entry.item.section))];
        return sectionOrder.flatMap((section) => ranked.filter((entry) => entry.item.section === section).slice(0, 8).map((entry) => entry.item));
    }, [allItems, needle]);

    useEffect(() => {
        setActiveIndex(0);
    }, [query]);

    // keep the highlighted row in view while moving with the arrows
    useEffect(() => {
        listRef.current?.querySelector<HTMLElement>(`[data-index="${activeIndex}"]`)?.scrollIntoView({ block: 'nearest' });
    }, [activeIndex]);

    const activeId = results.length > 0 ? `${listId}-option-${activeIndex}` : undefined;

    const move = (delta: number) => {
        if (results.length === 0) return;
        setActiveIndex((current) => (current + delta + results.length) % results.length);
    };

    const choose = useCallback(async (item: PaletteItem) => {
        if (item.href) {
            window.location.assign(item.href);
            return;
        }
        if (!item.run) return;
        setRunning(item.id);
        try {
            await item.run();
            setOpen(false);
        } catch (err) {
            sileo.error({ title: t('common.error'), description: err instanceof Error && err.message ? err.message : t('header.search.actions.failed') });
        } finally {
            setRunning(null);
        }
    }, [t]);

    // Ctrl/⌘ + K focuses the field on desktop, opens the dialog on mobile.
    useEffect(() => {
        const onKeyDown = (event: globalThis.KeyboardEvent) => {
            if (event.key.toLowerCase() === 'k' && (event.metaKey || event.ctrlKey)) {
                event.preventDefault();
                setOpen(true);
                if (!isMobile) {
                    inputRef.current?.focus();
                    inputRef.current?.select();
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
                void choose(target);
            }
        } else if (event.key === 'Escape' && open) {
            event.preventDefault();
            setOpen(false);
            inputRef.current?.blur();
        }
    };

    const resultsList = results.length === 0 ? (
        <div className="flex flex-col items-center gap-2 px-4 py-8 text-center">
            {servers === null && open ? (
                <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" aria-hidden="true" />
            ) : (
                <Search className="h-4 w-4 text-muted-foreground/60" aria-hidden="true" />
            )}
            <p className="text-sm text-muted-foreground">{t('header.search.noResults')}</p>
        </div>
    ) : (
        <ul
            ref={listRef}
            id={listId}
            role="listbox"
            aria-label={t('header.search.label')}
            className="custom-scrollbar max-h-[min(26rem,60vh)] overflow-auto overscroll-contain p-1.5"
        >
            {results.map((item, index) => {
                const selected = index === activeIndex;
                const firstOfSection = index === 0 || results[index - 1].section !== item.section;
                return (
                    <Fragment key={item.id}>
                        {firstOfSection && (
                            <li role="presentation" className={cn('px-2.5 pb-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground/80', index > 0 ? 'pt-3' : 'pt-1.5')}>
                                {item.section}
                            </li>
                        )}
                        <li
                            id={`${listId}-option-${index}`}
                            role="option"
                            aria-selected={selected}
                            data-index={index}
                            onMouseEnter={() => setActiveIndex(index)}
                            // keep focus in the field, so typing carries on after a click
                            onMouseDown={(event) => event.preventDefault()}
                            onClick={() => void choose(item)}
                            // the field drives the list with the arrow keys; this covers an option reached directly
                            onKeyDown={(event) => {
                                if (event.key === 'Enter' || event.key === ' ') {
                                    event.preventDefault();
                                    void choose(item);
                                }
                            }}
                            className={cn(
                                'relative flex min-h-9 cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm transition-colors duration-150 animate-in fade-in-0 duration-200',
                                selected ? 'bg-accent text-accent-foreground' : 'text-foreground',
                            )}
                        >
                            {/* the accent rail slides in on the highlighted row */}
                            <span
                                aria-hidden="true"
                                className={cn('absolute left-0 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-full bg-primary transition-all duration-200 ease-panel', selected ? 'opacity-100' : 'scale-y-0 opacity-0')}
                            />
                            {running === item.id ? (
                                <Loader2 className="h-4 w-4 shrink-0 animate-spin text-primary-bright" aria-hidden="true" />
                            ) : (
                                <item.icon
                                    className={cn('h-4 w-4 shrink-0 transition-colors duration-150', selected ? 'text-primary-bright' : 'text-muted-foreground')}
                                    aria-hidden="true"
                                />
                            )}
                            <span className="min-w-0 flex-1 truncate">
                                <Highlight text={item.label} needle={needle} />
                            </span>
                            {item.hint && <span className="max-w-[45%] shrink-0 truncate text-xs text-muted-foreground">{item.hint}</span>}
                            {selected && <CornerDownLeft className="h-3.5 w-3.5 shrink-0 text-muted-foreground animate-in fade-in-0 duration-150" aria-hidden="true" />}
                        </li>
                    </Fragment>
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

    const footer = (
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
    );

    return (
        <div className={cn('flex items-center', className)}>
            <div ref={containerRef} className="relative hidden md:block">
                <Search
                    className={cn('pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 transition-colors duration-200', open ? 'text-primary-bright' : 'text-muted-foreground')}
                    aria-hidden="true"
                />
                <Input
                    {...fieldProps}
                    ref={inputRef}
                    onFocus={() => setOpen(true)}
                    className={cn(
                        'h-9 w-60 bg-background/40 pl-8 pr-3 text-sm transition-[width,box-shadow,border-color] duration-300 ease-panel hover:border-border lg:w-72 lg:pr-16 [&::-webkit-search-cancel-button]:hidden',
                        open && 'lg:w-80 shadow-[0_0_0_4px_hsl(var(--primary)/0.08)]',
                    )}
                />
                <kbd className="pointer-events-none absolute right-2 top-1/2 hidden -translate-y-1/2 rounded border border-border/60 bg-muted/70 px-1.5 py-0.5 font-mono text-[10px] leading-none text-muted-foreground lg:block">
                    {t('header.search.hint')}
                </kbd>

                {open && (
                    <div className="animate-in fade-in-0 zoom-in-95 slide-in-from-top-1 absolute right-0 top-[calc(100%+8px)] z-50 w-[26rem] max-w-[calc(100vw-2rem)] overflow-hidden rounded-xl border border-border/60 bg-popover/95 shadow-elevation-3 backdrop-blur-md duration-200 ease-panel">
                        {resultsList}
                        {footer}
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
