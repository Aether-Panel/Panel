'use client';

import { useState, useEffect } from 'react';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { AlertCircle, ListChecks, Loader2, Sparkles } from 'lucide-react';
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion';
import { useTranslations } from '@/contexts/translations-context';
import { ScrollArea } from '@/components/ui/scroll-area';
import { cn } from '@/lib/utils';

type LogEntry = {
  time: string;
  message: string;
};

export default function AISummary({ logs, serverId }: { logs?: LogEntry[], serverId?: string }) {
  const [detectedErrors, setDetectedErrors] = useState<{ id: string, text: string, display: string }[]>([]);
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
    const errorsFound: { id: string, text: string, display: string }[] = [];
    
    recentLogs.forEach((log, index) => {
      if (errorRegex.test(log.message)) {
        // `text` (sent to the AI) keeps the detection time; the row shows only the
        // line itself, which already carries the server's own timestamp.
        errorsFound.push({ id: `err-${index}-${log.time}`, text: `[${log.time}] ${log.message}`, display: log.message });
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
        throw new Error(data.error || t('servers.aiSummary.fail'));
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

  const allSelected = detectedErrors.length > 0 && detectedErrors.every((e) => selectedErrors.has(e.id));
  // Only the visible, checked lines are sent, so that is the count to show.
  const selectedCount = detectedErrors.filter((e) => selectedErrors.has(e.id)).length;

  // Rendered inside the console's Card, so this is a plain header + body, not another card.
  return (
    <div>
      <div className="flex flex-col gap-4 border-b border-border/50 p-5 sm:flex-row sm:items-center sm:justify-between md:p-6">
        <div className="min-w-0 space-y-1">
          <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('servers.aiSummary.title')}</h2>
          <p className="text-sm text-muted-foreground">{t('servers.aiSummary.selectDescription')}</p>
        </div>
        <Button
          onClick={handleSummarize}
          disabled={loading || selectedCount === 0}
          className="shrink-0 self-start sm:self-auto"
        >
          {loading ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Sparkles aria-hidden="true" />}
          {t('servers.aiSummary.analyzeButton')}
          <span className="text-xs tabular-nums opacity-80">({selectedCount})</span>
        </Button>
      </div>

      <div className="flex flex-col gap-4 p-5 md:p-6">
        {detectedErrors.length > 0 ? (
          <div className="overflow-hidden rounded-xl border border-border/60 bg-background/30">
            <div className="flex items-center justify-between gap-3 border-b border-border/50 px-3 py-2">
              <span className="text-sm font-medium">
                {t('servers.aiSummary.detected')}
                <span className="ml-1.5 text-xs tabular-nums text-muted-foreground">{detectedErrors.length}</span>
              </span>
              <Button
                variant="ghost"
                size="sm"
                onClick={allSelected ? deselectAll : selectAll}
                className="h-8 text-xs text-muted-foreground hover:text-foreground"
              >
                {allSelected ? t('servers.aiSummary.selectNone') : t('servers.aiSummary.selectAll')}
              </Button>
            </div>
            <ScrollArea className="h-[220px] w-full">
              <ul className="divide-y divide-border/50">
                {detectedErrors.map((err) => {
                  const isSelected = selectedErrors.has(err.id);
                  const isError = /(error|exception|fail|critical|denied)/i.test(err.text);
                  return (
                    <li key={err.id}>
                      <label
                        htmlFor={err.id}
                        className={cn(
                          'flex cursor-pointer items-start gap-3 px-3 py-2.5 transition-colors duration-150 hover:bg-accent/30',
                          isSelected && 'bg-primary/5'
                        )}
                      >
                        <Checkbox
                          id={err.id}
                          checked={isSelected}
                          onCheckedChange={() => toggleError(err.id)}
                          className="mt-0.5"
                        />
                        <span
                          className={cn(
                            'min-w-0 break-words font-mono text-xs leading-relaxed [overflow-wrap:anywhere]',
                            isError ? 'text-destructive-bright' : 'text-warning'
                          )}
                        >
                          {err.display}
                        </span>
                      </label>
                    </li>
                  );
                })}
              </ul>
            </ScrollArea>
          </div>
        ) : (
          <div className="rounded-xl border border-dashed border-border/60 px-6 py-10 text-center">
            <ListChecks className="mx-auto mb-3 h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
            <p className="text-base font-semibold">{t('servers.aiSummary.emptyTitle')}</p>
            <p className="mx-auto mt-1 max-w-[48ch] text-sm text-muted-foreground">{t('servers.aiSummary.emptyDescription')}</p>
          </div>
        )}

        {errorMsg && (
          <div className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive-bright" role="alert">
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <span>{errorMsg}</span>
          </div>
        )}

        {loading && (
          <div className="flex items-center justify-center gap-3 py-8 text-sm text-muted-foreground" role="status">
            <Loader2 className="h-6 w-6 animate-spin" aria-hidden="true" />
            {t('servers.aiSummary.loading')}
          </div>
        )}

        {summary && (
          <div className="flex flex-col gap-4">
            <div className="rounded-xl border border-border/60 bg-background/30 p-4">
              <h3 className="mb-2 text-sm font-semibold">{t('servers.aiSummary.summaryTitle')}</h3>
              <p className="text-sm leading-relaxed text-foreground/90">{summary}</p>
            </div>
            {tips && (
              <Accordion type="single" collapsible className="w-full" defaultValue="item-1">
                <AccordionItem value="item-1">
                  <AccordionTrigger>
                    <span className="flex items-center gap-2">
                      <ListChecks className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                      {t('servers.aiSummary.suggestions')}
                    </span>
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
                    <span className="flex items-center gap-2">
                      <AlertCircle className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                      {t('servers.aiSummary.rootCauses')}
                    </span>
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
      </div>
    </div>
  );
}
