'use client';
import { useState, useEffect, useCallback } from 'react';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Archive, PlusCircle, Download, RotateCcw, Trash2, Loader2, Clock, FileArchive } from 'lucide-react';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { useToast } from '@/hooks/use-toast';
import { cn } from '@/lib/utils';

type BackupInfo = {
  id: number;
  name: string;
  fileName: string;
  createdAt: string;
};

type BackupsViewProps = {
  serverId: string;
};

function formatBackupDate(iso: string, lang: string) {
  const d = new Date(iso);
  const locale = lang === 'es' ? 'es-ES' : 'en-US';
  return {
    day: d.getDate(),
    month: d.toLocaleDateString(locale, { month: 'short' }),
    time: d.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' }),
    full: d.toLocaleString(locale),
  };
}

export default function BackupsView({ serverId }: BackupsViewProps) {
  const [backups, setBackups] = useState<BackupInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [backupName, setBackupName] = useState('');
  const [pendingAction, setPendingAction] = useState<{ type: 'restore' | 'delete'; backupId: number } | null>(null);

  const { t, language } = useTranslations();
  const { toast } = useToast();

  const fetchBackups = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.get(`/api/servers/${serverId}/backup`);
      setBackups(data || []);
    } catch (err) {
      console.error('Failed to fetch backups:', err);
    } finally {
      setLoading(false);
    }
  }, [serverId]);

  useEffect(() => {
    fetchBackups();
  }, [fetchBackups]);

  const handleCreate = async () => {
    if (!backupName.trim()) return;

    try {
      setIsCreating(true);
      await api.post(`/api/servers/${serverId}/backup/create?name=${encodeURIComponent(backupName)}`, {});
      toast({
        title: t('common.success'),
        description: t('servers.backups.createDialog.success')
      });
      setBackupName('');
      setIsDialogOpen(false);
      fetchBackups();
    } catch (err) {
      console.error('Failed to create backup:', err);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: t('servers.backups.createDialog.error')
      });
    } finally {
      setIsCreating(false);
    }
  };

  const handleRestore = async () => {
    if (!pendingAction || pendingAction.type !== 'restore') return;
    try {
      await api.post(`/api/servers/${serverId}/backup/restore/${pendingAction.backupId}`, {});
      toast({
        title: t('common.success'),
        description: t('servers.backups.restore.success')
      });
    } catch (err) {
      console.error('Failed to restore backup:', err);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: t('servers.backups.restore.error')
      });
    } finally {
      setPendingAction(null);
    }
  };

  const handleDelete = async () => {
    if (!pendingAction || pendingAction.type !== 'delete') return;
    try {
      await api.delete(`/api/servers/${serverId}/backup/${pendingAction.backupId}`);
      toast({
        title: t('common.success'),
        description: t('servers.backups.delete.success')
      });
      fetchBackups();
    } catch (err) {
      console.error('Failed to delete backup:', err);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: t('servers.backups.delete.error')
      });
    } finally {
      setPendingAction(null);
    }
  };

  const handleDownload = (backupId: number) => {
    window.open(`/api/servers/${serverId}/backup/download/${backupId}`, '_blank');
  };

  return (
    <div className="mt-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
      <Card className="border-0 bg-transparent shadow-none">
        <CardHeader className="flex flex-row items-center justify-between gap-4 px-0">
          <div className="flex items-center gap-4">
            <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl border border-primary/30 bg-gradient-to-br from-primary/25 via-accent/15 to-transparent text-primary shadow-[0_0_20px_rgb(0_0_0/0.3)]">
              <Archive className="h-5 w-5" />
            </div>
            <div>
              <CardTitle className="font-headline text-2xl">{t('servers.backups.title')}</CardTitle>
              <CardDescription>{t('servers.backups.description', { count: backups.length })}</CardDescription>
            </div>
          </div>
          <Button onClick={() => setIsDialogOpen(true)} className="shrink-0">
            <PlusCircle className="mr-2 h-4 w-4" />
            {t('servers.backups.create')}
          </Button>
        </CardHeader>

        <CardContent className="mt-4 px-0">
          {loading ? (
            <div className="flex justify-center py-16">
              <Loader2 className="h-8 w-8 animate-spin text-primary" />
            </div>
          ) : backups.length === 0 ? (
            <div className="flex flex-col items-center justify-center rounded-xl border-2 border-dashed border-border/70 bg-card/50 p-14 text-center backdrop-blur-sm">
              <div className="mx-auto mb-4 grid h-14 w-14 place-items-center rounded-2xl border border-primary/25 bg-gradient-to-br from-primary/20 via-accent/10 to-transparent text-primary">
                <Archive className="h-6 w-6" />
              </div>
              <h3 className="font-headline text-lg font-semibold">{t('servers.backups.empty.title')}</h3>
              <p className="mt-1 max-w-sm text-sm text-muted-foreground">{t('servers.backups.empty.description')}</p>
              <Button onClick={() => setIsDialogOpen(true)} className="mt-6">
                <PlusCircle className="mr-2 h-4 w-4" />
                {t('servers.backups.create')}
              </Button>
            </div>
          ) : (
            <div>
              {backups.map((backup, index) => {
                const date = formatBackupDate(backup.createdAt, language);
                const isLast = index === backups.length - 1;
                return (
                  <div key={backup.id} className={cn('relative', isLast ? '' : 'pb-7')}>
                    <div
                      className={cn(
                        'absolute left-[5px] top-0 w-px bg-gradient-to-b from-primary/60 via-border/70 to-transparent',
                        isLast ? 'bottom-4' : 'bottom-0'
                      )}
                      aria-hidden
                    />
                    <div className="absolute left-0 top-6 grid h-3 w-3 place-items-center">
                      <span className="h-3 w-3 rounded-full bg-primary ring-4 ring-primary/15" />
                    </div>

                    <div className="group relative ml-8 flex flex-col gap-4 overflow-hidden rounded-xl border border-border/80 bg-card p-5 transition-all duration-200 hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-[0_8px_30px_rgb(0_0_0/0.35)]">
                      <div className="pointer-events-none absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-primary via-accent to-transparent" />
                      <div className="pointer-events-none absolute -right-6 -top-6 opacity-[0.06] transition-transform duration-300 group-hover:scale-110">
                        <FileArchive className="h-28 w-28" />
                      </div>

                      <div className="relative flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
                        <div className="flex min-w-0 items-center gap-4">
                          <div className="flex h-16 w-16 shrink-0 flex-col items-center justify-center rounded-xl border border-border/80 bg-background/50 backdrop-blur-sm">
                            <span className="font-mono text-2xl font-black leading-none text-primary">{date.day}</span>
                            <span className="mt-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">{date.month}</span>
                          </div>
                          <div className="min-w-0">
                            <h4 className="truncate font-headline text-lg font-semibold leading-tight">{backup.name}</h4>
                            <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-foreground">
                              <span className="flex items-center gap-1" title={date.full}>
                                <Clock className="h-3 w-3" />
                                {date.time}
                              </span>
                              <span className="font-mono text-xs opacity-70">{backup.fileName}</span>
                            </div>
                          </div>
                        </div>
                        <div className="relative flex shrink-0 items-center gap-2">
                          <Button variant="outline" size="sm" onClick={() => handleDownload(backup.id)}>
                            <Download className="mr-2 h-4 w-4" />
                            {t('servers.backups.download')}
                          </Button>
                          <Button variant="outline" size="sm" onClick={() => setPendingAction({ type: 'restore', backupId: backup.id })}>
                            <RotateCcw className="mr-2 h-4 w-4" />
                            {t('servers.backups.restore.button')}
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                            onClick={() => setPendingAction({ type: 'delete', backupId: backup.id })}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('servers.backups.createDialog.title')}</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="backup-name">{t('servers.backups.createDialog.nameLabel')}</Label>
              <Input
                id="backup-name"
                placeholder={t('servers.backups.createDialog.namePlaceholder')}
                value={backupName}
                onChange={(e) => setBackupName(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsDialogOpen(false)}>{t('common.cancel')}</Button>
            <Button onClick={handleCreate} disabled={isCreating || !backupName.trim()}>
              {isCreating && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t('servers.backups.createDialog.createButton')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={pendingAction !== null} onOpenChange={(open) => !open && setPendingAction(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pendingAction?.type === 'restore' ? t('servers.backups.restore.button') : t('servers.backups.delete.button')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pendingAction?.type === 'restore' ? t('servers.backups.restore.confirm') : t('servers.backups.delete.confirm')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel') || 'Cancel'}</AlertDialogCancel>
            <AlertDialogAction
              onClick={pendingAction?.type === 'restore' ? handleRestore : handleDelete}
              className={pendingAction?.type === 'delete' ? 'bg-red-500 hover:bg-red-600' : ''}
            >
              {pendingAction?.type === 'restore' ? t('servers.backups.restore.button') : t('servers.backups.delete.button')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
