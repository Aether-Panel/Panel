'use client';
import { useState, useEffect, useCallback } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { Plus, Trash2, Database, Globe, ExternalLink, Loader2, Eye, EyeOff, Copy, Check, Lock, User as UserIcon, CheckCircle2 } from 'lucide-react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from "@/lib/toast";

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
  const { t } = useTranslations();
  return (
    <div className="flex flex-col gap-1 px-4 py-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
      <span className="flex shrink-0 items-center gap-1.5 text-sm text-muted-foreground">
        <Icon className="h-3.5 w-3.5" aria-hidden="true" />
        {label}
      </span>
      <span className="flex min-w-0 items-center gap-2 sm:justify-end">
        <code className="mr-auto min-w-0 truncate font-mono text-sm text-foreground sm:mr-0">
          {secret && !revealed ? '••••••••••••' : value}
        </code>
        {secret && onToggleReveal && (
          <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground sm:h-6 sm:w-6" onClick={onToggleReveal} aria-label={revealed ? t('servers.database.hidePassword') : t('servers.database.showPassword')}>
            {revealed ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
          </Button>
        )}
        {onCopy && (
          <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground sm:h-6 sm:w-6" onClick={onCopy} aria-label={t('servers.database.copyField', { field: label })}>
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
  

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      const [dbResult, hostResult] = await Promise.allSettled([
        api.get(`/api/servers/${serverId}/databases`),
        api.get('/api/databasehosts')
      ]);
      if (dbResult.status === 'fulfilled') {
        setDatabases(Array.isArray(dbResult.value) ? dbResult.value : []);
      } else {
        console.error('Failed to fetch databases:', dbResult.reason);
      }
      if (hostResult.status === 'fulfilled') {
        setHosts(Array.isArray(hostResult.value) ? hostResult.value : []);
      } else {
        console.error('Failed to fetch database hosts:', hostResult.reason);
      }
    } finally {
      setLoading(false);
    }
  }, [serverId]);

  useEffect(() => {
    void fetchData();
  }, [fetchData]);

  const handleCreate = async () => {
    if (!newDbName || !selectedHost) return;

    setIsCreating(true);
    const startedAt = Date.now();
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 25000);
    try {
      const result = await api.post(`/api/servers/${serverId}/databases`, {
        database_host_id: Number.parseInt(selectedHost),
        database_name: newDbName
      }, controller.signal);

      setCreatedDb(result);
      setIsSuccessOpen(true);

      sileo.success({
        title: t('common.success'),
        description: t('servers.database.createDialog.createSuccess')
      });

      setNewDbName('');
      setSelectedHost('');
      setIsDialogOpen(false);
      void fetchData();
    } catch (err: any) {
      clearTimeout(timeoutId);
      console.error('Failed to create database:', err);
      sileo.error({
        title: t('common.error'),
        description: err?.name === 'AbortError'
          ? t('servers.database.createDialog.createError')
          : (err?.message || t('servers.database.createDialog.createError'))
      });
    } finally {
      clearTimeout(timeoutId);
      const remaining = Math.max(0, 1200 - (Date.now() - startedAt));
      setTimeout(() => setIsCreating(false), remaining);
    }
  };

  const handleDelete = async () => {
    if (!pendingDb) return;
    try {
      await api.delete(`/api/servers/${serverId}/databases/${pendingDb}`);
      sileo.success({
        title: t('common.success'),
        description: t('servers.database.deleteSuccess')
      });
      void fetchData();
    } catch (err) {
      console.error('Failed to delete database:', err);
      sileo.error({
        title: t('common.error'),
        description: t('servers.database.deleteError')
      });
    } finally {
      setPendingDb(null);
    }
  };

  const copyToClipboard = (text: string, field: string) => {
    void navigator.clipboard.writeText(text);
    setCopiedField(field);
    setTimeout(() => setCopiedField(null), 2000);
    sileo.success({
      description: t('common.copied'),
    });
  };

  const togglePassword = (id: number) => {
    setShowPassword((prev: Record<number, boolean>) => ({ ...prev, [id]: !prev[id] }));
  };

  return (
    <div className="mt-6">
      <Card>
      <div className="flex flex-col gap-4 border-b border-border/50 p-5 sm:flex-row sm:items-center sm:justify-between md:p-6">
        <div className="min-w-0 space-y-1">
          <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('servers.database.title')}</h2>
          <p className="text-sm text-muted-foreground">{t('servers.database.description')}</p>
        </div>
        <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
          <DialogTrigger asChild>
            <Button className="shrink-0 self-start sm:self-auto">
              <Plus aria-hidden="true" />
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
                {isCreating && <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />}
                {t('servers.database.createDialog.createButton')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <div className="flex flex-col gap-4 p-5 md:p-6">
        {loading ? (
          <div className="flex justify-center py-16">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-label={t('common.loading')} />
          </div>
        ) : databases.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border/60 px-6 py-14 text-center">
            <Database className="mb-3 h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
            <h3 className="text-base font-semibold">{t('servers.database.empty')}</h3>
            <Button className="mt-6" onClick={() => setIsDialogOpen(true)}>
              <Plus aria-hidden="true" />
              {t('servers.database.newDb')}
            </Button>
          </div>
        ) : (
          databases.map((db: DatabaseInfo) => (
            <div key={db.id} className="overflow-hidden rounded-xl border border-border/60 bg-background/30">
              <div className="flex items-center justify-between gap-4 border-b border-border/50 px-4 py-3.5 sm:px-5">
                <div className="flex min-w-0 items-center gap-3.5">
                  <div className="grid h-10 w-10 shrink-0 place-items-center rounded-lg border border-border/60 bg-card text-muted-foreground">
                    <Database className="h-4 w-4" aria-hidden="true" />
                  </div>
                  <div className="min-w-0">
                    <h4 className="truncate text-[15px] font-semibold leading-tight">{db.database_name}</h4>
                    <div className="mt-1 flex min-w-0 items-center gap-1 text-xs text-muted-foreground">
                      <Globe className="h-3 w-3 shrink-0" aria-hidden="true" />
                      <span className="truncate">{db.host_name}</span>
                    </div>
                  </div>
                </div>
                <AlertDialog open={pendingDb === db.id} onOpenChange={(open) => !open && setPendingDb(null)}>
                  <AlertDialogTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="shrink-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright"
                      aria-label={t('common.delete')}
                      onClick={() => setPendingDb(db.id)}
                    >
                      <Trash2 className="h-4 w-4" aria-hidden="true" />
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
                      <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                      <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                        {t('common.delete')}
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </div>

              <div className="divide-y divide-border/50">
                <CredentialRow
                  icon={UserIcon}
                  label={t('servers.database.table.user')}
                  value={db.username}
                  copied={copiedField === `user-${db.id}`}
                  onCopy={() => copyToClipboard(db.username, `user-${db.id}`)}
                />
                <CredentialRow
                  icon={Lock}
                  label={t('servers.database.password')}
                  value={db.password || ''}
                  secret
                  revealed={!!showPassword[db.id]}
                  onToggleReveal={() => togglePassword(db.id)}
                  copied={copiedField === `pass-${db.id}`}
                  onCopy={() => copyToClipboard(db.password || '', `pass-${db.id}`)}
                />
                <CredentialRow
                  icon={Globe}
                  label={t('servers.database.hostPort')}
                  value={`${db.host}:${db.port}`}
                  copied={copiedField === `host-${db.id}`}
                  onCopy={() => copyToClipboard(`${db.host}:${db.port}`, `host-${db.id}`)}
                />
              </div>
            </div>
          ))
        )}
      </div>
      </Card>

      <Dialog open={isSuccessOpen} onOpenChange={setIsSuccessOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <CheckCircle2 className="h-5 w-5 text-success" aria-hidden="true" />
              {t('servers.database.createdDialog.title')}
            </DialogTitle>
            <DialogDescription>
              {t('servers.database.createdDialog.description')}
            </DialogDescription>
          </DialogHeader>

          {createdDb && (
            <div className="my-2 divide-y divide-border/50 overflow-hidden rounded-xl border border-border/60 bg-background/30">
              {[
                { id: 'host', label: t('servers.database.host'), value: createdDb.host, icon: Globe },
                { id: 'port', label: t('servers.database.port'), value: createdDb.port.toString(), icon: ExternalLink },
                { id: 'database', label: t('servers.database.table.name'), value: createdDb.database_name, icon: Database },
                { id: 'user', label: t('servers.database.table.user'), value: createdDb.username, icon: UserIcon },
                { id: 'password', label: t('servers.database.password'), value: createdDb.password || '********', icon: Lock, isSecret: true },
              ].map((field) => (
                <CredentialRow
                  key={field.id}
                  icon={field.icon as typeof Database}
                  label={field.label}
                  value={field.value}
                  secret={field.isSecret}
                  revealed={field.isSecret ? !!showPassword[0] : undefined}
                  onToggleReveal={field.isSecret ? () => togglePassword(0) : undefined}
                  copied={copiedField === field.id}
                  onCopy={() => copyToClipboard(field.value, field.id)}
                />
              ))}
            </div>
          )}

          <DialogFooter>
            <Button className="w-full" onClick={() => setIsSuccessOpen(false)}>{t('servers.database.createdDialog.confirm')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
