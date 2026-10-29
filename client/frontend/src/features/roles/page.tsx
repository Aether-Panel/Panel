'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useState, useMemo, useCallback, useRef, useId } from 'react';
import { PageHeader } from '@/components/page-header';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Checkbox } from '@/components/ui/checkbox';
import { Skeleton } from '@/components/ui/skeleton';
import { Check, ChevronDown, Loader2, Lock, Minus, Pencil, Plus, Search, ShieldCheck, Trash2 } from '@/components/icons';
import { Textarea } from '@/components/ui/textarea';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from "@/lib/toast";
import { cn } from '@/lib/utils';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';

type Role = {
  id?: number;
  name: string;
  description: string;
  scopes: string[];
};

type RoleDraft = Pick<Role, 'name' | 'description' | 'scopes'>;

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
    keys: ["server.console", "server.console.send"]
  },
  {
    label: "roles.permissionGroups.serverManagement",
    keys: ["server.start", "server.stop", "server.kill", "server.reload", "server.install",
           "server.definition.view", "server.definition.edit",
           "server.data.view", "server.data.edit", "server.data.edit.admin", "server.ports.edit",
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
  // these scopes are also the prefix of others, so their own label lives under another key
  'server.console': 'server.consoleView',
  'server.data.edit': 'server.dataEdit',
  'server.ports.edit': 'server.portsEdit',
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

/** The two roles the backend seeds (`internal/database/upgrade.go`) can't be edited or deleted. */
const isProtectedRole = (role: Role) =>
  role.name === 'admin' || role.name === 'Administrador' || role.name === 'Usuario';

/** The `admin` super-scope grants everything, so the effective set is every known permission. */
const isFullAccess = (role: Role) => (role.scopes || []).includes('admin');

const effectiveScopes = (role: Role): Set<string> =>
  new Set(isFullAccess(role) ? allPermissionKeys : (role.scopes || []));

/** Group checkbox: off when nothing is picked, on when everything is, indeterminate in between. */
function groupCheckState(count: number, total: number): boolean | 'indeterminate' {
  if (count === 0) return false;
  if (count === total) return true;
  return 'indeterminate';
}

function usePermissionLabel() {
  const { t } = useTranslations();
  return useCallback((permissionKey: string): string => {
    const mapped = translationKeyOverride[permissionKey] || permissionKey;
    return t(`permissions.${mapped}`);
  }, [t]);
}

/** Thin coverage meter: how much of the permission catalogue a role holds. */
function CoverageBar({ value, className }: Readonly<{ value: number; className?: string }>) {
  return (
    <div className={cn('h-1 w-full overflow-hidden rounded-full bg-secondary', className)}>
      <div
        className="h-full rounded-full bg-primary transition-[width] duration-500 ease-panel"
        style={{ width: `${Math.round(Math.min(1, Math.max(0, value)) * 100)}%` }}
      />
    </div>
  );
}

/**
 * Create and edit share one editor: name and description on top, then the
 * permission catalogue grouped by area. Each group has a tri-state checkbox
 * (all / some / none) and every permission shows its readable name with the
 * scope key underneath, so operators who know the keys can still find them.
 */
function RoleEditorDialog({
  open,
  onOpenChange,
  mode,
  initial,
  onSubmit,
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mode: 'create' | 'edit';
  initial: RoleDraft;
  onSubmit: (draft: RoleDraft) => Promise<boolean>;
}>) {
  const { t } = useTranslations();
  const getPermissionLabel = usePermissionLabel();
  const [name, setName] = useState(initial.name);
  const [description, setDescription] = useState(initial.description);
  const [scopes, setScopes] = useState<string[]>(initial.scopes);
  const [query, setQuery] = useState('');
  const [saving, setSaving] = useState(false);
  const [triedSubmit, setTriedSubmit] = useState(false);

  // Each opening starts from the role being edited (or a blank role).
  useEffect(() => {
    if (!open) return;
    setName(initial.name);
    setDescription(initial.description);
    setScopes(initial.scopes);
    setQuery('');
    setTriedSubmit(false);
  }, [open, initial]);

  const selected = useMemo(() => new Set(scopes), [scopes]);

  const filteredGroups = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return permissionGroups;
    return permissionGroups
      .map(group => ({
        ...group,
        keys: group.keys.filter(k => getPermissionLabel(k).toLowerCase().includes(q) || k.toLowerCase().includes(q)),
      }))
      .filter(group => group.keys.length > 0);
  }, [query, getPermissionLabel]);

  const toggle = useCallback((key: string, checked: boolean) => {
    setScopes(prev => (checked ? [...new Set([...prev, key])] : prev.filter(p => p !== key)));
  }, []);

  const toggleMany = useCallback((keys: string[], select: boolean) => {
    setScopes(prev => {
      if (select) return [...new Set([...prev, ...keys])];
      return prev.filter(p => !keys.includes(p));
    });
  }, []);

  const visibleKeys = filteredGroups.flatMap(g => g.keys);
  const allVisibleSelected = visibleKeys.length > 0 && visibleKeys.every(k => selected.has(k));
  const knownSelected = allPermissionKeys.filter(k => selected.has(k)).length;

  const submit = async () => {
    setTriedSubmit(true);
    if (!name.trim()) return;
    setSaving(true);
    const ok = await onSubmit({ name: name.trim(), description, scopes });
    setSaving(false);
    if (ok) onOpenChange(false);
  };

  const nameMissing = triedSubmit && !name.trim();

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!saving) onOpenChange(next); }}>
      <DialogContent className="flex max-h-[min(90svh,52rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-3xl">
        <DialogHeader className="border-b border-border/60 p-6 pb-5 text-left">
          <DialogTitle className="text-xl font-semibold tracking-[-0.02em]">
            {mode === 'create' ? t('roles.addDialog.title') : t('roles.editDialog.title')}
          </DialogTitle>
          <DialogDescription>
            {mode === 'create' ? t('roles.addDialog.description') : t('roles.editDialog.description')}
          </DialogDescription>
        </DialogHeader>

        <div className="custom-scrollbar min-h-0 flex-1 space-y-6 overflow-y-auto p-6">
          <div className="grid gap-4 sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
            <div className="space-y-2">
              <Label htmlFor="role-name">{t('roles.addDialog.nameLabel')}</Label>
              <Input
                id="role-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t('roles.addDialog.namePlaceholder')}
                aria-invalid={nameMissing}
                className={cn(nameMissing && 'border-destructive focus-visible:ring-destructive/40')}
                autoFocus
              />
              {nameMissing && <p className="text-xs text-destructive-bright">{t('roles.nameRequired')}</p>}
            </div>
            <div className="space-y-2">
              <Label htmlFor="role-description">{t('roles.addDialog.descriptionLabel')}</Label>
              <Textarea
                id="role-description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder={t('roles.addDialog.descriptionPlaceholder')}
                className="min-h-[2.5rem] resize-none"
                rows={2}
              />
            </div>
          </div>

          <div className="space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <h3 className="text-sm font-semibold">{t('roles.addDialog.permissionsLabel')}</h3>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-8 px-2.5 text-xs text-muted-foreground hover:text-foreground"
                onClick={() => toggleMany(visibleKeys, !allVisibleSelected)}
                disabled={visibleKeys.length === 0}
              >
                {allVisibleSelected ? t('roles.deselectAll') : t('roles.selectAll')}
              </Button>
            </div>

            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
              <Input
                type="search"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t('roles.searchPermissions')}
                aria-label={t('roles.searchPermissions')}
                className="pl-9"
              />
            </div>

            {filteredGroups.length === 0 ? (
              <p className="rounded-xl border border-dashed border-border/60 px-4 py-8 text-center text-sm text-muted-foreground">
                {t('roles.noMatches', { query: query.trim() })}
              </p>
            ) : (
              <div className="space-y-3">
                {filteredGroups.map((group) => {
                  const count = group.keys.filter(k => selected.has(k)).length;
                  const state = groupCheckState(count, group.keys.length);
                  const groupId = `group-${group.label}`;
                  return (
                    <section key={group.label} aria-labelledby={groupId} className="rounded-xl border border-border/60 bg-background/30">
                      <div className="flex items-center gap-3 px-4 py-3">
                        <Checkbox
                          checked={state}
                          onCheckedChange={() => toggleMany(group.keys, state !== true)}
                          aria-labelledby={groupId}
                        />
                        <span id={groupId} className="text-sm font-semibold">{t(group.label)}</span>
                        <span className="ml-auto text-xs tabular-nums text-muted-foreground">
                          {count}/{group.keys.length}
                        </span>
                      </div>
                      <div className="grid gap-0.5 border-t border-border/50 p-1.5 sm:grid-cols-2">
                        {group.keys.map(permissionKey => {
                          const id = `perm-${permissionKey}`;
                          return (
                            <label
                              key={permissionKey}
                              htmlFor={id}
                              className="flex cursor-pointer items-start gap-3 rounded-lg px-2.5 py-2 transition-colors duration-150 hover:bg-accent/50"
                            >
                              <Checkbox
                                id={id}
                                className="mt-0.5"
                                checked={selected.has(permissionKey)}
                                onCheckedChange={(checked) => toggle(permissionKey, !!checked)}
                              />
                              <span className="min-w-0">
                                <span className="block text-sm leading-snug text-foreground">{getPermissionLabel(permissionKey)}</span>
                                <span className="block truncate font-mono text-[11px] text-muted-foreground">{permissionKey}</span>
                              </span>
                            </label>
                          );
                        })}
                      </div>
                    </section>
                  );
                })}
              </div>
            )}
          </div>
        </div>

        <DialogFooter className="flex-row items-center justify-between gap-3 border-t border-border/60 px-6 py-4 sm:justify-between">
          <span className="text-xs tabular-nums text-muted-foreground">
            {t('roles.selectedCount', { count: knownSelected, total: allPermissionKeys.length })}
          </span>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
              {t('roles.addDialog.cancel')}
            </Button>
            <Button onClick={submit} disabled={saving}>
              {saving && <Loader2 className="animate-spin" />}
              {mode === 'create' ? t('roles.addDialog.create') : t('roles.editDialog.save')}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** One role in the left-hand list: name, what it is for and how much it can do. */
function RoleListItem({ role, active, onSelect }: Readonly<{ role: Role; active: boolean; onSelect: () => void }>) {
  const { t } = useTranslations();
  const granted = effectiveScopes(role).size;
  const full = isFullAccess(role);

  return (
    <button
      type="button"
      onClick={onSelect}
      aria-current={active ? 'true' : undefined}
      className={cn(
        'group relative flex w-full flex-col gap-3 rounded-xl border px-4 py-3.5 text-left transition-colors duration-200 ease-panel focus-ring',
        active
          ? 'border-primary/45 bg-primary/[0.07]'
          : 'border-border/60 bg-card hover:border-border hover:bg-accent/40',
      )}
    >
      <span className="flex w-full items-start gap-3">
        <span
          aria-hidden="true"
          className={cn(
            'grid h-9 w-9 shrink-0 place-items-center rounded-lg border text-sm font-semibold',
            full ? 'border-primary/30 bg-primary/10 text-primary-bright' : 'border-border/60 bg-secondary text-foreground',
          )}
        >
          {full ? <ShieldCheck className="h-4 w-4" /> : role.name.trim().charAt(0).toUpperCase()}
        </span>
        <span className="min-w-0 flex-1">
          <span className="flex items-center gap-2">
            <span className="truncate text-sm font-semibold text-foreground">{role.name}</span>
            {isProtectedRole(role) && (
              <Lock className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-label={t('roles.protected')} />
            )}
          </span>
          <span className="mt-0.5 line-clamp-1 text-xs text-muted-foreground">
            {role.description || t('roles.noDescription')}
          </span>
        </span>
      </span>
      <span className="flex w-full items-center gap-3">
        <CoverageBar value={granted / allPermissionKeys.length} className="flex-1" />
        <span className="shrink-0 text-[11px] tabular-nums text-muted-foreground">
          {full ? t('roles.fullAccess') : `${granted}/${allPermissionKeys.length}`}
        </span>
      </span>
    </button>
  );
}

/**
 * One permission area in the role detail. On phones it collapses to a single
 * row (name, count, meter) that opens on tap, so a role doesn't become a wall
 * of seventy lines; from `lg` up every area is open.
 */
function PermissionGroupView({
  label,
  keys,
  granted,
  getPermissionLabel,
}: Readonly<{
  label: string;
  keys: string[];
  granted: Set<string>;
  getPermissionLabel: (key: string) => string;
}>) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const count = keys.filter(k => granted.has(k)).length;
  const listId = useId();

  return (
    <section className="rounded-xl border border-border/60 bg-background/30">
      <button
        type="button"
        onClick={() => setOpen(o => !o)}
        aria-expanded={open}
        aria-controls={listId}
        className="flex w-full items-center gap-3 rounded-xl px-4 py-3 text-left focus-ring lg:pointer-events-none lg:pb-0"
      >
        <h3 className="text-sm font-semibold text-foreground">{label}</h3>
        <span className={cn('ml-auto text-xs tabular-nums', count > 0 ? 'text-foreground' : 'text-muted-foreground')}>
          {count}/{keys.length}
        </span>
        <ChevronDown
          aria-hidden="true"
          className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200 ease-panel lg:hidden', open && 'rotate-180')}
        />
      </button>
      <ul id={listId} className={cn('p-2 pt-0 lg:block lg:pt-1.5', open ? 'block' : 'hidden')}>
        {keys.map((permissionKey) => {
          const has = granted.has(permissionKey);
          return (
            <li key={permissionKey} className="flex items-center gap-2.5 rounded-md px-2 py-1.5 text-sm">
              {has ? (
                <Check className="h-3.5 w-3.5 shrink-0 text-success" aria-label={t('roles.granted')} />
              ) : (
                <Minus className="h-3.5 w-3.5 shrink-0 text-muted-foreground/50" aria-label={t('roles.notGranted')} />
              )}
              <span className={cn('min-w-0 truncate', has ? 'text-foreground' : 'text-muted-foreground/70')} title={permissionKey}>
                {getPermissionLabel(permissionKey)}
              </span>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

/** The selected role: header with actions, then every permission area and what this role holds in it. */
function RoleDetail({
  role,
  onEdit,
  onDelete,
}: Readonly<{
  role: Role;
  onEdit: () => void;
  onDelete: () => void;
}>) {
  const { t } = useTranslations();
  const getPermissionLabel = usePermissionLabel();
  const granted = effectiveScopes(role);
  const full = isFullAccess(role);
  const locked = isProtectedRole(role);

  return (
    <Card className="overflow-hidden">
      <div className="flex flex-col gap-4 border-b border-border/60 p-5 sm:flex-row sm:items-start sm:justify-between md:p-6">
        <div className="min-w-0 space-y-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-xl font-semibold tracking-[-0.02em] text-foreground">{role.name}</h2>
            {locked && (
              <span className="inline-flex items-center gap-1 rounded-md border border-border/60 bg-secondary px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">
                <Lock className="h-3 w-3" aria-hidden="true" />
                {t('roles.protected')}
              </span>
            )}
          </div>
          <p className="max-w-[62ch] text-sm text-muted-foreground">{role.description || t('roles.noDescription')}</p>
        </div>

        {locked ? (
          <p className="max-w-[40ch] text-xs text-muted-foreground sm:text-right">{t('roles.protectedNote')}</p>
        ) : (
          <div className="flex shrink-0 gap-2">
            <Button variant="outline" size="sm" onClick={onEdit}>
              <Pencil aria-hidden="true" />
              {t('roles.edit')}
            </Button>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="outline" size="sm" className="text-destructive-bright hover:border-destructive/50 hover:bg-destructive/10 hover:text-destructive-bright">
                  <Trash2 aria-hidden="true" />
                  {t('roles.delete')}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t('roles.deleteDialog.title')}</AlertDialogTitle>
                  <AlertDialogDescription>{t('roles.deleteDialog.description')}</AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={onDelete}
                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                  >
                    {t('common.delete')}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        )}
      </div>

      <div className="space-y-5 p-5 md:p-6">
        <div className="space-y-2">
          <div className="flex items-baseline justify-between gap-3 text-sm">
            <span className="font-medium text-foreground">
              {full ? t('roles.fullAccess') : t('roles.permissionsCount', { count: granted.size, total: allPermissionKeys.length })}
            </span>
          </div>
          <CoverageBar value={granted.size / allPermissionKeys.length} className="h-1.5" />
          {full && <p className="text-xs text-muted-foreground">{t('roles.adminNote')}</p>}
        </div>

        {granted.size === 0 ? (
          <p className="rounded-xl border border-dashed border-border/60 px-4 py-8 text-center text-sm text-muted-foreground">
            {t('roles.noPermissions')}
          </p>
        ) : (
          <div className="gap-3 lg:columns-2 [&>*]:mb-3 [&>*]:break-inside-avoid">
            {permissionGroups.map((group) => (
              <PermissionGroupView
                key={group.label}
                label={t(group.label)}
                keys={group.keys}
                granted={granted}
                getPermissionLabel={getPermissionLabel}
              />
            ))}
          </div>
        )}
      </div>
    </Card>
  );
}

function RolesSkeleton() {
  return (
    <div className="flex flex-col gap-8" aria-busy="true">
      <div className="space-y-2">
        <Skeleton className="h-9 w-64" />
        <Skeleton className="h-4 w-96 max-w-full" />
      </div>
      <div className="grid gap-6 xl:grid-cols-[minmax(0,20rem)_minmax(0,1fr)]">
        <div className="space-y-2">
          {Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-[5.25rem] rounded-xl" />)}
        </div>
        <Skeleton className="h-[32rem] rounded-xl" />
      </div>
    </div>
  );
}

const EMPTY_DRAFT: RoleDraft = { name: '', description: '', scopes: [] };

export default function RolesPage() {
  const { role, hasScope } = useAuth();
  const [isMounted, setIsMounted] = useState(false);
  const { t } = useTranslations();

  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedId, setSelectedId] = useState<number | undefined>(undefined);
  // The editor keeps its last mode while it closes, so the title doesn't flip mid-animation.
  const [editor, setEditor] = useState<{ mode: 'create' | 'edit'; role?: Role }>({ mode: 'create' });
  const [editorOpen, setEditorOpen] = useState(false);
  const detailRef = useRef<HTMLDivElement>(null);

  const fetchRoles = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.get('/api/roles');
      setRoles(data || []);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('roles.toast.loadFailed') });
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    setIsMounted(true);
    if (role && !hasScope('admin')) {
      window.location.href = '/dashboard';
    }
    void fetchRoles();
  }, [role, hasScope, fetchRoles]);

  // Keep a valid selection: the first role by default, and a neighbour after a delete.
  useEffect(() => {
    if (roles.length === 0) return;
    if (!roles.some(r => r.id === selectedId)) setSelectedId(roles[0].id);
  }, [roles, selectedId]);

  const selectedRole = roles.find(r => r.id === selectedId) ?? roles[0];

  const selectRole = (id?: number) => {
    setSelectedId(id);
    // On narrow screens the detail sits below the list: bring it into view.
    if (window.matchMedia('(max-width: 1279px)').matches) {
      requestAnimationFrame(() => detailRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }));
    }
  };

  const handleCreate = useCallback(async (draft: RoleDraft) => {
    try {
      await api.post('/api/roles', draft);
      sileo.success({ title: t('common.success'), description: t('roles.toast.created') });
      void fetchRoles();
      return true;
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('roles.toast.createFailed') });
      return false;
    }
  }, [t, fetchRoles]);

  const handleUpdate = useCallback(async (id: number | undefined, draft: RoleDraft) => {
    try {
      await api.post(`/api/roles/${id}`, draft);
      sileo.success({ title: t('common.success'), description: t('roles.toast.updated') });
      void fetchRoles();
      return true;
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('roles.toast.updateFailed') });
      return false;
    }
  }, [t, fetchRoles]);

  const handleDeleteRole = useCallback(async (id: number) => {
    try {
      await api.delete(`/api/roles/${id}`);
      sileo.success({ title: t('common.success'), description: t('roles.toast.deleted') });
      void fetchRoles();
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('roles.toast.deleteFailed') });
    }
  }, [t, fetchRoles]);

  const editorInitial = useMemo<RoleDraft>(() => {
    if (editor.mode !== 'edit' || !editor.role) return EMPTY_DRAFT;
    return { name: editor.role.name, description: editor.role.description || '', scopes: editor.role.scopes || [] };
  }, [editor]);

  if (!isMounted || !hasScope('admin') || (loading && roles.length === 0)) {
    return <RolesSkeleton />;
  }

  return (
    <div className="stagger flex flex-col gap-8">
      <PageHeader title={t('roles.title')} description={t('roles.description')}>
        <Button onClick={() => { setEditor({ mode: 'create' }); setEditorOpen(true); }}>
          <Plus aria-hidden="true" />
          {t('roles.addRole')}
        </Button>
      </PageHeader>

      {roles.length === 0 ? (
        <Card className="flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
          <ShieldCheck className="h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
          <p className="text-sm text-muted-foreground">{t('roles.empty')}</p>
        </Card>
      ) : (
        <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,20rem)_minmax(0,1fr)]">
          <nav aria-label={t('roles.list')} className="grid gap-2 md:grid-cols-2 xl:sticky xl:top-24 xl:grid-cols-1">
            {roles.map(r => (
              <RoleListItem
                key={r.id ?? r.name}
                role={r}
                active={r.id === selectedRole?.id}
                onSelect={() => selectRole(r.id)}
              />
            ))}
          </nav>

          <div ref={detailRef} className="min-w-0 scroll-mt-24">
            {selectedRole && (
              <RoleDetail
                key={selectedRole.id}
                role={selectedRole}
                onEdit={() => { setEditor({ mode: 'edit', role: selectedRole }); setEditorOpen(true); }}
                onDelete={() => selectedRole.id && handleDeleteRole(selectedRole.id)}
              />
            )}
          </div>
        </div>
      )}

      <RoleEditorDialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        mode={editor.mode}
        initial={editorInitial}
        onSubmit={(draft) => (editor.mode === 'edit' ? handleUpdate(editor.role?.id, draft) : handleCreate(draft))}
      />
    </div>
  );
}
