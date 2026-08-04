'use client';
import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { useTranslations } from '@/contexts/translations-context';
import { useServerSettings, type ServerSettings, type SettingVariable } from '@/hooks/use-server-settings';
import { useToast } from '@/hooks/use-toast';
import { useAuth } from '@/contexts/providers';
import { cn } from '@/lib/utils';
import {
  Loader2,
  Settings,
  NotebookPen,
  Gauge,
  Box,
  Power,
  Puzzle,
  Cpu,
  MemoryStick,
  HardDrive,
  ShieldCheck,
  type LucideIcon,
} from 'lucide-react';

type SettingsViewProps = {
  serverId: string;
};

type SectionHeaderProps = {
  icon: LucideIcon;
  title: string;
  description?: string;
  right?: React.ReactNode;
};

function SectionHeader({ icon: Icon, title, description, right }: SectionHeaderProps) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div className="flex items-start gap-3">
        <div className="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-primary/25 bg-gradient-to-br from-primary/20 via-accent/10 to-transparent text-primary">
          <Icon className="h-4 w-4" />
        </div>
        <div className="space-y-0.5">
          <h3 className="font-headline text-base font-semibold leading-tight">{title}</h3>
          {description && <p className="text-sm leading-relaxed text-muted-foreground">{description}</p>}
        </div>
      </div>
      {right}
    </div>
  );
}

type ResourceTileProps = {
  icon: LucideIcon;
  label: string;
  hint: string;
  unit: string;
  value: number;
  onChange: (value: number) => void;
};

function ResourceTile({ icon: Icon, label, hint, unit, value, onChange }: ResourceTileProps) {
  return (
    <div className="group relative flex flex-col gap-3 overflow-hidden rounded-xl border border-border/80 bg-card p-5 transition-all duration-200 hover:border-primary/40 hover:shadow-[0_8px_30px_rgb(0_0_0/0.3)]">
      <div className="pointer-events-none absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-primary via-accent to-transparent" />
      <div className="flex items-center gap-3">
        <div className="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-primary/25 bg-gradient-to-br from-primary/20 via-accent/10 to-transparent text-primary">
          <Icon className="h-4 w-4" />
        </div>
        <Label className="text-sm font-bold uppercase tracking-tight">{label}</Label>
      </div>
      <div className="relative">
        <Input
          type="number"
          value={value}
          onChange={(e) => onChange(parseInt(e.target.value) || 0)}
          className="bg-accent/5 pr-14 font-mono text-right focus:bg-accent/10 transition-colors"
        />
        <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 font-mono text-xs font-medium text-muted-foreground">
          {unit}
        </span>
      </div>
      <p className="text-xs leading-relaxed text-muted-foreground">{hint}</p>
    </div>
  );
}

export default function SettingsView({ serverId }: SettingsViewProps) {
  const { t } = useTranslations();
  const { hasScope } = useAuth();
  const { settings, loading, error, saveSettings, isMinecraftJava } = useServerSettings(serverId);
  const { toast } = useToast();
  const [localSettings, setLocalSettings] = useState<ServerSettings | null>(null);
  const [saving, setSaving] = useState(false);
  const [pluginsEnabled, setPluginsEnabled] = useState(true);

  const canViewResources =
    hasScope('admin') ||
    hasScope('server.admin') ||
    hasScope('server.admin.config.view') ||
    hasScope('server.admin.config.manage') ||
    hasScope('server.definition.edit');

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
    if (!variable || variable.internal || ['cpu', 'memory', 'disk'].includes(name)) return null;

    const displayName = variable.display || name;
    const description = variable.desc;
    const isAdminOnly = !variable.userEdit;

    const adminBadge = isAdminOnly && (
      <span className="text-[10px] bg-amber-500/10 text-amber-500 px-1.5 py-0.5 rounded border border-amber-500/20 font-bold uppercase">
        Admin Only
      </span>
    );

    const isBinaryOption =
      variable.type === 'option' &&
      variable.options &&
      variable.options.length === 2 &&
      variable.options.every((o: any) => ['true', 'false', true, false].includes(o.value));

    if (isBinaryOption) {
      return (
        <div key={name} className="space-y-2">
          <div className="flex flex-col gap-1">
            <div className="flex items-center gap-2">
              <Label htmlFor={name} className="text-sm font-bold uppercase tracking-tight">{displayName}</Label>
              {adminBadge}
            </div>
            {description && <p className="text-xs text-muted-foreground leading-relaxed">{description}</p>}
          </div>
          <RadioGroup
            value={String(variable.value)}
            onValueChange={(val) => handleVariableChange(name, val)}
            className="flex gap-2"
          >
            {variable.options?.map((opt: any) => {
              const active = String(variable.value) === String(opt.value);
              return (
                <div
                  key={String(opt.value)}
                  className={cn(
                    'flex cursor-pointer items-center gap-2 rounded-lg border px-4 py-2.5 transition-all duration-150 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/10',
                    active ? 'border-primary bg-primary/10' : 'border-muted-foreground/20 bg-accent/5 hover:border-muted-foreground/40'
                  )}
                >
                  <RadioGroupItem
                    value={String(opt.value)}
                    id={`${name}-${String(opt.value)}`}
                    className="border-muted-foreground/40"
                  />
                  <Label
                    htmlFor={`${name}-${String(opt.value)}`}
                    className="cursor-pointer font-medium text-sm"
                  >
                    {opt.display}
                  </Label>
                </div>
              );
            })}
          </RadioGroup>
        </div>
      );
    }

    switch (variable.type) {
      case 'boolean':
        return (
          <div key={name} className="flex items-center justify-between rounded-xl border bg-accent/5 p-5 transition-all hover:bg-accent/10">
            <div className="space-y-1 pr-4">
              <div className="flex items-center gap-2">
                <Label htmlFor={name} className="font-bold text-base leading-none cursor-pointer">{displayName}</Label>
                {adminBadge}
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
          <div key={name} className="space-y-2">
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-2">
                <Label htmlFor={name} className="text-sm font-bold uppercase tracking-tight">{displayName}</Label>
                {adminBadge}
              </div>
              {description && <p className="text-xs text-muted-foreground leading-relaxed">{description}</p>}
            </div>
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
          <div key={name} className="space-y-2">
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-2">
                <Label htmlFor={name} className="text-sm font-bold uppercase tracking-tight">{displayName}</Label>
                {adminBadge}
              </div>
              {description && <p className="text-xs text-muted-foreground leading-relaxed">{description}</p>}
            </div>
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
          <div key={name} className="space-y-2">
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-2">
                <Label htmlFor={name} className="text-sm font-bold uppercase tracking-tight">{displayName}</Label>
                {adminBadge}
              </div>
              {description && <p className="text-xs text-muted-foreground leading-relaxed">{description}</p>}
            </div>
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

  const serverType = localSettings.definition?.type;

  const adminResourcesBadge = (
    <span className="flex shrink-0 items-center gap-1.5 rounded-full border border-amber-500/25 bg-amber-500/10 px-3 py-1 text-[11px] font-bold uppercase tracking-wider text-amber-500">
      <ShieldCheck className="h-3.5 w-3.5" />
      {t('servers.settings.resourcesAdminBadge')}
    </span>
  );

  return (
    <div className="mt-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
      <Card className="border-0 bg-transparent shadow-none">
        <CardHeader className="flex flex-row items-center gap-4 px-0">
          <div className="flex items-center gap-4">
            <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl border border-primary/30 bg-gradient-to-br from-primary/25 via-accent/15 to-transparent text-primary shadow-[0_0_20px_rgb(0_0_0/0.3)]">
              <Settings className="h-5 w-5" />
            </div>
            <div>
              <CardTitle className="font-headline text-2xl">{t('servers.settings.title')}</CardTitle>
              <CardDescription>{t('servers.settings.description')}</CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="mt-4 space-y-10 px-0">

          {/* Server Identity Section */}
          <section className="space-y-5">
            <SectionHeader
              icon={NotebookPen}
              title={t('servers.settings.generalTitle') || 'General Information'}
              description={t('servers.settings.nameDescription')}
            />
            <div className="grid max-w-2xl grid-cols-1 gap-6 md:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="display-name" className="text-sm font-bold uppercase tracking-tight">
                  {t('servers.settings.nameLabel') || 'Server Name'}
                </Label>
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
              {serverType && (
                <div className="space-y-2">
                  <Label className="text-sm font-bold uppercase tracking-tight">
                    {t('servers.settings.serverTypeLabel')}
                  </Label>
                  <div className="flex h-10 items-center rounded-lg border border-border/80 bg-accent/5 px-3">
                    <span className="font-mono text-sm">{serverType}</span>
                  </div>
                </div>
              )}
            </div>
          </section>

          {/* Resource Limits Section (Admin Only) */}
          {canViewResources && (
            <section className="space-y-5">
              <SectionHeader
                icon={Gauge}
                title={t('servers.settings.resourcesTitle')}
                description={t('servers.settings.resourcesDescription')}
                right={adminResourcesBadge}
              />
              <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
                <ResourceTile
                  icon={Cpu}
                  label={t('servers.settings.cpuLabel')}
                  hint={t('servers.settings.cpuHint')}
                  unit="%"
                  value={(localSettings.variables['cpu']?.value as number) ?? 100}
                  onChange={(val) => handleVariableChange('cpu', val)}
                />
                <ResourceTile
                  icon={MemoryStick}
                  label={t('servers.settings.memoryLabel')}
                  hint={t('servers.settings.memoryHint')}
                  unit="MB"
                  value={(localSettings.variables['memory']?.value as number) ?? 1024}
                  onChange={(val) => handleVariableChange('memory', val)}
                />
                <ResourceTile
                  icon={HardDrive}
                  label={t('servers.settings.diskLabel')}
                  hint={t('servers.settings.diskHint')}
                  unit="MB"
                  value={(localSettings.variables['disk']?.value as number) ?? 10240}
                  onChange={(val) => handleVariableChange('disk', val)}
                />
              </div>
            </section>
          )}

          {/* Grouped Variables */}
          {groups.map((group, idx) => (
            <section key={idx} className="space-y-5">
              <SectionHeader
                icon={Box}
                title={group.display}
                description={group.description}
              />
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                {group.variables.map(varName => renderVariable(varName, localSettings.variables[varName]))}
              </div>
            </section>
          ))}

          {/* Groupless Variables */}
          {grouplessVars.length > 0 && (
            <section className="space-y-5">
              <SectionHeader
                icon={Settings}
                title={t('templates.categories.NoGroup') || 'Variables'}
              />
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                {grouplessVars.map(varName => renderVariable(varName, localSettings.variables[varName]))}
              </div>
            </section>
          )}

          {/* Flags Section */}
          <section className="space-y-5">
            <SectionHeader
              icon={Power}
              title={t('servers.settings.autoStartTitle')}
            />
            <div className="divide-y divide-border/60 overflow-hidden rounded-xl border border-border/80 bg-card">
              <div className="flex items-center justify-between gap-4 p-4 transition-colors hover:bg-accent/5 sm:p-5">
                <div className="space-y-0.5">
                  <h4 className="font-semibold">{t('servers.settings.startOnBootLabel')}</h4>
                  <p className="text-sm text-muted-foreground">{t('servers.settings.startOnBootDescription')}</p>
                </div>
                <Switch
                  checked={localSettings.flags.autoStart}
                  onCheckedChange={(val) => handleFlagChange('autoStart', val)}
                />
              </div>
              <div className="flex items-center justify-between gap-4 p-4 transition-colors hover:bg-accent/5 sm:p-5">
                <div className="space-y-0.5">
                  <h4 className="font-semibold">{t('servers.settings.restartOnCrashLabel')}</h4>
                  <p className="text-sm text-muted-foreground">{t('servers.settings.restartOnCrashDescription')}</p>
                </div>
                <Switch
                  checked={localSettings.flags.autoRestartOnCrash}
                  onCheckedChange={(val) => handleFlagChange('autoRestartOnCrash', val)}
                />
              </div>
              <div className="flex items-center justify-between gap-4 p-4 transition-colors hover:bg-accent/5 sm:p-5">
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
          </section>

          {/* Plugin Settings (Minecraft Java Only) */}
          {isMinecraftJava && (
            <section className="space-y-5">
              <SectionHeader
                icon={Puzzle}
                title={t('servers.settings.pluginsTitle')}
              />
              <div className="divide-y divide-border/60 overflow-hidden rounded-xl border border-border/80 bg-card">
                <div className="flex items-center justify-between gap-4 p-4 transition-colors hover:bg-accent/5 sm:p-5">
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
            </section>
          )}

          {/* Sticky Save Bar */}
          <div className="sticky bottom-4 z-20 flex items-center justify-end rounded-xl border border-border/80 bg-card/85 px-5 py-4 shadow-[0_-8px_30px_rgb(0_0_0/0.25)] backdrop-blur-md">
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
