'use client';
import { useEffect, useState } from 'react';
import { KeyRound, Loader2, Plus, Trash2 } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { EmptyState } from '@/components/empty-state';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
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
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';

type SSHKey = {
  id: number;
  name: string;
  fingerprint: string;
  type: string;
  createdAt: string;
};

/** The public keys the user signs in to SFTP with instead of the password. */
export function SSHKeys() {
  const { t } = useTranslations();
  const [keys, setKeys] = useState<SSHKey[] | null>(null);
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState('');
  const [publicKey, setPublicKey] = useState('');
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [pendingDelete, setPendingDelete] = useState<SSHKey | null>(null);

  const load = async () => {
    try {
      setKeys(await api.get<SSHKey[]>('/api/self/sshkeys'));
    } catch {
      setKeys([]);
      sileo.error({ title: t('common.error'), description: t('profileSettings.sshKeys.loadError') });
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const openAdd = () => {
    setName('');
    setPublicKey('');
    setFormError(null);
    setAdding(true);
  };

  const handleAdd = async () => {
    setSaving(true);
    setFormError(null);
    try {
      await api.post('/api/self/sshkeys', { name, publicKey });
      setAdding(false);
      sileo.success({ title: t('common.success'), description: t('profileSettings.sshKeys.added') });
      await load();
    } catch (e: any) {
      // the reason (not a public key, too weak, already added) belongs next to the field
      setFormError(e?.message || t('profileSettings.sshKeys.addError'));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!pendingDelete) return;
    try {
      await api.delete(`/api/self/sshkeys/${pendingDelete.id}`);
      sileo.success({ title: t('common.success'), description: t('profileSettings.sshKeys.deleted') });
      await load();
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e?.message || t('profileSettings.sshKeys.deleteError') });
    } finally {
      setPendingDelete(null);
    }
  };

  if (keys === null) {
    return (
      <div className="flex h-20 items-center justify-center text-muted-foreground">
        <Loader2 className="h-5 w-5 animate-spin" aria-label={t('common.loading')} />
      </div>
    );
  }

  return (
    <>
      {keys.length === 0 ? (
        <EmptyState
          icon={KeyRound}
          size="sm"
          title={t('profileSettings.sshKeys.emptyTitle')}
          description={t('profileSettings.sshKeys.emptyDescription')}
          action={
            <Button onClick={openAdd}>
              <Plus aria-hidden="true" />
              {t('profileSettings.sshKeys.add')}
            </Button>
          }
        />
      ) : (
        <div className="space-y-3">
          <ul className="divide-y divide-border/50 rounded-xl border border-border/60 bg-background/30">
            {keys.map(key => (
              <li key={key.id} className="flex items-start justify-between gap-3 px-4 py-3">
                <div className="min-w-0 space-y-0.5">
                  <p className="truncate text-sm font-medium">{key.name}</p>
                  <code className="block truncate font-mono text-xs text-muted-foreground">{key.fingerprint}</code>
                  <p className="text-xs text-muted-foreground">
                    {key.type} · {t('profileSettings.sshKeys.addedOn', { date: new Date(key.createdAt).toLocaleDateString() })}
                  </p>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-9 w-9 shrink-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright"
                  onClick={() => setPendingDelete(key)}
                  aria-label={t('profileSettings.sshKeys.deleteAria', { name: key.name })}
                >
                  <Trash2 aria-hidden="true" />
                </Button>
              </li>
            ))}
          </ul>
          <Button variant="outline" size="sm" onClick={openAdd}>
            <Plus aria-hidden="true" />
            {t('profileSettings.sshKeys.add')}
          </Button>
        </div>
      )}

      <Dialog open={adding} onOpenChange={setAdding}>
        <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('profileSettings.sshKeys.addTitle')}</DialogTitle>
            <DialogDescription>{t('profileSettings.sshKeys.addDescription')}</DialogDescription>
          </DialogHeader>
          <form
            id="ssh-key-form"
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault();
              void handleAdd();
            }}
          >
            <div className="space-y-2">
              <Label htmlFor="ssh-key-name">{t('profileSettings.sshKeys.nameLabel')}</Label>
              <Input id="ssh-key-name" value={name} maxLength={100} placeholder={t('profileSettings.sshKeys.namePlaceholder')} onChange={e => setName(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="ssh-key-value">{t('profileSettings.sshKeys.keyLabel')}</Label>
              <Textarea
                id="ssh-key-value"
                required
                rows={4}
                spellCheck={false}
                autoCapitalize="off"
                autoCorrect="off"
                value={publicKey}
                placeholder="ssh-ed25519 AAAAC3Nza… usuario@equipo"
                onChange={e => setPublicKey(e.target.value)}
                aria-invalid={formError ? true : undefined}
                aria-describedby={formError ? 'ssh-key-error' : 'ssh-key-hint'}
                className="break-all font-mono text-xs"
              />
              {formError ? (
                <p id="ssh-key-error" role="alert" className="text-xs text-destructive-bright">{formError}</p>
              ) : (
                <p id="ssh-key-hint" className="text-xs text-muted-foreground">{t('profileSettings.sshKeys.keyHint')}</p>
              )}
            </div>
          </form>
          <DialogFooter>
            <Button variant="outline" onClick={() => setAdding(false)}>{t('common.cancel')}</Button>
            <Button type="submit" form="ssh-key-form" disabled={saving || publicKey.trim() === ''}>
              {saving && <Loader2 className="animate-spin" aria-hidden="true" />}
              {t('profileSettings.sshKeys.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={pendingDelete !== null} onOpenChange={(open) => !open && setPendingDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('profileSettings.sshKeys.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('profileSettings.sshKeys.deleteDescription', { name: pendingDelete?.name ?? '' })}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void handleDelete()} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
              {t('profileSettings.sshKeys.deleteConfirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
