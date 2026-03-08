'use client';

import { useState } from 'react';
const summarizeServerAlerts = async ({ logs }: { logs: string }) => {
  return new Promise<{ summary: string }>((resolve) => {
    setTimeout(() => {
      resolve({ summary: "(Mock AI) Analysis complete: The logs indicate a sudden spike in resource usage followed by a timeout. Please review recent deployments." });
    }, 1500);
  });
};

const generateTroubleshootingTips = async ({ alertSummary }: { alertSummary: string }) => {
  return new Promise<{ suggestions: string[]; rootCauses: string[] }>((resolve) => {
    setTimeout(() => {
      resolve({
        suggestions: ["Increase server memory limit", "Enable verbose logging for the failing service"],
        rootCauses: ["Application memory leak", "Database connection pool exhaustion"]
      });
    }, 1500);
  });
};
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { AlertCircle, Lightbulb, ListChecks, Loader2, Sparkles } from 'lucide-react';
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion';
import { Textarea } from '@/components/ui/textarea';
import { Label } from '@/components/ui/label';
import { useTranslations } from '@/contexts/translations-context';

export default function AISummary() {
  const [logsToAnalyze, setLogsToAnalyze] = useState('');
  const [summary, setSummary] = useState('');
  const [tips, setTips] = useState<{ suggestions: string[]; rootCauses: string[] } | null>(null);
  const [loading, setLoading] = useState(false);
  const { t } = useTranslations();

  const handleSummarize = async () => {
    if (!logsToAnalyze) return;
    setLoading(true);
    setSummary('');
    setTips(null);
    try {
      const { summary: newSummary } = await summarizeServerAlerts({ logs: logsToAnalyze });
      setSummary(newSummary);

      const newTips = await generateTroubleshootingTips({ alertSummary: newSummary });
      setTips(newTips);
    } catch (error) {
      console.error('AI operation failed:', error);
      setSummary(t('servers.aiSummary.fail'));
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
          {t('servers.aiSummary.description')}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="grid w-full gap-2">
          <Label htmlFor="logs-input">{t('servers.aiSummary.logsLabel')}</Label>
          <Textarea
            id="logs-input"
            placeholder={t('servers.aiSummary.logsPlaceholder')}
            value={logsToAnalyze}
            onChange={(e) => setLogsToAnalyze(e.target.value)}
            className="min-h-[120px] font-mono text-xs"
          />
        </div>
        <Button onClick={handleSummarize} disabled={loading || !logsToAnalyze}>
          {loading ? (
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          ) : (
            <Sparkles className="mr-2 h-4 w-4" />
          )}
          {t('servers.aiSummary.analyzeButton')}
        </Button>

        {loading && (
          <div className="flex flex-col items-center justify-center gap-4 py-8">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
            <p className="text-muted-foreground">{t('servers.aiSummary.loading')}</p>
          </div>
        )}
        {!loading && !summary && (
          <div className="flex flex-col items-center justify-center gap-4 py-8 text-center">
            <Sparkles className="h-8 w-8 text-muted-foreground" />
            <p className="text-muted-foreground">{t('servers.aiSummary.ready')}</p>
          </div>
        )}
        {summary && (
          <div className="flex flex-col gap-6">
            <div>
              <h3 className="mb-2 font-semibold">{t('servers.aiSummary.summaryTitle')}</h3>
              <p className="text-sm text-foreground/90">{summary}</p>
            </div>
            {tips && (
              <Accordion type="single" collapsible className="w-full" defaultValue="item-1">
                <AccordionItem value="item-1">
                  <AccordionTrigger>
                    <div className="flex items-center gap-2">
                      <ListChecks />
                      <span>{t('servers.aiSummary.suggestions')}</span>
                    </div>
                  </AccordionTrigger>
                  <AccordionContent>
                    <ul className="list-disc space-y-2 pl-6">
                      {tips.suggestions.map((tip, i) => (
                        <li key={i} className="text-sm">{tip}</li>
                      ))}
                    </ul>
                  </AccordionContent>
                </AccordionItem>
                <AccordionItem value="item-2">
                  <AccordionTrigger>
                    <div className="flex items-center gap-2">
                      <AlertCircle />
                      <span>{t('servers.aiSummary.rootCauses')}</span>
                    </div>
                  </AccordionTrigger>
                  <AccordionContent>
                    <ul className="list-disc space-y-2 pl-6">
                      {tips.rootCauses.map((cause, i) => (
                        <li key={i} className="text-sm">{cause}</li>
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
