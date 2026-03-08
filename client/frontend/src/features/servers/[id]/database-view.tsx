'use client';
import { useState, useEffect, useCallback } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { PlusCircle, Trash2, Database, Shield, Globe, ExternalLink, Loader2, Eye, EyeOff, Copy, Check, Lock } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
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

  const handleDelete = async (dbId: number) => {
    if (!confirm(t('servers.database.deleteConfirm'))) return;

    try {
      await api.delete(`/api/servers/${serverId}/databases/${dbId}`);
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
    <div className="mt-6 flex flex-col gap-6">
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">{t('servers.database.title')}</h2>
          <p className="text-muted-foreground">{t('servers.database.description')}</p>
        </div>
        <div className="flex gap-2">
          <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
            <DialogTrigger asChild>
              <Button>
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
      </div>

      {/* Database Creation Success Modal */}
      <Dialog open={isSuccessOpen} onOpenChange={setIsSuccessOpen}>
        <DialogContent className="max-w-md bg-gradient-to-b from-background to-background/50 border-primary/20">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-primary">
              <PlusCircle className="h-5 w-5" />
              Base de Datos Creada
            </DialogTitle>
            <DialogDescription>
              Guarda estas credenciales. Por seguridad, la contraseña no se volverá a mostrar así de claro.
            </DialogDescription>
          </DialogHeader>

          {createdDb && (
            <div className="grid gap-3 py-4">
              {[
                { label: 'Host', value: createdDb.host, icon: Globe },
                { label: 'Puerto', value: createdDb.port.toString(), icon: ExternalLink },
                { label: 'Base de Datos', value: createdDb.database_name, icon: Database },
                { label: 'Usuario', value: createdDb.username, icon: Shield },
                { label: 'Contraseña', value: createdDb.password || '********', icon: Lock, isSecret: true },
              ].map((field) => (
                <div key={field.label} className="relative group">
                  <Label className="text-[10px] uppercase text-muted-foreground ml-1 mb-1 block">
                    {field.label}
                  </Label>
                  <div className="flex items-center gap-2 p-2 rounded-lg bg-secondary/30 border border-border/50 group-hover:border-primary/30 transition-colors">
                    <field.icon className="h-4 w-4 text-muted-foreground" />
                    <code className="text-sm font-mono flex-1 overflow-hidden text-ellipsis whitespace-nowrap">
                      {field.isSecret ? (showPassword[0] ? field.value : '••••••••••••') : field.value}
                    </code>
                    <div className="flex gap-1">
                      {field.isSecret && (
                        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => togglePassword(0)}>
                          {showPassword[0] ? <EyeOff className="h-3 w-3" /> : <Eye className="h-3 w-3" />}
                        </Button>
                      )}
                      <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => copyToClipboard(field.value, field.label)}>
                        {copiedField === field.label ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
                      </Button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}

          <DialogFooter>
            <Button className="w-full" onClick={() => setIsSuccessOpen(false)}>Entendido</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <div className="grid gap-6">
        {databases.length === 0 ? (
          <Card className="border-0 bg-card/50 backdrop-blur-sm shadow-xl">
            <CardContent className="flex flex-col items-center justify-center py-12 text-center">
              <Database className="h-12 w-12 text-muted-foreground/30 mb-4" />
              <h3 className="text-lg font-medium">{t('servers.database.empty')}</h3>
            </CardContent>
          </Card>
        ) : (
          databases.map((db: DatabaseInfo) => (
            <Card key={db.id} className="border-0 bg-card/50 backdrop-blur-sm overflow-hidden border-l-4 border-l-primary shadow-lg hover:shadow-primary/5 transition-all">
              <CardHeader className="pb-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="p-2 rounded-md bg-primary/10">
                      <Database className="h-5 w-5 text-primary" />
                    </div>
                    <div>
                      <CardTitle className="text-lg">{db.database_name}</CardTitle>
                      <CardDescription className="flex items-center gap-2">
                        <Globe className="h-3 w-3" />
                        {db.host_name} ({db.host}:{db.port})
                      </CardDescription>
                    </div>
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-destructive hover:text-destructive hover:bg-destructive/10"
                    onClick={() => handleDelete(db.id)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                  <div className="space-y-1.5 p-3 rounded-md bg-background/50 border group relative">
                    <div className="flex items-center justify-between">
                      <Label className="text-xs text-muted-foreground uppercase flex items-center gap-1.5">
                        <Shield className="h-3 w-3" />
                        {t('servers.database.table.user')}
                      </Label>
                      <Button variant="ghost" size="icon" className="h-5 w-5 opacity-0 group-hover:opacity-100 transition-opacity" onClick={() => copyToClipboard(db.username, `user-${db.id}`)}>
                        {copiedField === `user-${db.id}` ? <Check className="h-2.5 w-2.5" /> : <Copy className="h-2.5 w-2.5" />}
                      </Button>
                    </div>
                    <code className="text-sm font-mono block">{db.username}</code>
                  </div>

                  <div className="space-y-1.5 p-3 rounded-md bg-background/50 border group relative">
                    <div className="flex items-center justify-between">
                      <Label className="text-xs text-muted-foreground uppercase flex items-center gap-1.5">
                        <Lock className="h-3 w-3" />
                        Contraseña
                      </Label>
                      <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                        <Button variant="ghost" size="icon" className="h-5 w-5" onClick={() => togglePassword(db.id)}>
                          {showPassword[db.id] ? <EyeOff className="h-2.5 w-2.5" /> : <Eye className="h-2.5 w-2.5" />}
                        </Button>
                        <Button variant="ghost" size="icon" className="h-5 w-5" onClick={() => copyToClipboard(db.password || '', `pass-${db.id}`)}>
                          {copiedField === `pass-${db.id}` ? <Check className="h-2.5 w-2.5" /> : <Copy className="h-2.5 w-2.5" />}
                        </Button>
                      </div>
                    </div>
                    <code className="text-sm font-mono block">
                      {showPassword[db.id] ? db.password : '••••••••••••'}
                    </code>
                  </div>

                  <div className="space-y-1.5 p-3 rounded-md bg-background/50 border md:col-span-2 lg:col-span-1 group relative">
                    <div className="flex items-center justify-between">
                      <Label className="text-xs text-muted-foreground uppercase flex items-center gap-1.5">
                        <Globe className="h-3 w-3" />
                        Host / Puerto
                      </Label>
                      <Button variant="ghost" size="icon" className="h-5 w-5 opacity-0 group-hover:opacity-100 transition-opacity" onClick={() => copyToClipboard(`${db.host}:${db.port}`, `host-${db.id}`)}>
                        {copiedField === `host-${db.id}` ? <Check className="h-2.5 w-2.5" /> : <Copy className="h-2.5 w-2.5" />}
                      </Button>
                    </div>
                    <div className="flex items-center justify-between">
                      <code className="text-sm font-mono">{db.host}:{db.port}</code>
                      <Badge variant="secondary" className="text-[10px]">{t('servers.database.anyHost')}</Badge>
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))
        )}
      </div>
    </div>
  );
}
