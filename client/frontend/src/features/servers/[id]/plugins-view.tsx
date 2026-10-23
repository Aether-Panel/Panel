'use client';
import { useState, useEffect, useCallback } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { Puzzle, Search, Download, Trash2, ExternalLink, Loader2, CheckCircle2, ShieldCheck } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from "@/lib/toast";
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { EmptyState } from '@/components/empty-state';

type PluginInfo = {
  name: string;
  version: string;
  size: number;
};

type PluginSearchResult = {
  id: string;
  name: string;
  description: string;
  author: string;
  iconUrl: string;
  downloads: number;
  version: string;
};

type PluginsViewProps = {
  serverId: string;
};

export default function PluginsView({ serverId }: PluginsViewProps) {
  const [activeTab, setActiveTab] = useState('installed');
  const [installedPlugins, setInstalledPlugins] = useState<PluginInfo[]>([]);
  const [searchResults, setSearchResults] = useState<PluginSearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [installingId, setInstallingId] = useState<string | null>(null);
  const [pendingPlugin, setPendingPlugin] = useState<string | null>(null);
  // Only used to tell "no search yet" apart from "search returned nothing".
  const [hasSearched, setHasSearched] = useState(false);
  const { t } = useTranslations();
  

  const fetchInstalled = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.get(`/api/servers/${serverId}/plugins`);
      setInstalledPlugins(data || []);
    } catch (err) {
      console.error('Failed to fetch installed plugins:', err);
    } finally {
      setLoading(false);
    }
  }, [serverId]);

  useEffect(() => {
    void fetchInstalled();
  }, [fetchInstalled]);

  const handleSearch = async (e?: React.FormEvent) => {
    e?.preventDefault();
    if (!searchQuery.trim()) return;

    try {
      setLoading(true);
      const data = await api.get(`/api/servers/${serverId}/plugins/search?q=${encodeURIComponent(searchQuery)}`);
      setSearchResults(data || []);
      setHasSearched(true);
      setActiveTab('marketplace');
    } catch (err) {
      console.error('Failed to search projects on Modrinth:', err);
      sileo.error({
        title: t('common.error'),
        description: t('servers.plugins.notifications.searchError')
      });
    } finally {
      setLoading(false);
    }
  };

  const handleInstall = async (plugin: PluginSearchResult) => {
    try {
      setInstallingId(plugin.id);
      await api.post(`/api/servers/${serverId}/plugins/${plugin.id}`, {});
      sileo.success({
        title: t('common.success'),
        description: t('servers.plugins.notifications.installSuccess', { name: plugin.name })
      });
      void fetchInstalled();
    } catch (err) {
      console.error('Failed to install project:', err);
      sileo.error({
        title: t('common.error'),
        description: t('servers.plugins.notifications.installError')
      });
    } finally {
      setInstallingId(null);
    }
  };

  const handleDelete = async () => {
    if (!pendingPlugin) return;
    try {
      await api.delete(`/api/servers/${serverId}/plugins?name=${encodeURIComponent(pendingPlugin)}`);
      sileo.success({
        title: t('common.success'),
        description: t('servers.plugins.notifications.uninstallSuccess')
      });
      void fetchInstalled();
    } catch (err) {
      console.error('Failed to delete plugin:', err);
      sileo.error({
        title: t('common.error'),
        description: t('servers.plugins.notifications.uninstallError')
      });
    } finally {
      setPendingPlugin(null);
    }
  };

  const formatSize = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Number.parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const isInstalled = (pluginName: string) => {
    // Modrinth usually provides slugs or clean names.
    // This is a fuzzy check to see if we already have it.
    const normalizedName = pluginName.toLowerCase().replaceAll(/\s+/g, '_');
    return installedPlugins.some(p => p.name.toLowerCase().includes(normalizedName));
  };

  return (
    <Card className="mt-6">
      <div className="flex flex-col gap-4 border-b border-border/50 p-5 sm:flex-row sm:items-center sm:justify-between md:p-6">
        <div className="min-w-0 space-y-1">
          <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('servers.plugins.title')}</h2>
          <p className="text-sm text-muted-foreground">
            {t('servers.plugins.description', { count: installedPlugins.length })}
          </p>
          <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <ShieldCheck className="h-3.5 w-3.5 shrink-0 text-success" aria-hidden="true" />
            {t('servers.plugins.restorePointNote')}
          </p>
        </div>
        <form onSubmit={handleSearch} role="search" className="flex w-full items-center gap-2 sm:max-w-sm">
          <div className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
            <Input
              type="search"
              placeholder={t('servers.plugins.searchPlaceholder')}
              aria-label={t('servers.plugins.searchPlaceholder')}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9"
            />
          </div>
          <Button type="submit" disabled={loading} className="shrink-0">
            {loading && <Loader2 className="animate-spin" aria-hidden="true" />}
            {t('servers.plugins.searchButton')}
          </Button>
        </form>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="p-5 md:p-6">
        <TabsList className="grid w-full grid-cols-2 sm:inline-grid sm:w-auto">
          <TabsTrigger value="installed">{t('servers.plugins.tabs.installed')}</TabsTrigger>
          <TabsTrigger value="marketplace">{t('servers.plugins.tabs.marketplace')}</TabsTrigger>
        </TabsList>

        <TabsContent value="installed" className="mt-5">
          {loading && installedPlugins.length === 0 ? (
            <div className="flex justify-center py-16">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-label={t('common.loading')} />
            </div>
          ) : installedPlugins.length === 0 ? (
            <EmptyState
              icon={Puzzle}
              title={t('servers.plugins.installed.emptyTitle')}
              description={t('servers.plugins.installed.emptyDescription')}
              action={
                <Button variant="outline" onClick={() => setActiveTab('marketplace')}>
                  <Search aria-hidden="true" />
                  {t('servers.plugins.installed.browse')}
                </Button>
              }
            />
          ) : (
            <ul className="divide-y divide-border/50 overflow-hidden rounded-xl border border-border/60 bg-background/30">
              {installedPlugins.map((plugin) => (
                <li key={plugin.name} className="flex items-center justify-between gap-4 px-4 py-3 transition-colors duration-150 hover:bg-accent/30">
                  <div className="flex min-w-0 items-center gap-3">
                    <span className="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-border/60 bg-card">
                      <Puzzle className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                    </span>
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{plugin.name}</p>
                      <p className="mt-0.5 text-xs tabular-nums text-muted-foreground">{formatSize(plugin.size)}</p>
                    </div>
                  </div>
                  <AlertDialog open={pendingPlugin === plugin.name} onOpenChange={(open) => !open && setPendingPlugin(null)}>
                    <AlertDialogTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-9 w-9 shrink-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright"
                        aria-label={`${t('servers.plugins.installed.uninstall')} ${plugin.name}`}
                        onClick={() => setPendingPlugin(plugin.name)}
                      >
                        <Trash2 aria-hidden="true" />
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>{t('servers.plugins.installed.uninstall')}</AlertDialogTitle>
                        <AlertDialogDescription>
                          {t('servers.plugins.installed.uninstallConfirm')}
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                        <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                          {t('servers.plugins.installed.uninstall')}
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                </li>
              ))}
            </ul>
          )}
        </TabsContent>

        <TabsContent value="marketplace" className="mt-5">
          {loading ? (
            <div className="flex justify-center py-16">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-label={t('common.loading')} />
            </div>
          ) : searchResults.length === 0 ? (
            <EmptyState
              icon={Search}
              title={hasSearched ? t('servers.plugins.empty.title') : t('servers.plugins.marketplace.title')}
              description={hasSearched ? t('servers.plugins.empty.description') : t('servers.plugins.marketplace.description')}
            />
          ) : (
            <ul className="divide-y divide-border/50 overflow-hidden rounded-xl border border-border/60 bg-background/30">
              {searchResults.map((plugin) => (
                <li key={plugin.id} className="flex flex-col gap-4 p-4 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex min-w-0 flex-1 items-start gap-3.5">
                    <Avatar className="h-10 w-10 shrink-0 rounded-lg ring-1 ring-border/60">
                      <AvatarImage src={plugin.iconUrl} alt="" />
                      <AvatarFallback className="rounded-lg bg-card">
                        <Puzzle className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                      </AvatarFallback>
                    </Avatar>
                    <div className="min-w-0 space-y-1">
                      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                        <h4 className="truncate text-sm font-semibold">{plugin.name}</h4>
                        {plugin.version && (
                          <span className="rounded-md border border-border/60 px-1.5 py-px font-mono text-[11px] text-muted-foreground">
                            {plugin.version}
                          </span>
                        )}
                      </div>
                      <p className="line-clamp-2 text-sm text-muted-foreground">{plugin.description}</p>
                      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                        <span>{t('servers.plugins.marketplace.author')}: <span className="text-foreground/80">{plugin.author}</span></span>
                        <span className="tabular-nums">{t('servers.plugins.marketplace.downloads')}: {plugin.downloads.toLocaleString()}</span>
                      </div>
                    </div>
                  </div>
                  <div className="flex shrink-0 items-center gap-2 pl-[3.375rem] sm:pl-0">
                    <Button variant="outline" size="sm" asChild>
                      <a href={`https://modrinth.com/project/${plugin.id}`} target="_blank" rel="noopener noreferrer">
                        <ExternalLink aria-hidden="true" />
                        Modrinth
                      </a>
                    </Button>
                    {isInstalled(plugin.name) ? (
                      <span className="inline-flex h-9 items-center gap-1.5 rounded-md bg-success/10 px-3 text-sm font-medium text-success">
                        <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
                        {t('servers.plugins.marketplace.installed')}
                      </span>
                    ) : (
                      <Button
                        size="sm"
                        disabled={installingId === plugin.id}
                        onClick={() => handleInstall(plugin)}
                      >
                        {installingId === plugin.id ? (
                          <>
                            <Loader2 className="animate-spin" aria-hidden="true" />
                            {t('servers.plugins.marketplace.installing')}
                          </>
                        ) : (
                          <>
                            <Download aria-hidden="true" />
                            {t('servers.plugins.marketplace.install')}
                          </>
                        )}
                      </Button>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </TabsContent>
      </Tabs>
    </Card>
  );
}
