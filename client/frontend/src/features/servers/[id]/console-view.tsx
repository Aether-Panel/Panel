'use client';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Send } from 'lucide-react';
import { useState, useRef, useEffect } from 'react';
import AISummary from './ai-summary';
import { AnsiText } from '@/lib/ansi-utils';
import { useTranslations } from '@/contexts/translations-context';

type LogEntry = {
  time: string;
  message: string;
};

export default function ConsoleView({ serverId, logs, addLog }: { serverId: string, logs: LogEntry[], addLog: (message: string) => void }) {
  const [command, setCommand] = useState('');
  const { t } = useTranslations();
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
    /**This duplicates the commands sent through the console.
    It is kept to verify correct command delivery when modifications or new integrations are added.*/
    //addLog(`$ ${cmdToSend}`); 

    try {
      // PufferPanel expects raw string body for console commands
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
    if (upperMessage.includes('ERROR') || upperMessage.includes('FATAL')) return 'text-red-500';
    if (upperMessage.includes('WARN') || upperMessage.includes('WARNING')) return 'text-yellow-400';
    if (upperMessage.includes('INFO')) return 'text-blue-300';
    if (logMessage.startsWith('$')) return 'text-green-400';
    if (logMessage.startsWith('>')) return 'text-gray-400';
    return 'text-gray-300';
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
        className="flex flex-col bg-[#0c0c0c] rounded-lg border border-white/[0.06]"
        style={{ height: termHeight }}
      >
        <div
          className="px-4 py-3 font-mono text-sm flex-1 overflow-y-auto min-h-0 custom-scrollbar leading-[1.5]"
          ref={scrollRef}
        >
          {logs.length === 0 ? (
            <div className="flex items-center justify-center h-full select-none">
              <span className="text-muted-foreground/20 text-xs">waiting for server output</span>
            </div>
          ) : (
            <>
              {logs.map((log, index) => {
                const levelColor = getLogLevelColor(log.message);
                const hasTimestamp = /^\[?\d{2}:\d{2}:\d{2}\]?/.test(log.message.trim());
                return (
                  <div key={index} className="flex gap-2 hover:bg-white/[0.015] px-1.5 rounded-sm transition-colors">
                    {!hasTimestamp && (
                      <span className="text-gray-600/40 shrink-0 select-none w-[4.5rem] text-xs leading-[1.5]">
                        {log.time}
                      </span>
                    )}
                    {hasTimestamp && <span className="w-[4.5rem] shrink-0" />}
                    <span className={`${levelColor} whitespace-pre-wrap break-all leading-[1.5]`}>
                      <AnsiText text={log.message} />
                    </span>
                  </div>
                )
              })}
              <div className="flex items-center gap-2 mt-1.5 px-1.5 text-gray-500">
                <span className="text-green-400/70">$</span>
                <span className="w-1.5 h-3.5 bg-green-400/40" />
              </div>
            </>
          )}
        </div>

        <div className="flex items-center gap-3 px-4 py-2.5 border-t border-white/[0.04] shrink-0">
          <span className="text-green-400/70 font-mono text-sm shrink-0 select-none">$</span>
          <Input
            type="text"
            placeholder="Type a command..."
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
          >
            <Send className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      <Card className="mt-6">
        <CardContent className="p-0">
          <AISummary logs={logs} serverId={serverId} />
        </CardContent>
      </Card>
    </div>
  );
}
