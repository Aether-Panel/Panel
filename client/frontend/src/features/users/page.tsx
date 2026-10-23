'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useState } from 'react';
import type { Dispatch, SetStateAction } from 'react';
import type { User } from '@/lib/data';
import { PageHeader } from '@/components/page-header';
import { Button } from '@/components/ui/button';
import { AlertTriangle, MoreHorizontal, Plus, Server as ServerIcon, Trash2, Users as UsersIcon } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { UserAvatar as AccountAvatar } from '@/components/user-avatar';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger, DropdownMenuSeparator } from '@/components/ui/dropdown-menu';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from "@/lib/toast";
import { EmptyState } from '@/components/empty-state';

type Field<T> = [T, Dispatch<SetStateAction<T>>];

/** Username, email, password and role: shared by the add and edit dialogs. */
function UserFields({
  ids,
  username,
  email,
  password,
  roleId,
  roles,
  mode,
}: {
  ids: { username: string; email: string; password: string; role: string };
  username: Field<string>;
  email: Field<string>;
  password: Field<string>;
  roleId: Field<string>;
  roles: any[];
  mode: 'add' | 'edit';
}) {
  const { t } = useTranslations();
  const dialog = mode === 'add' ? 'addDialog' : 'editDialog';
  return (
    <div className="grid gap-4">
      <div className="space-y-2">
        <Label htmlFor={ids.username}>{t(`users.${dialog}.nameLabel`)}</Label>
        <Input
          id={ids.username}
          value={username[0]}
          onChange={(e) => username[1](e.target.value)}
          placeholder={mode === 'add' ? t('users.addDialog.namePlaceholder') : undefined}
          autoComplete="off"
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor={ids.email}>{t(`users.${dialog}.emailLabel`)}</Label>
        <Input
          id={ids.email}
          type="email"
          value={email[0]}
          onChange={(e) => email[1](e.target.value)}
          placeholder={mode === 'add' ? t('users.addDialog.emailPlaceholder') : undefined}
          autoComplete="off"
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor={ids.password}>{t('users.addDialog.passwordLabel')}</Label>
        <Input
          id={ids.password}
          type="password"
          value={password[0]}
          onChange={(e) => password[1](e.target.value)}
          placeholder={mode === 'add' ? t('users.addDialog.passwordPlaceholder') : t('users.editDialog.passwordPlaceholder')}
          autoComplete="new-password"
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor={ids.role}>{t(`users.${dialog}.roleLabel`)}</Label>
        <Select value={roleId[0]} onValueChange={roleId[1]}>
          <SelectTrigger id={ids.role}>
            <SelectValue placeholder={t(`users.${dialog}.rolePlaceholder`)} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="none">{t('users.explicitPermissionsOnly')}</SelectItem>
            {roles.map(r => (
              <SelectItem key={r.id} value={r.id.toString()}>{r.name}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  );
}

function UserAvatar({ user }: { user: User }) {
  return (
    <AccountAvatar
      identity={user.email || user.name}
      fallback={(user.name || user.email || '?').charAt(0).toUpperCase()}
      className="h-9 w-9 ring-1 ring-border/60"
    />
  );
}

function UsersSkeleton() {
  return (
    <div className="flex flex-col gap-8" aria-busy="true">
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div className="space-y-2">
          <Skeleton className="h-9 w-64" />
          <Skeleton className="h-4 w-96 max-w-full" />
        </div>
        <Skeleton className="h-10 w-40 rounded-lg" />
      </div>
      <Card className="overflow-hidden">
        <div className="space-y-2 border-b border-border/50 p-5 md:p-6">
          <Skeleton className="h-5 w-40" />
          <Skeleton className="h-4 w-48 max-w-full" />
        </div>
        <div className="space-y-2 p-5 md:p-6">
          {Array.from({ length: 5 }, (_, i) => <Skeleton key={i} className="h-14 rounded-xl" />)}
        </div>
      </Card>
    </div>
  );
}

export default function UsersPage() {
  const { role, hasScope, user: currentUser, fetchSelf } = useAuth();
  const [users, setUsers] = useState<User[]>([]);
  const [roles, setRoles] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [isMounted, setIsMounted] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [viewingUser, setViewingUser] = useState<User | null>(null);
  const [viewingServers, setViewingServers] = useState<{ id: string; name: string }[] | null>(null);
  const [deletingUser, setDeletingUser] = useState<User | null>(null);
  const [isAddUserDialogOpen, setIsAddUserDialogOpen] = useState(false);
  const { t } = useTranslations();


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
        role: u.role?.name || (u.roleId ? t('users.roleFallback', { id: u.roleId }) : t('users.noRole')),
        assignedServers: [] // TODO: fetch permissions if needed
      })));
      setLoadFailed(false);
    } catch (e: any) {
      setLoadFailed(true);
      sileo.error({ title: t('common.error'), description: t('users.toast.loadFailed') });
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
    void fetchUsers();
    void fetchRoles();
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
        roleId: formRoleId === 'none' ? null : Number.parseInt(formRoleId)
      });
      sileo.success({ title: t('common.success'), description: t('users.toast.created') });
      setIsAddUserDialogOpen(false);
      void fetchUsers();
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('users.toast.createFailed') });
    }
  };

  const handleUpdateUser = async () => {
    if (!editingUser) return;
    try {
      await api.post(`/api/users/${editingUser.id}`, {
        username: formUsername,
        email: formEmail,
        password: formPassword || undefined,
        roleId: formRoleId === 'none' ? null : Number.parseInt(formRoleId)
      });
      sileo.success({ title: t('common.success'), description: t('users.toast.updated') });

      // If we just updated ourselves, re-fetch self to update UI role/scopes
      if (editingUser.id === currentUser?.id) {
        console.log('Detected self-update, re-fetching session data');
        await fetchSelf();
      }

      setEditingUser(null);
      void fetchUsers();
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('users.toast.updateFailed') });
    }
  };

  const handleDeleteUser = async (id: number | string) => {
    try {
      await api.delete(`/api/users/${id}`);
      sileo.success({ title: t('common.success'), description: t('users.toast.deleted') });
      void fetchUsers();
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('users.toast.deleteFailed') });
    }
  };

  // Servers are looked up when the dialog opens: the user list does not include them
  useEffect(() => {
    if (!viewingUser) return;
    let cancelled = false;
    setViewingServers(null);
    api.get(`/api/servers?username=${encodeURIComponent(viewingUser.username || viewingUser.name)}&limit=100`)
      .then((resp: any) => {
        if (cancelled) return;
        setViewingServers((resp.servers || []).map((s: any) => ({ id: s.id, name: s.name || s.id })));
      })
      .catch(() => {
        if (cancelled) return;
        setViewingServers([]);
        sileo.error({ title: t('common.error'), description: t('users.serversDialog.loadFailed') });
      });
    return () => { cancelled = true; };
  }, [viewingUser]);


  if (!isMounted || !hasScope('users.info.view') || (loading && users.length === 0)) {
    return <UsersSkeleton />;
  }

  const roleBadge = (user: User) => (
    <Badge variant={user.role === 'admin' ? 'default' : 'secondary'} className="max-w-[12rem] truncate">
      {user.role}
    </Badge>
  );

  const renderActions = (user: User) => (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="h-9 w-9 text-muted-foreground hover:text-foreground"
          aria-label={t('users.actions.openMenu', { name: user.name })}
        >
          <MoreHorizontal aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>{t('users.actions.menuLabel')}</DropdownMenuLabel>
        <DropdownMenuItem onClick={() => setEditingUser(user)}>{t('users.actions.edit')}</DropdownMenuItem>
        <DropdownMenuItem onClick={() => setViewingUser(user)}>{t('users.actions.viewServers')}</DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="text-destructive-bright focus:bg-destructive/10 focus:text-destructive-bright"
          onClick={() => setTimeout(() => setDeletingUser(user), 50)}
        >
          <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
          {t('users.actions.delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );

  return (
    <div className="flex flex-col gap-8">
      <PageHeader title={t('users.title')} description={t('users.description')}>
        <Dialog open={isAddUserDialogOpen} onOpenChange={setIsAddUserDialogOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus aria-hidden="true" />
              {t('users.addUser')}
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader className="text-left">
              <DialogTitle>{t('users.addDialog.title')}</DialogTitle>
              <DialogDescription>{t('users.addDialog.description')}</DialogDescription>
            </DialogHeader>
            <div className="py-2">
              <UserFields
                mode="add"
                ids={{ username: 'username', email: 'email', password: 'password', role: 'role' }}
                username={[formUsername, setFormUsername]}
                email={[formEmail, setFormEmail]}
                password={[formPassword, setFormPassword]}
                roleId={[formRoleId, setFormRoleId]}
                roles={roles}
              />
            </div>
            <DialogFooter className="gap-2 sm:gap-0">
              <Button variant="outline" onClick={() => setIsAddUserDialogOpen(false)}>{t('users.addDialog.cancel')}</Button>
              <Button type="submit" onClick={handleAddUser}>{t('users.addDialog.create')}</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </PageHeader>

      <Card className="overflow-hidden">
        <div className="flex flex-col gap-1 border-b border-border/50 p-5 md:p-6">
          <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('users.allUsers')}</h2>
          <p className="text-sm text-muted-foreground">{t('users.listDescription', { count: users.length })}</p>
        </div>

        <div className="p-5 md:p-6">
          {loadFailed && users.length === 0 ? (
            <EmptyState
              icon={AlertTriangle}
              tone="error"
              title={t('users.error.title')}
              description={t('users.toast.loadFailed')}
              action={
                <Button variant="outline" onClick={() => void fetchUsers()}>
                  {t('common.retry')}
                </Button>
              }
            />
          ) : users.length === 0 ? (
            <EmptyState
              icon={UsersIcon}
              title={t('users.empty.title')}
              description={t('users.empty.description')}
              action={
                <Button onClick={() => setIsAddUserDialogOpen(true)}>
                  <Plus aria-hidden="true" />
                  {t('users.addUser')}
                </Button>
              }
            />
          ) : (
            <>
              {/* Phones and tablets (the sidebar leaves too little room for the table): stacked rows. */}
              <ul className="divide-y divide-border/50 overflow-hidden rounded-xl border border-border/60 bg-background/30 lg:hidden">
                {users.map((user) => (
                  <li key={user.id} className="flex items-center gap-3 py-3 pl-4 pr-2">
                    <UserAvatar user={user} />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-semibold">{user.name}</p>
                      <p className="truncate text-xs text-muted-foreground">{user.email}</p>
                      <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1">
                        {roleBadge(user)}
                      </div>
                    </div>
                    {renderActions(user)}
                  </li>
                ))}
              </ul>

              {/* Desktop: the full table. */}
              <div className="hidden overflow-hidden rounded-xl border border-border/60 bg-background/30 lg:block">
                <Table>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent">
                      <TableHead>{t('users.table.user')}</TableHead>
                      <TableHead>{t('users.table.role')}</TableHead>
                      <TableHead className="w-14"><span className="sr-only">{t('users.table.actions')}</span></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {users.map((user) => (
                      <TableRow key={user.id} className="hover:bg-accent/30">
                        <TableCell>
                          <div className="flex min-w-0 items-center gap-3">
                            <UserAvatar user={user} />
                            <div className="min-w-0">
                              <p className="truncate font-medium">{user.name}</p>
                              <p className="truncate text-xs text-muted-foreground">{user.email}</p>
                            </div>
                          </div>
                        </TableCell>
                        <TableCell>{roleBadge(user)}</TableCell>
                        <TableCell className="py-2 text-right">{renderActions(user)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </>
          )}
        </div>
      </Card>

      {/* Edit User Dialog */}
      <Dialog open={!!editingUser} onOpenChange={(isOpen) => !isOpen && setEditingUser(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader className="text-left">
            <DialogTitle>{t('users.editDialog.title', { name: editingUser?.name || '' })}</DialogTitle>
            <DialogDescription>
              {t('users.editDialog.description', { name: editingUser?.name || '' })}
            </DialogDescription>
          </DialogHeader>
          <div className="py-2">
            <UserFields
              mode="edit"
              ids={{ username: 'edit-username', email: 'edit-email', password: 'edit-password', role: 'edit-role' }}
              username={[formUsername, setFormUsername]}
              email={[formEmail, setFormEmail]}
              password={[formPassword, setFormPassword]}
              roleId={[formRoleId, setFormRoleId]}
              roles={roles}
            />
          </div>
          <DialogFooter className="gap-2 sm:gap-0">
            <Button variant="outline" onClick={() => setEditingUser(null)}>{t('users.editDialog.cancel')}</Button>
            <Button type="submit" onClick={handleUpdateUser}>{t('users.editDialog.save')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* View Servers Dialog */}
      <Dialog open={!!viewingUser} onOpenChange={(isOpen) => !isOpen && setViewingUser(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader className="text-left">
            <DialogTitle>{t('users.serversDialog.title', { name: viewingUser?.name || '' })}</DialogTitle>
            <DialogDescription>{t('users.serversDialog.description')}</DialogDescription>
          </DialogHeader>
          <div className="py-2">
            {viewingServers === null ? (
              <div className="space-y-2" role="status" aria-label={t('common.loading')}>
                <Skeleton className="h-11 w-full rounded-xl" />
                <Skeleton className="h-11 w-full rounded-xl" />
              </div>
            ) : viewingServers.length > 0 ? (
              <ul className="divide-y divide-border/50 overflow-hidden rounded-xl border border-border/60 bg-background/30">
                {viewingServers.map(server => (
                  <li key={server.id}>
                    <a href={`/servers/view/?id=${encodeURIComponent(server.id)}`} className="block px-4 py-3 text-sm font-medium transition-colors hover:bg-accent/40 focus-visible:bg-accent/40 focus-visible:outline-none">
                      {server.name}
                    </a>
                  </li>
                ))}
              </ul>
            ) : (
              <EmptyState icon={ServerIcon} size="sm" as="p" title={t('users.serversDialog.noServers')} />
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setViewingUser(null)}>{t('users.serversDialog.close')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation */}
      <AlertDialog open={!!deletingUser} onOpenChange={(isOpen) => !isOpen && setDeletingUser(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('users.deleteDialog.title')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('users.deleteDialog.description', { name: deletingUser?.name || '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => { if (deletingUser) void handleDeleteUser(deletingUser.id); }}
            >
              {t('users.deleteDialog.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
