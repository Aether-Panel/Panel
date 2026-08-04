'use client';
import { useState, useEffect, useCallback } from 'react';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { PlusCircle, Loader2, Trash2, Users as UsersIcon, ShieldCheck } from 'lucide-react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { useToast } from '@/hooks/use-toast';

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
  const { toast } = useToast();
  const { t } = useTranslations();

  const fetchUsers = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.get(`/api/servers/${serverId}/user`);
      setUsers(data || []);
    } catch (err) {
      console.error('Failed to fetch users:', err);
      toast({
        variant: 'destructive',
        title: t('common.error') || 'Error',
        description: t('servers.users.fetchError') || 'Failed to load users'
      });
    } finally {
      setLoading(false);
    }
  }, [serverId, t, toast]);

  useEffect(() => {
    fetchUsers();
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

      toast({
        title: t('common.success') || 'Success',
        description: t('servers.users.addSuccess') || 'User added successfully'
      });
      setEmail('');
      setIsDialogOpen(false);
      fetchUsers();
    } catch (err: any) {
      console.error('Failed to add user:', err);
      toast({
        variant: 'destructive',
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
      toast({
        title: t('common.success') || 'Success',
        description: t('servers.users.revokeSuccess') || 'Access revoked successfully'
      });
      fetchUsers();
    } catch (err: any) {
      console.error('Failed to revoke access:', err);
      toast({
        variant: 'destructive',
        title: t('common.error') || 'Error',
        description: err.message || t('servers.users.revokeError') || 'Failed to revoke access'
      });
    }
  };

  return (
    <div className="mt-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-4">
          <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl border border-primary/30 bg-gradient-to-br from-primary/25 via-accent/15 to-transparent text-primary shadow-[0_0_20px_rgb(0_0_0/0.3)]">
            <UsersIcon className="h-5 w-5" />
          </div>
          <div>
            <h2 className="font-headline text-2xl font-bold tracking-tight">{t('servers.users.title')}</h2>
            <p className="text-muted-foreground">{t('servers.users.description')}</p>
          </div>
        </div>
        <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
          <DialogTrigger asChild>
            <Button className="shrink-0">
              <PlusCircle className="mr-2 h-4 w-4" />
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
                <div className="grid grid-cols-4 items-center gap-4">
                  <Label htmlFor="email" className="text-right">
                    {t('servers.users.addDialog.emailLabel')}
                  </Label>
                  <Input
                    id="email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder={t('servers.users.addDialog.emailPlaceholder')}
                    className="col-span-3"
                    required
                    disabled={addingUser}
                  />
                </div>
              </div>
              <DialogFooter>
                <Button type="submit" disabled={addingUser || !email}>
                  {addingUser && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                  {t('servers.users.addDialog.addButton')}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      <div className="mt-6 flex flex-col gap-4">
        {loading ? (
          <div className="flex justify-center py-16">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
          </div>
        ) : users.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border-2 border-dashed border-border/70 bg-card/50 p-14 text-center backdrop-blur-sm">
            <div className="mx-auto mb-4 grid h-14 w-14 place-items-center rounded-2xl border border-primary/25 bg-gradient-to-br from-primary/20 via-accent/10 to-transparent text-primary">
              <UsersIcon className="h-6 w-6" />
            </div>
            <h3 className="font-headline text-lg font-semibold">{t('servers.users.noUsers') || 'No users have access to this server yet.'}</h3>
            <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
              <DialogTrigger asChild>
                <Button className="mt-6">
                  <PlusCircle className="mr-2 h-4 w-4" />
                  {t('servers.users.addUser')}
                </Button>
              </DialogTrigger>
            </Dialog>
          </div>
        ) : (
          users.map((user) => (
            <div key={user.email} className="group relative flex items-center justify-between gap-4 overflow-hidden rounded-xl border border-border/80 bg-card p-4 transition-all duration-200 hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-[0_8px_30px_rgb(0_0_0/0.35)]">
              <div className="pointer-events-none absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-primary via-accent to-transparent" />
              <div className="pointer-events-none absolute -right-8 -top-8 opacity-[0.05] transition-transform duration-300 group-hover:scale-110">
                <UsersIcon className="h-28 w-28" />
              </div>

              <div className="relative flex min-w-0 items-center gap-4">
                <Avatar className="h-11 w-11 shrink-0 ring-1 ring-border">
                  <AvatarFallback className="bg-gradient-to-br from-primary/25 to-accent/20 text-lg font-bold text-primary">
                    {(user.username || user.email).charAt(0).toUpperCase()}
                  </AvatarFallback>
                </Avatar>
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="truncate font-headline font-semibold leading-tight">{user.username || user.email.split('@')[0]}</p>
                    <Badge
                      variant="outline"
                      className="hidden shrink-0 items-center gap-1 border-primary/25 bg-primary/10 px-1.5 py-px text-[10px] font-medium text-primary sm:inline-flex"
                    >
                      <ShieldCheck className="h-2.5 w-2.5" />
                      {user.scopes?.length || 0} permisos
                    </Badge>
                  </div>
                  <p className="truncate font-mono text-xs text-muted-foreground">{user.email}</p>
                </div>
              </div>

              <div className="relative shrink-0">
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                    >
                      <Trash2 className="h-4 w-4 sm:mr-2" />
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
                      <AlertDialogCancel>{t('common.cancel') || 'Cancel'}</AlertDialogCancel>
                      <AlertDialogAction onClick={() => handleRevoke(user.email)} className="bg-red-500 hover:bg-red-600">
                        {t('servers.users.revoke')}
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
