'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useState } from 'react';
import { users as initialUsers, servers } from '@/lib/data';
import type { User } from '@/lib/data';
import { PageHeader } from '@/components/page-header';
import { Button } from '@/components/ui/button';
import { MoreHorizontal, PlusCircle, Loader2 } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Badge } from '@/components/ui/badge';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger, DropdownMenuSeparator } from '@/components/ui/dropdown-menu';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { useToast } from '@/hooks/use-toast';

export default function UsersPage() {
  const { role, hasScope, user: currentUser, fetchSelf } = useAuth();
  const [users, setUsers] = useState<User[]>([]);
  const [roles, setRoles] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [isMounted, setIsMounted] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [viewingUser, setViewingUser] = useState<User | null>(null);
  const [isAddUserDialogOpen, setIsAddUserDialogOpen] = useState(false);
  const { t } = useTranslations();
  const { toast } = useToast();

  // State for the form
  const [formName, setFormName] = useState('');
  const [formUsername, setFormUsername] = useState('');
  const [formEmail, setFormEmail] = useState('');
  const [formPassword, setFormPassword] = useState('');
  const [formRoleId, setFormRoleId] = useState<string>('none');

  const fetchUsers = async () => {
    setLoading(true);
    try {
      const resp = await api.get('/api/users');
      // The API returns { users: [...], metadata: {...} }
      const usersData = resp.users || [];
      setUsers(usersData.map((u: any) => ({
        id: u.id,
        name: u.username, // Using username as name
        username: u.username,
        email: u.email,
        roleId: u.roleId,
        role: u.role?.name || (u.roleId ? `Role ${u.roleId}` : 'No Role'),
        avatar: `https://avatar.vercel.sh/${u.email}`,
        assignedServers: [] // TODO: fetch permissions if needed
      })));
    } catch (e: any) {
      toast({ title: t('common.error'), description: 'Failed to fetch users.', variant: 'destructive' });
    } finally {
      setLoading(false);
    }
  };

  const fetchRoles = async () => {
    try {
      const data = await api.get('/api/roles');
      setRoles(data || []);
    } catch (e: any) { }
  };

  useEffect(() => {
    setIsMounted(true);
    if (role && !hasScope('users.info.view')) {
      window.location.href = '/dashboard';
    }
    fetchUsers();
    fetchRoles();
  }, [role, hasScope]);

  useEffect(() => {
    if (editingUser) {
      setFormName(editingUser.username || editingUser.name);
      setFormUsername(editingUser.username || editingUser.name);
      setFormEmail(editingUser.email);
      setFormRoleId(editingUser.roleId?.toString() || 'none');
      setFormPassword('');
    } else {
      setFormName('');
      setFormUsername('');
      setFormEmail('');
      setFormPassword('');
      setFormRoleId('none');
    }
  }, [editingUser, isAddUserDialogOpen]);

  const handleAddUser = async () => {
    try {
      await api.post('/api/users', {
        username: formUsername,
        email: formEmail,
        password: formPassword,
        roleId: formRoleId === 'none' ? null : parseInt(formRoleId)
      });
      toast({ title: t('common.success'), description: 'User created successfully.' });
      setIsAddUserDialogOpen(false);
      fetchUsers();
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Failed to create user.', variant: 'destructive' });
    }
  };

  const handleUpdateUser = async () => {
    if (!editingUser) return;
    try {
      await api.post(`/api/users/${editingUser.id}`, {
        username: formUsername,
        email: formEmail,
        password: formPassword || undefined,
        roleId: formRoleId === 'none' ? null : parseInt(formRoleId)
      });
      toast({ title: t('common.success'), description: 'User updated successfully.' });

      // If we just updated ourselves, re-fetch self to update UI role/scopes
      if (editingUser.id === currentUser?.id) {
        console.log('Detected self-update, re-fetching session data');
        await fetchSelf();
      }

      setEditingUser(null);
      fetchUsers();
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Failed to update user.', variant: 'destructive' });
    }
  };

  const handleDeleteUser = async (id: number | string) => {
    try {
      await api.delete(`/api/users/${id}`);
      toast({ title: t('common.success'), description: 'User deleted successfully.' });
      fetchUsers();
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Failed to delete user.', variant: 'destructive' });
    }
  };

  const getAssignedServers = (user: User | null) => {
    if (!user) return [];
    return servers.filter(server => user.assignedServers.includes(server.id));
  };


  if (!isMounted || !hasScope('users.info.view') || loading) {
    return (
      <div className="flex h-full items-center justify-center min-h-[400px]">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-8">
      <PageHeader title={t('users.title')} description={t('users.description')}>
        <Dialog open={isAddUserDialogOpen} onOpenChange={setIsAddUserDialogOpen}>
          <DialogTrigger asChild>
            <Button>
              <PlusCircle className="mr-2" />
              {t('users.addUser')}
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[425px]">
            <DialogHeader>
              <DialogTitle>{t('users.addDialog.title')}</DialogTitle>
              <DialogDescription>{t('users.addDialog.description')}</DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid grid-cols-4 items-center gap-4">
                <Label htmlFor="username" className="text-right">{t('users.addDialog.nameLabel')}</Label>
                <Input id="username" value={formUsername} onChange={(e) => setFormUsername(e.target.value)} placeholder={t('users.addDialog.namePlaceholder')} className="col-span-3" />
              </div>
              <div className="grid grid-cols-4 items-center gap-4">
                <Label htmlFor="email" className="text-right">{t('users.addDialog.emailLabel')}</Label>
                <Input id="email" type="email" value={formEmail} onChange={(e) => setFormEmail(e.target.value)} placeholder={t('users.addDialog.emailPlaceholder')} className="col-span-3" />
              </div>
              <div className="grid grid-cols-4 items-center gap-4">
                <Label htmlFor="password" className="text-right">{t('users.addDialog.passwordLabel')}</Label>
                <Input id="password" type="password" value={formPassword} onChange={(e) => setFormPassword(e.target.value)} placeholder={t('users.addDialog.passwordPlaceholder')} className="col-span-3" />
              </div>
              <div className="grid grid-cols-4 items-center gap-4">
                <Label htmlFor="role" className="text-right">{t('users.addDialog.roleLabel')}</Label>
                <Select value={formRoleId} onValueChange={setFormRoleId}>
                  <SelectTrigger id="role" className="col-span-3">
                    <SelectValue placeholder={t('users.addDialog.rolePlaceholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">Explicit Permissions Only</SelectItem>
                    {roles.map(r => (
                      <SelectItem key={r.id} value={r.id.toString()}>{r.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setIsAddUserDialogOpen(false)}>{t('users.addDialog.cancel')}</Button>
              <Button type="submit" onClick={handleAddUser}>{t('users.addDialog.create')}</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </PageHeader>

      <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
        <Card className="border-0">
          <CardHeader>
            <CardTitle>{t('users.allUsers')}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('users.table.user')}</TableHead>
                  <TableHead className="hidden md:table-cell">{t('users.table.role')}</TableHead>
                  <TableHead className="hidden md:table-cell">{t('users.table.assignedServers')}</TableHead>
                  <TableHead className="text-right">{t('users.table.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((user) => (
                  <TableRow key={user.id}>
                    <TableCell>
                      <div className="flex items-center gap-3">
                        <Avatar>
                          <AvatarImage src={user.avatar} alt={user.name} />
                          <AvatarFallback>{user.name.charAt(0)}</AvatarFallback>
                        </Avatar>
                        <div>
                          <p className="font-medium">{user.name}</p>
                          <p className="text-sm text-muted-foreground">{user.email}</p>
                          <div className="flex items-center gap-2 md:hidden mt-1">
                            <Badge variant={user.role === 'admin' ? 'default' : 'secondary'} className="capitalize">{user.role}</Badge>
                            <span className="text-xs text-muted-foreground">{t('users.table.serversCount', { count: user.assignedServers.length })}</span>
                          </div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell className="hidden md:table-cell">
                      <Badge variant={user.role === 'admin' ? 'default' : 'secondary'} className="capitalize">{user.role}</Badge>
                    </TableCell>
                    <TableCell className="hidden md:table-cell">{t('users.table.serversCount', { count: user.assignedServers.length })}</TableCell>
                    <TableCell className="text-right">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" className="h-8 w-8 p-0">
                            <span className="sr-only">Open menu</span>
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuLabel>{t('users.actions.menuLabel')}</DropdownMenuLabel>
                          <DropdownMenuItem onClick={() => setEditingUser(user)}>{t('users.actions.edit')}</DropdownMenuItem>
                          <DropdownMenuItem onClick={() => setViewingUser(user)}>{t('users.actions.viewServers')}</DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem className="text-red-500" onClick={() => handleDeleteUser(user.id)}>{t('users.actions.delete')}</DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>

      {/* Edit User Dialog */}
      <Dialog open={!!editingUser} onOpenChange={(isOpen) => !isOpen && setEditingUser(null)}>
        <DialogContent className="sm:max-w-[425px]">
          <DialogHeader>
            <DialogTitle>{t('users.editDialog.title', { name: editingUser?.name || '' })}</DialogTitle>
            <DialogDescription>
              {t('users.editDialog.description', { name: editingUser?.name || '' })}
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid grid-cols-4 items-center gap-4">
              <Label htmlFor="edit-username" className="text-right">{t('users.editDialog.nameLabel')}</Label>
              <Input id="edit-username" value={formUsername} onChange={(e) => setFormUsername(e.target.value)} className="col-span-3" />
            </div>
            <div className="grid grid-cols-4 items-center gap-4">
              <Label htmlFor="edit-email" className="text-right">{t('users.editDialog.emailLabel')}</Label>
              <Input id="edit-email" type="email" value={formEmail} onChange={(e) => setFormEmail(e.target.value)} className="col-span-3" />
            </div>
            <div className="grid grid-cols-4 items-center gap-4">
              <Label htmlFor="edit-password" className="text-right">{t('users.addDialog.passwordLabel')}</Label>
              <Input id="edit-password" type="password" value={formPassword} onChange={(e) => setFormPassword(e.target.value)} placeholder="Leave blank to keep current" className="col-span-3" />
            </div>
            <div className="grid grid-cols-4 items-center gap-4">
              <Label htmlFor="edit-role" className="text-right">{t('users.editDialog.roleLabel')}</Label>
              <Select value={formRoleId} onValueChange={setFormRoleId}>
                <SelectTrigger id="edit-role" className="col-span-3">
                  <SelectValue placeholder={t('users.editDialog.rolePlaceholder')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">Explicit Permissions Only</SelectItem>
                  {roles.map(r => (
                    <SelectItem key={r.id} value={r.id.toString()}>{r.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditingUser(null)}>{t('users.editDialog.cancel')}</Button>
            <Button type="submit" onClick={handleUpdateUser}>{t('users.editDialog.save')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* View Servers Dialog */}
      <Dialog open={!!viewingUser} onOpenChange={(isOpen) => !isOpen && setViewingUser(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('users.serversDialog.title', { name: viewingUser?.name || '' })}</DialogTitle>
            <DialogDescription>{t('users.serversDialog.description')}</DialogDescription>
          </DialogHeader>
          <div className="py-4">
            {getAssignedServers(viewingUser).length > 0 ? (
              <ul className="space-y-2">
                {getAssignedServers(viewingUser).map(server => (
                  <li key={server.id} className="rounded-md border p-3 text-sm">{server.name}</li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground text-center">{t('users.serversDialog.noServers')}</p>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setViewingUser(null)}>{t('users.serversDialog.close')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
