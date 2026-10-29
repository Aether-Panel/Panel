'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useRef, useState } from 'react';
import { PageHeader } from '@/components/page-header';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Code, Bot, Server as ServerIcon, Database, Plus, LayoutTemplate, MoreHorizontal,
  Gamepad2, Globe, Cpu, Radio, Network, Box, Terminal, Shield, Play, Upload, FileJson, Trash2, Loader2
} from '@/components/icons';
import type { AppIcon } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { useTemplates } from '@/hooks/use-templates';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { sileo } from "@/lib/toast";
import { Textarea } from '@/components/ui/textarea';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { EmptyState } from '@/components/empty-state';
import { gameLogo } from '@/lib/game-logos';

// Optional icon mapping if we want to guess icon by category later, or just a default
function getIconForCategory(category?: string): AppIcon {
  if (!category) return LayoutTemplate;
  const c = category.toLowerCase();

  // Games & Servers
  if (c.includes('minecraft') || c.includes('forge') || c.includes('fabric') || c.includes('paper')) return Box;
  if (c.includes('game') || c.includes('steam') || c.includes('csgo') || c.includes('source') || c.includes('rust')) return Gamepad2;
  if (c.includes('proxy') || c.includes('bungee') || c.includes('velocity') || c.includes('waterfall')) return Network;
  if (c.includes('voice') || c.includes('teamspeak') || c.includes('mumble')) return Radio;

  // Dev & Web
  if (c.includes('web') || c.includes('node') || c.includes('react')) return Globe;
  if (c.includes('database') || c.includes('sql') || c.includes('redis')) return Database;
  if (c.includes('bot') || c.includes('discord')) return Bot;
  if (c.includes('software') || c.includes('app')) return Code;

  // System Tools
  if (c.includes('core') || c.includes('system')) return Cpu;
  if (c.includes('tool') || c.includes('script') || c.includes('utility')) return Terminal;
  if (c.includes('security') || c.includes('vpn')) return Shield;
  if (c.includes('media') || c.includes('video') || c.includes('music')) return Play;

  return ServerIcon; // default
}

const defaultTemplateJson = `{
  "name": "",
  "display": "",
  "type": "other",
  "install": [],
  "run": {
    "command": "",
    "stop": "",
    "workingDirectory": "",
    "environmentVars": {}
  }
}`;

function TemplateItem({
  template,
  canManage,
  onEdit,
  onDelete,
}: Readonly<{
  template: any;
  /** Local templates the user may edit get an actions menu (edit / delete). */
  canManage: boolean;
  onEdit?: () => void;
  onDelete?: () => void;
}>) {
  const { t } = useTranslations();
  const Icon = getIconForCategory(template.type);
  const logo = gameLogo(template);
  const categoryLabel = template.type || t('templates.categories.other');
  const title = template.display || template.name;

  return (
    <article className="flex flex-col gap-4 rounded-xl border border-border/60 bg-background/30 p-4">
      <div className="flex items-start gap-3">
        <span className="grid h-10 w-10 shrink-0 place-items-center overflow-hidden rounded-lg border border-border/60 bg-secondary text-muted-foreground" aria-hidden="true">
          {logo
            ? <img src={logo} alt="" width={40} height={40} loading="lazy" className="h-full w-full object-cover" />
            : <Icon className="h-5 w-5" />}
        </span>
        <div className="min-w-0 flex-1 pt-0.5">
          <h3 className="truncate text-sm font-semibold text-foreground">{title}</h3>
          <p className="mt-0.5 truncate text-xs text-muted-foreground">
            <span className="font-mono">{template.name}</span>
            {template.repoName && (
              <>
                <span aria-hidden="true"> · </span>
                {template.repoName}
              </>
            )}
          </p>
        </div>
        {canManage && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="-mr-1 -mt-1 h-9 w-9 shrink-0 text-muted-foreground hover:text-foreground"
                aria-label={t('templates.actions.openMenu', { name: title })}
              >
                <MoreHorizontal aria-hidden="true" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuLabel>{t('common.actions')}</DropdownMenuLabel>
              <DropdownMenuItem onClick={() => setTimeout(() => onEdit?.())}>{t('common.edit')}</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                className="text-destructive-bright focus:bg-destructive/10 focus:text-destructive-bright"
                onClick={() => setTimeout(() => onDelete?.(), 50)}
              >
                <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
                {t('common.delete')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
      <div className="mt-auto flex items-center justify-between gap-3">
        <Badge variant="secondary" className="min-w-0 max-w-[60%] font-mono text-[11px] font-normal" title={categoryLabel}>
          <span className="truncate">{categoryLabel}</span>
        </Badge>
        <Button
          variant="outline"
          size="sm"
          className="shrink-0"
          onClick={() => {
            const repoParam = template.repoId ? '&repo=' + template.repoId : '';
            window.location.href = `/servers/?create=true&template=${encodeURIComponent(template.name)}${repoParam}`;
          }}
        >
          {t('templates.use')}
        </Button>
      </div>
    </article>
  );
}

function TemplateGrid({ children }: Readonly<{ children: React.ReactNode }>) {
  return <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">{children}</div>;
}

function EmptyTemplates({ title, description, action }: Readonly<{ title: string; description: string; action?: React.ReactNode }>) {
  return <EmptyState icon={LayoutTemplate} title={title} description={description} action={action} />;
}

/** One tab's body: the skeleton grid while loading, the empty state when there is nothing, the grid otherwise. */
function TemplateTabBody({ loading, placeholder, isEmpty, empty, children }: Readonly<{
  loading: boolean;
  placeholder: React.ReactNode;
  isEmpty: boolean;
  empty: React.ReactNode;
  children: React.ReactNode;
}>) {
  if (loading) return <>{placeholder}</>;
  if (isEmpty) return <>{empty}</>;
  return <TemplateGrid>{children}</TemplateGrid>;
}

function TemplatesSkeleton() {
  return (
    <div className="flex flex-col gap-8" aria-busy="true">
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div className="space-y-2">
          <Skeleton className="h-9 w-64" />
          <Skeleton className="h-4 w-80 max-w-full" />
        </div>
        <Skeleton className="h-10 w-48 rounded-lg" />
      </div>
      <Card className="overflow-hidden">
        <div className="space-y-2 border-b border-border/50 p-5 md:p-6">
          <Skeleton className="h-5 w-48" />
          <Skeleton className="h-4 w-64 max-w-full" />
        </div>
        <div className="p-5 md:p-6">
          <TemplateGrid>
            {Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-[7.5rem] rounded-xl" />)}
          </TemplateGrid>
        </div>
      </Card>
    </div>
  );
}

export default function TemplatesPage() {
  const { role, hasScope } = useAuth();

  const [isMounted, setIsMounted] = useState(false);

  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [templateJson, setTemplateJson] = useState(defaultTemplateJson);
  const [templateObj, setTemplateObj] = useState<any>(JSON.parse(defaultTemplateJson));
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [activeTab, setActiveTab] = useState('local');
  const [deletingTemplate, setDeletingTemplate] = useState<any>(null);
  // Name of the local template open in the editor; null while creating a new one.
  const [editingName, setEditingName] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const loadFileRef = useRef<HTMLInputElement>(null);

  const updateField = (field: string, value: string) => {
    const newObj = { ...templateObj, [field]: value };
    setTemplateObj(newObj);
    setTemplateJson(JSON.stringify(newObj, null, 2));
  };

  const updateRunField = (field: string, value: string) => {
    const newObj = { ...templateObj, run: { ...templateObj.run, [field]: value } };
    setTemplateObj(newObj);
    setTemplateJson(JSON.stringify(newObj, null, 2));
  };

  const handleJsonChange = (val: string) => {
    setTemplateJson(val);
    try {
      setTemplateObj(JSON.parse(val));
    } catch {
      // Half-typed JSON is expected while editing; the form keeps the last valid object.
    }
  };

  const { t } = useTranslations();

  const { repos, loading: reposLoading, getTemplatesForRepo, getTemplateDetails, saveTemplate, uploadTemplates, deleteTemplate } = useTemplates();
  const [allTemplates, setAllTemplates] = useState<any[]>([]);
  const [loadingTemplates, setLoadingTemplates] = useState(false);

  const loadAllTemplates = async () => {
    setLoadingTemplates(true);
    try {
      const loaded: any[] = [];
      // Each repo is its own request (and never throws), so they can load side by side;
      // Promise.all keeps the results in repo order.
      const perRepo = await Promise.all(repos.map((repo) => getTemplatesForRepo(repo.id)));
      perRepo.forEach((tpls, index) => {
        const repo = repos[index];
        tpls.forEach((t: any) => {
          loaded.push({
            ...t,
            repoId: repo.id,
            repoName: repo.name
          });
        });
      });

      const localTpls = await getTemplatesForRepo(0);
      localTpls.forEach((t: any) => {
        loaded.push({ ...t, repoId: 0, repoName: 'Local' });
      });

      const uniqueLoaded = loaded.filter((v, i, a) => a.findIndex(t => (t.name === v.name && t.repoId === v.repoId)) === i);

      setAllTemplates(uniqueLoaded);
    } finally {
      setLoadingTemplates(false);
    }
  };

  const handleUpload = async (files: FileList | null) => {
    if (!files || files.length === 0) return;
    setUploading(true);
    try {
      const uploaded = await uploadTemplates(Array.from(files));
      sileo.success({ title: t('common.success'), description: t('templates.toast.uploaded', { count: uploaded.length }) });
      await loadAllTemplates();
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e?.message || t('templates.toast.uploadFailed') });
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const handleLoadFromFile = async (files: FileList | null) => {
    if (!files || files.length === 0) return;
    const file = files[0];
    try {
      const text = await file.text();
      const parsed = JSON.parse(text);
      if (!parsed.name) {
        parsed.name = (file.name || '').replace(/\.json$/i, '');
      }
      setTemplateJson(JSON.stringify(parsed, null, 2));
      setTemplateObj(parsed);
    } catch (e) {
      console.error('Failed to read template file:', e);
      sileo.error({ title: t('common.error'), description: t('templates.toast.invalidJson') });
    } finally {
      if (loadFileRef.current) loadFileRef.current.value = '';
    }
  };

  useEffect(() => {
    setIsMounted(true);
  }, []);

  useEffect(() => {
    if (isMounted && role && !hasScope('templates.view')) {
      window.location.href = '/dashboard/';
    }
  }, [role, hasScope, isMounted]);

  useEffect(() => {
    if (!hasScope('templates.view')) return;

    if (repos && repos.length > 0) {
      void loadAllTemplates();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [repos, hasScope]);

  const handleDeleteTemplate = async (template: any) => {
    try {
      await deleteTemplate(template.name);
      sileo.success({ title: t('common.success'), description: t('templates.toast.deleted', { name: template.name }) });
      await loadAllTemplates();
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e?.message || t('templates.toast.deleteFailed') });
    }
  };

  const openCreate = () => {
    // Coming from an edit: start the new template from a clean slate.
    if (editingName !== null) {
      setEditingName(null);
      setTemplateJson(defaultTemplateJson);
      setTemplateObj(JSON.parse(defaultTemplateJson));
    }
    setIsCreateOpen(true);
  };

  const openEdit = async (template: any) => {
    const details = await getTemplateDetails(0, template.name);
    if (!details) {
      sileo.error({ title: t('common.error'), description: t('templates.toast.loadFailed') });
      return;
    }
    setEditingName(template.name);
    setTemplateJson(JSON.stringify(details, null, 2));
    setTemplateObj(details);
    setIsCreateOpen(true);
  };

  if (!isMounted || !hasScope('templates.view') || reposLoading) {
    return <TemplatesSkeleton />;
  }

  const localTemplates = allTemplates.filter(t => t.repoId === 0);
  const communityTemplates = allTemplates.filter(t => t.repoId !== 0);
  const canEditLocal = hasScope('templates.local.edit');
  const canCreate = hasScope('templates.repo.create');

  const loadingGrid = (
    <TemplateGrid>
      {Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-[7.5rem] rounded-xl" />)}
    </TemplateGrid>
  );

  const countPill = (count: number) => (
    <span className="ml-1.5 rounded bg-secondary px-1.5 text-[11px] tabular-nums text-muted-foreground">{count}</span>
  );

  return (
    <div className="stagger flex flex-col gap-8">
      <PageHeader title={t('templates.title')} description={t('templates.description')}>
        {canEditLocal && (
          <>
            <Button variant="outline" onClick={() => fileInputRef.current?.click()} disabled={uploading}>
              {uploading ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Upload aria-hidden="true" />}
              {uploading ? t('templates.uploading') : t('templates.upload')}
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              accept=".json,application/json"
              multiple
              className="hidden"
              onChange={(e) => handleUpload(e.target.files)}
            />
          </>
        )}
        {canCreate && (
          <Button onClick={openCreate}>
            <Plus aria-hidden="true" />
            {t('templates.create')}
          </Button>
        )}
        {(canCreate || canEditLocal) && (
          <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
            <DialogContent className="flex h-[min(90svh,48rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-4xl">
              <DialogHeader className="border-b border-border/60 p-6 pb-5 text-left">
                <DialogTitle className="text-xl font-semibold tracking-[-0.02em]">
                  {editingName !== null ? t('templates.editDialog.title') : t('templates.create')}
                </DialogTitle>
                <DialogDescription>{t('templates.createDialog.description')}</DialogDescription>
              </DialogHeader>
              <div className="flex min-h-0 flex-1 flex-col p-6">
                <Tabs defaultValue="general" className="flex min-h-0 flex-1 flex-col">
                  <TabsList className="mb-5 self-start">
                    <TabsTrigger value="general">{t('templates.General')}</TabsTrigger>
                    <TabsTrigger value="run">{t('templates.RunConfig')}</TabsTrigger>
                    <TabsTrigger value="json">{t('templates.Json')}</TabsTrigger>
                  </TabsList>

                  <TabsContent value="general" className="custom-scrollbar mt-0 min-h-0 flex-1 space-y-4 overflow-y-auto">
                    <div className="space-y-2">
                      <Label htmlFor="tpl-name">{t('templates.fields.name')}</Label>
                      <Input id="tpl-name" className="font-mono" value={templateObj?.name || ''} onChange={(e) => updateField('name', e.target.value)} placeholder={t('templates.fields.namePlaceholder')} />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="tpl-display">{t('templates.fields.display')}</Label>
                      <Input id="tpl-display" value={templateObj?.display || ''} onChange={(e) => updateField('display', e.target.value)} placeholder={t('templates.fields.displayPlaceholder')} />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="tpl-type">{t('templates.fields.type')}</Label>
                      <Input id="tpl-type" className="font-mono" value={templateObj?.type || ''} onChange={(e) => updateField('type', e.target.value)} placeholder={t('templates.fields.typePlaceholder')} />
                    </div>
                  </TabsContent>

                  <TabsContent value="run" className="custom-scrollbar mt-0 min-h-0 flex-1 space-y-4 overflow-y-auto">
                    <div className="space-y-2">
                      <Label htmlFor="tpl-command">{t('templates.fields.startCommand')}</Label>
                      <Input id="tpl-command" className="font-mono" value={templateObj?.run?.command || ''} onChange={(e) => updateRunField('command', e.target.value)} placeholder={t('templates.fields.startCommandPlaceholder')} />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="tpl-stop">{t('templates.fields.stopCommand')}</Label>
                      <Input id="tpl-stop" className="font-mono" value={templateObj?.run?.stop || ''} onChange={(e) => updateRunField('stop', e.target.value)} placeholder={t('templates.fields.stopCommandPlaceholder')} />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="tpl-workdir">{t('templates.fields.workingDirectory')}</Label>
                      <Input id="tpl-workdir" className="font-mono" value={templateObj?.run?.workingDirectory || ''} onChange={(e) => updateRunField('workingDirectory', e.target.value)} placeholder="" />
                    </div>
                  </TabsContent>

                  <TabsContent value="json" className="mt-0 flex min-h-0 flex-1 flex-col data-[state=inactive]:hidden">
                    <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                      <Label htmlFor="tpl-json">{t('templates.fields.json')}</Label>
                      <Button variant="outline" size="sm" type="button" onClick={() => loadFileRef.current?.click()}>
                        <FileJson aria-hidden="true" />
                        {t('templates.fields.loadFromFile')}
                      </Button>
                      <input
                        ref={loadFileRef}
                        type="file"
                        accept=".json,application/json"
                        className="hidden"
                        onChange={(e) => handleLoadFromFile(e.target.files)}
                      />
                    </div>
                    <Textarea
                      id="tpl-json"
                      className="custom-scrollbar min-h-[16rem] w-full flex-1 resize-none font-mono text-xs leading-relaxed"
                      value={templateJson}
                      onChange={(e) => handleJsonChange(e.target.value)}
                      placeholder={t('templates.fields.jsonPlaceholder')}
                      spellCheck={false}
                    />
                  </TabsContent>
                </Tabs>
              </div>
              <DialogFooter className="gap-2 border-t border-border/60 px-6 py-4 sm:gap-0">
                <Button variant="outline" onClick={() => setIsCreateOpen(false)}>
                  {t('common.cancel')}
                </Button>
                <Button disabled={saving} onClick={async () => {
                  try {
                    setSaving(true);
                    const parsed = JSON.parse(templateJson);
                    if (!parsed.name) throw new Error(t('templates.toast.nameRequired'));
                    await saveTemplate(parsed.name, parsed);
                    sileo.success({ title: t('common.success'), description: editingName !== null ? t('templates.toast.updated') : t('templates.toast.created') });
                    setIsCreateOpen(false);
                    setTemplateJson(defaultTemplateJson);
                    if (editingName !== null) {
                      setEditingName(null);
                      setTemplateObj(JSON.parse(defaultTemplateJson));
                    }
                    // Refresh
                    await loadAllTemplates();
                  } catch (e: any) {
                    sileo.error({ title: t('common.error'), description: e.message || t('templates.toast.saveFailed') });
                  } finally {
                    setSaving(false);
                  }
                }}>
                  {saving && <Loader2 className="animate-spin" aria-hidden="true" />}
                  {saving ? t('common.saving') : t('templates.save')}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        )}
      </PageHeader>

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <Card className="overflow-hidden">
          <div className="flex flex-col gap-4 border-b border-border/50 p-5 sm:flex-row sm:items-center sm:justify-between md:p-6">
            <div className="min-w-0 space-y-1">
              <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('templates.listTitle')}</h2>
              <p className="text-sm text-muted-foreground">
                {activeTab === 'local' ? t('templates.localDescription') : t('templates.communityDescription')}
              </p>
            </div>
            <TabsList className="w-full shrink-0 sm:w-auto">
              <TabsTrigger value="local" className="flex-1 sm:flex-none">{t('templates.tabs.local')}{!loadingTemplates && countPill(localTemplates.length)}</TabsTrigger>
              <TabsTrigger value="community" className="flex-1 sm:flex-none">{t('templates.tabs.community')}{!loadingTemplates && countPill(communityTemplates.length)}</TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value="local" className="mt-0 p-5 md:p-6">
            <TemplateTabBody
              loading={loadingTemplates}
              placeholder={loadingGrid}
              isEmpty={localTemplates.length === 0}
              empty={
                <EmptyTemplates
                  title={t('templates.empty.localTitle')}
                  description={t('templates.empty.localDescription')}
                  action={canCreate ? (
                    <Button onClick={openCreate}>
                      <Plus aria-hidden="true" />
                      {t('templates.create')}
                    </Button>
                  ) : undefined}
                />
              }
            >
              {localTemplates.map((template, idx) => (
                <TemplateItem
                  key={`local-${template.name}-${idx}`}
                  template={template}
                  canManage={canEditLocal}
                  onEdit={() => void openEdit(template)}
                  onDelete={() => setDeletingTemplate(template)}
                />
              ))}
            </TemplateTabBody>
          </TabsContent>

          <TabsContent value="community" className="mt-0 p-5 md:p-6">
            <TemplateTabBody
              loading={loadingTemplates}
              placeholder={loadingGrid}
              isEmpty={communityTemplates.length === 0}
              empty={
                <EmptyTemplates
                  title={t('templates.empty.communityTitle')}
                  description={t('templates.empty.communityDescription')}
                />
              }
            >
              {communityTemplates.map((template, idx) => (
                <TemplateItem
                  key={`com-${template.repoId}-${template.name}-${idx}`}
                  template={template}
                  canManage={false}
                />
              ))}
            </TemplateTabBody>
          </TabsContent>
        </Card>
      </Tabs>

      <AlertDialog open={!!deletingTemplate} onOpenChange={(open) => !open && setDeletingTemplate(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('templates.deleteDialog.title')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('templates.deleteDialog.description', { name: deletingTemplate?.display || deletingTemplate?.name || '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => { if (deletingTemplate) void handleDeleteTemplate(deletingTemplate); }}
            >
              {t('common.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
