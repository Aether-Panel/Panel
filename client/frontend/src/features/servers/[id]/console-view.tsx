'use client';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Send } from 'lucide-react';
import { useState, useRef, useEffect } from 'react';
import AISummary from './ai-summary';
import { AnsiText } from '@/lib/ansi-utils';
import { useTranslations } from '@/contexts/translations-context';
import { useSettings } from '@/hooks/use-settings';

type LogEntry = {
  time: string;
  message: string;
};

export default function ConsoleView({ serverId, logs, addLog }: { serverId: string, logs: LogEntry[], addLog: (message: string) => void }) {
  const [command, setCommand] = useState('');
  const { t } = useTranslations();
  const { settings } = useSettings();
  const scrollRef = useRef<HTMLDivElement>(null);

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

  const handleKeyPress = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') {
      handleSendCommand();
    }
  };

  const getLogLevelColor = (logMessage: string) => {
    const upperMessage = logMessage.toUpperCase();
    if (upperMessage.includes('ERROR') || upperMessage.includes('FATAL')) return 'text-[hsl(var(--destructive))]';
    if (upperMessage.includes('WARN') || upperMessage.includes('WARNING')) return 'text-[hsl(var(--warning))]';
    if (upperMessage.includes('INFO')) return 'text-[hsl(var(--chart-2))]';
    if (logMessage.startsWith('$')) return 'text-[hsl(var(--success))]';
    if (logMessage.startsWith('>')) return 'text-white/30';
    return 'text-white/60';
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
        className="flex flex-col bg-card rounded-lg border border-white/[0.06] shadow-[0_2px_2px_rgba(0,0,0,0.2)]"
        style={{ height: termHeight }}
      >
        <div
          className="px-4 py-3 font-mono text-sm flex-1 overflow-y-auto min-h-0 custom-scrollbar leading-[1.5]"
          ref={scrollRef}
        >
          {logs.length === 0 ? (
            <div className="flex items-center justify-center h-full select-none">
              <span className="text-white/30 text-xs">{t('servers.console.waitingOutput')}</span>
            </div>
          ) : (
            <>
              {logs.map((log, index) => {
                const levelColor = getLogLevelColor(log.message);
                return (
                  <div key={index} className="flex gap-2 hover:bg-white/[0.015] px-1.5 rounded-sm transition-colors">
                    <span className={`${levelColor} whitespace-pre-wrap break-all leading-[1.5]`}>
                      <AnsiText text={log.message} />
                    </span>
                  </div>
                );
              })}
              <div className="flex items-center gap-2 mt-1.5 px-1.5 text-muted-foreground">
                <span className="text-[hsl(var(--success))]/70">$</span>
                <span className="w-1.5 h-3.5 bg-[hsl(var(--success))]/40" />
              </div>
            </>
          )}
        </div>

        <div className="flex items-center gap-3 px-4 py-2.5 border-t border-white/[0.04] shrink-0">
          <span className="text-[hsl(var(--success))]/70 font-mono text-sm shrink-0 select-none">$</span>
          <Input
            type="text"
            placeholder={t('servers.console.commandPlaceholder')}
            className="border-0 bg-transparent p-0 font-mono text-sm text-gray-300 placeholder:text-gray-600 focus-visible:ring-0 focus-visible:ring-offset-0 h-auto shadow-none"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            onKeyDown={handleKeyPress}
          />
          <Button
            type="submit"
            onClick={handleSendCommand}
            size="sm"
            variant="ghost"
            className="shrink-0 h-7 px-2 text-muted-foreground hover:text-foreground"
            title={t('servers.console.sendButton')}
          >
            <Send className="h-3.5 w-3.5" />
          </Button>
        </div>
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
