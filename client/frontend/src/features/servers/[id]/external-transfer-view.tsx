'use client';
import { useState, useEffect, useCallback, useRef, type ReactNode } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { sileo } from "@/lib/toast";
import { Loader2, Copy, Check, CheckCircle2, DownloadCloud, UploadCloud, Clock, KeyRound, ArrowRight, Info, TriangleAlert, Ban, CircleX, Activity } from 'lucide-react';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { api, ApiError } from '@/lib/api-client';
import { useTranslations } from '@/contexts/translations-context';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';
import { EmptyState } from '@/components/empty-state';

type ExternalTransferViewProps = {
  serverId: string;
};

interface StoredTransfer {
  token: string;
  expires_in: number;
  session_id: string;
  generatedAt: number; // Date.now() al generar
}

// Estado de la sesión más reciente que este panel creó para el servidor (lado origen)
type OriginStatus = 'CREATED' | 'VALIDATED' | 'MIGRATING' | 'CONSUMED' | 'COMPLETED' | 'FAILED' | 'CANCELLED';

interface OriginSession {
  session_id: string;
  status: OriginStatus;
  expired: boolean;
  expires_at: string;
  last_heartbeat_at?: string | null;
  updated_at: string;
}

interface TransferStatusResponse {
  status?: string;
  pulling?: boolean;
  origin?: OriginSession;
}

type ImportOutcome = 'done' | 'cancelled' | 'cancelledByOrigin';
type CancelTarget = 'export' | 'import';

const LIVE_ORIGIN_STATUSES = new Set<OriginStatus>(['CREATED', 'VALIDATED', 'MIGRATING']);

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

// Qué mostrar en la pestaña de exportación según la sesión de origen
const describeOrigin = (origin: OriginSession | null, hasToken: boolean) => {
  const originLive = !!origin && !origin.expired && LIVE_ORIGIN_STATUSES.has(origin.status);
  const originMigrating = originLive && origin?.status === 'MIGRATING';
  // token generado desde otro navegador: no lo tenemos, pero se puede cancelar
  const tokenElsewhere = originLive && !originMigrating && !hasToken;
  const exportOutcome =
    !originLive && origin && (origin.status === 'COMPLETED' || origin.status === 'CANCELLED') ? origin.status : null;
  return { originMigrating, tokenElsewhere, exportOutcome };
};

const formatTime = (s: number) => {
  const m = Math.floor(s / 60).toString().padStart(2, '0');
  const sec = (s % 60).toString().padStart(2, '0');
  return `${m}:${sec}`;
};

export default function ExternalTransferView({ serverId }: Readonly<ExternalTransferViewProps>) {
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
  const [origin, setOrigin] = useState<OriginSession | null>(null);
  const [pulling, setPulling] = useState(false);
  const [importOutcome, setImportOutcome] = useState<ImportOutcome | null>(null);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [cancelTarget, setCancelTarget] = useState<CancelTarget>('export');
  const [cancelling, setCancelling] = useState(false);
  const lastStatusRef = useRef('');

  // Al montar: recuperar token guardado si aún es válido
  useEffect(() => {
    const stored = loadStoredTransfer(serverId);
    if (stored) {
      setSessionData(stored);
      const elapsed = (Date.now() - stored.generatedAt) / 1000;
      setTimeLeft(Math.max(0, Math.round(900 - elapsed)));
    }
  }, [serverId]);

  // Un estado nuevo de la importación: terminada, cancelada, fallida o el paso en curso
  const applyImportStatus = useCallback((status: string) => {
    setImporting(true);
    setActiveTab('import');

    if (status === 'DONE') {
      sileo.success({ title: t('servers.extransfer.toast.doneTitle'), description: t('servers.extransfer.toast.doneDescription') });
      setImportStep('');
      setImporting(false);
      setImportOutcome('done');
    } else if (status === 'CANCELLED' || status === 'CANCELLED_BY_ORIGIN') {
      const byOrigin = status === 'CANCELLED_BY_ORIGIN';
      sileo.info({
        title: t('servers.extransfer.toast.cancelledTitle'),
        description: t(byOrigin ? 'servers.extransfer.toast.cancelledByOriginDescription' : 'servers.extransfer.toast.cancelledDescription')
      });
      setImportStep('');
      setImporting(false);
      setImportOutcome(byOrigin ? 'cancelledByOrigin' : 'cancelled');
    } else if (status.startsWith('ERROR: ')) {
      sileo.error({ title: t('servers.extransfer.toast.failedTitle'), description: status.substring(7) });
      setImportStep('');
      setImporting(false);
    } else {
      setImportStep(status);
    }
  }, [t]);

  // Polling de estado de transferencia (importación en curso y sesión de origen)
  const checkStatus = useCallback(async () => {
    try {
      const res = await fetch(`/api/servers/${serverId}/extransfer/status`);
      if (!res.ok) return;
      const data: TransferStatusResponse = await res.json();
      setOrigin(data.origin ?? null);
      setPulling(!!data.pulling);

      const status = data.status ?? '';
      if (status && status !== lastStatusRef.current) {
        lastStatusRef.current = status;
        applyImportStatus(status);
      } else if (!status && lastStatusRef.current) {
        // El backend limpió el estado, terminamos.
        lastStatusRef.current = '';
        setImporting(false);
        setImportStep('');
      }
    } catch {
      // Ignorar errores de red en polling
    }
  }, [serverId, applyImportStatus]);

  useEffect(() => {
    void checkStatus();
    const interval = setInterval(() => void checkStatus(), 2000);
    return () => clearInterval(interval);
  }, [checkStatus]);

  // Cuando el destino ya consumió el token (o la sesión terminó), el token deja de ser útil
  useEffect(() => {
    if (!origin || !sessionData || origin.session_id !== sessionData.session_id) return;
    if (origin.status === 'CREATED' || origin.status === 'VALIDATED') return;
    setSessionData(null);
    setTimeLeft(null);
    clearTransfer(serverId);
  }, [origin, sessionData, serverId]);

  // Cuenta regresiva
  useEffect(() => {
    if (timeLeft === null) return;
    if (timeLeft <= 0) {
      setSessionData(null);
      clearTransfer(serverId);
      setTimeLeft(null);
      sileo.info({ title: t('servers.extransfer.toast.expiredTitle'), description: t('servers.extransfer.toast.expiredDescription') });
      return;
    }
    const timer = setTimeout(() => setTimeLeft(prev => (prev !== null ? prev - 1 : null)), 1000);
    return () => clearTimeout(timer);
  }, [timeLeft, serverId]);

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
      setImportOutcome(null);
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

  const askCancel = (target: CancelTarget) => {
    setCancelTarget(target);
    setCancelOpen(true);
  };

  const forgetToken = () => {
    setSessionData(null);
    setTimeLeft(null);
    clearTransfer(serverId);
  };

  const cancelTransfer = async () => {
    setCancelling(true);
    try {
      const data = await api.post<{ cancelled_pull: boolean; cancelled_sessions: number }>(`/api/servers/${serverId}/extransfer/cancel`, {});
      // la importación cancelada la anuncia el polling cuando el proceso termina
      if (data.cancelled_sessions > 0) {
        forgetToken();
        sileo.success({ title: t('servers.extransfer.toast.cancelledTitle'), description: t('servers.extransfer.toast.cancelledDescription') });
      }
    } catch (e: unknown) {
      if (e instanceof ApiError && e.data?.error_code === 'NO_ACTIVE_TRANSFER') {
        if (cancelTarget === 'export') forgetToken();
        sileo.info({ title: t('servers.extransfer.toast.nothingToCancel') });
      } else {
        sileo.error({ title: t('servers.extransfer.toast.cancelError'), description: e instanceof Error ? e.message : undefined });
      }
    } finally {
      setCancelling(false);
      void checkStatus();
    }
  };

  const { originMigrating, tokenElsewhere, exportOutcome } = describeOrigin(origin, !!sessionData);
  const showImportCancel = importing || pulling;

  const cancelButton = (target: CancelTarget, className?: string) => (
    <Button
      variant="outline"
      size="sm"
      onClick={() => askCancel(target)}
      disabled={cancelling}
      className={cn('border-destructive/40 text-destructive-bright hover:bg-destructive/10 hover:text-destructive-bright', className)}
    >
      {cancelling ? <Loader2 className="animate-spin" aria-hidden="true" /> : <CircleX aria-hidden="true" />}
      {cancelling ? t('servers.extransfer.cancel.cancelling') : t('servers.extransfer.cancel.button')}
    </Button>
  );

  const originUrl = typeof window !== 'undefined' ? window.location.origin : '';

  let exportBody: ReactNode;
  if (originMigrating && origin) {
    exportBody = <ExportInProgress origin={origin} cancelAction={cancelButton('export')} />;
  } else if (tokenElsewhere && origin) {
    exportBody = (
      <div className="flex flex-col gap-3 rounded-xl border border-border/60 bg-background/30 p-4 sm:flex-row sm:items-center sm:justify-between sm:p-5">
        <div className="flex min-w-0 gap-3">
          <KeyRound className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          <div className="min-w-0 space-y-1 text-sm">
            <p className="font-medium">{t('servers.extransfer.export.sessionTitle')}</p>
            <p className="text-pretty text-muted-foreground">{t('servers.extransfer.export.tokenElsewhere')}</p>
          </div>
        </div>
        {cancelButton('export', 'w-full sm:w-auto')}
      </div>
    );
  } else if (sessionData) {
    exportBody = (
      <TokenCard
        sessionData={sessionData}
        timeLeft={timeLeft}
        copied={copied}
        originUrl={originUrl}
        onCopy={copyToClipboard}
        cancelAction={cancelButton('export')}
      />
    );
  } else {
    exportBody = <GenerateTokenPrompt loading={loading} onGenerate={generateTransferToken} />;
  }

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

          <ExportOutcomeNotice outcome={exportOutcome} />

          {exportBody}
        </TabsContent>

        {/* IMPORT */}
        <TabsContent value="import" className="mt-5 space-y-5">
          <ImportOutcomeNotice outcome={importOutcome} />

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

            <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
              <Button onClick={executeImport} disabled={importing} className="w-full min-w-0 sm:w-auto sm:max-w-full">
                <ImportButtonContent importing={importing} step={importStep} />
              </Button>
              {showImportCancel && cancelButton('import', 'h-10 w-full sm:w-auto')}
            </div>
          </div>
        </TabsContent>
      </Tabs>

      <AlertDialog open={cancelOpen} onOpenChange={setCancelOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('servers.extransfer.cancel.title')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(cancelTarget === 'import' ? 'servers.extransfer.cancel.importDescription' : 'servers.extransfer.cancel.exportDescription')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('servers.extransfer.cancel.keep')}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => void cancelTransfer()}
            >
              {t('servers.extransfer.cancel.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}

function ExportOutcomeNotice({ outcome }: Readonly<{ outcome: OriginStatus | null }>) {
  const { t } = useTranslations();
  return (
    <>
      {outcome === 'COMPLETED' && (
        <output className="flex gap-3 rounded-xl border border-success/30 bg-success/10 p-4">
          <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-success" aria-hidden="true" />
          <div className="space-y-1 text-sm">
            <p className="font-medium text-success">{t('servers.extransfer.export.completedTitle')}</p>
            <p className="text-pretty text-muted-foreground">{t('servers.extransfer.export.completedBody')}</p>
          </div>
        </output>
      )}
      {outcome === 'CANCELLED' && (
        <output className="flex gap-3 rounded-xl border border-border/60 bg-background/30 p-4">
          <Ban className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          <div className="space-y-1 text-sm">
            <p className="font-medium">{t('servers.extransfer.export.cancelledTitle')}</p>
            <p className="text-pretty text-muted-foreground">{t('servers.extransfer.export.cancelledBody')}</p>
          </div>
        </output>
      )}
    </>
  );
}

function ExportInProgress({ origin, cancelAction }: Readonly<{ origin: OriginSession; cancelAction: ReactNode }>) {
  const { t, language } = useTranslations();
  const lastHeartbeat = origin.last_heartbeat_at ? new Date(origin.last_heartbeat_at) : null;
  return (
    <div className="overflow-hidden rounded-xl border border-border/60 bg-background/30">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border/50 px-4 py-3 sm:px-5">
        <div className="flex items-center gap-2">
          <Loader2 className="h-4 w-4 animate-spin text-primary motion-reduce:animate-none" aria-hidden="true" />
          <h3 className="text-sm font-semibold">{t('servers.extransfer.export.inProgressTitle')}</h3>
        </div>
        {cancelAction}
      </div>
      <div className="space-y-2 p-4 text-sm sm:p-5">
        <p className="text-pretty text-muted-foreground">{t('servers.extransfer.export.inProgressBody')}</p>
        <output className="flex items-center gap-2 text-xs text-muted-foreground">
          <Activity className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          {lastHeartbeat ? (
            <span>
              {t('servers.extransfer.export.lastHeartbeat')}{' '}
              <time dateTime={lastHeartbeat.toISOString()} className="tabular-nums">
                {lastHeartbeat.toLocaleTimeString(language)}
              </time>
            </span>
          ) : (
            <span>{t('servers.extransfer.export.noHeartbeat')}</span>
          )}
        </output>
      </div>
      <div className="flex min-w-0 items-center gap-1.5 border-t border-border/50 px-4 py-3 text-xs text-muted-foreground sm:px-5">
        <span className="shrink-0">{t('servers.extransfer.export.sessionId')}</span>
        <span className="truncate font-mono">{origin.session_id}</span>
      </div>
    </div>
  );
}

function GenerateTokenPrompt({ loading, onGenerate }: Readonly<{ loading: boolean; onGenerate: () => void }>) {
  const { t } = useTranslations();
  return (
    <EmptyState
      icon={KeyRound}
      size="sm"
      title={t('servers.extransfer.export.emptyTitle')}
      description={t('servers.extransfer.export.emptyDescription')}
      action={
        <Button onClick={onGenerate} disabled={loading}>
          {loading ? <Loader2 className="animate-spin" aria-hidden="true" /> : <KeyRound aria-hidden="true" />}
          {loading ? t('servers.extransfer.export.generating') : t('servers.extransfer.export.generate')}
        </Button>
      }
    />
  );
}

type TokenCardProps = {
  sessionData: StoredTransfer;
  timeLeft: number | null;
  copied: boolean;
  originUrl: string;
  onCopy: (text: string) => void;
  cancelAction: ReactNode;
};

function TokenCard({ sessionData, timeLeft, copied, originUrl, onCopy, cancelAction }: Readonly<TokenCardProps>) {
  const { t } = useTranslations();
  return (
    <div className="overflow-hidden rounded-xl border border-border/60 bg-background/30">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border/50 px-4 py-3 sm:px-5">
        <div className="flex items-center gap-2">
          <CheckCircle2 className="h-4 w-4 text-success" aria-hidden="true" />
          <h3 className="text-sm font-semibold">{t('servers.extransfer.export.sessionTitle')}</h3>
        </div>
        <div className="flex flex-wrap items-center gap-2">
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
          {cancelAction}
        </div>
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
              onClick={() => onCopy(originUrl)}
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
            <Button onClick={() => onCopy(sessionData.token)} className="shrink-0">
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
  );
}

function ImportOutcomeNotice({ outcome }: Readonly<{ outcome: ImportOutcome | null }>) {
  const { t } = useTranslations();
  return (
    <>
      {outcome === 'done' && (
        <output className="flex gap-3 rounded-xl border border-success/30 bg-success/10 p-4">
          <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-success" aria-hidden="true" />
          <div className="space-y-1 text-sm">
            <p className="font-medium text-success">{t('servers.extransfer.toast.doneTitle')}</p>
            <p className="text-pretty text-muted-foreground">{t('servers.extransfer.toast.doneDescription')}</p>
          </div>
        </output>
      )}
      {(outcome === 'cancelled' || outcome === 'cancelledByOrigin') && (
        <output className="flex gap-3 rounded-xl border border-border/60 bg-background/30 p-4">
          <Ban className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          <div className="space-y-1 text-sm">
            <p className="font-medium">{t('servers.extransfer.import.cancelledTitle')}</p>
            <p className="text-pretty text-muted-foreground">
              {t(outcome === 'cancelledByOrigin' ? 'servers.extransfer.import.cancelledByOriginBody' : 'servers.extransfer.import.cancelledBody')}
            </p>
          </div>
        </output>
      )}
    </>
  );
}

function ImportButtonContent({ importing, step }: Readonly<{ importing: boolean; step: string }>) {
  const { t } = useTranslations();
  if (importing) {
    return (
      <>
        <Loader2 className="animate-spin" aria-hidden="true" />
        <span className="truncate">{step || t('servers.extransfer.import.migrating')}</span>
      </>
    );
  }
  return (
    <>
      {t('servers.extransfer.import.start')}
      <ArrowRight aria-hidden="true" />
    </>
  );
}
