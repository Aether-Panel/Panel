'use client';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Send } from 'lucide-react';
import { useState, useRef, useEffect } from 'react';
import AISummary from './ai-summary';
import { useTranslations } from '@/contexts/translations-context';
import { AnsiText } from '@/lib/ansi-utils';

type LogEntry = {
  time: string;
  message: string;
};

export default function ConsoleView({ serverId, logs, addLog }: { serverId: string, logs: LogEntry[], addLog: (message: string) => void }) {
  const [command, setCommand] = useState('');
  const { t } = useTranslations();

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

  return (
    <div className="mt-6 space-y-6">
      <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
        <Card className="border-0">
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>{t('servers.console.title')}</CardTitle>
          </CardHeader>
          <CardContent className="pt-6">
            <div
              className="bg-[#0c0c0c] text-gray-200 font-mono text-sm p-4 rounded-lg h-[500px] overflow-y-auto custom-scrollbar border border-white/5"
              style={{ overflowAnchor: 'none' }}
            >
              {logs.map((log, index) => {
                const levelColor = getLogLevelColor(log.message);
                const hasTimestamp = /^\[?\d{2}:\d{2}:\d{2}\]?/.test(log.message.trim());
                return (
                  <div key={index} className="flex gap-4 hover:bg-white/5 px-2 rounded transition-colors group">
                    {!hasTimestamp && (
                      <span className="text-gray-600 shrink-0 select-none opacity-50 group-hover:opacity-100 transition-opacity">
                        [{log.time}]
                      </span>
                    )}
                    <div className={`${levelColor} whitespace-pre-wrap break-all leading-relaxed`}>
                      <AnsiText text={log.message} />
                    </div>
                  </div>
                )
              })}
              <div className="flex items-center mt-2 px-2">
                <span className="text-green-400 mr-2">$</span>
                <div className="w-2 h-4 bg-green-400 animate-pulse"></div>
              </div>
            </div>
            <div className="mt-4 flex w-full items-center space-x-2">
              <Input
                type="text"
                placeholder={t('servers.console.commandPlaceholder')}
                className="font-mono"
                value={command}
                onChange={(e) => setCommand(e.target.value)}
                onKeyDown={handleKeyPress}
              />
              <Button type="submit" onClick={handleSendCommand}>
                <Send className="mr-2 h-4 w-4" />
                {t('servers.console.sendButton')}
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
      <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
        <AISummary logs={logs} serverId={serverId} />
      </div>
    </div>
  );
}
