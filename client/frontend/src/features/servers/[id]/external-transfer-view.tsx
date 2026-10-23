'use client';
import { useState, useEffect } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { sileo } from "@/lib/toast";
import { Loader2, Copy, Check, CheckCircle2, DownloadCloud, UploadCloud, Clock, KeyRound, ArrowRight, Info, TriangleAlert } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';

type ExternalTransferViewProps = {
  serverId: string;
};

interface StoredTransfer {
  token: string;
  expires_in: number;
  session_id: string;
  generatedAt: number; // Date.now() al generar
}

const storageKey = (serverId: string) => `extransfer_token_${serverId}`;

const loadStoredTransfer = (serverId: string): StoredTransfer | null => {
  try {
    const raw = localStorage.getItem(storageKey(serverId));
    if (!raw) return null;
    const data: StoredTransfer = JSON.parse(raw);
    const elapsed = (Date.now() - data.generatedAt) / 1000;
    if (elapsed >= 900) {
      localStorage.removeItem(storageKey(serverId));
      return null;
    }
    return data;
  } catch {
    return null;
  }
};

const saveTransfer = (serverId: string, data: StoredTransfer) => {
  localStorage.setItem(storageKey(serverId), JSON.stringify(data));
};

const clearTransfer = (serverId: string) => {
  localStorage.removeItem(storageKey(serverId));
};

export default function ExternalTransferView({ serverId }: ExternalTransferViewProps) {
  const { t } = useTranslations();
  

  const [loading, setLoading] = useState(false);
  const [sessionData, setSessionData] = useState<StoredTransfer | null>(null);
  const [copied, setCopied] = useState(false);
  const [importUrl, setImportUrl] = useState('');
  const [importToken, setImportToken] = useState('');
  const [importing, setImporting] = useState(false);
  const [importStep, setImportStep] = useState('');
  const [timeLeft, setTimeLeft] = useState<number | null>(null);
  const [activeTab, setActiveTab] = useState('export');

  // Al montar: recuperar token guardado si aún es válido
  useEffect(() => {
    const stored = loadStoredTransfer(serverId);
    if (stored) {
      setSessionData(stored);
      const elapsed = (Date.now() - stored.generatedAt) / 1000;
      setTimeLeft(Math.max(0, Math.round(900 - elapsed)));
    }
  }, [serverId]);

  // Polling de estado de transferencia
  useEffect(() => {
    let lastStatus = '';
    const checkStatus = async () => {
      try {
        const res = await fetch(`/api/servers/${serverId}/extransfer/status`);
        if (res.ok) {
          const data = await res.json();
          if (data.status && data.status !== lastStatus) {
            lastStatus = data.status;
            setImporting(true);
            setActiveTab('import');

            if (data.status === 'DONE') {
              sileo.success({ title: t('servers.extransfer.toast.doneTitle'), description: t('servers.extransfer.toast.doneDescription') });
              setImportStep('');
              setImporting(false);
            } else if (data.status.startsWith('ERROR: ')) {
              sileo.error({ title: t('servers.extransfer.toast.failedTitle'), description: data.status.substring(7) });
              setImportStep('');
              setImporting(false);
            } else {
              setImportStep(data.status);
            }
          } else if (!data.status && lastStatus) {
            // El backend limpió el estado, terminamos.
            lastStatus = '';
            setImporting(false);
            setImportStep('');
          }
        }
      } catch (e) {
        // Ignorar errores de red en polling
      }
    };

    void checkStatus();
    const interval = setInterval(checkStatus, 2000);
    return () => clearInterval(interval);
  }, [serverId]);

  // Cuenta regresiva
  useEffect(() => {
    if (timeLeft === null) return;
    if (timeLeft <= 0) {
      setSessionData(null);
      clearTransfer(serverId);
      setTimeLeft(null);
      sileo.success({ title: t('servers.extransfer.toast.expiredTitle'), description: t('servers.extransfer.toast.expiredDescription') });
      return;
    }
    const timer = setTimeout(() => setTimeLeft(prev => (prev !== null ? prev - 1 : null)), 1000);
    return () => clearTimeout(timer);
  }, [timeLeft, serverId]);

  const formatTime = (s: number) => {
    const m = Math.floor(s / 60).toString().padStart(2, '0');
    const sec = (s % 60).toString().padStart(2, '0');
    return `${m}:${sec}`;
  };

  // Generar token (usa cookies del panel, sin necesidad de Bearer)
  const generateTransferToken = async () => {
    try {
      setLoading(true);
      const res = await fetch(`/api/servers/${serverId}/extransfer/create`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include' // usa la cookie de sesión del panel
      });

      const data = await res.json();
      if (!res.ok) throw new Error(data.message || t('servers.extransfer.toast.generateError'));

      const stored: StoredTransfer = {
        token: data.token,
        expires_in: data.expires_in ?? 900,
        session_id: data.session_id,
        generatedAt: Date.now()
      };

      setSessionData(stored);
      setTimeLeft(900);
      saveTransfer(serverId, stored); // ← persiste en localStorage

      sileo.success({
        title: t('servers.extransfer.toast.generatedTitle'),
        description: t('servers.extransfer.toast.generatedDescription')
      });
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message });
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = (text: string) => {
    void navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
    sileo.success({ title: t('common.copied') });
  };

  const executeImport = async () => {
    if (!importUrl || !importToken) {
      sileo.error({ title: t('servers.extransfer.toast.missingTitle'), description: t('servers.extransfer.toast.missingDescription') });
      return;
    }
    try {
      setImporting(true);
      setImportStep(t('servers.extransfer.import.starting'));
      const res = await fetch(`/api/servers/${serverId}/extransfer/pull`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ origin_url: importUrl, token: importToken })
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.message || t('servers.extransfer.toast.importError'));
      }

      // El polling (useEffect) se encargará de actualizar el estado a partir de aquí.
    } catch (e: any) {
      sileo.error({ title: t('servers.extransfer.toast.importFailedTitle'), description: e.message });
      setImportStep('');
      setImporting(false);
    }
  };

  const originUrl = typeof window !== 'undefined' ? window.location.origin : '';

  return (
    <Card className="mt-6">
      <div className="space-y-1 border-b border-border/50 p-5 md:p-6">
        <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('servers.extransfer.title')}</h2>
        <p className="text-sm text-muted-foreground">{t('servers.extransfer.description')}</p>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="p-5 md:p-6">
        <TabsList className="grid w-full grid-cols-2 sm:inline-grid sm:w-auto">
          <TabsTrigger value="export" className="gap-2">
            <UploadCloud className="h-4 w-4" aria-hidden="true" />
            {t('servers.extransfer.tabs.export')}
          </TabsTrigger>
          <TabsTrigger value="import" className="gap-2">
            <DownloadCloud className="h-4 w-4" aria-hidden="true" />
            {t('servers.extransfer.tabs.import')}
          </TabsTrigger>
        </TabsList>

        {/* EXPORT */}
        <TabsContent value="export" className="mt-5 space-y-5">
          <div className="flex gap-3 rounded-xl border border-border/60 bg-background/30 p-4">
            <Info className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <div className="space-y-1 text-sm">
              <p className="font-medium">{t('servers.extransfer.export.infoTitle')}</p>
              <p className="text-pretty text-muted-foreground">{t('servers.extransfer.export.infoBody')}</p>
            </div>
          </div>

          {!sessionData ? (
            <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border/60 px-6 py-12 text-center">
              <KeyRound className="mb-3 h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
              <h3 className="text-base font-semibold">{t('servers.extransfer.export.emptyTitle')}</h3>
              <p className="mt-1 max-w-md text-pretty text-sm text-muted-foreground">{t('servers.extransfer.export.emptyDescription')}</p>
              <Button onClick={generateTransferToken} disabled={loading} className="mt-6">
                {loading ? <Loader2 className="animate-spin" aria-hidden="true" /> : <KeyRound aria-hidden="true" />}
                {loading ? t('servers.extransfer.export.generating') : t('servers.extransfer.export.generate')}
              </Button>
            </div>
          ) : (
            <div className="overflow-hidden rounded-xl border border-border/60 bg-background/30">
              <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border/50 px-4 py-3 sm:px-5">
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-4 w-4 text-success" aria-hidden="true" />
                  <h3 className="text-sm font-semibold">{t('servers.extransfer.export.sessionTitle')}</h3>
                </div>
                {timeLeft !== null && (
                  <span
                    className={cn(
                      'inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium',
                      timeLeft < 120 ? 'bg-destructive/10 text-destructive-bright' : 'bg-success/10 text-success'
                    )}
                  >
                    <Clock className="h-3.5 w-3.5" aria-hidden="true" />
                    {t('servers.extransfer.export.expiresIn')}
                    <span className="tabular-nums">{formatTime(timeLeft)}</span>
                  </span>
                )}
              </div>

              <div className="grid gap-5 p-4 sm:p-5">
                <div className="space-y-2">
                  <Label htmlFor="extransfer-origin-url">{t('servers.extransfer.export.urlLabel')}</Label>
                  <div className="flex gap-2">
                    <Input id="extransfer-origin-url" readOnly value={originUrl} className="min-w-0 font-mono" />
                    <Button
                      variant="outline"
                      size="icon"
                      className="shrink-0"
                      onClick={() => copyToClipboard(originUrl)}
                      aria-label={t('servers.extransfer.export.copyUrl')}
                    >
                      <Copy aria-hidden="true" />
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground">{t('servers.extransfer.export.urlHelp')}</p>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="extransfer-token">{t('servers.extransfer.export.tokenLabel')}</Label>
                  <div className="flex flex-col gap-2 sm:flex-row">
                    <Input id="extransfer-token" readOnly value={sessionData.token} className="min-w-0 font-mono" />
                    <Button onClick={() => copyToClipboard(sessionData.token)} className="shrink-0">
                      {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
                      {copied ? t('servers.extransfer.export.copied') : t('servers.extransfer.export.copyToken')}
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground">{t('servers.extransfer.export.tokenHelp')}</p>
                </div>
              </div>

              <div className="flex flex-col gap-2 border-t border-border/50 px-4 py-3 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between sm:px-5">
                <p className="flex items-center gap-2" role="status">
                  <span className="relative flex h-2 w-2" aria-hidden="true">
                    <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-primary opacity-60 motion-reduce:animate-none" />
                    <span className="relative inline-flex h-2 w-2 rounded-full bg-primary" />
                  </span>
                  {t('servers.extransfer.export.waiting')}
                </p>
                <p className="flex min-w-0 items-center gap-1.5">
                  <span className="shrink-0">{t('servers.extransfer.export.sessionId')}</span>
                  <span className="truncate font-mono">{sessionData.session_id}</span>
                </p>
              </div>
            </div>
          )}
        </TabsContent>

        {/* IMPORT */}
        <TabsContent value="import" className="mt-5 space-y-5">
          <div className="flex gap-3 rounded-xl border border-warning/30 bg-warning/10 p-4" role="note">
            <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0 text-warning" aria-hidden="true" />
            <div className="space-y-1 text-sm">
              <p className="font-medium text-warning">{t('servers.extransfer.import.warningTitle')}</p>
              <p className="text-pretty text-muted-foreground">{t('servers.extransfer.import.warningBody')}</p>
            </div>
          </div>

          <div className="grid gap-5 rounded-xl border border-border/60 bg-background/30 p-4 sm:p-5">
            <div className="space-y-2">
              <Label htmlFor="extransfer-import-url">{t('servers.extransfer.import.urlLabel')}</Label>
              <Input
                id="extransfer-import-url"
                type="url"
                inputMode="url"
                placeholder="https://panel.example.com"
                value={importUrl}
                onChange={e => setImportUrl(e.target.value)}
                className="font-mono"
              />
              <p className="text-xs text-muted-foreground">{t('servers.extransfer.import.urlHelp')}</p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="extransfer-import-token">{t('servers.extransfer.import.tokenLabel')}</Label>
              <Input
                id="extransfer-import-token"
                type="password"
                autoComplete="off"
                placeholder={t('servers.extransfer.import.tokenPlaceholder')}
                value={importToken}
                onChange={e => setImportToken(e.target.value)}
                className="font-mono"
              />
              <p className="text-xs text-muted-foreground">{t('servers.extransfer.import.tokenHelp')}</p>
            </div>

            <div>
              <Button onClick={executeImport} disabled={importing} className="w-full sm:w-auto">
                {importing ? (
                  <>
                    <Loader2 className="animate-spin" aria-hidden="true" />
                    <span className="truncate">{importStep || t('servers.extransfer.import.migrating')}</span>
                  </>
                ) : (
                  <>
                    {t('servers.extransfer.import.start')}
                    <ArrowRight aria-hidden="true" />
                  </>
                )}
              </Button>
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </Card>
  );
}
