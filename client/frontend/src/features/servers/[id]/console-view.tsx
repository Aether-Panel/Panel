'use client';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Send, Terminal } from 'lucide-react';
import { useId, useState, useRef, useEffect } from 'react';
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

const CONSOLE_STATUS: Record<ConsoleStatus, { labelKey: string; dot: string; text: string }> = {
  connected: {
    labelKey: 'servers.console.status.connected',
    dot: 'bg-success shadow-[0_0_8px_hsl(var(--success)/0.9)]',
    text: 'text-success',
  },
  connecting: {
    labelKey: 'servers.console.status.connecting',
    dot: 'bg-warning shadow-[0_0_8px_hsl(var(--warning)/0.8)]',
    text: 'text-warning',
  },
  disconnected: {
    labelKey: 'servers.console.status.disconnected',
    dot: 'bg-destructive shadow-[0_0_8px_hsl(var(--destructive)/0.8)]',
    text: 'text-destructive-bright',
  },
};

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
  const connection = CONSOLE_STATUS[status] ?? CONSOLE_STATUS.disconnected;

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs]);

  const handleSendCommand = async () => {
    if (command.trim() === '') return;
    const cmdToSend = command;
    setCommand('');

    try {
      await fetch(`/api/servers/${serverId}/console`, {
        method: 'POST',
        body: cmdToSend,
        credentials: 'include'
      });
    } catch (e) {
      console.error('Failed to send command:', e);
      addLog(`[Error] Failed to send: ${cmdToSend}`);
    }
  };

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void handleSendCommand();
  };

  const getLogLevelColor = (logMessage: string) => {
    const upperMessage = logMessage.toUpperCase();
    if (upperMessage.includes('ERROR') || upperMessage.includes('FATAL')) return 'text-destructive-bright';
    if (upperMessage.includes('WARN') || upperMessage.includes('WARNING')) return 'text-warning';
    if (upperMessage.includes('INFO')) return 'text-chart-2';
    if (logMessage.startsWith('$')) return 'text-success';
    if (logMessage.startsWith('>')) return 'text-muted-foreground';
    return 'text-foreground/85';
  };

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

  return (
    <div ref={containerRef} className="mt-6">
      <div
        className="flex flex-col overflow-hidden rounded-xl border border-border/60 bg-background shadow-elevation-3"
        style={{ height: termHeight }}
      >
        {/* Terminal window chrome */}
        <div className="relative flex shrink-0 items-center justify-between gap-3 border-b border-border/60 bg-surface-1/80 px-4 py-2.5">
          <div className="flex items-center gap-1.5" aria-hidden="true">
            <span className="h-3 w-3 rounded-full bg-[#FF5F57]" />
            <span className="h-3 w-3 rounded-full bg-[#FEBC2E]" />
            <span className="h-3 w-3 rounded-full bg-[#28C840]" />
          </div>
          <span className="pointer-events-none absolute left-1/2 hidden -translate-x-1/2 truncate font-mono text-[11px] text-muted-foreground sm:block">
            {t('servers.console.windowLabel')}
          </span>
          <span role="status" className="flex shrink-0 items-center gap-2">
            <span className={cn('h-2 w-2 rounded-full', connection.dot)} aria-hidden="true" />
            <span className={cn('font-mono text-[11px] font-medium', connection.text)}>
              {t(connection.labelKey)}
            </span>
          </span>
        </div>

        <div
          ref={scrollRef}
          role="log"
          tabIndex={0}
          aria-label={t('servers.console.title')}
          className="custom-scrollbar min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-3 font-mono text-sm leading-[1.6] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/40"
        >
          {logs.length === 0 ? (
            <div className="flex h-full select-none flex-col items-center justify-center gap-2 text-muted-foreground">
              <Terminal className="h-5 w-5 text-muted-foreground/50" aria-hidden="true" />
              <span className="text-xs">{t('servers.console.waitingOutput')}</span>
            </div>
          ) : (
            <>
              {logs.map((log, index) => {
                const levelColor = getLogLevelColor(log.message);
                return (
                  <div key={index} className="flex gap-3 rounded-sm px-1.5 transition-colors duration-150 hover:bg-white/[0.03]">
                    <span className="hidden shrink-0 select-none pt-px text-[11px] text-muted-foreground/60 sm:inline">
                      {log.time}
                    </span>
                    <span className={cn('whitespace-pre-wrap break-all', levelColor)}>
                      <AnsiText text={log.message} />
                    </span>
                  </div>
                );
              })}
              <div className="mt-1.5 flex items-center gap-2 px-1.5">
                <span className="text-success/70" aria-hidden="true">$</span>
                <span className="h-3.5 w-1.5 animate-pulse rounded-[1px] bg-success/50" aria-hidden="true" />
              </div>
            </>
          )}
        </div>

        {/* Pinned command bar */}
        <form
          onSubmit={handleSubmit}
          className="flex shrink-0 items-center gap-3 border-t border-border/60 bg-surface-1/60 px-4 py-2.5 transition-colors duration-200 focus-within:border-primary/50 focus-within:ring-2 focus-within:ring-ring/40"
        >
          <label htmlFor={commandId} className="sr-only">
            {t('servers.console.commandPlaceholder')}
          </label>
          <span className="shrink-0 select-none font-mono text-sm font-bold text-success" aria-hidden="true">$</span>
          <Input
            id={commandId}
            type="text"
            placeholder={t('servers.console.commandPlaceholder')}
            className="h-auto border-0 bg-transparent p-0 font-mono text-sm text-foreground shadow-none placeholder:text-muted-foreground/60 focus-visible:border-transparent focus-visible:ring-0 focus-visible:ring-offset-0"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
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
