'use client';
import { useState, useEffect, useCallback } from 'react';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { EmptyState } from '@/components/empty-state';
import { Button } from '@/components/ui/button';
import { Archive, Plus, Download, RotateCcw, Trash2, Loader2, Clock, Cloud, HardDrive } from 'lucide-react';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from "@/lib/toast";
import { cn, formatBytes } from '@/lib/utils';

type BackupInfo = {
  id: number;
  name: string;
  fileName: string;
  createdAt: string;
  /** Bytes; 0 until the archive is written. */
  fileSize?: number;
  /** "local", "offsite" or "both". */
  location?: string;
};

/** Backups made by scheduled tasks; the node names their files with this prefix. */
const isAutomatic = (backup: BackupInfo) => backup.fileName.startsWith('auto-');
/** The name the panel stores for them, shown translated instead. */
const AUTOMATIC_BACKUP_NAME = 'Automatic backup';

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
    void fetchBackups();
  }, [fetchBackups]);

  const handleCreate = async () => {
    if (!backupName.trim()) return;

    try {
      setIsCreating(true);
      await api.post(`/api/servers/${serverId}/backup/create?name=${encodeURIComponent(backupName)}`, {});
      sileo.success({
        title: t('common.success'),
        description: t('servers.backups.createDialog.success')
      });
      setBackupName('');
      setIsDialogOpen(false);
      void fetchBackups();
    } catch (err) {
      console.error('Failed to create backup:', err);
      sileo.error({
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
      sileo.success({
        title: t('common.success'),
        description: t('servers.backups.restore.success')
      });
    } catch (err) {
      console.error('Failed to restore backup:', err);
      sileo.error({
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
      sileo.success({
        title: t('common.success'),
        description: t('servers.backups.delete.success')
      });
      void fetchBackups();
    } catch (err) {
      console.error('Failed to delete backup:', err);
      sileo.error({
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
    <div className="mt-6">
      <Card>
        <CardHeader className="flex flex-col gap-4 space-y-0 border-b border-border/50 p-5 sm:flex-row sm:items-center sm:justify-between md:p-6">
          <div className="min-w-0 space-y-1">
            <CardTitle className="text-lg font-semibold tracking-[-0.02em]">{t('servers.backups.title')}</CardTitle>
            <CardDescription>{t('servers.backups.description', { count: backups.length })}</CardDescription>
          </div>
          <Button onClick={() => setIsDialogOpen(true)} className="shrink-0 self-start sm:self-auto">
            <Plus aria-hidden="true" />
            {t('servers.backups.create')}
          </Button>
        </CardHeader>

        {backups.length > 0 && <BackupUsage backups={backups} t={t} />}

        <CardContent className="p-5 md:p-6">
          {loading ? (
            <div className="flex justify-center py-16">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : backups.length === 0 ? (
            <EmptyState
              icon={Archive}
              title={t('servers.backups.empty.title')}
              description={t('servers.backups.empty.description')}
              action={
                <Button onClick={() => setIsDialogOpen(true)}>
                  <Plus aria-hidden="true" />
                  {t('servers.backups.create')}
                </Button>
              }
            />
          ) : (
            /* Timeline: one dot per backup, centred on its date tile (1px border
               + p-4/p-5 + half the 56px tile = 45px / 49px), and a rail from
               each dot's centre to the next one's, so it reads as one line. */
            <ol className="relative">
              {backups.map((backup, index) => {
                const date = formatBackupDate(backup.createdAt, language);
                const isLast = index === backups.length - 1;
                return (
                  <li key={backup.id} className={cn('relative pl-8 sm:pl-10', !isLast && 'pb-4')}>
                    {!isLast && (
                      <span
                        aria-hidden="true"
                        className="absolute -bottom-[45px] left-[5px] top-[45px] w-px bg-border sm:-bottom-[49px] sm:top-[49px]"
                      />
                    )}
                    <span
                      aria-hidden="true"
                      className={cn(
                        'absolute left-0 top-[40px] z-10 h-[11px] w-[11px] rounded-full border-2 sm:top-[44px]',
                        index === 0 ? 'border-primary bg-primary' : 'border-border bg-card',
                      )}
                    />

                    <div className="flex min-w-0 flex-col gap-4 rounded-xl border border-border/60 bg-background/30 p-4 transition-colors duration-200 hover:border-border sm:flex-row sm:items-center sm:justify-between sm:p-5">
                      <div className="flex min-w-0 items-center gap-4">
                        <div className="flex h-14 w-14 shrink-0 flex-col items-center justify-center rounded-lg border border-border/60 bg-card">
                          <span className="text-xl font-semibold leading-none tabular-nums text-foreground">{date.day}</span>
                          <span className="mt-1 text-[11px] text-muted-foreground">{date.month}</span>
                        </div>
                        <div className="min-w-0">
                          <div className="flex min-w-0 items-center gap-2">
                            <h4 className="truncate text-[15px] font-semibold leading-tight">
                              {isAutomatic(backup) && backup.name === AUTOMATIC_BACKUP_NAME ? t('servers.backups.automatic.name') : backup.name}
                            </h4>
                            {isAutomatic(backup) && <Badge variant="secondary" className="shrink-0">{t('servers.backups.automatic.badge')}</Badge>}
                          </div>
                          <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                            <span className="flex items-center gap-1 tabular-nums" title={date.full}>
                              <Clock className="h-3 w-3" aria-hidden="true" />
                              {date.time}
                            </span>
                            {(backup.fileSize ?? 0) > 0 && (
                              <span className="flex items-center gap-1 tabular-nums">
                                <HardDrive className="h-3 w-3" aria-hidden="true" />
                                {formatBytes(backup.fileSize ?? 0)}
                              </span>
                            )}
                            {(backup.location === 'both' || backup.location === 'offsite') && (
                              <span
                                className="flex items-center gap-1 text-primary-bright"
                                title={backup.location === 'offsite' ? t('servers.backups.offsite.onlyHint') : t('servers.backups.offsite.bothHint')}
                              >
                                <Cloud className="h-3 w-3" aria-hidden="true" />
                                {backup.location === 'offsite' ? t('servers.backups.offsite.only') : t('servers.backups.offsite.both')}
                              </span>
                            )}
                            <span className="min-w-0 truncate font-mono">{backup.fileName}</span>
                          </div>
                        </div>
                      </div>
                      <div className="grid w-full shrink-0 grid-cols-[1fr_1fr_auto] items-center gap-2 sm:flex sm:w-auto">
                        <Button variant="outline" size="sm" className="min-w-0" onClick={() => handleDownload(backup.id)}>
                          <Download aria-hidden="true" />
                          {t('servers.backups.download')}
                        </Button>
                        <Button variant="outline" size="sm" className="min-w-0" onClick={() => setPendingAction({ type: 'restore', backupId: backup.id })}>
                          <RotateCcw aria-hidden="true" />
                          {t('servers.backups.restore.button')}
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-9 w-9 shrink-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright"
                          aria-label={t('servers.backups.delete.button')}
                          onClick={() => setPendingAction({ type: 'delete', backupId: backup.id })}
                        >
                          <Trash2 aria-hidden="true" />
                        </Button>
                      </div>
                    </div>
                  </li>
                );
              })}
            </ol>
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
              {isCreating && <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />}
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
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={pendingAction?.type === 'restore' ? handleRestore : handleDelete}
              className={pendingAction?.type === 'delete' ? 'bg-destructive text-destructive-foreground hover:bg-destructive/90' : ''}
            >
              {pendingAction?.type === 'restore' ? t('servers.backups.restore.button') : t('servers.backups.delete.button')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

/**
 * How much room the backups take: the total, and a bar split by backup (automatic ones in
 * a lighter tone) so the big ones stand out.
 */
function BackupUsage({ backups, t }: { backups: BackupInfo[]; t: (key: string, options?: Record<string, string | number>) => string }) {
  const sized = backups.filter((b) => (b.fileSize ?? 0) > 0);
  const total = sized.reduce((sum, b) => sum + (b.fileSize ?? 0), 0);
  if (total === 0) return null;
  const largest = sized.reduce((max, b) => Math.max(max, b.fileSize ?? 0), 0);
  const offsite = backups.filter((b) => b.location === 'both' || b.location === 'offsite').length;

  return (
    <div className="space-y-3 border-b border-border/50 px-5 py-4 md:px-6">
      <div className="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-1">
        <p className="text-sm">
          <span className="text-muted-foreground">{t('servers.backups.usage.label')} </span>
          <span className="font-semibold tabular-nums text-foreground">{formatBytes(total)}</span>
        </p>
        <p className="text-xs text-muted-foreground">
          {t('servers.backups.usage.largest', { size: formatBytes(largest) })}
          {offsite > 0 && <> · <span className="text-primary-bright">{t('servers.backups.usage.offsite', { count: offsite })}</span></>}
        </p>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-secondary/80" role="img" aria-label={`${t('servers.backups.usage.label')} ${formatBytes(total)}`}>
        {/* the segments slide in together, like the panel's other meters */}
        <div className="meter-grow flex h-full w-full gap-0.5">
          {sized.map((backup) => (
            <span
              key={backup.id}
              title={`${backup.fileName} · ${formatBytes(backup.fileSize ?? 0)}`}
              className={cn('h-full transition-[width] duration-500 ease-panel', isAutomatic(backup) ? 'bg-primary/50' : 'bg-primary')}
              style={{ width: `${((backup.fileSize ?? 0) / total) * 100}%` }}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
