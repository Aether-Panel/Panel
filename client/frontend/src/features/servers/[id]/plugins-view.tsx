'use client';
import { useState, useEffect, useCallback } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Puzzle, Search, Download, Trash2, ExternalLink, Loader2, CheckCircle2 } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { useToast } from '@/hooks/use-toast';
import { Badge } from '@/components/ui/badge';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';

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
  const { t } = useTranslations();
  const { toast } = useToast();

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
    fetchInstalled();
  }, [fetchInstalled]);

  const handleSearch = async (e?: React.FormEvent) => {
    e?.preventDefault();
    if (!searchQuery.trim()) return;

    try {
      setLoading(true);
      const data = await api.get(`/api/servers/${serverId}/plugins/search?q=${encodeURIComponent(searchQuery)}`);
      setSearchResults(data || []);
      setActiveTab('marketplace');
    } catch (err) {
      console.error('Failed to search projects on Modrinth:', err);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: 'Failed to search on Modrinth'
      });
    } finally {
      setLoading(false);
    }
  };

  const handleInstall = async (plugin: PluginSearchResult) => {
    try {
      setInstallingId(plugin.id);
      await api.post(`/api/servers/${serverId}/plugins/${plugin.id}`, {});
      toast({
        title: t('common.success'),
        description: t('servers.plugins.notifications.installSuccess', { name: plugin.name })
      });
      fetchInstalled();
    } catch (err) {
      console.error('Failed to install project:', err);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: t('servers.plugins.notifications.installError')
      });
    } finally {
      setInstallingId(null);
    }
  };

  const handleDelete = async (pluginName: string) => {
    if (!confirm(t('servers.plugins.installed.uninstallConfirm'))) return;

    try {
      await api.delete(`/api/servers/${serverId}/plugins?name=${encodeURIComponent(pluginName)}`);
      toast({
        title: t('common.success'),
        description: t('servers.plugins.notifications.uninstallSuccess')
      });
      fetchInstalled();
    } catch (err) {
      console.error('Failed to delete plugin:', err);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: t('servers.plugins.notifications.uninstallError')
      });
    }
  };

  const formatSize = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const isInstalled = (pluginName: string) => {
    // Modrinth usually provides slugs or clean names.
    // This is a fuzzy check to see if we already have it.
    const normalizedName = pluginName.toLowerCase().replace(/\s+/g, '_');
    return installedPlugins.some(p => p.name.toLowerCase().includes(normalizedName));
  };

  return (
    <div className="mt-6 flex flex-col gap-6">
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">{t('servers.plugins.title')}</h2>
          <p className="text-muted-foreground">
            {t('servers.plugins.description', { count: installedPlugins.length })}
          </p>
        </div>
        <form onSubmit={handleSearch} className="flex w-full max-w-md items-center space-x-2">
          <Input
            type="search"
            placeholder={t('servers.plugins.searchPlaceholder')}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
          <Button type="submit" disabled={loading}>
            {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
          </Button>
        </form>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <TabsList className="grid w-full max-w-[400px] grid-cols-2">
          <TabsTrigger value="installed">{t('servers.plugins.tabs.installed')}</TabsTrigger>
          <TabsTrigger value="marketplace">{t('servers.plugins.tabs.marketplace')}</TabsTrigger>
        </TabsList>

        <TabsContent value="installed" className="mt-6">
          <Card className="border-0 bg-card/50 backdrop-blur-sm">
            <CardHeader>
              <CardTitle className="text-lg">{t('servers.plugins.installed.title')}</CardTitle>
              <CardDescription>{t('servers.plugins.installed.description')}</CardDescription>
            </CardHeader>
            <CardContent>
              {installedPlugins.length === 0 && !loading ? (
                <div className="flex flex-col items-center justify-center py-12 text-center">
                  <Puzzle className="h-12 w-12 text-muted-foreground/30 mb-4" />
                  <h3 className="text-lg font-medium">{t('servers.plugins.empty.title')}</h3>
                  <p className="text-sm text-muted-foreground">{t('servers.plugins.empty.description')}</p>
                </div>
              ) : (
                <div className="grid gap-4 md:grid-cols-2">
                  {installedPlugins.map((plugin) => (
                    <div key={plugin.name} className="flex items-center justify-between p-4 rounded-lg border bg-background/50">
                      <div className="flex items-center gap-3">
                        <div className="p-2 rounded-md bg-primary/10">
                          <Puzzle className="h-5 w-5 text-primary" />
                        </div>
                        <div>
                          <p className="font-medium">{plugin.name}</p>
                          <p className="text-xs text-muted-foreground">
                            {formatSize(plugin.size)}
                          </p>
                        </div>
                      </div>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="text-destructive hover:text-destructive hover:bg-destructive/10"
                        onClick={() => handleDelete(plugin.name)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="marketplace" className="mt-6">
          <Card className="border-0 bg-card/50 backdrop-blur-sm">
            <CardHeader>
              <CardTitle className="text-lg">{t('servers.plugins.marketplace.title')}</CardTitle>
              <CardDescription>{t('servers.plugins.marketplace.description')}</CardDescription>
            </CardHeader>
            <CardContent>
              {searchResults.length === 0 && !loading ? (
                <div className="flex flex-col items-center justify-center py-12 text-center">
                  <Search className="h-12 w-12 text-muted-foreground/30 mb-4" />
                  <h3 className="text-lg font-medium">{t('servers.plugins.empty.title')}</h3>
                  <p className="text-sm text-muted-foreground">{t('servers.plugins.marketplace.description')}</p>
                </div>
              ) : (
                <div className="flex flex-col gap-4">
                  {searchResults.map((plugin) => (
                    <div key={plugin.id} className="flex flex-col md:flex-row md:items-center justify-between p-4 rounded-lg border bg-background/50 gap-4">
                      <div className="flex items-start gap-4 flex-1">
                        <Avatar className="h-12 w-12 rounded-lg">
                          <AvatarImage src={plugin.iconUrl} />
                          <AvatarFallback className="rounded-lg bg-primary/10">
                            <Puzzle className="h-6 w-6 text-primary" />
                          </AvatarFallback>
                        </Avatar>
                        <div className="space-y-1">
                          <div className="flex items-center gap-2">
                            <h4 className="font-bold">{plugin.name}</h4>
                            <Badge variant="secondary" className="text-[10px] px-1 h-4">
                              {plugin.version}
                            </Badge>
                          </div>
                          <p className="text-sm text-muted-foreground line-clamp-2">{plugin.description}</p>
                          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                            <span>{t('servers.plugins.marketplace.author')}: {plugin.author}</span>
                            <span>{t('servers.plugins.marketplace.downloads')}: {plugin.downloads.toLocaleString()}</span>
                          </div>
                        </div>
                      </div>
                      <div className="flex items-center gap-2 shrink-0">
                        <Button variant="outline" size="sm" asChild>
                          <a href={`https://modrinth.com/project/${plugin.id}`} target="_blank" rel="noopener noreferrer">
                            <ExternalLink className="h-4 w-4 mr-2" />
                            Modrinth
                          </a>
                        </Button>
                        {isInstalled(plugin.name) ? (
                          <Button disabled size="sm" variant="secondary" className="bg-green-500/10 text-green-500 hover:bg-green-500/10">
                            <CheckCircle2 className="h-4 w-4 mr-2" />
                            {t('servers.plugins.marketplace.installed')}
                          </Button>
                        ) : (
                          <Button
                            size="sm"
                            disabled={installingId === plugin.id}
                            onClick={() => handleInstall(plugin)}
                          >
                            {installingId === plugin.id ? (
                              <>
                                <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                                {t('servers.plugins.marketplace.installing')}
                              </>
                            ) : (
                              <>
                                <Download className="h-4 w-4 mr-2" />
                                {t('servers.plugins.marketplace.install')}
                              </>
                            )}
                          </Button>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
              {loading && (
                <div className="flex justify-center py-8">
                  <Loader2 className="h-8 w-8 animate-spin text-primary" />
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
