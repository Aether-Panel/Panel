'use client';
import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { AlertOctagon, Bot, ChevronRight, Lightbulb, Loader2, RotateCw, SearchCheck, ShieldCheck, Sparkles, X } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { EmptyState } from '@/components/empty-state';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { cn } from '@/lib/utils';
import { keyedByContent } from '@/lib/keyed';
import { AnsiText } from '@/lib/ansi-utils';

export type CrashReport = {
  id: string;
  time: string;
  exitCode: number;
  lines: string[];
  restarted: boolean;
  attempt: number;
  limit: number;
};

type Analysis = { summary: string; rootCauses: string[]; suggestions: string[] };

/** Fired by the page when the server's socket reports a crash, so every view refreshes. */
export const CRASH_EVENT = 'aether:server-crash';

/** How long a crash stays announced on the console tab. */
const BANNER_WINDOW_MS = 24 * 60 * 60 * 1000;

/** Exit codes worth explaining; the rest get a generic line. */
const KNOWN_EXIT_CODES = new Set([1, 134, 137, 139, 143]);

const ERROR_LINE = /(error|exception|fatal|panic|crash|out ?of ?memory|killed|segfault|failed|caused by)/i;

export function useCrashReports(serverId: string, enabled = true) {
  const [reports, setReports] = useState<CrashReport[]>([]);
  const [loaded, setLoaded] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const data = await api.get<CrashReport[]>(`/api/servers/${serverId}/crashes`);
      setReports(Array.isArray(data) ? data : []);
    } catch {
      // no console permission, or an older node without crash reports: nothing to show
      setReports([]);
    } finally {
      setLoaded(true);
    }
  }, [serverId]);

  useEffect(() => {
    if (!enabled) return;
    void refresh();
    const onCrash = () => void refresh();
    window.addEventListener(CRASH_EVENT, onCrash);
    return () => window.removeEventListener(CRASH_EVENT, onCrash);
  }, [enabled, refresh]);

  return { reports, loaded, refresh };
}

type Translate = (key: string, options?: { [key: string]: string | number }) => string;

/** What a crash meant and whether the server came back, for the crash toast. */
export function crashSummary(t: Translate, report: Partial<CrashReport>): string {
  const code = Number(report.exitCode);
  const meaning = KNOWN_EXIT_CODES.has(code) ? t(`servers.crashes.exitCodes.${code}`) : t('servers.crashes.exitCodes.other');
  const recovery = report.restarted
    ? t('servers.crashes.restarted', { attempt: report.attempt ?? 1, limit: report.limit ?? 1 })
    : t('servers.crashes.notRestarted');
  return `${meaning}; ${recovery}.`;
}

function useFormat() {
  const { t, language } = useTranslations();
  const locale = language === 'es' ? 'es-ES' : 'en-US';
  const time = (iso: string) => new Date(iso).toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' });
  const date = (iso: string) => new Date(iso).toLocaleString(locale, { dateStyle: 'medium', timeStyle: 'short' });
  const exitMeaning = (code: number) =>
    KNOWN_EXIT_CODES.has(code) ? t(`servers.crashes.exitCodes.${code}`) : t('servers.crashes.exitCodes.other');
  const restartText = (r: CrashReport) =>
    r.restarted ? t('servers.crashes.restarted', { attempt: r.attempt, limit: r.limit }) : t('servers.crashes.notRestarted');
  return { t, language, time, date, exitMeaning, restartText };
}

function dismissKey(serverId: string) {
  return `crashDismissed_${serverId}`;
}

/** The newest crash, shown above the console until it is dismissed or a day passes. */
export function CrashBanner({ serverId, onOpen }: Readonly<{ serverId: string; onOpen: (report: CrashReport) => void }>) {
  const { reports } = useCrashReports(serverId);
  const { t, time, exitMeaning, restartText } = useFormat();
  const [dismissed, setDismissed] = useState<string | null>(null);

  useEffect(() => {
    try {
      setDismissed(localStorage.getItem(dismissKey(serverId)));
    } catch {
      setDismissed(null);
    }
  }, [serverId]);

  const latest = reports[0];
  if (!latest || latest.id === dismissed || Date.now() - new Date(latest.time).getTime() > BANNER_WINDOW_MS) return null;

  const dismiss = () => {
    setDismissed(latest.id);
    try {
      localStorage.setItem(dismissKey(serverId), latest.id);
    } catch {
      // storage blocked: it just comes back on the next visit
    }
  };

  return (
    <output
      className="relative mt-4 block overflow-hidden rounded-xl border border-destructive/30 bg-destructive/[0.06] px-3 py-2.5 shadow-[0_0_32px_-12px_hsl(var(--destructive)/0.6)] animate-in fade-in-0 slide-in-from-top-2 duration-500 ease-panel sm:px-4"
    >
      {/* a faint wash from the left edge, like light from the warning icon */}
      <div aria-hidden="true" className="pointer-events-none absolute inset-y-0 left-0 w-48 bg-gradient-to-r from-destructive/15 to-transparent" />
      {/* one slim line above the console, so it does not eat into the console's height */}
      <div className="relative flex flex-wrap items-center gap-x-3 gap-y-2">
        <span className="relative flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-destructive/40 bg-destructive/15 text-destructive-bright">
          <span aria-hidden="true" className="absolute inset-0 animate-ping rounded-lg bg-destructive/20 [animation-duration:2.4s]" />
          <AlertOctagon className="relative h-4 w-4" aria-hidden="true" />
        </span>
        <div className="flex min-w-0 flex-1 flex-col gap-x-3 sm:flex-row sm:items-baseline">
          <p className="shrink-0 text-sm font-semibold text-foreground">{t('servers.crashes.banner.title', { time: time(latest.time) })}</p>
          <p className="min-w-0 truncate text-xs text-muted-foreground">
            {t('servers.crashes.exitCode', { code: latest.exitCode })} · {exitMeaning(latest.exitCode)} · {restartText(latest)}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button size="sm" variant="outline" className="h-8 border-destructive/40 hover:bg-destructive/10" onClick={() => onOpen(latest)}>
            <SearchCheck aria-hidden="true" />
            {t('servers.crashes.banner.open')}
          </Button>
          <Button size="icon" variant="ghost" className="h-8 w-8 text-muted-foreground" aria-label={t('servers.crashes.banner.dismiss')} onClick={dismiss}>
            <X aria-hidden="true" />
          </Button>
        </div>
      </div>
    </output>
  );
}

/** Recent crashes for the overview tab, newest first. */
export function CrashHistory({ serverId, onOpen }: Readonly<{ serverId: string; onOpen: (report: CrashReport) => void }>) {
  const { reports, loaded } = useCrashReports(serverId);
  const { t, date, exitMeaning } = useFormat();

  let content: ReactNode;
  if (!loaded) {
    content = (
      <div className="flex justify-center py-8">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" aria-hidden="true" />
        <span className="sr-only">{t('common.loading')}</span>
      </div>
    );
  } else if (reports.length === 0) {
    content = (
      <EmptyState
        icon={ShieldCheck}
        size="sm"
        title={t('servers.crashes.history.emptyTitle')}
        description={t('servers.crashes.history.emptyDescription')}
      />
    );
  } else {
    content = (
      <ul className="space-y-2">
        {reports.map((report, index) => (
          <li
            key={report.id}
            className="animate-in fade-in-0 slide-in-from-bottom-1 fill-mode-both duration-300 ease-panel"
            style={{ animationDelay: `${Math.min(index, 6) * 40}ms` }}
          >
            <button
              type="button"
              onClick={() => onOpen(report)}
              className="focus-ring group flex w-full items-center gap-4 rounded-xl border border-border/60 bg-background/30 p-3.5 text-left transition-colors duration-200 hover:border-destructive/40 hover:bg-destructive/[0.04]"
            >
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-destructive/30 bg-destructive/10 text-destructive-bright">
                <AlertOctagon className="h-4 w-4" aria-hidden="true" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium">{date(report.time)}</span>
                <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                  {t('servers.crashes.exitCode', { code: report.exitCode })} · {exitMeaning(report.exitCode)}
                </span>
              </span>
              {report.restarted && (
                <span className="hidden shrink-0 items-center gap-1 rounded-md border border-border/50 bg-secondary/60 px-2 py-0.5 text-xs text-secondary-foreground sm:inline-flex">
                  <RotateCw className="h-3 w-3" aria-hidden="true" />
                  {t('servers.crashes.autoRestarted')}
                </span>
              )}
              <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200 group-hover:translate-x-0.5" aria-hidden="true" />
            </button>
          </li>
        ))}
      </ul>
    );
  }

  return (
    <Card>
      <CardHeader className="space-y-1 border-b border-border/50 p-5 md:p-6">
        <CardTitle className="text-lg font-semibold tracking-[-0.02em]">{t('servers.crashes.history.title')}</CardTitle>
        <CardDescription>{t('servers.crashes.history.description')}</CardDescription>
      </CardHeader>
      <CardContent className="p-5 md:p-6">
        {content}
      </CardContent>
    </Card>
  );
}

/** One crash in full: what happened, the console before it, and the AI's reading of it. */
export function CrashReportDialog({ serverId, report, onOpenChange }: Readonly<{ serverId: string; report: CrashReport | null; onOpenChange: (open: boolean) => void }>) {
  const { t, language, date, exitMeaning, restartText } = useFormat();
  const [analysis, setAnalysis] = useState<Analysis | null>(null);
  const [analysing, setAnalysing] = useState(false);
  const [analysisError, setAnalysisError] = useState<string | null>(null);

  useEffect(() => {
    setAnalysis(null);
    setAnalysisError(null);
    setAnalysing(false);
  }, [report?.id]);

  const explain = async () => {
    if (!report) return;
    setAnalysing(true);
    setAnalysisError(null);
    try {
      const data = await api.post<Analysis>(`/api/servers/${serverId}/ai/analyze`, {
        logs: report.lines,
        language,
        exitCode: report.exitCode,
      });
      setAnalysis({ summary: data.summary ?? '', rootCauses: data.rootCauses ?? [], suggestions: data.suggestions ?? [] });
    } catch (err) {
      const notConfigured = err instanceof ApiError && err.status === 400 && /api key/i.test(err.message);
      setAnalysisError(notConfigured ? t('servers.crashes.ai.notConfigured') : t('servers.crashes.ai.error'));
    } finally {
      setAnalysing(false);
    }
  };

  return (
    <Dialog open={report !== null} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        {report && (
          <>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <AlertOctagon className="h-5 w-5 text-destructive-bright" aria-hidden="true" />
                {t('servers.crashes.dialog.title')}
              </DialogTitle>
              <DialogDescription>{date(report.time)}</DialogDescription>
            </DialogHeader>

            <dl className="grid gap-2 sm:grid-cols-3">
              {[
                { label: t('servers.crashes.dialog.exitCode'), value: String(report.exitCode), mono: true },
                { label: t('servers.crashes.dialog.meaning'), value: exitMeaning(report.exitCode) },
                { label: t('servers.crashes.dialog.recovery'), value: restartText(report) },
              ].map((item) => (
                <div key={item.label} className="rounded-lg border border-border/60 bg-background/40 px-3 py-2.5">
                  <dt className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{item.label}</dt>
                  <dd className={cn('mt-1 text-sm text-foreground first-letter:uppercase', item.mono && 'font-mono tabular-nums')}>{item.value}</dd>
                </div>
              ))}
            </dl>

            <section aria-labelledby="crash-log-title" className="space-y-2">
              <h3 id="crash-log-title" className="text-sm font-semibold">{t('servers.crashes.dialog.lastLines')}</h3>
              {report.lines.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t('servers.crashes.dialog.noLines')}</p>
              ) : (
                <ol className="custom-scrollbar max-h-64 overflow-auto rounded-lg border border-border/60 bg-background py-2 font-mono text-[12px] leading-5" tabIndex={0} aria-labelledby="crash-log-title">
                  {keyedByContent(report.lines).map(({ key: lineKey, item: line }, index) => (
                    <li
                      key={lineKey}
                      className={cn(
                        'flex gap-3 px-3',
                        ERROR_LINE.test(line) && 'bg-destructive/10 text-destructive-bright',
                      )}
                    >
                      <span aria-hidden="true" className="w-6 shrink-0 select-none text-right text-muted-foreground/40 tabular-nums">{index + 1}</span>
                      <span className="min-w-0 whitespace-pre-wrap break-words"><AnsiText text={line} /></span>
                    </li>
                  ))}
                </ol>
              )}
            </section>

            <section aria-labelledby="crash-ai-title" aria-busy={analysing} className="rounded-xl border border-primary/25 bg-gradient-to-br from-primary/[0.07] via-transparent to-accent/[0.05] p-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <h3 id="crash-ai-title" className="flex items-center gap-2 text-sm font-semibold">
                  <Bot className="h-4 w-4 text-primary-bright" aria-hidden="true" />
                  {t('servers.crashes.ai.title')}
                </h3>
                {!analysis && (
                  <Button size="sm" onClick={explain} disabled={analysing || report.lines.length === 0} className="shadow-[0_0_20px_-6px_hsl(var(--primary)/0.8)]">
                    {analysing ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Sparkles aria-hidden="true" />}
                    {analysing ? t('servers.crashes.ai.thinking') : t('servers.crashes.ai.explain')}
                  </Button>
                )}
              </div>

              {analysing && (
                <div className="mt-4 space-y-2" aria-hidden="true">
                  {[92, 78, 85, 60].map((width, i) => (
                    <div key={i} className="skeleton-shimmer h-3 rounded-full" style={{ width: `${width}%` }} />
                  ))}
                </div>
              )}

              {analysisError && <p role="alert" className="mt-3 text-sm text-destructive-bright">{analysisError}</p>}

              {analysis && (
                <div className="mt-4 space-y-4 text-sm" aria-live="polite">
                  <p className="leading-relaxed text-foreground animate-in fade-in-0 duration-500">{analysis.summary}</p>
                  {[
                    { key: 'causes', icon: SearchCheck, title: t('servers.crashes.ai.causes'), items: analysis.rootCauses },
                    { key: 'fixes', icon: Lightbulb, title: t('servers.crashes.ai.fixes'), items: analysis.suggestions },
                  ].filter((group) => group.items.length > 0).map((group, groupIndex) => (
                    <div key={group.key}>
                      <h4 className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                        <group.icon className="h-3.5 w-3.5" aria-hidden="true" />
                        {group.title}
                      </h4>
                      <ul className="space-y-1.5">
                        {keyedByContent(group.items).map(({ key: itemKey, item }, i) => (
                          <li
                            key={itemKey}
                            className="flex gap-2 rounded-lg bg-background/40 px-3 py-2 text-foreground animate-in fade-in-0 slide-in-from-left-1 fill-mode-both duration-300 ease-panel"
                            style={{ animationDelay: `${(groupIndex * 3 + i) * 70 + 150}ms` }}
                          >
                            <span aria-hidden="true" className="mt-[7px] h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                            <span>{item}</span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  ))}
                </div>
              )}
            </section>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
