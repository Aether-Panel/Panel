'use client';
import { useState, useEffect, useCallback } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { PlusCircle, Loader2, Trash2 } from 'lucide-react';
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
    if (!confirm(t('servers.users.revokeConfirm') || 'Are you sure you want to revoke access?')) return;

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
    <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
      <Card className="border-0">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>{t('servers.users.title')}</CardTitle>
              <CardDescription>{t('servers.users.description')}</CardDescription>
            </div>
            <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
              <DialogTrigger asChild>
                <Button>
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
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="flex flex-col items-center justify-center py-12 space-y-4">
              <Loader2 className="h-8 w-8 animate-spin text-primary" />
              <p className="text-sm text-muted-foreground">{t('common.loading') || 'Loading users...'}</p>
            </div>
          ) : users.length === 0 ? (
            <div className="text-center py-12 border-2 border-dashed rounded-lg">
              <p className="text-muted-foreground">{t('servers.users.noUsers') || 'No users have access to this server yet.'}</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('servers.users.table.user')}</TableHead>
                  <TableHead className="hidden sm:table-cell">{t('servers.users.table.email')}</TableHead>
                  <TableHead className="text-right">{t('servers.users.table.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((user) => (
                  <TableRow key={user.email}>
                    <TableCell>
                      <div className="flex items-center gap-3">
                        <Avatar>
                          <AvatarFallback className="bg-primary/10 text-primary">
                            {(user.username || user.email).charAt(0).toUpperCase()}
                          </AvatarFallback>
                        </Avatar>
                        <div>
                          <p className="font-medium">{user.username || user.email.split('@')[0]}</p>
                          <p className="text-sm text-muted-foreground sm:hidden">{user.email}</p>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell className="hidden sm:table-cell">{user.email}</TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleRevoke(user.email)}
                        className="text-red-500 hover:bg-red-500/10 hover:text-red-500 transition-colors"
                      >
                        <Trash2 className="h-4 w-4 sm:mr-2" />
                        <span className="hidden sm:inline">
                          {t('servers.users.revoke')}
                        </span>
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
