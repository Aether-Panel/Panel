'use client';
import { useState, useEffect, useCallback } from 'react';
import { UserAvatar } from '@/components/user-avatar';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { Plus, Loader2, Trash2, Users as UsersIcon, ShieldCheck } from 'lucide-react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from "@/lib/toast";

type UsersViewProps = {
  serverId: string;
};

interface ServerUser {
  username: string;
  email: string;
  scopes: string[];
}

export default function UsersView({ serverId }: UsersViewProps) {
  const [users, setUsers] = useState<ServerUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [addingUser, setAddingUser] = useState(false);
  const [email, setEmail] = useState('');
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  
  const { t } = useTranslations();

  const fetchUsers = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.get(`/api/servers/${serverId}/user`);
      setUsers(data || []);
    } catch (err) {
      console.error('Failed to fetch users:', err);
      sileo.error({
        title: t('common.error') || 'Error',
        description: t('servers.users.fetchError') || 'Failed to load users'
      });
    } finally {
      setLoading(false);
    }
  }, [serverId, t]);

  useEffect(() => {
    void fetchUsers();
  }, [fetchUsers]);

  const handleAddUser = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email) return;

    try {
      setAddingUser(true);
      // Default scopes for a new user
      const defaultScopes = [
        'server.view',
        'server.console',
        'server.stats',
        'server.status'
      ];

      await api.put(`/api/servers/${serverId}/user/${email}`, {
        scopes: defaultScopes
      });

      sileo.success({
        title: t('common.success') || 'Success',
        description: t('servers.users.addSuccess') || 'User added successfully'
      });
      setEmail('');
      setIsDialogOpen(false);
      void fetchUsers();
    } catch (err: any) {
      console.error('Failed to add user:', err);
      sileo.error({
        title: t('common.error') || 'Error',
        description: err.message || t('servers.users.addError') || 'Failed to add user'
      });
    } finally {
      setAddingUser(false);
    }
  };

  const handleRevoke = async (userEmail: string) => {
    try {
      await api.delete(`/api/servers/${serverId}/user/${userEmail}`);
      sileo.success({
        title: t('common.success') || 'Success',
        description: t('servers.users.revokeSuccess') || 'Access revoked successfully'
      });
      void fetchUsers();
    } catch (err: any) {
      console.error('Failed to revoke access:', err);
      sileo.error({
        title: t('common.error') || 'Error',
        description: err.message || t('servers.users.revokeError') || 'Failed to revoke access'
      });
    }
  };

  return (
    <Card className="mt-6">
      <div className="flex flex-col gap-4 border-b border-border/50 p-5 sm:flex-row sm:items-center sm:justify-between md:p-6">
        <div className="min-w-0 space-y-1">
          <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('servers.users.title')}</h2>
          <p className="text-sm text-muted-foreground">{t('servers.users.description')}</p>
        </div>
        <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
          <DialogTrigger asChild>
            <Button className="shrink-0 self-start sm:self-auto">
              <Plus aria-hidden="true" />
              {t('servers.users.addUser')}
            </Button>
          </DialogTrigger>
          <DialogContent>
            <form onSubmit={handleAddUser}>
              <DialogHeader>
                <DialogTitle>{t('servers.users.addDialog.title')}</DialogTitle>
                <DialogDescription>{t('servers.users.addDialog.description')}</DialogDescription>
              </DialogHeader>
              <div className="grid gap-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="email">
                    {t('servers.users.addDialog.emailLabel')}
                  </Label>
                  <Input
                    id="email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder={t('servers.users.addDialog.emailPlaceholder')}
                    required
                    disabled={addingUser}
                  />
                </div>
              </div>
              <DialogFooter>
                <Button type="submit" disabled={addingUser || !email}>
                  {addingUser && <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />}
                  {t('servers.users.addDialog.addButton')}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      <div className="p-5 md:p-6">
        {loading ? (
          <div className="flex justify-center py-16">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-label={t('common.loading')} />
          </div>
        ) : users.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border/60 px-6 py-14 text-center">
            <UsersIcon className="mb-3 h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
            <h3 className="text-base font-semibold">{t('servers.users.noUsers')}</h3>
            <Button className="mt-6" onClick={() => setIsDialogOpen(true)}>
              <Plus aria-hidden="true" />
              {t('servers.users.addUser')}
            </Button>
          </div>
        ) : (
          <ul className="divide-y divide-border/50 overflow-hidden rounded-xl border border-border/60 bg-background/30">
          {users.map((user) => (
            <li key={user.email} className="flex items-center justify-between gap-4 px-4 py-3.5 transition-colors duration-150 hover:bg-accent/30">
              <div className="flex min-w-0 items-center gap-3.5">
                <UserAvatar
                  identity={user.email || user.username}
                  fallback={(user.username || user.email).charAt(0).toUpperCase()}
                  className="h-10 w-10 ring-1 ring-border/60"
                />
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="truncate text-sm font-semibold leading-tight">{user.username || user.email.split('@')[0]}</p>
                    <Badge
                      variant="outline"
                      className="hidden shrink-0 items-center gap-1 border-border/60 px-1.5 py-px text-[11px] font-medium text-muted-foreground sm:inline-flex"
                    >
                      <ShieldCheck className="h-2.5 w-2.5" aria-hidden="true" />
                      {t('servers.users.permissionsCount', { count: user.scopes?.length || 0 })}
                    </Badge>
                  </div>
                  <p className="mt-0.5 truncate text-xs text-muted-foreground">{user.email}</p>
                </div>
              </div>

              <div className="shrink-0">
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
            </li>
          ))}
          </ul>
        )}
      </div>
    </Card>
  );
}
