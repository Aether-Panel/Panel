'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { AlertCircle, Lightbulb, ListChecks, Loader2, Sparkles, CheckSquare, Square } from 'lucide-react';
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion';
import { useTranslations } from '@/contexts/translations-context';
import { ScrollArea } from '@/components/ui/scroll-area';

type LogEntry = {
  time: string;
  message: string;
};

export default function AISummary({ logs, serverId }: { logs?: LogEntry[], serverId?: string }) {
  const [detectedErrors, setDetectedErrors] = useState<{ id: string, text: string }[]>([]);
  const [selectedErrors, setSelectedErrors] = useState<Set<string>>(new Set());
  const [summary, setSummary] = useState('');
  const [tips, setTips] = useState<{ suggestions: string[]; rootCauses: string[] } | null>(null);
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState('');
  const { t } = useTranslations();

  // Escanear logs automáticamente
  useEffect(() => {
    if (!logs) return;
    const errorRegex = /(error|warn|fail|exception|timeout|denied|critical)/i;
    
    // Extraer las últimas 50 líneas para no sobrecargar
    const recentLogs = logs.slice(-50);
    const errorsFound: { id: string, text: string }[] = [];
    
    recentLogs.forEach((log, index) => {
      if (errorRegex.test(log.message)) {
        errorsFound.push({ id: `err-${index}-${log.time}`, text: `[${log.time}] ${log.message}` });
      }
    });

    setDetectedErrors(errorsFound);
    
    // Si hay errores nuevos que no están seleccionados, los autoseleccionamos
    if (errorsFound.length > 0 && selectedErrors.size === 0) {
      setSelectedErrors(new Set(errorsFound.map(e => e.id)));
    }
  }, [logs]);

  const toggleError = (id: string) => {
    const newSet = new Set(selectedErrors);
    if (newSet.has(id)) newSet.delete(id);
    else newSet.add(id);
    setSelectedErrors(newSet);
  };

  const selectAll = () => setSelectedErrors(new Set(detectedErrors.map(e => e.id)));
  const deselectAll = () => setSelectedErrors(new Set());

  const handleSummarize = async () => {
    if (selectedErrors.size === 0 || !serverId) return;
    setLoading(true);
    setSummary('');
    setTips(null);
    setErrorMsg('');

    try {
      const logsToSend = detectedErrors.filter(e => selectedErrors.has(e.id)).map(e => e.text);
      
      const res = await fetch(`/api/servers/${serverId}/ai/analyze`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ logs: logsToSend })
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || 'Failed to analyze logs');
      }

      setSummary(data.summary);
      setTips({
        suggestions: data.suggestions || [],
        rootCauses: data.rootCauses || []
      });
    } catch (error: any) {
      console.error('AI operation failed:', error);
      setErrorMsg(error.message || t('servers.aiSummary.fail'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card className="border-0">
      <CardHeader>
        <div className="flex items-center gap-2">
          <Lightbulb className="text-primary" />
          <CardTitle>{t('servers.aiSummary.title')}</CardTitle>
        </div>
        <CardDescription>
          Selecciona los problemas extraídos automáticamente de la consola para que la IA los analice.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        
        {detectedErrors.length > 0 ? (
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">Errores y Advertencias Detectados</span>
              <div className="flex gap-2">
                <Button variant="ghost" size="sm" onClick={selectAll} className="h-7 text-xs">Seleccionar Todos</Button>
                <Button variant="ghost" size="sm" onClick={deselectAll} className="h-7 text-xs">Ninguno</Button>
              </div>
            </div>
            <ScrollArea className="h-[200px] w-full rounded-md border p-4 bg-black/20">
              <div className="flex flex-col gap-3">
                {detectedErrors.map((err) => {
                  const isSelected = selectedErrors.has(err.id);
                  return (
                    <div 
                      key={err.id} 
                      className={`flex gap-3 items-start p-2 rounded cursor-pointer transition-colors ${isSelected ? 'bg-primary/10 border border-primary/20' : 'hover:bg-white/5 border border-transparent'}`}
                      onClick={() => toggleError(err.id)}
                    >
                      <div className="mt-0.5 text-primary">
                        {isSelected ? <CheckSquare className="h-4 w-4" /> : <Square className="h-4 w-4 opacity-50" />}
                      </div>
                      <span className="font-mono text-xs text-red-400 break-all leading-relaxed">{err.text}</span>
                    </div>
                  );
                })}
              </div>
            </ScrollArea>
          </div>
        ) : (
           <div className="flex items-center justify-center p-8 border border-dashed rounded-md bg-white/5">
             <span className="text-sm text-muted-foreground">Esperando detectar errores en la consola...</span>
           </div>
        )}

        {errorMsg && (
          <div className="p-3 bg-red-500/10 border border-red-500/20 rounded-md text-red-400 text-sm flex items-start gap-2">
            <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
            <span>{errorMsg}</span>
          </div>
        )}

        <Button onClick={handleSummarize} disabled={loading || selectedErrors.size === 0}>
          {loading ? (
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          ) : (
            <Sparkles className="mr-2 h-4 w-4" />
          )}
          {t('servers.aiSummary.analyzeButton')} ({selectedErrors.size})
        </Button>

        {loading && (
          <div className="flex flex-col items-center justify-center gap-4 py-8">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
            <p className="text-muted-foreground">{t('servers.aiSummary.loading')}</p>
          </div>
        )}
        
        {summary && (
          <div className="flex flex-col gap-6 mt-4">
            <div className="p-4 bg-primary/5 rounded-lg border border-primary/10">
              <h3 className="mb-3 font-semibold flex items-center gap-2">
                 <Sparkles className="h-4 w-4 text-primary" />
                 {t('servers.aiSummary.summaryTitle')}
              </h3>
              <p className="text-sm text-foreground/90 leading-relaxed">{summary}</p>
            </div>
            {tips && (
              <Accordion type="single" collapsible className="w-full" defaultValue="item-1">
                <AccordionItem value="item-1">
                  <AccordionTrigger>
                    <div className="flex items-center gap-2">
                      <ListChecks className="text-blue-400" />
                      <span>{t('servers.aiSummary.suggestions')}</span>
                    </div>
                  </AccordionTrigger>
                  <AccordionContent>
                    <ul className="list-disc space-y-2 pl-6">
                      {tips.suggestions.map((tip, i) => (
                        <li key={i} className="text-sm text-foreground/80">{tip}</li>
                      ))}
                    </ul>
                  </AccordionContent>
                </AccordionItem>
                <AccordionItem value="item-2">
                  <AccordionTrigger>
                    <div className="flex items-center gap-2">
                      <AlertCircle className="text-yellow-400" />
                      <span>{t('servers.aiSummary.rootCauses')}</span>
                    </div>
                  </AccordionTrigger>
                  <AccordionContent>
                    <ul className="list-disc space-y-2 pl-6">
                      {tips.rootCauses.map((cause, i) => (
                        <li key={i} className="text-sm text-foreground/80">{cause}</li>
                      ))}
                    </ul>
                  </AccordionContent>
                </AccordionItem>
              </Accordion>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
