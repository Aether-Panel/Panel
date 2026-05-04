'use client';
import { useState, useEffect } from 'react';
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { useToast } from '@/hooks/use-toast';
import { Loader2, Copy, ShieldAlert, ArrowRightLeft, CheckCircle2, DownloadCloud, UploadCloud, Clock } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

type ExternalTransferViewProps = {
  serverId: string;
};

// ─── Helpers de persistencia (15 min = 900 segundos) ─────────────────────────
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

// ─────────────────────────────────────────────────────────────────────────────

export default function ExternalTransferView({ serverId }: ExternalTransferViewProps) {
  const { t } = useTranslations();
  const { toast } = useToast();

  const [loading, setLoading] = useState(false);
  const [sessionData, setSessionData] = useState<StoredTransfer | null>(null);
  const [copied, setCopied] = useState(false);
  const [importUrl, setImportUrl] = useState('');
  const [importToken, setImportToken] = useState('');
  const [importing, setImporting] = useState(false);
  const [timeLeft, setTimeLeft] = useState<number | null>(null);

  // ── Al montar: recuperar token guardado si aún es válido ─────────────────
  useEffect(() => {
    const stored = loadStoredTransfer(serverId);
    if (stored) {
      setSessionData(stored);
      const elapsed = (Date.now() - stored.generatedAt) / 1000;
      setTimeLeft(Math.max(0, Math.round(900 - elapsed)));
    }
  }, [serverId]);

  // ── Cuenta regresiva ──────────────────────────────────────────────────────
  useEffect(() => {
    if (timeLeft === null) return;
    if (timeLeft <= 0) {
      setSessionData(null);
      clearTransfer(serverId);
      setTimeLeft(null);
      toast({ title: 'Transfer token expired', description: 'The 15-minute window has passed.' });
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

  // ── Generar token (usa cookies del panel, sin necesidad de Bearer) ─────────
  const generateTransferToken = async () => {
    try {
      setLoading(true);
      const res = await fetch(`/api/servers/${serverId}/extransfer/create`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include' // usa la cookie puffer_auth del panel
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

      toast({
        title: 'Token generated',
        description: 'Valid for 15 minutes. Will remain visible if you navigate away.'
      });
    } catch (e: any) {
      toast({ variant: 'destructive', title: 'Error', description: e.message });
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
    toast({ title: 'Copied to clipboard' });
  };

  const executeImport = async () => {
    if (!importUrl || !importToken) {
      toast({ variant: 'destructive', title: 'Missing fields', description: 'Please provide both the URL and Token.' });
      return;
    }
    try {
      setImporting(true);
      const res = await fetch(`/api/servers/${serverId}/extransfer/pull`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ origin_url: importUrl, token: importToken })
      });

      const data = await res.json();
      if (!res.ok) throw new Error(data.message || 'Failed to initiate import.');

      toast({
        title: 'Import Initiated',
        description: 'The server data is being pulled from the source. Please check the console.'
      });
    } catch (e: any) {
      toast({ variant: 'destructive', title: 'Import Failed', description: e.message });
    } finally {
      setImporting(false);
    }
  };

  return (
    <div className="mt-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
      <Card className="border-0 bg-transparent shadow-none">
        <CardHeader className="px-0">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-blue-500/10 text-blue-500">
              <ArrowRightLeft className="h-5 w-5" />
            </div>
            <div>
              <CardTitle className="text-2xl">Federated Server Transfer (Zero-Trust)</CardTitle>
              <CardDescription>
                Migrate this server seamlessly between different hosting providers.
              </CardDescription>
            </div>
          </div>
        </CardHeader>

        <Tabs defaultValue="export" className="w-full mt-6">
          <TabsList className="grid w-full grid-cols-2 mb-8">
            <TabsTrigger value="export" className="flex gap-2">
              <UploadCloud className="h-4 w-4" /> Export to remote panel
            </TabsTrigger>
            <TabsTrigger value="import" className="flex gap-2">
              <DownloadCloud className="h-4 w-4" /> Import from remote panel
            </TabsTrigger>
          </TabsList>

          {/* ── EXPORT ────────────────────────────────────────────────────── */}
          <TabsContent value="export" className="space-y-6 px-0 mt-0">
            <Alert variant="default" className="border-blue-500/20 bg-blue-500/5">
              <ShieldAlert className="h-4 w-4 text-blue-500" />
              <AlertTitle className="text-blue-500">How it works</AlertTitle>
              <AlertDescription className="text-muted-foreground mt-2">
                Generating a token will allow another panel to request the transfer of your server&apos;s data.
                The system uses military-grade cryptographic hashing. Give this token <strong>only</strong> to the provider you are migrating to.
                The token persists for 15 minutes even if you navigate away.
              </AlertDescription>
            </Alert>

            {!sessionData ? (
              <div className="flex flex-col items-center justify-center rounded-xl border border-dashed p-8 bg-accent/5">
                <ShieldAlert className="h-12 w-12 text-muted-foreground mb-4 opacity-20" />
                <h3 className="text-lg font-medium mb-2">Ready to generate token</h3>
                <p className="text-sm text-center text-muted-foreground max-w-md mb-6">
                  The token is valid for exactly 15 minutes. Once generated, head over to your destination hosting panel and enter the token to begin the Zero-Trust handshake.
                </p>
                <Button size="lg" onClick={generateTransferToken} disabled={loading} className="px-8 font-semibold">
                  {loading
                    ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Generating...</>
                    : 'Generate Transfer Token'
                  }
                </Button>
              </div>
            ) : (
              <div className="space-y-6 rounded-xl border p-6 bg-accent/5 relative overflow-hidden">
                <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-blue-500 to-indigo-500" />

                {/* Header con temporizador */}
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <CheckCircle2 className="h-6 w-6 text-green-500" />
                    <h3 className="text-lg font-bold">Transfer Session Created</h3>
                  </div>
                  {timeLeft !== null && (
                    <div className={`flex items-center gap-2 px-3 py-1 rounded-full text-sm font-mono font-bold transition-colors ${
                      timeLeft < 120
                        ? 'bg-red-500/10 text-red-400 animate-pulse'
                        : 'bg-green-500/10 text-green-400'
                    }`}>
                      <Clock className="h-3.5 w-3.5" />
                      {formatTime(timeLeft)}
                    </div>
                  )}
                </div>

                <div className="grid gap-4">
                  <div className="space-y-1.5">
                    <Label>Destination Import URL</Label>
                    <p className="text-sm text-muted-foreground">Provide this URL to your new hosting so they know where to pull the data from.</p>
                    <div className="flex gap-2 mt-2">
                      <Input readOnly value={window.location.origin} className="font-mono bg-background/50" />
                      <Button variant="outline" size="icon" onClick={() => copyToClipboard(window.location.origin)}>
                        <Copy className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>

                  <div className="space-y-1.5 mt-4">
                    <Label className="text-indigo-400">Secure Transfer Token</Label>
                    <p className="text-sm text-muted-foreground">
                      One-time crypto payload. Expires in {timeLeft !== null ? formatTime(timeLeft) : '—'}.
                    </p>
                    <div className="flex gap-2 mt-2 relative">
                      <Input readOnly value={sessionData.token} className="font-mono text-indigo-400 font-bold bg-background/50 pr-28" />
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

                <div className="mt-6 pt-6 border-t flex flex-col sm:flex-row gap-4 items-center justify-between text-sm text-muted-foreground">
                  <p>Status: <span className="text-blue-500 font-medium">Waiting for Destination Panel handshake...</span></p>
                  <p>Session ID: <span className="font-mono text-xs opacity-50">{sessionData.session_id}</span></p>
                </div>
              </div>
            )}
          </TabsContent>

          {/* ── IMPORT ────────────────────────────────────────────────────── */}
          <TabsContent value="import" className="space-y-6 px-0 mt-0">
            <Alert variant="default" className="border-indigo-500/20 bg-indigo-500/5">
              <DownloadCloud className="h-4 w-4 text-indigo-500" />
              <AlertTitle className="text-indigo-500">Warning: Overwrite Server</AlertTitle>
              <AlertDescription className="text-muted-foreground mt-2">
                Initiating a pull from an external provider will stop this current server and <strong>overwrite its files and configuration</strong> with the incoming data. Make sure you have a backup.
              </AlertDescription>
            </Alert>

            <div className="space-y-6 rounded-xl border p-6 bg-accent/5">
              <div className="grid gap-6">
                <div className="space-y-2">
                  <Label>Origin Panel URL</Label>
                  <Input
                    placeholder="https://panel.other-host.com"
                    value={importUrl}
                    onChange={e => setImportUrl(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">The API endpoint or main URL of the host providing the server.</p>
                </div>

                <div className="space-y-2">
                  <Label>Transfer Token</Label>
                  <Input
                    type="password"
                    placeholder="Paste the secure transfer token here..."
                    value={importToken}
                    onChange={e => setImportToken(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">The generated cryptographically secure hash provided by the origin panel.</p>
                </div>

                <Button size="lg" onClick={executeImport} disabled={importing} className="w-full sm:w-auto">
                  {importing
                    ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Verifying handshake...</>
                    : 'Initiate Secure Pull'
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
