'use client';
import { useCallback, useEffect, useRef, useState, type ReactNode, type RefObject } from 'react';
import { AlertTriangle, ChevronRight, History, RotateCw } from '@/components/icons';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { EmptyState } from '@/components/empty-state';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { cn, formatBytes } from '@/lib/utils';

export type FileVersion = {
  id: string;
  /** When this text was written to the file, unix ms. */
  savedAt: number;
  size: number;
};

type HistoryStatus = 'idle' | 'loading' | 'ready' | 'error';

/** How many versions the node keeps per file (internal/servers/file_history.go). */
export const FILE_HISTORY_LIMIT = 20;

// Same list the node keeps versions for; files without an extension are kept when they are text
const TRACKED_EXTENSIONS = new Set([
  'properties', 'yml', 'yaml', 'json', 'json5', 'toml', 'conf', 'cfg', 'ini', 'txt', 'xml',
  'sk', 'js', 'ts', 'lua', 'mcmeta', 'cnf', 'env',
]);

/** Whether saving this file keeps its previous text, so the history button is worth showing. */
export function isHistoryTracked(name: string) {
  const dot = name.lastIndexOf('.');
  if (dot < 0) return true;
  // ".env" is both a name and an extension
  return TRACKED_EXTENSIONS.has(name.slice(dot + 1).toLowerCase());
}

export function localeFor(language: string) {
  return language === 'es' ? 'es-ES' : 'en-US';
}

/** "hace 5 minutos" / "5 minutes ago", from the active locale. */
export function formatRelative(then: number, locale: string, now = Date.now()): string {
  const seconds = Math.min(0, Math.round((then - now) / 1000));
  const magnitude = Math.abs(seconds);
  // "hace 30 segundos" reads better than "ahora" in "la versión de …"; "ayer" is kept for days
  const exact = new Intl.RelativeTimeFormat(locale, { numeric: 'always' });
  const loose = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' });

  if (magnitude < 60) return exact.format(Math.min(seconds, -1), 'second');
  if (magnitude < 3600) return exact.format(Math.round(seconds / 60), 'minute');
  if (magnitude < 86400) return exact.format(Math.round(seconds / 3600), 'hour');
  if (magnitude < 86400 * 30) return loose.format(Math.round(seconds / 86400), 'day');
  if (magnitude < 86400 * 365) return loose.format(Math.round(seconds / (86400 * 30)), 'month');
  return loose.format(Math.round(seconds / (86400 * 365)), 'year');
}

function capitalize(text: string, locale: string) {
  return text.charAt(0).toLocaleUpperCase(locale) + text.slice(1);
}

export function formatExact(then: number, locale: string) {
  return new Date(then).toLocaleString(locale, { dateStyle: 'full', timeStyle: 'medium' });
}

function formatShort(then: number, locale: string) {
  return new Date(then).toLocaleString(locale, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

/**
 * The stored versions of one file. The list is loaded when the editor opens (for the count on
 * the button) and refreshed after each save or restore; version texts are cached while the
 * same file stays open.
 */
export function useFileHistory(serverId: string, path: string | null) {
  const [versions, setVersions] = useState<FileVersion[]>([]);
  const [status, setStatus] = useState<HistoryStatus>('idle');
  const [error, setError] = useState('');
  const requestRef = useRef(0);
  const contentCache = useRef(new Map<string, string>());

  const query = path === null ? '' : `?path=${encodeURIComponent(path)}`;
  const base = `/api/servers/${serverId}/file-history`;

  const refresh = useCallback(
    async (quiet = false) => {
      if (path === null) return;
      const request = ++requestRef.current;
      if (!quiet) setStatus('loading');
      try {
        const data = await api.get<FileVersion[]>(`${base}${query}`);
        if (request !== requestRef.current) return;
        setVersions(Array.isArray(data) ? data : []);
        setStatus('ready');
      } catch (e: any) {
        if (request !== requestRef.current) return;
        setError(e?.message || '');
        // a quiet refresh keeps the list it already has
        if (!quiet) setStatus('error');
      }
    },
    [base, path, query],
  );

  useEffect(() => {
    requestRef.current += 1;
    contentCache.current = new Map();
    setVersions([]);
    setError('');
    setStatus('idle');
    if (path !== null) void refresh();
  }, [path, refresh]);

  const getContent = useCallback(
    async (id: string) => {
      const cached = contentCache.current.get(id);
      if (cached !== undefined) return cached;
      const data = await api.get<string>(`${base}/${encodeURIComponent(id)}${query}`);
      const text = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
      contentCache.current.set(id, text);
      return text;
    },
    [base, query],
  );

  const restore = useCallback(
    async (id: string) => {
      await api.post(`${base}/${encodeURIComponent(id)}/restore${query}`, {});
    },
    [base, query],
  );

  return { versions, status, error, refresh, getContent, restore };
}

interface FileHistorySheetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  fileName: string;
  versions: FileVersion[];
  status: HistoryStatus;
  error: string;
  selectedId: string | null;
  onRetry: () => void;
  onSelect: (version: FileVersion) => void;
  /** Where the focus goes when the panel closes without a pick (the button that opened it). */
  returnFocusRef?: RefObject<HTMLElement | null>;
}

/** The timeline dot: the picked version, the newest one, or any other. */
function dotColor(selected: boolean, latest: boolean) {
  if (selected) return 'bg-primary-bright';
  return latest ? 'bg-foreground/70' : 'bg-muted-foreground/60';
}

/** Side panel listing a file's stored versions, newest first. */
export function FileHistorySheet({
  open,
  onOpenChange,
  fileName,
  versions,
  status,
  error,
  selectedId,
  onRetry,
  onSelect,
  returnFocusRef,
}: Readonly<FileHistorySheetProps>) {
  const { t, language } = useTranslations();
  const locale = localeFor(language);
  // picking a version hands the focus to the comparison instead of back to the trigger
  const pickedRef = useRef(false);
  const now = Date.now();

  const pick = (version: FileVersion) => {
    pickedRef.current = true;
    onSelect(version);
  };

  let body: ReactNode;
  if (status === 'loading' || status === 'idle') {
    body = (
      <div role="status" aria-live="polite">
        <span className="sr-only">{t('servers.fileHistory.loading')}</span>
        <HistorySkeleton />
      </div>
    );
  } else if (status === 'error') {
    body = (
      <EmptyState
        size="sm"
        tone="error"
        icon={AlertTriangle}
        title={t('servers.fileHistory.errorTitle')}
        description={error || t('servers.fileHistory.errorDescription')}
        action={
          <Button variant="outline" size="sm" onClick={onRetry}>
            <RotateCw aria-hidden="true" />
            {t('common.retry')}
          </Button>
        }
        className="mx-1 mt-1"
      />
    );
  } else if (versions.length === 0) {
    body = (
      <EmptyState
        size="sm"
        icon={History}
        title={t('servers.fileHistory.emptyTitle')}
        description={t('servers.fileHistory.emptyDescription')}
        className="mx-1 mt-1"
      />
    );
  } else {
    body = (
      <ol className="space-y-0.5" aria-label={t('servers.fileHistory.listLabel')}>
        {versions.map((version, index) => {
          const when = formatRelative(version.savedAt, locale, now);
          const relative = capitalize(when, locale);
          const exact = formatExact(version.savedAt, locale);
          const size = formatBytes(version.size);
          const selected = version.id === selectedId;
          const isLast = index === versions.length - 1;
          return (
            <li key={version.id} className="relative">
              {/* a rail ties the versions together, newest at the top */}
              {!isLast && <span aria-hidden="true" className="absolute bottom-[-6px] left-[23px] top-[30px] w-px bg-border/70" />}
              <button
                type="button"
                onClick={() => pick(version)}
                aria-current={selected ? 'true' : undefined}
                aria-label={t('servers.fileHistory.itemLabel', { when, size })}
                className={cn(
                  'focus-ring group flex w-full items-start gap-3 rounded-xl border px-3 py-2.5 text-left transition-colors duration-200 ease-panel',
                  selected
                    ? 'border-primary/40 bg-primary/[0.07]'
                    : 'border-transparent hover:border-border/60 hover:bg-background/40',
                )}
              >
                <span
                  aria-hidden="true"
                  className={cn(
                    'relative mt-1.5 grid h-[18px] w-[18px] shrink-0 place-items-center rounded-full border bg-card transition-colors duration-200',
                    selected ? 'border-primary/60' : 'border-border/80 group-hover:border-border',
                  )}
                >
                  <span className={cn('h-1.5 w-1.5 rounded-full', dotColor(selected, index === 0))} />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
                    <time
                      dateTime={new Date(version.savedAt).toISOString()}
                      title={exact}
                      className="text-sm font-medium text-foreground"
                    >
                      {relative}
                    </time>
                    {index === 0 && (
                      <span className="rounded-md border border-border/60 bg-secondary/60 px-1.5 py-px text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
                        {t('servers.fileHistory.latest')}
                      </span>
                    )}
                  </span>
                  <span className="mt-0.5 block truncate text-xs tabular-nums text-muted-foreground" title={exact}>
                    {formatShort(version.savedAt, locale)} · {size}
                  </span>
                </span>
                <ChevronRight
                  aria-hidden="true"
                  className={cn(
                    'mt-2 h-4 w-4 shrink-0 text-muted-foreground transition-[opacity,transform] duration-200 ease-panel',
                    selected ? 'opacity-100' : 'opacity-0 group-hover:translate-x-0.5 group-hover:opacity-100 group-focus-visible:opacity-100',
                  )}
                />
              </button>
            </li>
          );
        })}
      </ol>
    );
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        closeLabel={t('common.close')}
        className="flex w-full flex-col gap-0 p-0 sm:max-w-md"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          if (pickedRef.current) {
            pickedRef.current = false;
            return;
          }
          returnFocusRef?.current?.focus();
        }}
      >
        <SheetHeader className="space-y-1 border-b border-border/50 px-5 pb-4 pt-5 text-left">
          <SheetTitle className="flex items-center gap-2.5 pr-8 text-base">
            <span aria-hidden="true" className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-border/60 bg-background/30 text-muted-foreground">
              <History className="h-4 w-4" />
            </span>
            {t('servers.fileHistory.title')}
          </SheetTitle>
          <SheetDescription className="break-all text-xs">
            {t('servers.fileHistory.description', { name: fileName })}
          </SheetDescription>
        </SheetHeader>

        <div className="custom-scrollbar min-h-0 flex-1 overflow-y-auto px-3 py-3 sm:px-4">
          {body}
        </div>

        <p className="border-t border-border/50 px-5 py-3 text-xs leading-relaxed text-muted-foreground">
          {t('servers.fileHistory.retention', { count: FILE_HISTORY_LIMIT })}
        </p>
      </SheetContent>
    </Sheet>
  );
}

function HistorySkeleton() {
  return (
    <div aria-hidden="true" className="space-y-0.5">
      {Array.from({ length: 6 }).map((_, index) => (
        <div key={index} className="flex gap-3 px-3 py-2.5">
          <Skeleton className="mt-1.5 h-[18px] w-[18px] shrink-0 rounded-full" />
          <div className="flex-1 space-y-2 pt-1">
            <Skeleton className={cn('h-3.5', index % 2 === 0 ? 'w-28' : 'w-24')} />
            <Skeleton className="h-2.5 w-36" />
          </div>
        </div>
      ))}
    </div>
  );
}
