'use client';
import { useState, useEffect } from 'react';
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { sileo } from "@/lib/toast";
import { Loader2, Copy, ShieldAlert, ArrowRightLeft, CheckCircle2, DownloadCloud, UploadCloud, Clock, KeyRound, Globe, ArrowRight } from 'lucide-react';
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
              sileo.success({ title: 'Transferencia completa', description: 'El servidor ha sido migrado exitosamente.' });
              setImportStep('');
              setImporting(false);
            } else if (data.status.startsWith('ERROR: ')) {
              sileo.error({ title: 'Error en la migración', description: data.status.substring(7) });
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

    checkStatus();
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
      sileo.success({ title: 'Transfer token expired', description: 'The 15-minute window has passed.' });
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
      if (!res.ok) throw new Error(data.message || 'Failed to generate transfer token');

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
        title: 'Token generated',
        description: 'Valid for 15 minutes. Will remain visible if you navigate away.'
      });
    } catch (e: any) {
      sileo.error({ title: 'Error', description: e.message });
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
    sileo.success({ title: 'Copied to clipboard' });
  };

  const executeImport = async () => {
    if (!importUrl || !importToken) {
      sileo.error({ title: 'Missing fields', description: 'Please provide both the URL and Token.' });
      return;
    }
    try {
      setImporting(true);
      setImportStep('Iniciando...');
      const res = await fetch(`/api/servers/${serverId}/extransfer/pull`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ origin_url: importUrl, token: importToken })
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.message || 'Failed to initiate import.');
      }

      // El polling (useEffect) se encargará de actualizar el estado a partir de aquí.
    } catch (e: any) {
      sileo.error({ title: 'Import Failed', description: e.message });
      setImportStep('');
      setImporting(false);
    }
  };

  const originUrl = typeof window !== 'undefined' ? window.location.origin : '';

  return (
    <div className="mt-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
      <Card className="border-0 bg-transparent shadow-none">
        <CardHeader className="px-0">
          <div className="flex items-center gap-4">
            <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl border border-primary/30 bg-gradient-to-br from-primary/25 via-accent/15 to-transparent text-primary shadow-[0_0_20px_rgb(0_0_0/0.3)]">
              <ArrowRightLeft className="h-5 w-5" />
            </div>
            <div>
              <CardTitle className="font-headline text-2xl">Federated Server Transfer</CardTitle>
              <CardDescription className="flex items-center gap-1.5">
                Zero-Trust migration between hosting providers
                <span className="inline-flex items-center gap-1 rounded-md border border-primary/30 bg-primary/10 px-1.5 py-px text-[10px] font-semibold uppercase tracking-wide text-primary">
                  <KeyRound className="h-2.5 w-2.5" />
                  Zero-Trust
                </span>
              </CardDescription>
            </div>
          </div>
        </CardHeader>

        <Tabs value={activeTab} onValueChange={setActiveTab} className="mt-6 w-full">
          <TabsList className="mb-8 grid h-auto w-full grid-cols-2 overflow-hidden rounded-xl border border-border/70 bg-muted/50 p-0">
            <TabsTrigger value="export" className="group relative flex h-11 items-center justify-center gap-2 rounded-none border-r border-border/50 px-3 py-1.5 text-sm font-medium text-muted-foreground transition-colors duration-200 hover:bg-accent/40 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring after:pointer-events-none after:absolute after:bottom-0 after:left-1/2 after:h-0.5 after:w-14 after:-translate-x-1/2 after:rounded-full after:bg-primary after:opacity-0 after:transition-opacity after:duration-200 data-[state=active]:bg-card data-[state=active]:font-semibold data-[state=active]:text-foreground data-[state=active]:shadow-none data-[state=active]:after:opacity-100">
              <UploadCloud className="h-4 w-4 text-muted-foreground transition-colors duration-200 group-data-[state=active]:text-primary" />
              Export to remote panel
            </TabsTrigger>
            <TabsTrigger value="import" className="group relative flex h-11 items-center justify-center gap-2 rounded-none px-3 py-1.5 text-sm font-medium text-muted-foreground transition-colors duration-200 hover:bg-accent/40 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring after:pointer-events-none after:absolute after:bottom-0 after:left-1/2 after:h-0.5 after:w-14 after:-translate-x-1/2 after:rounded-full after:bg-primary after:opacity-0 after:transition-opacity after:duration-200 data-[state=active]:bg-card data-[state=active]:font-semibold data-[state=active]:text-foreground data-[state=active]:shadow-none data-[state=active]:after:opacity-100">
              <DownloadCloud className="h-4 w-4 text-muted-foreground transition-colors duration-200 group-data-[state=active]:text-primary" />
              Import from remote panel
            </TabsTrigger>
          </TabsList>

          {/*  EXPORT */}
          <TabsContent value="export" className="mt-0 space-y-6 px-0">
            <Alert className="border-primary/20 bg-primary/5">
              <ShieldAlert className="h-4 w-4 text-primary" />
              <AlertTitle className="text-primary">How it works</AlertTitle>
              <AlertDescription className="mt-2 text-muted-foreground">
                Generating a token will allow another panel to request the transfer of your server&apos;s data.
                The system uses military-grade cryptographic hashing. Give this token <strong>only</strong> to the provider you are migrating to.
                The token persists for 15 minutes even if you navigate away.
              </AlertDescription>
            </Alert>

            {!sessionData ? (
              <div className="group relative flex flex-col items-center justify-center overflow-hidden rounded-xl border border-dashed border-border/80 bg-card p-10 text-center">
                <img
                  src="/img/Fondos/minecraft-shaders-anime-hd-wallpaper-preview.jpg"
                  alt=""
                  aria-hidden="true"
                  className="pointer-events-none absolute inset-0 h-full w-full object-cover opacity-20"
                />
                <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-card via-card/85 to-card/40" />
                <div className="relative mx-auto mb-5 grid h-16 w-16 place-items-center rounded-2xl border border-primary/30 bg-gradient-to-br from-primary/25 via-accent/15 to-transparent shadow-[0_0_30px_rgb(0_0_0/0.4)]">
                  <ShieldAlert className="h-7 w-7 text-primary" />
                </div>
                <h3 className="relative mb-2 font-headline text-lg font-semibold">Ready to generate token</h3>
                <p className="relative mb-6 max-w-md text-sm text-muted-foreground">
                  The token is valid for exactly 15 minutes. Once generated, head over to your destination hosting panel and enter the token to begin the Zero-Trust handshake.
                </p>
                <Button size="lg" onClick={generateTransferToken} disabled={loading} className="relative px-8 font-semibold">
                  {loading
                    ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Generating...</>
                    : 'Generate Transfer Token'
                  }
                </Button>
              </div>
            ) : (
              <div className="group relative space-y-6 overflow-hidden rounded-xl border border-border/80 bg-card p-6">
                <img
                  src="/img/Fondos/minecraft-shaders-anime-hd-wallpaper-preview.jpg"
                  alt=""
                  aria-hidden="true"
                  className="pointer-events-none absolute inset-0 h-full w-full object-cover opacity-15"
                />
                <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-transparent via-card/80 to-card/95" />
                <div className="pointer-events-none absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-primary via-accent to-transparent" />

                {/* Header con temporizador */}
                <div className="relative flex flex-wrap items-center justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <div className="grid h-10 w-10 place-items-center rounded-lg bg-success/15 text-success">
                      <CheckCircle2 className="h-5 w-5" />
                    </div>
                    <h3 className="font-headline text-lg font-bold">Transfer Session Created</h3>
                  </div>
                  {timeLeft !== null && (
                    <div className={cn(
                      'flex items-center gap-2 rounded-full px-3 py-1 font-mono text-sm font-bold backdrop-blur-sm',
                      timeLeft < 120
                        ? 'bg-destructive/15 text-destructive animate-pulse'
                        : 'bg-success/15 text-success'
                    )}>
                      <Clock className="h-3.5 w-3.5" />
                      {formatTime(timeLeft)}
                    </div>
                  )}
                </div>

                <div className="relative grid gap-4">
                  <div className="space-y-1.5">
                    <Label>Destination Import URL</Label>
                    <p className="text-sm text-muted-foreground">Provide this URL to your new hosting so they know where to pull the data from.</p>
                    <div className="mt-2 flex gap-2">
                      <Input readOnly value={originUrl} className="bg-background/60 font-mono backdrop-blur-sm" />
                      <Button variant="outline" size="icon" className="shrink-0" onClick={() => copyToClipboard(originUrl)}>
                        <Copy className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>

                  <div className="space-y-1.5">
                    <Label className="text-primary">Secure Transfer Token</Label>
                    <p className="text-sm text-muted-foreground">
                      One-time crypto payload. Expires in {timeLeft !== null ? formatTime(timeLeft) : '—'}.
                    </p>
                    <div className="relative mt-2">
                      <Input readOnly value={sessionData.token} className="bg-background/60 pr-32 font-mono font-bold text-primary backdrop-blur-sm" />
                      <Button
                        variant="default"
                        onClick={() => copyToClipboard(sessionData.token)}
                        className="absolute right-1 top-1 h-8 px-4"
                      >
                        {copied ? 'Copied!' : 'Copy Token'}
                      </Button>
                    </div>
                  </div>
                </div>

                <div className="relative mt-6 flex flex-col items-center justify-between gap-4 border-t border-border/60 pt-6 text-sm text-muted-foreground sm:flex-row">
                  <p className="flex items-center gap-2">
                    <span className="relative flex h-2 w-2">
                      <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-primary opacity-60" />
                      <span className="relative inline-flex h-2 w-2 rounded-full bg-primary" />
                    </span>
                    Waiting for Destination Panel handshake...
                  </p>
                  <p className="flex items-center gap-1.5">
                    Session ID:
                    <span className="font-mono text-xs opacity-50">{sessionData.session_id}</span>
                  </p>
                </div>
              </div>
            )}
          </TabsContent>

          {/*IMPORT*/}
          <TabsContent value="import" className="mt-0 space-y-6 px-0">
            <Alert className="border-warning/25 bg-warning/10">
              <ShieldAlert className="h-4 w-4 text-warning" />
              <AlertTitle className="text-warning">Warning: Overwrite Server</AlertTitle>
              <AlertDescription className="mt-2 text-muted-foreground">
                Initiating a pull from an external provider will stop this current server and <strong>overwrite its files and configuration</strong> with the incoming data. Make sure you have a backup.
              </AlertDescription>
            </Alert>

            <div className="group relative space-y-6 overflow-hidden rounded-xl border border-border/80 bg-card p-6">
              <img
                src="/img/Fondos/minecraft-shaders-anime-hd-wallpaper-preview.jpg"
                alt=""
                aria-hidden="true"
                className="pointer-events-none absolute inset-0 h-full w-full object-cover opacity-15"
              />
              <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-transparent via-card/80 to-card/95" />
              <div className="pointer-events-none absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-primary via-accent to-transparent" />

              <div className="relative grid gap-6">
                <div className="space-y-2">
                  <Label className="flex items-center gap-1.5">
                    <Globe className="h-3.5 w-3.5 text-primary" />
                    Origin Panel URL
                  </Label>
                  <Input
                    placeholder="https://panel.other-host.com"
                    value={importUrl}
                    onChange={e => setImportUrl(e.target.value)}
                    className="bg-background/60 backdrop-blur-sm"
                  />
                  <p className="text-xs text-muted-foreground">The API endpoint or main URL of the host providing the server.</p>
                </div>

                <div className="space-y-2">
                  <Label className="flex items-center gap-1.5">
                    <KeyRound className="h-3.5 w-3.5 text-primary" />
                    Transfer Token
                  </Label>
                  <Input
                    type="password"
                    placeholder="Paste the secure transfer token here..."
                    value={importToken}
                    onChange={e => setImportToken(e.target.value)}
                    className="bg-background/60 backdrop-blur-sm"
                  />
                  <p className="text-xs text-muted-foreground">The generated cryptographically secure hash provided by the origin panel.</p>
                </div>

                <Button size="lg" onClick={executeImport} disabled={importing} className="w-full font-semibold sm:w-auto">
                  {importing
                    ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> {importStep || 'Migrating...'}</>
                    : <>Initiate Secure Pull <ArrowRight className="ml-2 h-4 w-4" /></>
                  }
                </Button>
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </Card>
    </div>
  );
}
