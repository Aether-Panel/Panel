'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useState } from 'react';
import { PageHeader } from '@/components/page-header';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Checkbox } from '@/components/ui/checkbox';
import { PlusCircle, Trash2, Loader2, ShieldCheck } from 'lucide-react';
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

const allPermissionKeys = [
  "admin", "login", "oauth2.auth", "nodes.view", "nodes.create", "nodes.edit", "nodes.delete", "nodes.deploy",
  "self.edit", "self.clients", "server.create", "settings.edit", "templates.view", "templates.local.edit",
  "templates.repo.create", "templates.repo.delete", "users.info.search", "users.info.view", "users.info.edit",
  "users.perms.view", "users.perms.edit", "uptime.view", "panel"
];


export default function RolesPage() {
  const { role, hasScope } = useAuth();
  const [isMounted, setIsMounted] = useState(false);
  const { t } = useTranslations();

  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const { toast } = useToast();

  const fetchRoles = async () => {
    setLoading(true);
    try {
      const data = await api.get('/api/roles');
      setRoles(data || []);
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Failed to fetch roles.', variant: 'destructive' });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setIsMounted(true);
    if (role && !hasScope('admin')) {
      window.location.href = '/dashboard';
    }
    fetchRoles();
  }, [role, hasScope]);

  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false);
  const [isAdding, setIsAdding] = useState(false);

  // New role form state
  const [newRoleName, setNewRoleName] = useState('');
  const [newRoleDescription, setNewRoleDescription] = useState('');
  const [newRolePermissions, setNewRolePermissions] = useState<string[]>([]);

  const handlePermissionChange = (permissionKey: string, checked: boolean) => {
    setNewRolePermissions(prev =>
      checked ? [...prev, permissionKey] : prev.filter(p => p !== permissionKey)
    );
  };

  const handleAddRole = async () => {
    if (!newRoleName) return;

    setIsAdding(true);
    try {
      await api.post('/api/roles', {
        name: newRoleName,
        description: newRoleDescription,
        scopes: newRolePermissions,
      });

      toast({ title: t('common.success'), description: 'Role created successfully.' });

      // Reset form and close dialog
      setNewRoleName('');
      setNewRoleDescription('');
      setNewRolePermissions([]);
      setIsAddDialogOpen(false);
      fetchRoles();
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Failed to create role.', variant: 'destructive' });
    } finally {
      setIsAdding(false);
    }
  };

  const handleDeleteRole = async (id: number) => {
    try {
      await api.delete(`/api/roles/${id}`);
      toast({ title: t('common.success'), description: 'Role deleted successfully.' });
      fetchRoles();
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Failed to delete role.', variant: 'destructive' });
    }
  };

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
                <div className="col-span-3 grid grid-cols-2 gap-x-6 gap-y-2 max-h-60 overflow-y-auto pr-2">
                  {allPermissionKeys.map(permissionKey => (
                    <div key={permissionKey} className="flex items-center space-x-2">
                      <Checkbox
                        id={`perm-${permissionKey}`}
                        onCheckedChange={(checked) => handlePermissionChange(permissionKey, !!checked)}
                        checked={newRolePermissions.includes(permissionKey)}
                      />
                      <label
                        htmlFor={`perm-${permissionKey}`}
                        className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
                      >
                        {t(`permissions.${permissionKey}`)}
                      </label>
                    </div>
                  ))}
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
                        <div className="flex items-center gap-2">
                          <Badge variant={role.name === 'admin' ? 'default' : 'secondary'} className="capitalize">{role.name}</Badge>
                          {role.name === 'admin' && <ShieldCheck className="h-4 w-4 text-primary" />}
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
                      {role.name !== 'admin' && (
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive hover:bg-destructive/10">
                              <Trash2 className="h-4 w-4" />
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>{t('roles.deleteDialog.title') || 'Are you sure?'}</AlertDialogTitle>
                              <AlertDialogDescription>
                                {t('roles.deleteDialog.description') || 'This will permanently delete the role. Users assigned to this role cannot be deleted until you reassign them.'}
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
                      )}
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
