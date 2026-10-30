'use client';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { ArrowDown, ChevronDown, ChevronUp, Maximize2, Minimize2, Search, Send, Star, Terminal, X } from '@/components/icons';
import { sileo } from '@/lib/toast';
import { useId, useState, useRef, useEffect, useMemo } from 'react';
import { createPortal } from 'react-dom';
import AISummary from './ai-summary';
import { AnsiText } from '@/lib/ansi-utils';
import { useTranslations } from '@/contexts/translations-context';
import { useSettings } from '@/hooks/use-settings';
import { cn } from '@/lib/utils';

type LogEntry = {
  id: number;
  time: string;
  message: string;
};

/** Mirrors the state of the `/socket?console` WebSocket owned by the page. */
export type ConsoleStatus = 'connecting' | 'connected' | 'disconnected';

type Level = 'error' | 'warn' | 'other';
type Filter = 'all' | 'error' | 'warn';

const HISTORY_LIMIT = 50;
const FAVORITES_LIMIT = 12;

/** How many lines the filter's chip counts; "all" shows no count. */
function filterCount(option: Filter, counts: { error: number; warn: number }): number | null {
  if (option === 'error') return counts.error;
  if (option === 'warn') return counts.warn;
  return null;
}

function levelOf(message: string): Level {
  const upper = message.toUpperCase();
  if (upper.includes('ERROR') || upper.includes('FATAL') || upper.includes('EXCEPTION')) return 'error';
  if (upper.includes('WARN')) return 'warn';
  return 'other';
}

/** A per-server list kept in this browser (command history, favourites). */
function useStoredList(key: string, limit: number) {
  const [items, setItems] = useState<string[]>([]);
  useEffect(() => {
    try {
      const stored = JSON.parse(localStorage.getItem(key) ?? '[]');
      setItems(Array.isArray(stored) ? stored.filter((v) => typeof v === 'string').slice(0, limit) : []);
    } catch {
      setItems([]);
    }
  }, [key, limit]);
  const save = (next: string[]) => {
    const trimmed = next.slice(0, limit);
    setItems(trimmed);
    try {
      localStorage.setItem(key, JSON.stringify(trimmed));
    } catch {
      // storage blocked: the list lives until the page closes
    }
  };
  return [items, save] as const;
}

/** Strips ANSI colour codes so searching matches what the reader sees. */
const plain = (text: string) => text.replace(/\x1b\[[0-9;?]*[ -/]*[@-~]/g, '');

export default function ConsoleView({
  serverId,
  logs,
  addLog,
  status = 'disconnected',
  canSend = true,
}: Readonly<{
  serverId: string;
  logs: LogEntry[];
  addLog: (message: string) => void;
  /** false for a sub-user who may read the console but not type in it */
  canSend?: boolean;
  status?: ConsoleStatus;
}>) {
  const [command, setCommand] = useState('');
  const { t } = useTranslations();
  const [history, saveHistory] = useStoredList(`consoleHistory_${serverId}`, HISTORY_LIMIT);
  const [favorites, saveFavorites] = useStoredList(`consoleFavorites_${serverId}`, FAVORITES_LIMIT);
  // -1 is the line being typed; 0 is the newest command in the history
  const historyIndex = useRef(-1);
  const typedBeforeHistory = useRef('');
  const [filter, setFilter] = useState<Filter>('all');
  const [searchOpen, setSearchOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [activeMatch, setActiveMatch] = useState(0);
  const searchRef = useRef<HTMLInputElement>(null);
  const commandRef = useRef<HTMLInputElement>(null);
  const { settings } = useSettings();
  const scrollRef = useRef<HTMLDivElement>(null);
  // "Expand" opens the console over the whole page; Esc or the same button brings it back
  const [expanded, setExpanded] = useState(false);
  const commandId = useId();

  // The only connection feedback is a short toast when the console comes back after a
  // drop; the first connection on opening the page is silent.
  const previousStatus = useRef<ConsoleStatus>(status);
  const wasConnected = useRef(false);
  useEffect(() => {
    if (status === 'connected' && previousStatus.current !== 'connected') {
      if (wasConnected.current) sileo.success({ title: t('servers.console.connectedToast') });
      wasConnected.current = true;
    }
    previousStatus.current = status;
  }, [status, t]);

  // Follow new output only while the reader is at the bottom; scrolling up to
  // read pauses it and offers a way back.
  const [pinned, setPinned] = useState(true);
  const onScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    setPinned(el.scrollHeight - el.scrollTop - el.clientHeight < 32);
  };
  const jumpToEnd = () => {
    const el = scrollRef.current;
    if (!el) return;
    el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' });
    setPinned(true);
  };

  // also when expanding or shrinking, which moves the console to a new element
  useEffect(() => {
    if (scrollRef.current && pinned) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs, pinned, expanded]);

  const handleSendCommand = async (override?: string) => {
    const cmdToSend = override ?? command;
    if (cmdToSend.trim() === '') return;
    if (override === undefined) setCommand('');
    historyIndex.current = -1;
    // newest first, without repeating the same command twice in a row
    saveHistory([cmdToSend, ...history.filter((h) => h !== cmdToSend)]);

    try {
      const res = await fetch(`/api/servers/${serverId}/console`, {
        method: 'POST',
        body: cmdToSend,
        credentials: 'include'
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        const error = data?.error;
        throw new Error(typeof error === 'string' ? error : error?.msg || t('servers.console.sendError'));
      }
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('servers.console.sendError') });
      addLog(`> ${t('servers.console.system.sendFailed', { command: cmdToSend })}`);
    }
  };

  const handleSubmit = (event: React.SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    void handleSendCommand();
  };

  // ↑ / ↓ walk the history like a terminal; Tab completes from it
  const onCommandKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowUp' && history.length > 0) {
      event.preventDefault();
      if (historyIndex.current === -1) typedBeforeHistory.current = command;
      historyIndex.current = Math.min(history.length - 1, historyIndex.current + 1);
      setCommand(history[historyIndex.current]);
    } else if (event.key === 'ArrowDown' && historyIndex.current >= 0) {
      event.preventDefault();
      historyIndex.current -= 1;
      setCommand(historyIndex.current === -1 ? typedBeforeHistory.current : history[historyIndex.current]);
    } else if (event.key === 'Tab' && command.trim() !== '') {
      const completion = history.find((h) => h.startsWith(command) && h !== command);
      if (completion) {
        event.preventDefault();
        setCommand(completion);
      }
    }
  };

  const isFavorite = command.trim() !== '' && favorites.includes(command.trim());
  const toggleFavorite = () => {
    const value = command.trim();
    if (!value) return;
    saveFavorites(isFavorite ? favorites.filter((f) => f !== value) : [value, ...favorites]);
  };

  // Neutral by default: only warnings, errors and the operator's own commands
  // carry colour, so the problems stand out from routine output.
  const getLogLevelColor = (logMessage: string) => {
    const upperMessage = logMessage.toUpperCase();
    if (upperMessage.includes('ERROR') || upperMessage.includes('FATAL') || upperMessage.includes('EXCEPTION')) return 'text-destructive-bright';
    if (upperMessage.includes('WARN')) return 'text-warning';
    if (logMessage.startsWith('$')) return 'text-success';
    if (logMessage.startsWith('>')) return 'text-muted-foreground';
    return 'text-foreground/80';
  };

  // Lines that already start with their own clock ("[19:36:40 INFO]") don't
  // repeat the time they were received.
  const hasOwnTimestamp = (logMessage: string) => /^\s*\[?\d{1,2}:\d{2}(:\d{2})?/.test(logMessage);

  const containerRef = useRef<HTMLDivElement>(null);
  const [termHeight, setTermHeight] = useState(500);
  useEffect(() => {
    if (!expanded) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setExpanded(false);
    };
    const overflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    window.addEventListener('keydown', onKey);
    commandRef.current?.focus();
    return () => {
      window.removeEventListener('keydown', onKey);
      document.body.style.overflow = overflow;
    };
  }, [expanded]);

  // The console runs down to the page's bottom padding (p-4 md:p-6 lg:p-8 in AppShell), so it
  // fills the space without scrolling the page. It is measured again when anything above it
  // changes height, such as the crash banner appearing.
  useEffect(() => {
    const measure = () => {
      if (!containerRef.current) return;
      const rect = containerRef.current.getBoundingClientRect();
      const top = rect.top + window.scrollY;
      let bottomPadding = 16;
      if (window.innerWidth >= 1024) bottomPadding = 32;
      else if (window.innerWidth >= 768) bottomPadding = 24;
      // never under 420px; for more room than the page leaves, "Expand" opens it over the page
      setTermHeight(Math.max(420, Math.round(window.innerHeight - top - bottomPadding)));
    };
    measure();
    window.addEventListener('resize', measure);
    const observer = new ResizeObserver(measure);
    observer.observe(document.body);
    return () => {
      window.removeEventListener('resize', measure);
      observer.disconnect();
    };
  }, []);

  const counts = useMemo(() => {
    let error = 0;
    let warn = 0;
    for (const log of logs) {
      const level = levelOf(log.message);
      if (level === 'error') error++;
      else if (level === 'warn') warn++;
    }
    return { error, warn };
  }, [logs]);
  const shown = useMemo(() => (filter === 'all' ? logs : logs.filter((log) => levelOf(log.message) === filter)), [logs, filter]);

  const needle = query.trim().toLowerCase();
  const matches = useMemo(
    () => (needle ? shown.filter((log) => plain(log.message).toLowerCase().includes(needle)).map((log) => log.id) : []),
    [shown, needle],
  );
  const currentMatch = matches.length > 0 ? matches[Math.min(activeMatch, matches.length - 1)] : null;
  useEffect(() => setActiveMatch(Math.max(0, matches.length - 1)), [needle, filter]);
  // bring the current match into view; reading a match pauses following new output
  useEffect(() => {
    if (currentMatch === null) return;
    setPinned(false);
    // scroll the console itself, never the page around it
    const scroller = scrollRef.current;
    const line = scroller?.querySelector<HTMLElement>(`[data-log="${currentMatch}"]`);
    if (scroller && line) {
      const target = line.offsetTop - scroller.clientHeight / 2 + line.offsetHeight / 2;
      scroller.scrollTo({ top: Math.max(0, target), behavior: 'smooth' });
    }
  }, [currentMatch]);
  const stepMatch = (delta: number) => {
    if (matches.length === 0) return;
    setActiveMatch((i) => (Math.min(i, matches.length - 1) + delta + matches.length) % matches.length);
  };
  const openSearch = () => {
    setSearchOpen(true);
    requestAnimationFrame(() => searchRef.current?.focus());
  };
  // Ctrl/⌘+F inside the console searches it instead of the page. The frame is not a control
  // itself, so it listens natively for the shortcut bubbling up from the controls inside it.
  const frameRef = useRef<HTMLDivElement>(null);
  const openSearchRef = useRef(openSearch);
  useEffect(() => {
    openSearchRef.current = openSearch;
  });
  useEffect(() => {
    const frame = frameRef.current;
    if (!frame) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === 'f' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        openSearchRef.current();
      }
    };
    frame.addEventListener('keydown', onKeyDown);
    return () => frame.removeEventListener('keydown', onKeyDown);
    // expanding moves the console to a new element: listen on that one
  }, [expanded]);
  const closeSearch = () => {
    setSearchOpen(false);
    setQuery('');
    commandRef.current?.focus();
  };

  // One rule per console so lines stay aligned: when most output already
  // carries its own clock, the received-at column is dropped for every line.
  const showTimes = useMemo(() => {
    const sample = shown.slice(-200);
    if (sample.length === 0) return false;
    const stamped = sample.filter((log) => hasOwnTimestamp(log.message)).length;
    return stamped / sample.length < 0.5;
  }, [shown]);

  let matchLabel = '';
  if (needle) matchLabel = matches.length ? `${Math.min(activeMatch, matches.length - 1) + 1}/${matches.length}` : '0/0';

  const frame = (
      <div
        className={cn(
          'relative flex flex-col overflow-hidden rounded-xl border border-border/60 bg-background shadow-elevation-1',
          expanded && 'fixed inset-3 z-[70] shadow-elevation-3 animate-in fade-in-0 zoom-in-95 duration-200 sm:inset-6',
        )}
        ref={frameRef}
        style={expanded ? undefined : { height: termHeight }}
      >
        <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-border/50 px-2 py-1.5 sm:px-3">
          <div role="radiogroup" aria-label={t('servers.console.filters.label')} className="flex items-center gap-1">
            {(['all', 'error', 'warn'] as Filter[]).map((option) => {
              const count = filterCount(option, counts);
              const active = filter === option;
              return (
                <button
                  key={option}
                  type="button"
                  role="radio"
                  aria-checked={active}
                  onClick={() => setFilter(option)}
                  className={cn(
                    'inline-flex h-7 items-center gap-1.5 rounded-md px-2.5 text-xs font-medium transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70',
                    active ? 'bg-secondary text-foreground' : 'text-muted-foreground hover:text-foreground',
                  )}
                >
                  {option !== 'all' && (
                    <span aria-hidden="true" className={cn('h-1.5 w-1.5 rounded-full', option === 'error' ? 'bg-destructive-bright' : 'bg-warning')} />
                  )}
                  {t(`servers.console.filters.${option}`)}
                  {count !== null && count > 0 && <span className="tabular-nums text-muted-foreground">{count}</span>}
                </button>
              );
            })}
          </div>

          {searchOpen ? (
            // on phones the search takes its own row under the filters
            <div className="ml-auto flex min-w-0 basis-full items-center justify-end gap-1 animate-in fade-in-0 slide-in-from-right-2 duration-200 ease-panel sm:basis-auto sm:flex-none">
              <div className="relative min-w-0 flex-1 sm:w-56 sm:flex-none">
                <Search className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
                <input
                  ref={searchRef}
                  type="search"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault();
                      stepMatch(e.shiftKey ? -1 : 1);
                    } else if (e.key === 'Escape') {
                      e.preventDefault();
                      closeSearch();
                    }
                  }}
                  aria-label={t('servers.console.search.label')}
                  placeholder={t('servers.console.search.placeholder')}
                  className="h-7 w-full rounded-md border border-border/60 bg-background/60 pl-7 pr-2 font-sans text-xs text-foreground outline-none transition-colors placeholder:text-muted-foreground/60 focus:border-primary/50 [&::-webkit-search-cancel-button]:hidden"
                />
              </div>
              <span className="w-12 shrink-0 text-center text-[11px] tabular-nums text-muted-foreground" aria-live="polite">
                {matchLabel}
              </span>
              <Button type="button" size="icon" variant="ghost" className="h-7 w-7" disabled={matches.length === 0} onClick={() => stepMatch(-1)} aria-label={t('servers.console.search.previous')}>
                <ChevronUp aria-hidden="true" />
              </Button>
              <Button type="button" size="icon" variant="ghost" className="h-7 w-7" disabled={matches.length === 0} onClick={() => stepMatch(1)} aria-label={t('servers.console.search.next')}>
                <ChevronDown aria-hidden="true" />
              </Button>
              <Button type="button" size="icon" variant="ghost" className="h-7 w-7" onClick={closeSearch} aria-label={t('servers.console.search.close')}>
                <X aria-hidden="true" />
              </Button>
            </div>
          ) : (
            <Button type="button" size="sm" variant="ghost" className="ml-auto h-7 gap-1.5 px-2 text-xs text-muted-foreground" onClick={openSearch}>
              <Search aria-hidden="true" className="!size-3.5" />
              {t('servers.console.search.open')}
              <kbd className="hidden rounded border border-border/60 bg-muted/70 px-1 font-mono text-[10px] leading-4 sm:inline">Ctrl F</kbd>
            </Button>
          )}
          <Button
            type="button"
            size="sm"
            variant="ghost"
            className="h-7 gap-1.5 px-2 text-xs text-muted-foreground"
            onClick={() => setExpanded((value) => !value)}
            aria-pressed={expanded}
          >
            {expanded ? <Minimize2 aria-hidden="true" className="!size-3.5" /> : <Maximize2 aria-hidden="true" className="!size-3.5" />}
            <span className="hidden sm:inline">{expanded ? t('servers.console.collapse') : t('servers.console.expand')}</span>
            <span className="sr-only sm:hidden">{expanded ? t('servers.console.collapse') : t('servers.console.expand')}</span>
          </Button>
        </div>

        <div
          ref={scrollRef}
          tabIndex={0}
          onScroll={onScroll}
          className="custom-scrollbar relative min-h-0 flex-1 overflow-y-auto overscroll-contain px-3 py-3 font-mono text-[13px] leading-6 sm:px-4 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/40"
        >
          {/* The scroller stays focusable for keyboard scrolling; the log role lives on its content. */}
          <div role="log" aria-label={t('servers.console.title')} aria-relevant="additions" className={cn(shown.length === 0 && 'h-full')}>
            {shown.length === 0 ? (
              <div className="flex h-full select-none flex-col items-center justify-center gap-2 text-muted-foreground">
                <Terminal className="h-5 w-5 text-muted-foreground/50" aria-hidden="true" />
                <span className="text-xs">{filter === 'all' ? t('servers.console.waitingOutput') : t(`servers.console.filters.empty.${filter}`)}</span>
              </div>
            ) : (
              shown.map((log) => {
                const levelColor = getLogLevelColor(log.message);
                const matched = needle !== '' && matches.includes(log.id);
                return (
                  <div
                    key={log.id}
                    data-log={log.id}
                    className={cn(
                      '-mx-2 flex gap-3 rounded px-2 transition-colors duration-200',
                      matched && 'bg-warning/10',
                      log.id === currentMatch && 'bg-warning/25 ring-1 ring-inset ring-warning/50',
                    )}
                  >
                    {showTimes && (
                      <span className="hidden shrink-0 select-none text-muted-foreground/50 sm:inline">
                        {log.time}
                      </span>
                    )}
                    <span className={cn('min-w-0 whitespace-pre-wrap break-words', levelColor)}>
                      <AnsiText text={log.message} />
                    </span>
                  </div>
                );
              })
            )}
          </div>
        </div>

        {!pinned && shown.length > 0 && (
          <Button
            type="button"
            size="sm"
            variant="secondary"
            onClick={jumpToEnd}
            className={cn('absolute left-1/2 h-8 -translate-x-1/2 gap-1.5 rounded-full px-3 text-xs shadow-elevation-2', favorites.length > 0 ? 'bottom-[5.75rem]' : 'bottom-16')}
          >
            <ArrowDown aria-hidden="true" className="!size-3.5" />
            {t('servers.console.jumpToEnd')}
          </Button>
        )}

        {favorites.length > 0 && (
          <fieldset className="flex min-w-0 shrink-0 items-center gap-1.5 overflow-x-auto border-t border-border/50 px-3 py-1.5 [scrollbar-width:none] sm:px-4 [&::-webkit-scrollbar]:hidden" aria-label={t('servers.console.favorites.label')}>
            <Star className="h-3 w-3 shrink-0 text-warning" weight="fill" aria-hidden="true" />
            {favorites.map((favorite) => (
              <span key={favorite} className="group inline-flex shrink-0 items-center rounded-md border border-border/60 bg-background/60 transition-colors duration-200 hover:border-primary/40">
                <button
                  type="button"
                  onClick={() => void handleSendCommand(favorite)}
                  title={t('servers.console.favorites.send', { command: favorite })}
                  className="max-w-[14rem] truncate rounded-l-md px-2 py-0.5 font-mono text-[12px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
                >
                  {favorite}
                </button>
                <button
                  type="button"
                  onClick={() => saveFavorites(favorites.filter((f) => f !== favorite))}
                  aria-label={t('servers.console.favorites.remove', { command: favorite })}
                  className="rounded-r-md px-1 py-0.5 text-muted-foreground opacity-60 transition-opacity hover:text-foreground group-hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
                >
                  <X className="h-3 w-3" aria-hidden="true" />
                </button>
              </span>
            ))}
          </fieldset>
        )}

        {/* Pinned command bar */}
        {!canSend ? (
          <p className="shrink-0 border-t border-border/60 px-4 py-2.5 text-xs text-muted-foreground">{t('servers.console.readOnly')}</p>
        ) : (
        <form
          onSubmit={handleSubmit}
          className="flex shrink-0 items-center gap-3 border-t border-border/60 px-3 py-2.5 transition-colors duration-200 focus-within:bg-surface-1/50 sm:px-4"
        >
          <label htmlFor={commandId} className="sr-only">
            {t('servers.console.commandPlaceholder')}
          </label>
          <span className="shrink-0 select-none font-mono text-sm font-semibold text-primary" aria-hidden="true">$</span>
          <Input
            id={commandId}
            type="text"
            placeholder={t('servers.console.commandPlaceholder')}
            className="h-auto border-0 bg-transparent p-0 font-mono text-[13px] text-foreground shadow-none placeholder:text-muted-foreground/60 focus-visible:border-transparent focus-visible:ring-0 focus-visible:ring-offset-0"
            ref={commandRef}
            value={command}
            onChange={(e) => {
              setCommand(e.target.value);
              historyIndex.current = -1;
            }}
            onKeyDown={onCommandKeyDown}
            autoComplete="off"
            spellCheck={false}
            aria-describedby={`${commandId}-hint`}
          />
          <span id={`${commandId}-hint`} className="sr-only">{t('servers.console.historyHint')}</span>
          <Button
            type="button"
            size="icon"
            variant="ghost"
            onClick={toggleFavorite}
            disabled={command.trim() === ''}
            aria-pressed={isFavorite}
            aria-label={isFavorite ? t('servers.console.favorites.unsave') : t('servers.console.favorites.save')}
            className="h-8 w-8 shrink-0 text-muted-foreground hover:text-warning disabled:opacity-30"
          >
            <Star aria-hidden="true" weight={isFavorite ? 'fill' : 'regular'} className={cn('transition-transform duration-200 ease-panel', isFavorite && 'scale-110 text-warning')} />
          </Button>
          <Button
            type="submit"
            size="icon"
            variant="ghost"
            disabled={command.trim() === ''}
            aria-label={t('servers.console.sendButton')}
            className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground disabled:opacity-40"
          >
            <Send aria-hidden="true" />
          </Button>
        </form>
        )}
      </div>
  );

  return (
    <div ref={containerRef} className="mt-4">
      {/* Expanded, the console leaves the page for <body>: the page's entrance animation
          would otherwise be what "fixed" is relative to, and the top bar and sidebar
          would cover it. Its place in the page keeps its height meanwhile. */}
      {expanded ? (
        <>
          <div style={{ height: termHeight }} aria-hidden="true" />
          {createPortal(
            <>
              <div aria-hidden="true" className="fixed inset-0 z-[60] bg-background/70 backdrop-blur-sm animate-in fade-in-0 duration-200" onClick={() => setExpanded(false)} />
              {frame}
            </>,
            document.body,
          )}
        </>
      ) : frame}

      {!settings['panel.settings.hideAiAnalysis'] && (
        <Card className="mt-6">
          <CardContent className="p-0">
            <AISummary logs={logs} serverId={serverId} />
          </CardContent>
        </Card>
      )}
    </div>
  );
}
