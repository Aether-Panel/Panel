'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useState, useMemo, useCallback } from 'react';
import { PageHeader } from '@/components/page-header';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Checkbox } from '@/components/ui/checkbox';
import { PlusCircle, Trash2, Loader2, ShieldCheck, Shield, Search, Pencil } from 'lucide-react';
import { Textarea } from '@/components/ui/textarea';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { useToast } from '@/hooks/use-toast';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';

type Role = {
  id?: number;
  name: string;
  description: string;
  scopes: string[];
};

const permissionGroups: { label: string; keys: string[] }[] = [
  {
    label: "roles.permissionGroups.general",
    keys: ["admin", "login", "panel", "oauth2.auth"]
  },
  {
    label: "roles.permissionGroups.nodes",
    keys: ["nodes.view", "nodes.create", "nodes.edit", "nodes.delete", "nodes.deploy"]
  },
  {
    label: "roles.permissionGroups.servers",
    keys: ["server.create", "server.view", "server.delete", "server.name.edit"]
  },
  {
    label: "roles.permissionGroups.serverConsole",
    keys: ["server.console.send"]
  },
  {
    label: "roles.permissionGroups.serverManagement",
    keys: ["server.start", "server.stop", "server.kill", "server.reload", "server.install",
           "server.definition.view", "server.definition.edit",
           "server.data.view", "server.data.edit.admin",
           "server.flags.view", "server.flags.edit",
           "server.tasks.view", "server.tasks.run", "server.tasks.create", "server.tasks.edit", "server.tasks.delete"]
  },
  {
    label: "roles.permissionGroups.serverFiles",
    keys: ["server.files.view", "server.files.edit", "server.sftp"]
  },
  {
    label: "roles.permissionGroups.serverBackups",
    keys: ["server.backup.view", "server.backup.create", "server.backup.restore", "server.backup.delete"]
  },
  {
    label: "roles.permissionGroups.serverUsers",
    keys: ["server.users.view", "server.users.create", "server.users.edit", "server.users.delete",
           "server.clients.view", "server.clients.edit", "server.clients.create", "server.clients.delete"]
  },
  {
    label: "roles.permissionGroups.serverAdmin",
    keys: ["server.admin.view", "server.admin.install.view", "server.admin.install.manage",
           "server.admin.transfer.view", "server.admin.transfer.manage",
           "server.admin.config.view", "server.admin.config.manage",
           "server.admin.assignments.view", "server.admin.assignments.manage"]
  },
  {
    label: "roles.permissionGroups.serverMisc",
    keys: ["server.stats", "server.status", "server.admin"]
  },
  {
    label: "roles.permissionGroups.templates",
    keys: ["templates.view", "templates.local.edit", "templates.repo.create", "templates.repo.delete"]
  },
  {
    label: "roles.permissionGroups.users",
    keys: ["users.info.search", "users.info.view", "users.info.edit",
           "users.perms.view", "users.perms.edit"]
  },
  {
    label: "roles.permissionGroups.other",
    keys: ["self.edit", "self.clients", "settings.edit", "uptime.view"]
  }
];

const allPermissionKeys = permissionGroups.flatMap(g => g.keys);

const translationKeyOverride: Record<string, string> = {
  'server.admin.view': 'server.adminSection.view',
  'server.admin.install.view': 'server.adminSection.installView',
  'server.admin.install.manage': 'server.adminSection.installManage',
  'server.admin.transfer.view': 'server.adminSection.transferView',
  'server.admin.transfer.manage': 'server.adminSection.transferManage',
  'server.admin.config.view': 'server.adminSection.configView',
  'server.admin.config.manage': 'server.adminSection.configManage',
  'server.admin.assignments.view': 'server.adminSection.assignmentsView',
  'server.admin.assignments.manage': 'server.adminSection.assignmentsManage',
};


export default function RolesPage() {
  const { role, hasScope } = useAuth();
  const [isMounted, setIsMounted] = useState(false);
  const { t } = useTranslations();

  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const { toast } = useToast();

  const fetchRoles = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.get('/api/roles');
      setRoles(data || []);
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Failed to fetch roles.', variant: 'destructive' });
    } finally {
      setLoading(false);
    }
  }, [t, toast]);

  useEffect(() => {
    setIsMounted(true);
    if (role && !hasScope('admin')) {
      window.location.href = '/dashboard';
    }
    fetchRoles();
  }, [role, hasScope, fetchRoles]);

  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false);
  const [isAdding, setIsAdding] = useState(false);

  // New role form state
  const [newRoleName, setNewRoleName] = useState('');
  const [newRoleDescription, setNewRoleDescription] = useState('');
  const [newRolePermissions, setNewRolePermissions] = useState<string[]>([]);
  const [searchQuery, setSearchQuery] = useState('');

  const getPermissionLabel = useCallback((permissionKey: string): string => {
    const mapped = translationKeyOverride[permissionKey] || permissionKey;
    return t(`permissions.${mapped}`);
  }, [t]);

  const handlePermissionChange = useCallback((permissionKey: string, checked: boolean) => {
    setNewRolePermissions(prev =>
      checked ? [...prev, permissionKey] : prev.filter(p => p !== permissionKey)
    );
  }, []);

  const handleSelectGroup = useCallback((keys: string[], select: boolean) => {
    setNewRolePermissions(prev => {
      if (select) {
        const newSet = new Set(prev);
        keys.forEach(k => newSet.add(k));
        return Array.from(newSet);
      } else {
        return prev.filter(p => !keys.includes(p));
      }
    });
  }, []);

  const handleAddRole = useCallback(async () => {
    if (!newRoleName) return;

    setIsAdding(true);
    try {
      await api.post('/api/roles', {
        name: newRoleName,
        description: newRoleDescription,
        scopes: newRolePermissions,
      });

      toast({ title: t('common.success'), description: 'Role created successfully.' });

      setNewRoleName('');
      setNewRoleDescription('');
      setNewRolePermissions([]);
      setSearchQuery('');
      setIsAddDialogOpen(false);
      fetchRoles();
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Failed to create role.', variant: 'destructive' });
    } finally {
      setIsAdding(false);
    }
  }, [newRoleName, newRoleDescription, newRolePermissions, t, toast, fetchRoles]);

  const handleDeleteRole = useCallback(async (id: number) => {
    try {
      await api.delete(`/api/roles/${id}`);
      toast({ title: t('common.success'), description: 'Role deleted successfully.' });
      fetchRoles();
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Failed to delete role.', variant: 'destructive' });
    }
  }, [t, toast, fetchRoles]);

  // Edit role state
  const [editRole, setEditRole] = useState<Role | null>(null);
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [editName, setEditName] = useState('');
  const [editDescription, setEditDescription] = useState('');
  const [editPermissions, setEditPermissions] = useState<string[]>([]);

  const openEditDialog = useCallback(async (role: Role) => {
    setEditRole(role);
    setEditName(role.name);
    setEditDescription(role.description || '');
    setEditPermissions(role.scopes || []);
    setIsEditDialogOpen(true);
  }, []);

  const handlePermissionChangeEdit = useCallback((permissionKey: string, checked: boolean) => {
    setEditPermissions(prev =>
      checked ? [...prev, permissionKey] : prev.filter(p => p !== permissionKey)
    );
  }, []);

  const handleSelectGroupEdit = useCallback((keys: string[], select: boolean) => {
    setEditPermissions(prev => {
      if (select) {
        const newSet = new Set(prev);
        keys.forEach(k => newSet.add(k));
        return Array.from(newSet);
      } else {
        return prev.filter(p => !keys.includes(p));
      }
    });
  }, []);

  const handleSaveEdit = useCallback(async () => {
    if (!editRole || !editName) return;

    setIsEditing(true);
    try {
      await api.post(`/api/roles/${editRole.id}`, {
        name: editName,
        description: editDescription,
        scopes: editPermissions,
      });

      toast({ title: t('common.success'), description: 'Role updated successfully.' });
      setIsEditDialogOpen(false);
      setEditRole(null);
      fetchRoles();
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Failed to update role.', variant: 'destructive' });
    } finally {
      setIsEditing(false);
    }
  }, [editRole, editName, editDescription, editPermissions, t, toast, fetchRoles]);

  const filteredGroups = useMemo(() => {
    if (!searchQuery) return permissionGroups;
    const q = searchQuery.toLowerCase();
    return permissionGroups
      .map(group => ({
        ...group,
        keys: group.keys.filter(k => {
          const label = getPermissionLabel(k).toLowerCase();
          return label.includes(q) || k.toLowerCase().includes(q);
        })
      }))
      .filter(group => group.keys.length > 0);
  }, [searchQuery, getPermissionLabel]);

  if (!isMounted || !hasScope('admin') || loading) {
    return (
      <div className="flex h-full items-center justify-center min-h-[400px]">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-8">
      <PageHeader title={t('roles.title')} description={t('roles.description')}>
        <Dialog open={isAddDialogOpen} onOpenChange={setIsAddDialogOpen}>
          <DialogTrigger asChild>
            <Button>
              <PlusCircle className="mr-2" />
              {t('roles.addRole')}
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[600px]">
            <DialogHeader>
              <DialogTitle>{t('roles.addDialog.title')}</DialogTitle>
              <DialogDescription>
                {t('roles.addDialog.description')}
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid grid-cols-4 items-center gap-4">
                <Label htmlFor="role-name" className="text-right">
                  {t('roles.addDialog.nameLabel')}
                </Label>
                <Input
                  id="role-name"
                  value={newRoleName}
                  onChange={(e) => setNewRoleName(e.target.value)}
                  className="col-span-3"
                  placeholder={t('roles.addDialog.namePlaceholder')}
                />
              </div>
              <div className="grid grid-cols-4 items-start gap-4">
                <Label htmlFor="role-description" className="text-right pt-2">
                  {t('roles.addDialog.descriptionLabel')}
                </Label>
                <Textarea
                  id="role-description"
                  value={newRoleDescription}
                  onChange={(e) => setNewRoleDescription(e.target.value)}
                  className="col-span-3"
                  placeholder={t('roles.addDialog.descriptionPlaceholder')}
                />
              </div>
              <div className="grid grid-cols-4 items-start gap-4">
                <Label className="text-right pt-2">
                  {t('roles.addDialog.permissionsLabel')}
                </Label>
                <div className="col-span-3 space-y-3 max-h-[28rem] overflow-y-auto pr-2">
                  <div className="relative">
                    <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input
                      placeholder="Search permissions..."
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      className="pl-8"
                    />
                  </div>
                  {filteredGroups.map((group) => {
                    const selectedCount = group.keys.filter(k => newRolePermissions.includes(k)).length;
                    const allSelected = selectedCount === group.keys.length;
                    return (
                      <div key={group.label} className="rounded-lg border bg-card/50 overflow-hidden">
                        <div className="flex items-center justify-between px-3 py-2 bg-muted/30 border-b">
                          <span className="text-sm font-semibold">{t(group.label)}</span>
                          <div className="flex items-center gap-2">
                            <span className="text-xs text-muted-foreground">{selectedCount}/{group.keys.length}</span>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-6 px-2 text-xs"
                              onClick={() => handleSelectGroup(group.keys, !allSelected)}
                            >
                              {allSelected ? 'Deselect all' : 'Select all'}
                            </Button>
                          </div>
                        </div>
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1.5 p-3">
                          {group.keys.map(permissionKey => (
                            <div key={permissionKey} className="flex items-center space-x-2">
                              <Checkbox
                                id={`perm-${permissionKey}`}
                                onCheckedChange={(checked) => handlePermissionChange(permissionKey, !!checked)}
                                checked={newRolePermissions.includes(permissionKey)}
                              />
                              <label
                                htmlFor={`perm-${permissionKey}`}
                                className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70 cursor-pointer"
                              >
                                {getPermissionLabel(permissionKey)}
                              </label>
                            </div>
                          ))}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setIsAddDialogOpen(false)} disabled={isAdding}>{t('roles.addDialog.cancel')}</Button>
              <Button type="submit" onClick={handleAddRole} disabled={isAdding}>
                {isAdding && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {t('roles.addDialog.create')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </PageHeader>

      <Dialog open={isEditDialogOpen} onOpenChange={(open) => { if (!open) { setIsEditDialogOpen(false); setEditRole(null); } }}>
        <DialogContent className="sm:max-w-[600px]">
          <DialogHeader>
            <DialogTitle>Edit Role: {editRole?.name}</DialogTitle>
            <DialogDescription>
              Update the role name, description, and permissions.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid grid-cols-4 items-center gap-4">
              <Label htmlFor="edit-role-name" className="text-right">
                {t('roles.addDialog.nameLabel')}
              </Label>
              <Input
                id="edit-role-name"
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
                className="col-span-3"
                placeholder={t('roles.addDialog.namePlaceholder')}
              />
            </div>
            <div className="grid grid-cols-4 items-start gap-4">
              <Label htmlFor="edit-role-description" className="text-right pt-2">
                {t('roles.addDialog.descriptionLabel')}
              </Label>
              <Textarea
                id="edit-role-description"
                value={editDescription}
                onChange={(e) => setEditDescription(e.target.value)}
                className="col-span-3"
                placeholder={t('roles.addDialog.descriptionPlaceholder')}
              />
            </div>
            <div className="grid grid-cols-4 items-start gap-4">
              <Label className="text-right pt-2">
                {t('roles.addDialog.permissionsLabel')}
              </Label>
              <div className="col-span-3 space-y-3 max-h-[28rem] overflow-y-auto pr-2">
                {permissionGroups.map((group) => {
                  const selectedCount = group.keys.filter(k => editPermissions.includes(k)).length;
                  const allSelected = selectedCount === group.keys.length;
                  return (
                    <div key={group.label} className="rounded-lg border bg-card/50 overflow-hidden">
                      <div className="flex items-center justify-between px-3 py-2 bg-muted/30 border-b">
                        <span className="text-sm font-semibold">{t(group.label)}</span>
                        <div className="flex items-center gap-2">
                          <span className="text-xs text-muted-foreground">{selectedCount}/{group.keys.length}</span>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-6 px-2 text-xs"
                            onClick={() => handleSelectGroupEdit(group.keys, !allSelected)}
                          >
                            {allSelected ? 'Deselect all' : 'Select all'}
                          </Button>
                        </div>
                      </div>
                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1.5 p-3">
                        {group.keys.map(permissionKey => (
                          <div key={permissionKey} className="flex items-center space-x-2">
                            <Checkbox
                              id={`edit-perm-${permissionKey}`}
                              onCheckedChange={(checked) => handlePermissionChangeEdit(permissionKey, !!checked)}
                              checked={editPermissions.includes(permissionKey)}
                            />
                            <label
                              htmlFor={`edit-perm-${permissionKey}`}
                              className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70 cursor-pointer"
                            >
                              {getPermissionLabel(permissionKey)}
                            </label>
                          </div>
                        ))}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setIsEditDialogOpen(false); setEditRole(null); }} disabled={isEditing}>
              {t('roles.addDialog.cancel')}
            </Button>
            <Button type="submit" onClick={handleSaveEdit} disabled={isEditing}>
              {isEditing && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Save Changes
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
        <Card className="border-0">
          <CardHeader>
            <CardTitle>{t('roles.availableRoles')}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('roles.table.role')}</TableHead>
                  <TableHead className="hidden md:table-cell">{t('roles.table.description')}</TableHead>
                  <TableHead className="hidden md:table-cell">{t('roles.table.permissions')}</TableHead>
                  <TableHead className="text-right">{t('common.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {roles.map((role) => (
                  <TableRow key={role.id}>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <Badge
                            variant={role.name === 'admin' || role.name === 'Administrador' ? 'default' : role.name === 'Usuario' ? 'secondary' : 'outline'}
                            className="capitalize"
                          >
                            {role.name}
                          </Badge>
                          {(role.name === 'admin' || role.name === 'Administrador') && (
                            <span className="inline-flex items-center gap-1 text-[10px] font-medium text-primary">
                              <ShieldCheck className="h-3 w-3" /> Admin
                            </span>
                          )}
                          {role.name === 'Usuario' && (
                            <span className="inline-flex items-center gap-1 text-[10px] font-medium text-muted-foreground">
                              <Shield className="h-3 w-3" /> Predeterminado
                            </span>
                          )}
                        </div>
                        <p className="text-sm text-muted-foreground mt-2 md:hidden">{role.description}</p>
                        <div className="flex flex-wrap gap-2 mt-2 md:hidden">
                          {role.scopes?.map(permissionKey => (
                            <Badge key={permissionKey} variant="outline" className="text-[10px]">{permissionKey}</Badge>
                          ))}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell className="hidden md:table-cell">{role.description}</TableCell>
                    <TableCell className="hidden md:table-cell">
                      <div className="flex flex-wrap gap-2 max-w-sm">
                        {role.scopes?.map(permissionKey => (
                          <Badge key={permissionKey} variant="outline" className="text-[10px]">{permissionKey}</Badge>
                        ))}
                        {(!role.scopes || role.scopes.length === 0) && <span className="text-xs text-muted-foreground">No permissions</span>}
                      </div>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1">
                        {role.name !== 'admin' && role.name !== 'Administrador' && role.name !== 'Usuario' ? (
                          <>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => openEditDialog(role)}
                            >
                              <Pencil className="h-4 w-4" />
                            </Button>
                            <AlertDialog>
                              <AlertDialogTrigger asChild>
                                <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive hover:bg-destructive/10">
                                  <Trash2 className="h-4 w-4" />
                                </Button>
                              </AlertDialogTrigger>
                              <AlertDialogContent>
                                <AlertDialogHeader>
                                  <AlertDialogTitle>{t('roles.deleteDialog.title')}</AlertDialogTitle>
                                  <AlertDialogDescription>
                                    {t('roles.deleteDialog.description')}
                                  </AlertDialogDescription>
                                </AlertDialogHeader>
                                <AlertDialogFooter>
                                  <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                                  <AlertDialogAction
                                    onClick={() => role.id && handleDeleteRole(role.id)}
                                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                                  >
                                    {t('common.delete')}
                                  </AlertDialogAction>
                                </AlertDialogFooter>
                              </AlertDialogContent>
                            </AlertDialog>
                          </>
                        ) : (
                          <span className="text-xs text-muted-foreground px-2">Protegido</span>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
