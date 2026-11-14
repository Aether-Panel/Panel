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
  Gamepad2, Globe, Cpu, Radio, Network, Box, Terminal, Shield, Play, Upload, Trash2, Loader2, Search, Pencil, Copy
} from '@/components/icons';
import { matchesTemplate } from '@/lib/template-search';
import type { AppIcon } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { useTemplates } from '@/hooks/use-templates';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { sileo } from "@/lib/toast";
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Input } from '@/components/ui/input';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { EmptyState } from '@/components/empty-state';
import { templateLogo } from '@/lib/template-logos';
import { TemplateLogo } from '@/components/template-logo';
import { duplicateHref, NewTemplateDialog } from './new-template-dialog';

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

function TemplateItem({
  template,
  canEdit,
  canDuplicate,
  onDelete,
}: Readonly<{
  template: any;
  /** A local template the user may change: edit and delete. */
  canEdit: boolean;
  /** Any template, local or community, can be copied into a new local one. */
  canDuplicate: boolean;
  onDelete?: () => void;
}>) {
  const { t } = useTranslations();
  const Icon = getIconForCategory(template.type);
  const logo = templateLogo(template);
  const categoryLabel = template.type || t('templates.categories.other');
  const title = template.display || template.name;

  return (
    <article className="flex flex-col gap-4 rounded-xl border border-border/60 bg-background/30 p-4">
      <div className="flex items-start gap-3">
        <span className="grid h-10 w-10 shrink-0 place-items-center overflow-hidden rounded-lg border border-border/60 bg-secondary text-muted-foreground" aria-hidden="true">
          <TemplateLogo logo={logo} className="h-full w-full rounded-none ring-0" fallback={<Icon className="h-5 w-5" />} />
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
        {(canEdit || canDuplicate) && (
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
              {canEdit && (
                <DropdownMenuItem asChild>
                  <a href={`/templates/editor/?${new URLSearchParams({ name: template.name })}`}>
                    <Pencil className="mr-2 h-4 w-4" aria-hidden="true" />
                    {t('common.edit')}
                  </a>
                </DropdownMenuItem>
              )}
              {canDuplicate && (
                <DropdownMenuItem asChild>
                  <a href={duplicateHref(template)}>
                    <Copy className="mr-2 h-4 w-4" aria-hidden="true" />
                    {t('templates.actions.duplicate')}
                  </a>
                </DropdownMenuItem>
              )}
              {canEdit && (
                <>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    className="text-destructive-bright focus:bg-destructive/10 focus:text-destructive-bright"
                    onClick={() => setTimeout(() => onDelete?.(), 50)}
                  >
                    <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
                    {t('common.delete')}
                  </DropdownMenuItem>
                </>
              )}
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
  const [uploading, setUploading] = useState(false);
  const [activeTab, setActiveTab] = useState('local');
  const [query, setQuery] = useState('');
  const [deletingTemplate, setDeletingTemplate] = useState<any>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const { t } = useTranslations();

  const { repos, loading: reposLoading, getTemplatesForRepo, uploadTemplates, deleteTemplate } = useTemplates();
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

  if (!isMounted || !hasScope('templates.view') || reposLoading) {
    return <TemplatesSkeleton />;
  }

  const localTemplates = allTemplates.filter(t => t.repoId === 0);
  const communityTemplates = allTemplates.filter(t => t.repoId !== 0);
  // the search narrows both tabs; the counts follow it so the other tab shows whether it has matches
  const shownLocal = localTemplates.filter(t => matchesTemplate(t, query));
  const shownCommunity = communityTemplates.filter(t => matchesTemplate(t, query));
  const noMatches = (
    <EmptyTemplates
      title={t('templates.search.noResultsTitle')}
      description={t('templates.search.noResultsDescription', { query: query.trim() })}
      action={<Button variant="outline" onClick={() => setQuery('')}>{t('templates.search.clear')}</Button>}
    />
  );
  // saving a template (PUT /api/templates/0/:name) needs templates.local.edit, and so do creating,
  // editing and duplicating one
  const canEditLocal = hasScope('templates.local.edit');

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
        {canEditLocal && (
          <Button onClick={() => setIsCreateOpen(true)}>
            <Plus aria-hidden="true" />
            {t('templates.create')}
          </Button>
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
              <TabsTrigger value="local" className="flex-1 sm:flex-none">{t('templates.tabs.local')}{!loadingTemplates && countPill(shownLocal.length)}</TabsTrigger>
              <TabsTrigger value="community" className="flex-1 sm:flex-none">{t('templates.tabs.community')}{!loadingTemplates && countPill(shownCommunity.length)}</TabsTrigger>
            </TabsList>
          </div>

          <div className="border-b border-border/50 px-5 py-3 md:px-6">
            <div className="relative max-w-md">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
              <Input
                type="search"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t('templates.search.placeholder')}
                aria-label={t('templates.search.label')}
                className="pl-9"
              />
            </div>
          </div>

          <TabsContent value="local" className="mt-0 p-5 md:p-6">
            <TemplateTabBody
              loading={loadingTemplates}
              placeholder={loadingGrid}
              isEmpty={shownLocal.length === 0}
              empty={localTemplates.length > 0 ? noMatches : (
                <EmptyTemplates
                  title={t('templates.empty.localTitle')}
                  description={t('templates.empty.localDescription')}
                  action={canEditLocal ? (
                    <Button onClick={() => setIsCreateOpen(true)}>
                      <Plus aria-hidden="true" />
                      {t('templates.create')}
                    </Button>
                  ) : undefined}
                />
              )}
            >
              {shownLocal.map((template, idx) => (
                <TemplateItem
                  key={`local-${template.name}-${idx}`}
                  template={template}
                  canEdit={canEditLocal}
                  canDuplicate={canEditLocal}
                  onDelete={() => setDeletingTemplate(template)}
                />
              ))}
            </TemplateTabBody>
          </TabsContent>

          <TabsContent value="community" className="mt-0 p-5 md:p-6">
            <TemplateTabBody
              loading={loadingTemplates}
              placeholder={loadingGrid}
              isEmpty={shownCommunity.length === 0}
              empty={communityTemplates.length > 0 ? noMatches : (
                <EmptyTemplates
                  title={t('templates.empty.communityTitle')}
                  description={t('templates.empty.communityDescription')}
                />
              )}
            >
              {shownCommunity.map((template, idx) => (
                <TemplateItem
                  key={`com-${template.repoId}-${template.name}-${idx}`}
                  template={template}
                  canEdit={false}
                  canDuplicate={canEditLocal}
                />
              ))}
            </TemplateTabBody>
          </TabsContent>
        </Card>
      </Tabs>

      {canEditLocal && (
        <NewTemplateDialog open={isCreateOpen} onOpenChange={setIsCreateOpen} templates={allTemplates} loading={loadingTemplates} />
      )}

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
