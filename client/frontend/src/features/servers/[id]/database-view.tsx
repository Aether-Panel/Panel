'use client';
import { useState, useEffect, useCallback } from 'react';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { PlusCircle, Trash2, Database, Globe, ExternalLink, Loader2, Eye, EyeOff, Copy, Check, Lock, User as UserIcon, CheckCircle2 } from 'lucide-react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { useToast } from '@/hooks/use-toast';

type DatabaseInfo = {
  id: number;
  database_name: string;
  username: string;
  password?: string;
  host_name: string;
  host: string;
  port: number;
  remote_connection?: string;
};

type DatabaseHost = {
  id: number;
  name: string;
  host: string;
  port: number;
};

type DatabaseViewProps = {
  serverId: string;
};

type CredentialRowProps = {
  icon: typeof Database;
  label: string;
  value: string;
  secret?: boolean;
  revealed?: boolean;
  onToggleReveal?: () => void;
  copied?: boolean;
  onCopy?: () => void;
};

function CredentialRow({ icon: Icon, label, value, secret, revealed, onToggleReveal, copied, onCopy }: CredentialRowProps) {
  return (
    <div className="flex items-center justify-between gap-4 px-4 py-3">
      <span className="flex shrink-0 items-center gap-1.5 text-sm text-muted-foreground">
        <Icon className="h-3.5 w-3.5" />
        {label}
      </span>
      <span className="flex min-w-0 items-center gap-2">
        <code className="truncate font-mono text-sm text-foreground">
          {secret && !revealed ? '••••••••••••' : value}
        </code>
        {secret && onToggleReveal && (
          <Button variant="ghost" size="icon" className="h-6 w-6 shrink-0 text-muted-foreground hover:text-foreground" onClick={onToggleReveal} aria-label={revealed ? 'Ocultar' : 'Mostrar'}>
            {revealed ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
          </Button>
        )}
        {onCopy && (
          <Button variant="ghost" size="icon" className="h-6 w-6 shrink-0 text-muted-foreground hover:text-foreground" onClick={onCopy} aria-label={`Copiar ${label}`}>
            {copied ? <Check className="h-3.5 w-3.5 text-success" /> : <Copy className="h-3.5 w-3.5" />}
          </Button>
        )}
      </span>
    </div>
  );
}

export default function DatabaseView({ serverId }: DatabaseViewProps) {
  const [databases, setDatabases] = useState<DatabaseInfo[]>([]);
  const [hosts, setHosts] = useState<DatabaseHost[]>([]);
  const [loading, setLoading] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [isSuccessOpen, setIsSuccessOpen] = useState(false);
  const [createdDb, setCreatedDb] = useState<DatabaseInfo | null>(null);
  const [newDbName, setNewDbName] = useState('');
  const [selectedHost, setSelectedHost] = useState<string>('');
  const [showPassword, setShowPassword] = useState<Record<number, boolean>>({});
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const [pendingDb, setPendingDb] = useState<number | null>(null);

  const { t } = useTranslations();
  const { toast } = useToast();

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      const [dbData, hostData] = await Promise.all([
        api.get(`/api/servers/${serverId}/databases`),
        api.get('/api/databasehosts')
      ]);
      setDatabases(dbData || []);
      setHosts(hostData || []);
    } catch (err) {
      console.error('Failed to fetch database data:', err);
    } finally {
      setLoading(false);
    }
  }, [serverId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleCreate = async () => {
    if (!newDbName || !selectedHost) return;

    try {
      setIsCreating(true);
      const result = await api.post(`/api/servers/${serverId}/databases`, {
        database_host_id: parseInt(selectedHost),
        database_name: newDbName
      });

      setCreatedDb(result);
      setIsSuccessOpen(true);

      toast({
        title: t('common.success'),
        description: t('servers.database.createDialog.createSuccess')
      });

      setNewDbName('');
      setSelectedHost('');
      setIsDialogOpen(false);
      fetchData();
    } catch (err: any) {
      console.error('Failed to create database:', err);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: err.message || t('servers.database.createDialog.createError')
      });
    } finally {
      setIsCreating(false);
    }
  };

  const handleDelete = async () => {
    if (!pendingDb) return;
    try {
      await api.delete(`/api/servers/${serverId}/databases/${pendingDb}`);
      toast({
        title: t('common.success'),
        description: t('servers.database.deleteSuccess')
      });
      fetchData();
    } catch (err) {
      console.error('Failed to delete database:', err);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: t('servers.database.deleteError')
      });
    } finally {
      setPendingDb(null);
    }
  };

  const copyToClipboard = (text: string, field: string) => {
    navigator.clipboard.writeText(text);
    setCopiedField(field);
    setTimeout(() => setCopiedField(null), 2000);
    toast({
      description: "Copiado al portapapeles",
    });
  };

  const togglePassword = (id: number) => {
    setShowPassword((prev: Record<number, boolean>) => ({ ...prev, [id]: !prev[id] }));
  };

  return (
    <div className="mt-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-4">
          <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl border border-primary/30 bg-gradient-to-br from-primary/25 via-accent/15 to-transparent text-primary shadow-[0_0_20px_rgb(0_0_0/0.3)]">
            <Database className="h-5 w-5" />
          </div>
          <div>
            <h2 className="font-headline text-2xl font-bold tracking-tight">{t('servers.database.title')}</h2>
            <p className="text-muted-foreground">{t('servers.database.description')}</p>
          </div>
        </div>
        <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
          <DialogTrigger asChild>
            <Button className="shrink-0">
              <PlusCircle className="mr-2 h-4 w-4" />
              {t('servers.database.newDb')}
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('servers.database.createDialog.title')}</DialogTitle>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="db-name">{t('servers.database.createDialog.nameLabel')}</Label>
                <Input
                  id="db-name"
                  placeholder={t('servers.database.createDialog.namePlaceholder')}
                  value={newDbName}
                  onChange={(e) => setNewDbName(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="db-host">{t('servers.database.createDialog.hostLabel')}</Label>
                <Select value={selectedHost} onValueChange={setSelectedHost}>
                  <SelectTrigger>
                    <SelectValue placeholder={t('servers.database.createDialog.hostPlaceholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    {hosts.map((host: DatabaseHost) => (
                      <SelectItem key={host.id} value={host.id.toString()}>
                        {host.name} ({host.host})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setIsDialogOpen(false)}>{t('common.cancel')}</Button>
              <Button onClick={handleCreate} disabled={isCreating || !newDbName || !selectedHost}>
                {isCreating && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {t('servers.database.createDialog.createButton')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <div className="mt-6 flex flex-col gap-6">
        {loading ? (
          <div className="flex justify-center py-16">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
          </div>
        ) : databases.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border-2 border-dashed border-border/70 bg-card/50 p-14 text-center backdrop-blur-sm">
            <div className="mx-auto mb-4 grid h-14 w-14 place-items-center rounded-2xl border border-primary/25 bg-gradient-to-br from-primary/20 via-accent/10 to-transparent text-primary">
              <Database className="h-6 w-6" />
            </div>
            <h3 className="font-headline text-lg font-semibold">{t('servers.database.empty')}</h3>
            <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
              <DialogTrigger asChild>
                <Button className="mt-6">
                  <PlusCircle className="mr-2 h-4 w-4" />
                  {t('servers.database.newDb')}
                </Button>
              </DialogTrigger>
            </Dialog>
          </div>
        ) : (
          databases.map((db: DatabaseInfo) => (
            <Card key={db.id} className="group relative overflow-hidden border border-border/80 bg-card">
              <div className="pointer-events-none absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-primary via-accent to-transparent" />
              <div className="pointer-events-none absolute -right-8 -top-8 opacity-[0.05] transition-transform duration-300 group-hover:scale-110">
                <Database className="h-32 w-32" />
              </div>

              <div className="relative flex items-start justify-between gap-4 border-b border-border/60 p-5">
                <div className="flex min-w-0 items-center gap-4">
                  <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl border border-primary/25 bg-primary/10 text-primary">
                    <Database className="h-5 w-5" />
                  </div>
                  <div className="min-w-0">
                    <h4 className="truncate font-headline text-lg font-semibold leading-tight">{db.database_name}</h4>
                    <div className="mt-0.5 flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
                      <span className="flex items-center gap-1">
                        <Globe className="h-3 w-3" />
                        {db.host_name}
                      </span>
                      <span className="font-mono text-xs opacity-70">
                        {db.host}:{db.port}
                      </span>
                    </div>
                  </div>
                </div>
                <AlertDialog open={pendingDb === db.id} onOpenChange={(open) => !open && setPendingDb(null)}>
                  <AlertDialogTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="shrink-0 text-destructive hover:bg-destructive/10 hover:text-destructive"
                      onClick={() => setPendingDb(db.id)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>{t('common.confirmDelete')}</AlertDialogTitle>
                      <AlertDialogDescription>
                        {t('servers.database.deleteConfirm')}
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>{t('common.cancel') || 'Cancel'}</AlertDialogCancel>
                      <AlertDialogAction onClick={handleDelete} className="bg-red-500 hover:bg-red-600">
                        {t('common.delete') || 'Delete'}
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </div>

              <CardContent className="divide-y divide-border/60 p-0">
                <CredentialRow
                  icon={UserIcon}
                  label={t('servers.database.table.user')}
                  value={db.username}
                  copied={copiedField === `user-${db.id}`}
                  onCopy={() => copyToClipboard(db.username, `user-${db.id}`)}
                />
                <CredentialRow
                  icon={Lock}
                  label="Contraseña"
                  value={db.password || ''}
                  secret
                  revealed={!!showPassword[db.id]}
                  onToggleReveal={() => togglePassword(db.id)}
                  copied={copiedField === `pass-${db.id}`}
                  onCopy={() => copyToClipboard(db.password || '', `pass-${db.id}`)}
                />
                <CredentialRow
                  icon={Globe}
                  label="Host / Puerto"
                  value={`${db.host}:${db.port}`}
                  copied={copiedField === `host-${db.id}`}
                  onCopy={() => copyToClipboard(`${db.host}:${db.port}`, `host-${db.id}`)}
                />
              </CardContent>
            </Card>
          ))
        )}
      </div>

      <Dialog open={isSuccessOpen} onOpenChange={setIsSuccessOpen}>
        <DialogContent className="max-w-md border-primary/20 bg-gradient-to-b from-background to-background/50">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-primary">
              <CheckCircle2 className="h-5 w-5" />
              Base de Datos Creada
            </DialogTitle>
            <DialogDescription>
              Guarda estas credenciales. Por seguridad, la contraseña no se volverá a mostrar así de claro.
            </DialogDescription>
          </DialogHeader>

          {createdDb && (
            <div className="space-y-3 py-4">
              {[
                { label: 'Host', value: createdDb.host, icon: Globe },
                { label: 'Puerto', value: createdDb.port.toString(), icon: ExternalLink },
                { label: 'Base de Datos', value: createdDb.database_name, icon: Database },
                { label: 'Usuario', value: createdDb.username, icon: UserIcon },
                { label: 'Contraseña', value: createdDb.password || '********', icon: Lock, isSecret: true },
              ].map((field) => (
                <CredentialRow
                  key={field.label}
                  icon={field.icon as typeof Database}
                  label={field.label}
                  value={field.value}
                  secret={field.isSecret}
                  revealed={field.isSecret ? !!showPassword[0] : undefined}
                  onToggleReveal={field.isSecret ? () => togglePassword(0) : undefined}
                  copied={copiedField === field.label}
                  onCopy={() => copyToClipboard(field.value, field.label)}
                />
              ))}
            </div>
          )}

          <DialogFooter>
            <Button className="w-full" onClick={() => setIsSuccessOpen(false)}>Entendido</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
