'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useState } from 'react';
import { PageHeader } from '@/components/page-header';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Code, Puzzle, Bot, Server as ServerIcon, Database, PlusCircle, LayoutTemplate,
  Gamepad2, Globe, Cpu, Radio, Network, Box, Terminal, Shield, Play
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';
import { useTemplates, type TemplateRepo } from '@/hooks/use-templates';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { useToast } from '@/hooks/use-toast';
import { Textarea } from '@/components/ui/textarea';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

// Optional icon mapping if we want to guess icon by category later, or just a default
function getIconForCategory(category?: string): LucideIcon {
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

function getBadgeStyle(category?: string) {
  if (!category) return 'border-slate-500/30 bg-slate-500/10 text-slate-400';
  const c = category.toLowerCase();

  if (c.includes('minecraft') || c.includes('game') || c.includes('survival')) return 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400';
  if (c.includes('bot') || c.includes('discord')) return 'border-indigo-500/30 bg-indigo-500/10 text-indigo-400';
  if (c.includes('web') || c.includes('app')) return 'border-blue-500/30 bg-blue-500/10 text-blue-400';
  if (c.includes('database') || c.includes('sql') || c.includes('redis')) return 'border-amber-500/30 bg-amber-500/10 text-amber-400';
  if (c.includes('proxy') || c.includes('network') || c.includes('bungee')) return 'border-cyan-500/30 bg-cyan-500/10 text-cyan-400';
  if (c.includes('voice') || c.includes('teamspeak') || c.includes('mumble')) return 'border-orange-500/30 bg-orange-500/10 text-orange-400';
  if (c.includes('security') || c.includes('vpn')) return 'border-red-500/30 bg-red-500/10 text-red-400';
  if (c.includes('tool') || c.includes('script') || c.includes('core')) return 'border-zinc-500/30 bg-zinc-500/10 text-zinc-400';

  return 'border-slate-500/30 bg-slate-500/10 text-slate-400';
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

export default function TemplatesPage() {
  const { role, hasScope } = useAuth();
  const { toast } = useToast();
  const [isMounted, setIsMounted] = useState(false);

  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [templateJson, setTemplateJson] = useState(defaultTemplateJson);
  const [templateObj, setTemplateObj] = useState<any>(JSON.parse(defaultTemplateJson));
  const [saving, setSaving] = useState(false);

  const updateField = (field: string, value: string) => {
    const newObj = { ...templateObj, [field]: value };
    setTemplateObj(newObj);
    setTemplateJson(JSON.stringify(newObj, null, 2));
  };

  const updateRunField = (field: string, value: string) => {
    const newObj = { ...templateObj, run: { ...(templateObj.run || {}), [field]: value } };
    setTemplateObj(newObj);
    setTemplateJson(JSON.stringify(newObj, null, 2));
  };

  const handleJsonChange = (val: string) => {
    setTemplateJson(val);
    try {
      setTemplateObj(JSON.parse(val));
    } catch (e) { }
  };

  const { t } = useTranslations();

  const { repos, loading: reposLoading, getTemplatesForRepo, saveTemplate } = useTemplates();
  const [allTemplates, setAllTemplates] = useState<any[]>([]);
  const [loadingTemplates, setLoadingTemplates] = useState(false);

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

    async function loadAll() {
      setLoadingTemplates(true);
      const loaded: any[] = [];
      for (const repo of repos) {
        const tpls = await getTemplatesForRepo(repo.id);
        tpls.forEach((t: any) => {
          loaded.push({
            ...t,
            repoId: repo.id,
            repoName: repo.name
          });
        });
      }

      const localTpls = await getTemplatesForRepo(0);
      localTpls.forEach((t: any) => {
        loaded.push({ ...t, repoId: 0, repoName: 'Local' });
      });

      const uniqueLoaded = loaded.filter((v, i, a) => a.findIndex(t => (t.name === v.name && t.repoId === v.repoId)) === i);

      setAllTemplates(uniqueLoaded);
      setLoadingTemplates(false);
    }

    if (repos && repos.length > 0) {
      loadAll();
    }
  }, [repos, hasScope]);

  if (!isMounted || !hasScope('templates.view') || reposLoading) {
    return (
      <div className="flex h-full items-center justify-center min-h-[400px]">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-8">
      <PageHeader title={t('templates.title')} description={t('templates.description')}>
        {hasScope('templates.repo.create') && (
          <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
            <DialogTrigger asChild>
              <Button>
                <PlusCircle className="mr-2" />
                {t('templates.create')}
              </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-4xl h-[80vh] flex flex-col">
              <DialogHeader>
                <DialogTitle>{t('templates.create')}</DialogTitle>
                <DialogDescription>
                  Define your template parameters below using JSON format.
                </DialogDescription>
              </DialogHeader>
              <div className="flex-1 min-h-0 py-4">
                <Tabs defaultValue="general" className="h-full flex flex-col">
                  <TabsList className="mb-4">
                    <TabsTrigger value="general">{t('templates.General') || 'General'}</TabsTrigger>
                    <TabsTrigger value="run">{t('templates.RunConfig') || 'Run Config'}</TabsTrigger>
                    <TabsTrigger value="json">{t('templates.Json') || 'JSON'}</TabsTrigger>
                  </TabsList>

                  <TabsContent value="general" className="flex-1 overflow-y-auto space-y-4 pr-2">
                    <div className="space-y-2">
                      <Label>Name (Unique ID)</Label>
                      <Input value={templateObj?.name || ''} onChange={(e) => updateField('name', e.target.value)} placeholder="e.g. spigot" />
                    </div>
                    <div className="space-y-2">
                      <Label>Display Name</Label>
                      <Input value={templateObj?.display || ''} onChange={(e) => updateField('display', e.target.value)} placeholder="e.g. Minecraft Spigot" />
                    </div>
                    <div className="space-y-2">
                      <Label>Category (Type)</Label>
                      <Input value={templateObj?.type || ''} onChange={(e) => updateField('type', e.target.value)} placeholder="e.g. web, game, bot" />
                    </div>
                  </TabsContent>

                  <TabsContent value="run" className="flex-1 overflow-y-auto space-y-4 pr-2">
                    <div className="space-y-2">
                      <Label>Start Command</Label>
                      <Input value={templateObj?.run?.command || ''} onChange={(e) => updateRunField('command', e.target.value)} placeholder="e.g. java -jar server.jar" />
                    </div>
                    <div className="space-y-2">
                      <Label>Stop Command</Label>
                      <Input value={templateObj?.run?.stop || ''} onChange={(e) => updateRunField('stop', e.target.value)} placeholder="e.g. stop" />
                    </div>
                    <div className="space-y-2">
                      <Label>Working Directory</Label>
                      <Input value={templateObj?.run?.workingDirectory || ''} onChange={(e) => updateRunField('workingDirectory', e.target.value)} placeholder="" />
                    </div>
                  </TabsContent>

                  <TabsContent value="json" className="flex-1 h-full min-h-[300px]">
                    <Textarea
                      className="h-full min-h-[300px] w-full font-mono text-sm"
                      value={templateJson}
                      onChange={(e) => handleJsonChange(e.target.value)}
                      placeholder="Paste or write your Template JSON here..."
                    />
                  </TabsContent>
                </Tabs>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsCreateOpen(false)}>
                  Cancel
                </Button>
                <Button disabled={saving} onClick={async () => {
                  try {
                    setSaving(true);
                    const parsed = JSON.parse(templateJson);
                    if (!parsed.name) throw new Error("Template must have a 'name' field.");
                    await saveTemplate(parsed.name, parsed);
                    toast({ title: 'Success', description: 'Template created successfully' });
                    setIsCreateOpen(false);
                    setTemplateJson(defaultTemplateJson);
                    // Refresh
                    setLoadingTemplates(true);
                    const loaded: any[] = [];
                    for (const repo of repos) {
                      const tpls = await getTemplatesForRepo(repo.id);
                      tpls.forEach((t: any) => loaded.push({ ...t, repoId: repo.id, repoName: repo.name }));
                    }
                    // Since local is repo 0 implicitly in PufferPanel API sometimes:
                    const localTpls = await getTemplatesForRepo(0);
                    localTpls.forEach((t: any) => loaded.push({ ...t, repoId: 0, repoName: 'Local' }));

                    // Deduplicate logic just in case repo 0 is listed in `repos`
                    const uniqueLoaded = loaded.filter((v, i, a) => a.findIndex(t => (t.name === v.name && t.repoId === v.repoId)) === i);

                    setAllTemplates(uniqueLoaded);
                    setLoadingTemplates(false);
                  } catch (e: any) {
                    toast({ title: 'Error', description: e.message || 'Invalid JSON format or network error', variant: 'destructive' });
                  } finally {
                    setSaving(false);
                  }
                }}>
                  {saving ? 'Saving...' : 'Save Template'}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        )}
      </PageHeader>

      {loadingTemplates ? (
        <div className="flex justify-center p-8">
          <div className="h-6 w-6 animate-spin rounded-full border-4 border-primary border-t-transparent" />
        </div>
      ) : (
        <Tabs defaultValue="local" className="w-full">
          <TabsList className="mb-6">
            <TabsTrigger value="local">Local Templates</TabsTrigger>
            <TabsTrigger value="community">Community Templates</TabsTrigger>
          </TabsList>

          <TabsContent value="local">
            {allTemplates.filter(t => t.repoId === 0).length === 0 ? (
              <div className="text-center p-8 text-muted-foreground border border-dashed rounded-lg">
                No local templates available. Click "Create Template" to add one.
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                {allTemplates.filter(t => t.repoId === 0).map((template, idx) => {
                  const Icon = getIconForCategory(template.type);
                  const categoryLabel = template.type || 'Other';
                  return (
                    <div key={`local-${template.name}-${idx}`} className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50 transition-all duration-300 hover:scale-105 hover:shadow-xl hover:shadow-primary/10">
                      <Card className="flex h-full cursor-pointer flex-col justify-between overflow-hidden border-0">
                        <CardHeader>
                          <div className="flex items-start justify-between">
                            <Icon className="h-10 w-10 text-muted-foreground" />
                            <Badge variant="outline" className={`whitespace-nowrap ${getBadgeStyle(template.type)}`}>
                              {categoryLabel}
                            </Badge>
                          </div>
                        </CardHeader>
                        <CardContent>
                          <CardTitle className="text-lg">{template.display || template.name}</CardTitle>
                          {template.repoName && <CardDescription>Repo: {template.repoName}</CardDescription>}
                        </CardContent>
                        <div className="p-6 pt-2">
                          <Button className="w-full" onClick={() => {
                            window.location.href = `/servers/?create=true&template=${encodeURIComponent(template.name)}${template.repoId ? `&repo=${template.repoId}` : ''}`;
                          }}>
                            {t('templates.use')}
                          </Button>
                        </div>
                      </Card>
                    </div>
                  );
                })}
              </div>
            )}
          </TabsContent>

          <TabsContent value="community">
            {allTemplates.filter(t => t.repoId !== 0).length === 0 ? (
              <div className="text-center p-8 text-muted-foreground border border-dashed rounded-lg">
                No community templates available. Add a community repository.
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                {allTemplates.filter(t => t.repoId !== 0).map((template, idx) => {
                  const Icon = getIconForCategory(template.type);
                  const categoryLabel = template.type || 'Other';
                  return (
                    <div key={`com-${template.repoId}-${template.name}-${idx}`} className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50 transition-all duration-300 hover:scale-105 hover:shadow-xl hover:shadow-primary/10">
                      <Card className="flex h-full cursor-pointer flex-col justify-between overflow-hidden border-0">
                        <CardHeader>
                          <div className="flex items-start justify-between">
                            <Icon className="h-10 w-10 text-muted-foreground" />
                            <Badge variant="outline" className={`whitespace-nowrap ${getBadgeStyle(template.type)}`}>
                              {categoryLabel}
                            </Badge>
                          </div>
                        </CardHeader>
                        <CardContent>
                          <CardTitle className="text-lg">{template.display || template.name}</CardTitle>
                          {template.repoName && <CardDescription>Repo: {template.repoName}</CardDescription>}
                        </CardContent>
                        <div className="p-6 pt-2">
                          <Button className="w-full" onClick={() => {
                            window.location.href = `/servers/?create=true&template=${encodeURIComponent(template.name)}${template.repoId ? `&repo=${template.repoId}` : ''}`;
                          }}>
                            {t('templates.use')}
                          </Button>
                        </div>
                      </Card>
                    </div>
                  );
                })}
              </div>
            )}
          </TabsContent>
        </Tabs>
      )}
    </div>
  );
}
