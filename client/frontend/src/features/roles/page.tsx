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
import { PlusCircle } from 'lucide-react';
import { Textarea } from '@/components/ui/textarea';
import { useTranslations } from '@/contexts/translations-context';

type Role = {
  name: string;
  description: string;
  permissions: string[]; // These are permission keys
};

const allPermissionKeys = [
  "grantAll", "login", "editOwnAccount", "manageOauthClients", "editPanelSettings",
  "createServer", "viewNodes", "createNodes", "editNodes", "deployNodes",
  "deleteNodes", "viewUsers", "viewUserInfo", "editUsers", "viewUserPermissions",
  "editUserPermissions", "viewTemplates", "manageLocalTemplates",
  "viewTemplateRepos", "addTemplateRepos", "deleteTemplateRepos",
];

const initialRolesData = [
  {
    name: 'admin',
    descriptionKey: 'roles.initial.admin.description',
    permissions: ['grantAll'],
  },
  {
    name: 'user',
    descriptionKey: 'roles.initial.user.description',
    permissions: ['login', 'editOwnAccount'],
  },
];


export default function RolesPage() {
  const { role } = useAuth();
  const [isMounted, setIsMounted] = useState(false);
  const { t } = useTranslations();

  const [roles, setRoles] = useState<Role[]>([]);

  useEffect(() => {
    setIsMounted(true);
    if (role && role !== 'admin') {
      window.location.href = '/dashboard';
    }
  }, [role]);

  useEffect(() => {
    setRoles(initialRolesData.map(r => ({
      name: r.name,
      description: t(r.descriptionKey),
      permissions: r.permissions
    })));
  }, [t]);

  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false);

  // New role form state
  const [newRoleName, setNewRoleName] = useState('');
  const [newRoleDescription, setNewRoleDescription] = useState('');
  const [newRolePermissions, setNewRolePermissions] = useState<string[]>([]);

  const handlePermissionChange = (permissionKey: string, checked: boolean) => {
    setNewRolePermissions(prev =>
      checked ? [...prev, permissionKey] : prev.filter(p => p !== permissionKey)
    );
  };

  const handleAddRole = () => {
    if (!newRoleName || !newRoleDescription) return; // Basic validation

    const newRole: Role = {
      name: newRoleName,
      description: newRoleDescription,
      permissions: newRolePermissions,
    };

    setRoles(prev => [...prev, newRole]);

    // Reset form and close dialog
    setNewRoleName('');
    setNewRoleDescription('');
    setNewRolePermissions([]);
    setIsAddDialogOpen(false);
  };

  if (!isMounted || role !== 'admin') {
    return (
      <div className="flex h-full items-center justify-center">
        <p>Loading...</p>
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
              <Button variant="outline" onClick={() => setIsAddDialogOpen(false)}>{t('roles.addDialog.cancel')}</Button>
              <Button type="submit" onClick={handleAddRole}>{t('roles.addDialog.create')}</Button>
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
                </TableRow>
              </TableHeader>
              <TableBody>
                {roles.map((role) => (
                  <TableRow key={role.name}>
                    <TableCell>
                      <Badge variant={role.name === 'admin' ? 'default' : 'secondary'} className="capitalize">{role.name}</Badge>
                      <p className="text-sm text-muted-foreground mt-2 md:hidden">{role.description}</p>
                      <div className="flex flex-wrap gap-2 mt-2 md:hidden">
                        {role.permissions.map(permissionKey => (
                          <Badge key={permissionKey} variant="outline">{t(`permissions.${permissionKey}`)}</Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell className="hidden md:table-cell">{role.description}</TableCell>
                    <TableCell className="hidden md:table-cell">
                      <div className="flex flex-wrap gap-2">
                        {role.permissions.map(permissionKey => (
                          <Badge key={permissionKey} variant="outline">{t(`permissions.${permissionKey}`)}</Badge>
                        ))}
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
