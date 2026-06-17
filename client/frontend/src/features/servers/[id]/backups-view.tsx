'use client';
import { useState, useEffect, useCallback } from 'react';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Archive, PlusCircle, Download, RotateCcw, Trash2, Loader2, Calendar, FileArchive } from 'lucide-react';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { useToast } from '@/hooks/use-toast';

type BackupInfo = {
  id: number;
  name: string;
  fileName: string;
  createdAt: string;
};

type BackupsViewProps = {
  serverId: string;
};

export default function BackupsView({ serverId }: BackupsViewProps) {
  const [backups, setBackups] = useState<BackupInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [backupName, setBackupName] = useState('');
  const [pendingAction, setPendingAction] = useState<{ type: 'restore' | 'delete'; backupId: number } | null>(null);

  const { t } = useTranslations();
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
    <div className="mt-6 flex flex-col gap-6">
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">{t('servers.backups.title')}</h2>
          <p className="text-muted-foreground">{t('servers.backups.description', { count: backups.length })}</p>
        </div>
        <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
          <DialogTrigger asChild>
            <Button>
              <PlusCircle className="mr-2 h-4 w-4" />
              {t('servers.backups.create')}
            </Button>
          </DialogTrigger>
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
      </div>

      <div className="grid gap-4">
        {backups.length === 0 && !loading ? (
          <Card className="border-0 bg-card/50 backdrop-blur-sm">
            <CardContent className="flex flex-col items-center justify-center py-12 text-center">
              <Archive className="h-12 w-12 text-muted-foreground/30 mb-4" />
              <h3 className="text-lg font-medium">{t('servers.backups.empty.title')}</h3>
              <p className="text-sm text-muted-foreground">{t('servers.backups.empty.description')}</p>
            </CardContent>
          </Card>
        ) : (
          backups.map((backup) => (
            <Card key={backup.id} className="border-0 bg-card/50 backdrop-blur-sm overflow-hidden border-l-4 border-l-primary">
              <CardContent className="p-0">
                <div className="flex flex-col md:flex-row md:items-center justify-between p-4 gap-4">
                  <div className="flex items-center gap-4">
                    <div className="p-3 rounded-lg bg-primary/10">
                      <FileArchive className="h-6 w-6 text-primary" />
                    </div>
                    <div>
                      <h4 className="font-bold text-lg">{backup.name}</h4>
                      <div className="flex items-center gap-3 text-sm text-muted-foreground">
                        <span className="flex items-center gap-1">
                          <Calendar className="h-3 w-3" />
                          {new Date(backup.createdAt).toLocaleString()}
                        </span>
                        <span>•</span>
                        <span className="font-mono">{backup.fileName}</span>
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button variant="outline" size="sm" onClick={() => handleDownload(backup.id)}>
                      <Download className="h-4 w-4 mr-2" />
                      {t('servers.backups.download')}
                    </Button>
                    <Button variant="outline" size="sm" onClick={() => setPendingAction({ type: 'restore', backupId: backup.id })}>
                      <RotateCcw className="h-4 w-4 mr-2" />
                      {t('servers.backups.restore.button')}
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="text-destructive hover:text-destructive hover:bg-destructive/10"
                      onClick={() => setPendingAction({ type: 'delete', backupId: backup.id })}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))
        )}
        {loading && (
          <div className="flex justify-center py-8">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
          </div>
        )}
      </div>

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
