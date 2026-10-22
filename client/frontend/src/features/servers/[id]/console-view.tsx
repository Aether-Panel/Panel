'use client';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { ArrowDown, Send, Terminal } from 'lucide-react';
import { sileo } from '@/lib/toast';
import { useId, useState, useRef, useEffect, useMemo } from 'react';
import AISummary from './ai-summary';
import { AnsiText } from '@/lib/ansi-utils';
import { useTranslations } from '@/contexts/translations-context';
import { useSettings } from '@/hooks/use-settings';
import { cn } from '@/lib/utils';

type LogEntry = {
  time: string;
  message: string;
};

/** Mirrors the state of the `/socket?console` WebSocket owned by the page. */
export type ConsoleStatus = 'connecting' | 'connected' | 'disconnected';

export default function ConsoleView({
  serverId,
  logs,
  addLog,
  status = 'disconnected',
}: {
  serverId: string;
  logs: LogEntry[];
  addLog: (message: string) => void;
  status?: ConsoleStatus;
}) {
  const [command, setCommand] = useState('');
  const { t } = useTranslations();
  const { settings } = useSettings();
  const scrollRef = useRef<HTMLDivElement>(null);
  const commandId = useId();

  // The only connection feedback is a short toast when the console connects
  // (again, after a drop); no permanent status chrome on the terminal.
  const previousStatus = useRef<ConsoleStatus>(status);
  useEffect(() => {
    if (status === 'connected' && previousStatus.current !== 'connected') {
      sileo.success({ title: t('servers.console.connectedToast') });
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

  useEffect(() => {
    if (scrollRef.current && pinned) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs, pinned]);

  const handleSendCommand = async () => {
    if (command.trim() === '') return;
    const cmdToSend = command;
    setCommand('');

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
      addLog(`[Error] Failed to send: ${cmdToSend}`);
    }
  };

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void handleSendCommand();
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
    const measure = () => {
      if (containerRef.current) {
        const rect = containerRef.current.getBoundingClientRect();
        const space = window.innerHeight - rect.top - 40;
        setTermHeight(Math.max(350, space));
      }
    };
    measure();
    window.addEventListener('resize', measure);
    return () => window.removeEventListener('resize', measure);
  }, []);

  const shown = logs;

  // One rule per console so lines stay aligned: when most output already
  // carries its own clock, the received-at column is dropped for every line.
  const showTimes = useMemo(() => {
    const sample = shown.slice(-200);
    if (sample.length === 0) return false;
    const stamped = sample.filter((log) => hasOwnTimestamp(log.message)).length;
    return stamped / sample.length < 0.5;
  }, [shown]);

  return (
    <div ref={containerRef} className="mt-6">
      <div
        className="relative flex flex-col overflow-hidden rounded-xl border border-border/60 bg-background shadow-elevation-1"
        style={{ height: termHeight }}
      >
        <div
          ref={scrollRef}
          role="log"
          tabIndex={0}
          onScroll={onScroll}
          aria-label={t('servers.console.title')}
          className="custom-scrollbar min-h-0 flex-1 overflow-y-auto overscroll-contain px-3 py-3 font-mono text-[13px] leading-6 sm:px-4 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/40"
        >
          {shown.length === 0 ? (
            <div className="flex h-full select-none flex-col items-center justify-center gap-2 text-muted-foreground">
              <Terminal className="h-5 w-5 text-muted-foreground/50" aria-hidden="true" />
              <span className="text-xs">{t('servers.console.waitingOutput')}</span>
            </div>
          ) : (
            shown.map((log, index) => {
              const levelColor = getLogLevelColor(log.message);
              return (
                <div key={index} className="flex gap-3">
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

        {!pinned && shown.length > 0 && (
          <Button
            type="button"
            size="sm"
            variant="secondary"
            onClick={jumpToEnd}
            className="absolute bottom-16 left-1/2 h-8 -translate-x-1/2 gap-1.5 rounded-full px-3 text-xs shadow-elevation-2"
          >
            <ArrowDown aria-hidden="true" className="!size-3.5" />
            {t('servers.console.jumpToEnd')}
          </Button>
        )}

        {/* Pinned command bar */}
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
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            autoComplete="off"
            spellCheck={false}
          />
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
      </div>

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
