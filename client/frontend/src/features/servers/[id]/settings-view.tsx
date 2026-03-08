'use client';
import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Separator } from '@/components/ui/separator';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useTranslations } from '@/contexts/translations-context';
import { useServerSettings, type ServerSettings, type SettingVariable } from '@/hooks/use-server-settings';
import { useToast } from '@/hooks/use-toast';
import { Loader2 } from 'lucide-react';

type SettingsViewProps = {
  serverId: string;
};

export default function SettingsView({ serverId }: SettingsViewProps) {
  const { t } = useTranslations();
  const { settings, loading, error, saveSettings, isMinecraftJava } = useServerSettings(serverId);
  const { toast } = useToast();
  const [localSettings, setLocalSettings] = useState<ServerSettings | null>(null);
  const [saving, setSaving] = useState(false);
  const [pluginsEnabled, setPluginsEnabled] = useState(true);

  useEffect(() => {
    if (settings) {
      setLocalSettings(JSON.parse(JSON.stringify(settings)));

      // Load plugins enabled from localStorage
      const stored = localStorage.getItem(`pluginsEnabled_${serverId}`);
      if (stored !== null) {
        setPluginsEnabled(stored === 'true');
      }
    }
  }, [settings, serverId]);

  if (loading) {
    return (
      <div className="flex h-[400px] items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  if (error || !localSettings) {
    return (
      <div className="flex h-[400px] flex-col items-center justify-center gap-4">
        <p className="text-destructive font-medium">{t('common.error')}: {error?.message || 'Failed to load settings'}</p>
        <Button onClick={() => window.location.reload()}>{t('common.retry') || 'Retry'}</Button>
      </div>
    );
  }

  const handleVariableChange = (name: string, value: any) => {
    setLocalSettings(prev => {
      if (!prev) return prev;
      const next = { ...prev };
      next.variables[name].value = value;
      return next;
    });
  };

  const handleFlagChange = (flag: keyof ServerSettings['flags'], value: boolean) => {
    setLocalSettings(prev => {
      if (!prev) return prev;
      return {
        ...prev,
        flags: {
          ...prev.flags,
          [flag]: value
        }
      };
    });
  };


  const onSave = async () => {
    try {
      setSaving(true);
      await saveSettings(localSettings);

      // Save plugins enabled to localStorage
      localStorage.setItem(`pluginsEnabled_${serverId}`, pluginsEnabled.toString());
      window.dispatchEvent(new CustomEvent('server:plugins-enabled-changed', { detail: pluginsEnabled }));

      toast({
        title: t('common.success') || 'Success',
        description: t('servers.settings.saveSuccess' as any) || 'Settings saved successfully'
      });
    } catch (e) {
      toast({
        variant: 'destructive',
        title: t('common.error') || 'Error',
        description: t('servers.settings.saveError' as any) || 'Failed to save settings'
      });
    } finally {
      setSaving(false);
    }
  };

  const renderVariable = (name: string, variable?: SettingVariable) => {
    if (!variable || variable.internal) return null;

    const displayName = variable.display || name;
    const description = variable.desc;
    const isAdminOnly = !variable.userEdit;

    const LabelSection = () => (
      <div className="flex flex-col gap-1 mb-2">
        <div className="flex items-center gap-2">
          <Label htmlFor={name} className="text-sm font-bold uppercase tracking-tight">{displayName}</Label>
          {isAdminOnly && (
            <span className="text-[10px] bg-amber-500/10 text-amber-500 px-1.5 py-0.5 rounded border border-amber-500/20 font-bold uppercase">
              Admin Only
            </span>
          )}
        </div>
        {description && <p className="text-xs text-muted-foreground leading-relaxed">{description}</p>}
      </div>
    );

    switch (variable.type) {
      case 'boolean':
        return (
          <div key={name} className="flex items-center justify-between rounded-xl border bg-accent/5 p-5 transition-all hover:bg-accent/10">
            <div className="space-y-1 pr-4">
              <div className="flex items-center gap-2">
                <Label htmlFor={name} className="font-bold text-base leading-none cursor-pointer">{displayName}</Label>
                {isAdminOnly && (
                  <span className="text-[10px] bg-amber-500/10 text-amber-500 px-1.5 py-0.5 rounded border border-amber-500/20 font-bold uppercase">
                    Admin Only
                  </span>
                )}
              </div>
              {description && <p className="text-sm text-muted-foreground leading-snug">{description}</p>}
            </div>
            <Switch
              id={name}
              checked={!!variable.value}
              onCheckedChange={(val) => handleVariableChange(name, val)}
            />
          </div>
        );
      case 'option':
        return (
          <div key={name} className="space-y-1">
            <LabelSection />
            <Select
              value={String(variable.value)}
              onValueChange={(val) => handleVariableChange(name, val)}
            >
              <SelectTrigger id={name} className="bg-accent/5 border-muted-foreground/20">
                <SelectValue placeholder="Select an option" />
              </SelectTrigger>
              <SelectContent>
                {variable.options?.map((opt: any) => (
                  <SelectItem key={opt.value} value={String(opt.value)}>
                    {opt.display}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        );
      case 'integer':
        return (
          <div key={name} className="space-y-1">
            <LabelSection />
            <Input
              id={name}
              type="number"
              value={(variable.value as number) ?? 0}
              onChange={(e) => handleVariableChange(name, parseInt(e.target.value) || 0)}
              className="bg-accent/5 border-muted-foreground/20 focus:bg-accent/10 transition-colors"
            />
          </div>
        );
      default:
        return (
          <div key={name} className="space-y-1">
            <LabelSection />
            <Input
              id={name}
              value={(variable.value as string) ?? ""}
              onChange={(e) => handleVariableChange(name, e.target.value)}
              className="bg-accent/5 border-muted-foreground/20 focus:bg-accent/10 transition-colors"
            />
          </div>
        );
    }
  };

  const groups = localSettings.groups || [];
  const grouplessVars = Object.keys(localSettings.variables).filter(name => {
    return !groups.some(g => g.variables.includes(name));
  });

  const hasVariables = Object.keys(localSettings.variables).length > 0;

  return (
    <div className="mt-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
      <Card className="border-0 bg-transparent shadow-none">
        <CardHeader className="px-0">
          <CardTitle className="text-2xl">{t('servers.settings.title')}</CardTitle>
          <CardDescription>{t('servers.settings.description')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-12 px-0">

          {/* Server Identity Section */}
          <div className="space-y-4">
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 rounded-full bg-primary" />
                <h3 className="text-xl font-bold">{t('servers.settings.generalTitle') || 'General Information'}</h3>
              </div>
              <p className="text-sm text-muted-foreground ml-4">Basic identity and metadata of your server instance.</p>
              <Separator className="mt-2" />
            </div>
            <div className="grid grid-cols-1 gap-6 md:grid-cols-2 ml-4">
              <div className="space-y-2">
                <div className="flex flex-col gap-1">
                  <Label htmlFor="display-name" className="text-sm font-bold uppercase tracking-tight">{t('servers.settings.nameLabel') || 'Server Name'}</Label>
                  <p className="text-xs text-muted-foreground leading-relaxed">{t('servers.settings.nameDescription') || 'The friendly name of this server.'}</p>
                </div>
                <Input
                  id="display-name"
                  value={localSettings.definition?.display || ''}
                  onChange={(e) => {
                    setLocalSettings(prev => {
                      if (!prev) return prev;
                      return {
                        ...prev,
                        definition: { ...prev.definition, display: e.target.value }
                      };
                    });
                  }}
                  className="bg-accent/5 focus:bg-accent/10 transition-colors"
                />
              </div>
            </div>
          </div>

          {/* Grouped Variables */}
          {groups.map((group, idx) => (
            <div key={idx} className="space-y-4">
              <div className="flex flex-col gap-1">
                <div className="flex items-center gap-2">
                  <div className="h-2 w-2 rounded-full bg-primary" />
                  <h3 className="text-xl font-bold uppercase tracking-wider text-sm text-muted-foreground">{group.display}</h3>
                </div>
                {group.description && <p className="text-sm text-muted-foreground ml-4">{group.description}</p>}
                <Separator className="mt-2" />
              </div>
              <div className="grid grid-cols-1 gap-6 md:grid-cols-2 ml-4">
                {group.variables.map(varName => renderVariable(varName, localSettings.variables[varName]))}
              </div>
            </div>
          ))}

          {/* Groupless Variables */}
          {grouplessVars.length > 0 && (
            <div className="space-y-4">
              <div className="flex flex-col gap-1">
                <div className="flex items-center gap-2">
                  <div className="h-2 w-2 rounded-full bg-primary" />
                  <h3 className="text-xl font-bold uppercase tracking-wider text-sm text-muted-foreground">{t('templates.categories.NoGroup') || 'Variables'}</h3>
                </div>
                <Separator className="mt-2" />
              </div>
              <div className="grid grid-cols-1 gap-6 md:grid-cols-2 ml-4">
                {grouplessVars.map(varName => renderVariable(varName, localSettings.variables[varName]))}
              </div>
            </div>
          )}

          {/* Flags Section */}
          <div className="space-y-4 pt-4">
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 rounded-full bg-primary" />
                <h3 className="text-xl font-bold">{t('servers.settings.autoStartTitle')}</h3>
              </div>
              <Separator className="mt-2" />
            </div>
            <div className="grid gap-4 ml-4">
              <div className="flex items-center justify-between rounded-lg border p-4 transition-colors hover:bg-accent/5">
                <div className="space-y-0.5">
                  <h4 className="font-semibold">{t('servers.settings.startOnBootLabel')}</h4>
                  <p className="text-sm text-muted-foreground">{t('servers.settings.startOnBootDescription')}</p>
                </div>
                <Switch
                  checked={localSettings.flags.autoStart}
                  onCheckedChange={(val) => handleFlagChange('autoStart', val)}
                />
              </div>
              <div className="flex items-center justify-between rounded-lg border p-4 transition-colors hover:bg-accent/5">
                <div className="space-y-0.5">
                  <h4 className="font-semibold">{t('servers.settings.restartOnCrashLabel')}</h4>
                  <p className="text-sm text-muted-foreground">{t('servers.settings.restartOnCrashDescription')}</p>
                </div>
                <Switch
                  checked={localSettings.flags.autoRestartOnCrash}
                  onCheckedChange={(val) => handleFlagChange('autoRestartOnCrash', val)}
                />
              </div>
              <div className="flex items-center justify-between rounded-lg border p-4 transition-colors hover:bg-accent/5">
                <div className="space-y-0.5">
                  <h4 className="font-semibold">{t('servers.settings.restartOnStopLabel')}</h4>
                  <p className="text-sm text-muted-foreground">{t('servers.settings.restartOnStopDescription')}</p>
                </div>
                <Switch
                  checked={localSettings.flags.autoRestartOnGraceful}
                  onCheckedChange={(val) => handleFlagChange('autoRestartOnGraceful', val)}
                />
              </div>
            </div>
          </div>

          {/* Plugin Settings (Minecraft Java Only) */}
          {isMinecraftJava && (
            <div className="space-y-4 pt-4">
              <div className="flex flex-col gap-1">
                <h3 className="text-xl font-bold text-primary">{t('servers.settings.pluginsTitle')}</h3>
                <Separator className="mt-2" />
              </div>
              <div className="flex items-center justify-between rounded-lg border p-4 transition-colors hover:bg-accent/5">
                <div className="space-y-0.5">
                  <h4 className="font-semibold">{t('servers.settings.enablePluginsLabel')}</h4>
                  <p className="text-sm text-muted-foreground">{t('servers.settings.enablePluginsDescription')}</p>
                </div>
                <Switch
                  checked={pluginsEnabled}
                  onCheckedChange={setPluginsEnabled}
                />
              </div>
            </div>
          )}

          <div className="flex justify-end gap-2 pt-6 sticky bottom-0 bg-background/80 backdrop-blur-sm py-4 border-t">
            <Button size="lg" onClick={onSave} disabled={saving} className="px-10 font-bold">
              {saving ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  {t('common.saving') || 'Saving...'}
                </>
              ) : (
                t('servers.settings.saveButton')
              )}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
