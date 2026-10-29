'use client';
import { useState, useEffect, useCallback, type ReactNode } from 'react';
import { UserAvatar } from '@/components/user-avatar';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { Plus, Loader2, Trash2, Users as UsersIcon, ShieldCheck, Crown, SlidersHorizontal } from '@/components/icons';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { useTranslations } from '@/contexts/translations-context';
import { useAuth } from '@/contexts/providers';
import { api } from '@/lib/api-client';
import { sileo } from "@/lib/toast";
import { EmptyState } from '@/components/empty-state';

type UsersViewProps = {
  serverId: string;
};

interface ServerUser {
  username: string;
  email: string;
  scopes: (string | { value: string })[];
  owner?: boolean;
}

/**
 * What an owner can hand to a sub-user, grouped as the role editor groups them. It is the
 * owner's own set (the backend's scopes.OwnerScopes): nobody gives more than they hold.
 * server.view is always included, without it the server is not listed at all.
 */
const SUB_USER_GROUPS: { label: string; keys: string[] }[] = [
  { label: 'roles.permissionGroups.serverConsole', keys: ['server.console', 'server.console.send', 'server.stats', 'server.status'] },
  { label: 'servers.users.groups.power', keys: ['server.start', 'server.stop', 'server.kill', 'server.reload', 'server.install'] },
  { label: 'roles.permissionGroups.serverFiles', keys: ['server.files.view', 'server.files.edit', 'server.sftp'] },
  { label: 'roles.permissionGroups.serverBackups', keys: ['server.backup.view', 'server.backup.create', 'server.backup.restore', 'server.backup.delete'] },
  { label: 'servers.users.groups.settings', keys: ['server.data.view', 'server.data.edit', 'server.flags.view', 'server.flags.edit', 'server.name.edit', 'server.definition.view'] },
  { label: 'servers.users.groups.tasks', keys: ['server.tasks.view', 'server.tasks.run', 'server.tasks.create', 'server.tasks.edit', 'server.tasks.delete'] },
  {
    label: 'roles.permissionGroups.serverUsers',
    keys: ['server.users.view', 'server.users.create', 'server.users.edit', 'server.users.delete',
      'server.clients.view', 'server.clients.create', 'server.clients.edit', 'server.clients.delete'],
  },
];

/** A new sub-user can watch and switch the server on and off; the owner adds the rest. */
const DEFAULT_SUB_USER_SCOPES = ['server.console', 'server.stats', 'server.status', 'server.start', 'server.stop'];

// these scopes are also the prefix of others, so their own label lives under another key
const LABEL_OVERRIDE: Record<string, string> = {
  'server.console': 'server.consoleView',
  'server.data.edit': 'server.dataEdit',
};

const scopeValue = (s: string | { value: string }) => (typeof s === 'string' ? s : s.value);

export default function UsersView({ serverId }: Readonly<UsersViewProps>) {
  const [users, setUsers] = useState<ServerUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  // the dialog adds a user (email empty) or edits one (email set)
  const [dialog, setDialog] = useState<{ mode: 'add' | 'edit'; email: string; scopes: Set<string> } | null>(null);
  const [pendingOwner, setPendingOwner] = useState<ServerUser | null>(null);

  const { t } = useTranslations();
  const { hasScope } = useAuth();
  const isAdmin = hasScope('admin');

  const fetchUsers = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.get<ServerUser[]>(`/api/servers/${serverId}/user`);
      // the owner first, then by name
      setUsers((data || []).sort((a, b) => Number(!!b.owner) - Number(!!a.owner) || (a.username || a.email).localeCompare(b.username || b.email)));
    } catch (err) {
      console.error('Failed to fetch users:', err);
      sileo.error({
        title: t('common.error'),
        description: t('servers.users.fetchError')
      });
    } finally {
      setLoading(false);
    }
  }, [serverId, t]);

  useEffect(() => {
    void fetchUsers();
  }, [fetchUsers]);

  const openAdd = () => setDialog({ mode: 'add', email: '', scopes: new Set(DEFAULT_SUB_USER_SCOPES) });
  const openEdit = (user: ServerUser) => setDialog({ mode: 'edit', email: user.email, scopes: new Set(user.scopes.map(scopeValue)) });

  const toggle = (key: string, on: boolean) =>
    setDialog((d) => {
      if (!d) return d;
      const next = new Set(d.scopes);
      if (on) next.add(key);
      else next.delete(key);
      return { ...d, scopes: next };
    });

  const toggleGroup = (keys: string[], on: boolean) =>
    setDialog((d) => {
      if (!d) return d;
      const next = new Set(d.scopes);
      keys.forEach((k) => (on ? next.add(k) : next.delete(k)));
      return { ...d, scopes: next };
    });

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!dialog?.email) return;

    try {
      setSaving(true);
      await api.put(`/api/servers/${serverId}/user/${encodeURIComponent(dialog.email)}`, {
        scopes: ['server.view', ...[...dialog.scopes].filter((s) => s !== 'server.view')],
      });
      sileo.success({
        title: t('common.success'),
        description: dialog.mode === 'add' ? t('servers.users.addSuccess') : t('servers.users.editSuccess'),
      });
      setDialog(null);
      void fetchUsers();
    } catch (err: any) {
      console.error('Failed to save user:', err);
      sileo.error({
        title: t('common.error'),
        description: err.message || t('servers.users.addError')
      });
    } finally {
      setSaving(false);
    }
  };

  const handleRevoke = async (userEmail: string) => {
    try {
      await api.delete(`/api/servers/${serverId}/user/${encodeURIComponent(userEmail)}`);
      sileo.success({
        title: t('common.success'),
        description: t('servers.users.revokeSuccess')
      });
      void fetchUsers();
    } catch (err: any) {
      console.error('Failed to revoke access:', err);
      sileo.error({
        title: t('common.error'),
        description: err.message || t('servers.users.revokeError')
      });
    }
  };

  const handleMakeOwner = async () => {
    if (!pendingOwner) return;
    try {
      await api.put(`/api/servers/${serverId}/owner/${encodeURIComponent(pendingOwner.email)}`, {});
      sileo.success({ title: t('common.success'), description: t('servers.users.ownerChanged', { name: pendingOwner.username || pendingOwner.email }) });
      void fetchUsers();
    } catch (err: any) {
      sileo.error({ title: t('common.error'), description: err.message || t('servers.users.ownerError') });
    } finally {
      setPendingOwner(null);
    }
  };

  const permissionLabel = (key: string) => t(`permissions.${LABEL_OVERRIDE[key] || key}`);

  let body: ReactNode;
  if (loading) {
    body = (
      <div className="flex justify-center py-16">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-label={t('common.loading')} />
      </div>
    );
  } else if (users.length === 0) {
    body = (
      <EmptyState
        icon={UsersIcon}
        title={t('servers.users.noUsers')}
        action={
          <Button onClick={openAdd}>
            <Plus aria-hidden="true" />
            {t('servers.users.addUser')}
          </Button>
        }
      />
    );
  } else {
    body = (
      <ul className="divide-y divide-border/50 overflow-hidden rounded-xl border border-border/60 bg-background/30">
      {users.map((user) => {
        const name = user.username || user.email.split('@')[0];
        return (
        <li key={user.email} className="flex items-center justify-between gap-4 px-4 py-3.5 transition-colors duration-150 hover:bg-accent/30">
          <div className="flex min-w-0 items-center gap-3.5">
            <UserAvatar
              identity={user.email || user.username}
              fallback={(user.username || user.email).charAt(0).toUpperCase()}
              className="h-10 w-10 ring-1 ring-border/60"
            />
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <p className="truncate text-sm font-semibold leading-tight">{name}</p>
                {user.owner ? (
                  <Badge className="shrink-0 items-center gap-1 border-warning/30 bg-warning/10 px-1.5 py-px text-[11px] font-medium text-warning hover:bg-warning/10">
                    <Crown className="h-2.5 w-2.5" aria-hidden="true" />
                    {t('servers.users.owner')}
                  </Badge>
                ) : (
                  <Badge
                    variant="outline"
                    className="hidden shrink-0 items-center gap-1 border-border/60 px-1.5 py-px text-[11px] font-medium text-muted-foreground sm:inline-flex"
                  >
                    <ShieldCheck className="h-2.5 w-2.5" aria-hidden="true" />
                    {t('servers.users.permissionsCount', { count: user.scopes?.length || 0 })}
                  </Badge>
                )}
              </div>
              <p className="mt-0.5 truncate text-xs text-muted-foreground">{user.email}</p>
            </div>
          </div>

          {user.owner ? (
            <p className="hidden shrink-0 text-xs text-muted-foreground sm:block">{t('servers.users.ownerHint')}</p>
          ) : (
          <div className="flex shrink-0 items-center gap-1">
            {isAdmin && (
              <Button
                variant="ghost"
                size="sm"
                className="h-9 w-9 px-0 text-muted-foreground sm:w-auto sm:px-3"
                onClick={() => setPendingOwner(user)}
                aria-label={t('servers.users.makeOwnerAria', { name })}
              >
                <Crown aria-hidden="true" />
                <span className="hidden sm:inline">{t('servers.users.makeOwner')}</span>
              </Button>
            )}
            <Button
              variant="ghost"
              size="sm"
              className="h-9 w-9 px-0 text-muted-foreground sm:w-auto sm:px-3"
              onClick={() => openEdit(user)}
              aria-label={t('servers.users.editAria', { name })}
            >
              <SlidersHorizontal aria-hidden="true" />
              <span className="hidden sm:inline">{t('servers.users.edit')}</span>
            </Button>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-9 w-9 px-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright sm:w-auto sm:px-3"
                  aria-label={t('servers.users.revoke')}
                >
                  <Trash2 aria-hidden="true" />
                  <span className="hidden sm:inline">
                    {t('servers.users.revoke')}
                  </span>
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t('servers.users.revoke')}</AlertDialogTitle>
                  <AlertDialogDescription>
                    {t('servers.users.revokeConfirm')}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                  <AlertDialogAction onClick={() => handleRevoke(user.email)} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                    {t('servers.users.revoke')}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
          )}
        </li>
        );
      })}
      </ul>
    );
  }

  return (
    <Card className="mt-6">
      <div className="flex flex-col gap-4 border-b border-border/50 p-5 sm:flex-row sm:items-center sm:justify-between md:p-6">
        <div className="min-w-0 space-y-1">
          <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('servers.users.title')}</h2>
          <p className="text-sm text-muted-foreground">{t('servers.users.description')}</p>
        </div>
        <Button className="shrink-0 self-start sm:self-auto" onClick={openAdd}>
          <Plus aria-hidden="true" />
          {t('servers.users.addUser')}
        </Button>
      </div>

      <div className="p-5 md:p-6">
        {body}
      </div>

      <Dialog open={dialog !== null} onOpenChange={(open) => !open && setDialog(null)}>
        <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
          <form onSubmit={handleSave} className="grid gap-5">
            <DialogHeader>
              <DialogTitle>{dialog?.mode === 'edit' ? t('servers.users.editDialog.title') : t('servers.users.addDialog.title')}</DialogTitle>
              <DialogDescription>
                {dialog?.mode === 'edit' ? t('servers.users.editDialog.description', { email: dialog.email }) : t('servers.users.addDialog.description')}
              </DialogDescription>
            </DialogHeader>

            {dialog?.mode === 'add' && (
              <div className="space-y-2">
                <Label htmlFor="sub-user-email">{t('servers.users.addDialog.emailLabel')}</Label>
                <Input
                  id="sub-user-email"
                  type="email"
                  value={dialog.email}
                  onChange={(e) => setDialog((d) => (d ? { ...d, email: e.target.value } : d))}
                  placeholder={t('servers.users.addDialog.emailPlaceholder')}
                  required
                  disabled={saving}
                />
              </div>
            )}

            <fieldset className="space-y-3">
              <legend className="text-sm font-medium">{t('servers.users.permissionsTitle')}</legend>
              <p className="text-xs text-muted-foreground">{t('servers.users.permissionsHint')}</p>
              <div className="grid gap-3 sm:grid-cols-2">
                {SUB_USER_GROUPS.map((group) => {
                  const picked = group.keys.filter((k) => dialog?.scopes.has(k)).length;
                  let groupState: boolean | 'indeterminate' = false;
                  if (picked === group.keys.length) groupState = true;
                  else if (picked > 0) groupState = 'indeterminate';
                  const groupId = `group-${group.label}`;
                  return (
                    <div key={group.label} className="rounded-lg border border-border/60 bg-background/30 p-3">
                      <div className="flex items-center gap-2 border-b border-border/50 pb-2">
                        <Checkbox
                          id={groupId}
                          checked={groupState}
                          onCheckedChange={(v) => toggleGroup(group.keys, v === true)}
                        />
                        <Label htmlFor={groupId} className="text-sm font-semibold">{t(group.label)}</Label>
                      </div>
                      <ul className="mt-2 space-y-1.5">
                        {group.keys.map((key) => {
                          const id = `scope-${key}`;
                          return (
                            <li key={key} className="flex items-center gap-2">
                              <Checkbox id={id} checked={dialog?.scopes.has(key) ?? false} onCheckedChange={(v) => toggle(key, v === true)} />
                              <Label htmlFor={id} className="text-sm font-normal text-muted-foreground">{permissionLabel(key)}</Label>
                            </li>
                          );
                        })}
                      </ul>
                    </div>
                  );
                })}
              </div>
            </fieldset>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialog(null)}>{t('common.cancel')}</Button>
              <Button type="submit" disabled={saving || !dialog?.email}>
                {saving && <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />}
                {dialog?.mode === 'edit' ? t('servers.users.editDialog.save') : t('servers.users.addDialog.addButton')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={pendingOwner !== null} onOpenChange={(open) => !open && setPendingOwner(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('servers.users.makeOwnerTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('servers.users.makeOwnerDescription', { name: pendingOwner?.username || pendingOwner?.email || '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void handleMakeOwner()}>{t('servers.users.makeOwner')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
