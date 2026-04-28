'use client';
import { useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle, CardFooter } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { useToast } from '@/hooks/use-toast';
import { Loader2, Copy, ShieldAlert, ArrowRightLeft, CheckCircle2, DownloadCloud, UploadCloud } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

type ExternalTransferViewProps = {
  serverId: string;
};

export default function ExternalTransferView({ serverId }: ExternalTransferViewProps) {
  const { t } = useTranslations();
  const { toast } = useToast();
  
  const [loading, setLoading] = useState(false);
  const [sessionData, setSessionData] = useState<{ token: string; expires_in: number; session_id: string } | null>(null);
  const [copied, setCopied] = useState(false);
  const [importUrl, setImportUrl] = useState('');
  const [importToken, setImportToken] = useState('');
  const [importing, setImporting] = useState(false);

  const generateTransferToken = async () => {
    try {
      setLoading(true);
      // PufferPanel authenticates API via cookies implicitly.
      const res = await fetch(`/api/servers/${serverId}/extransfer/create`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        }
      });
      
      const data = await res.json();
      
      if (!res.ok) {
        throw new Error(data.message || 'Failed to generate transfer token');
      }
      
      setSessionData(data);
      toast({
        title: 'Success',
        description: 'Federated transfer token generated securely.'
      });
    } catch (e: any) {
      toast({
        variant: 'destructive',
        title: 'Error',
        description: e.message
      });
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
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ origin_url: importUrl, token: importToken })
      });
      
      const data = await res.json();
      if (!res.ok) throw new Error(data.message || 'Failed to initiate import.');

      toast({
        title: 'Import Initiated',
        description: 'The server data is being pulled from the source. Please check the console.'
      });
    } catch (e: any) {
      toast({
        variant: 'destructive',
        title: 'Import Failed',
        description: e.message
      });
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
            <TabsTrigger value="export" className="flex gap-2"><UploadCloud className="h-4 w-4"/> Export to remote panel</TabsTrigger>
            <TabsTrigger value="import" className="flex gap-2"><DownloadCloud className="h-4 w-4"/> Import from remote panel</TabsTrigger>
          </TabsList>

          <TabsContent value="export" className="space-y-6 px-0 mt-0">
          <Alert variant="default" className="border-blue-500/20 bg-blue-500/5">
            <ShieldAlert className="h-4 w-4 text-blue-500" />
            <AlertTitle className="text-blue-500">How it works</AlertTitle>
            <AlertDescription className="text-muted-foreground mt-2">
              Generating a token will allow another competing panel or host to request the transfer of your server's data. 
              The system uses military-grade cryptographic hashing. Give this token <strong>only</strong> to the provider you are migrating to.
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
                {loading ? (
                  <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Generating...</>
                ) : (
                  'Generate Transfer Token'
                )}
              </Button>
            </div>
          ) : (
            <div className="space-y-6 rounded-xl border p-6 bg-accent/5 relative overflow-hidden">
              <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-blue-500 to-indigo-500" />
              
              <div className="flex items-center gap-3">
                <CheckCircle2 className="h-6 w-6 text-green-500" />
                <h3 className="text-lg font-bold">Transfer Session Created</h3>
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
                  <p className="text-sm text-muted-foreground">This is your one-time crypto payload. It expires in 15 minutes.</p>
                  <div className="flex gap-2 mt-2 relative">
                    <Input readOnly value={sessionData.token} className="font-mono text-indigo-400 font-bold bg-background/50 pr-24" />
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
                  {importing ? (
                    <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Verifying handshake...</>
                  ) : (
                    'Initiate Secure Pull'
                  )}
                </Button>
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </Card>
    </div>
  );
}
