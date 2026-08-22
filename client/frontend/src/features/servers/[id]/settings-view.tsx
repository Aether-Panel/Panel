'use client';
import { useState, useEffect, useRef } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Progress } from '@/components/ui/progress';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { useTranslations } from '@/contexts/translations-context';
import { useServerSettings, type ServerSettings, type SettingVariable } from '@/hooks/use-server-settings';
import { sileo } from "@/lib/toast";
import { api } from '@/lib/api-client';
import { useAuth } from '@/contexts/providers';
import { cn } from '@/lib/utils';
import {
  Loader2, Settings, Box, Power, Puzzle, Cpu, MemoryStick, HardDrive,
  ShieldCheck, Trash2, Plus, Key, Wrench, Clock, Cog, BarChart3,
} from 'lucide-react';

type SettingsViewProps = {
  serverId: string;
  serverStatus?: 'online' | 'offline' | 'pending';
  onPortsSaved?: () => void;
};

const isPort = (n: string) => n === 'port' || /^port\d+$/.test(n);
const isSkipped = (n: string) => n === 'cpu' || n === 'memory' || n === 'disk' || isPort(n);
const isInternalMeta = (n: string) => isSkipped(n) || /^resolved|^forge|^javaVersion|^build|^git/i.test(n);

export default function SettingsView({ serverId, serverStatus, onPortsSaved }: SettingsViewProps) {
  const { t } = useTranslations();
  const { hasScope } = useAuth();
  const { settings, loading, error, saveSettings, isMinecraftJava } = useServerSettings(serverId);

  const [localSettings, setLocalSettings] = useState<ServerSettings | null>(null);
  const [saving, setSaving] = useState(false);
  const [pluginsEnabled, setPluginsEnabled] = useState(true);

  const canViewResources = hasScope('admin') || hasScope('server.admin') || hasScope('server.admin.config.view') || hasScope('server.admin.config.manage');
  const canEditAdminData = hasScope('server.data.edit.admin');
  const canViewPorts = hasScope('server.data.view');
  const canManagePorts = canEditAdminData;

  const [ports, setPorts] = useState<number[]>([]);
  const [portNotes, setPortNotes] = useState<Record<string, string>>({});
  const initialPrimaryRef = useRef<number | null>(null);
  const savedPortsRef = useRef<number[]>([]);
  const portsInitializedRef = useRef(false);
  const isRunning = serverStatus !== 'offline';

  useEffect(() => {
    if (settings) {
      setLocalSettings(JSON.parse(JSON.stringify(settings)));
      const stored = localStorage.getItem(`pluginsEnabled_${serverId}`);
      if (stored !== null) setPluginsEnabled(stored === 'true');
      if (!portsInitializedRef.current) {
        const lp: number[] = [];
        for (let i = 1; ; i++) {
          const key = i === 1 ? 'port' : `port${i}`;
          const v = settings.variables[key]?.value;
          if (v === undefined || v === null || v === '') break;
          const num = Number(v);
          if (num > 0) lp.push(num); else break;
        }
        setPorts(lp);
        initialPrimaryRef.current = lp[0] ?? null;
        savedPortsRef.current = [...lp];
        portsInitializedRef.current = true;
        if (canViewPorts) {
          api.get(`/api/servers/${serverId}`).then((data: any) => {
            if (data?.portNotes) setPortNotes(data.portNotes);
          }).catch(() => {});
        }
      }
    }
  }, [settings, serverId, canViewPorts]);

  if (loading) return <div className="flex h-[400px] items-center justify-center"><Loader2 className="h-8 w-8 animate-spin text-primary" /></div>;
  if (error || !localSettings) return (
    <div className="flex h-[400px] flex-col items-center justify-center gap-4">
      <p className="text-destructive font-medium">{t('common.error')}: {error?.message || 'Failed to load settings'}</p>
      <Button onClick={() => window.location.reload()}>{t('common.retry') || 'Retry'}</Button>
    </div>
  );

  const handleVariableChange = (name: string, value: any) => {
    setLocalSettings(prev => { if (!prev) return prev; return { ...prev, variables: { ...prev.variables, [name]: { ...prev.variables[name], value } } }; });
  };
  const handleFlagChange = (flag: keyof ServerSettings['flags'], value: boolean) => {
    setLocalSettings(prev => { if (!prev) return prev; return { ...prev, flags: { ...prev.flags, [flag]: value } }; });
  };
  const removePort = (idx: number) => {
    if (idx === 0 && isRunning) { sileo.error({ title: t('common.error') || 'Error', description: t('servers.settings.portsRequireOffline' as any) || 'The server must be stopped to change the primary port.' }); return; }
    setPorts(prev => prev.filter((_, i) => i !== idx));
  };

  const onSave = async () => {
    try {
      setSaving(true);
      const validPorts = ports.filter(p => p > 0);
      if (canEditAdminData) {
        if (validPorts.length === 0) { sileo.error({ title: t('common.error') || 'Error', description: t('servers.settings.portRequired' as any) || 'You must assign at least one port.' }); return; }
        const seen = new Set<number>();
        for (const p of validPorts) {
          if (p < 1024 || p > 65535) { sileo.error({ title: t('common.error') || 'Error', description: (t('servers.settings.portInvalid' as any) || 'Port {port} is invalid (1024-65535).').replace('{port}', String(p)) }); return; }
          if (seen.has(p)) { sileo.error({ title: t('common.error') || 'Error', description: (t('servers.settings.portDuplicate' as any) || 'Port {port} is already assigned to this server.').replace('{port}', String(p)) }); return; }
          seen.add(p);
        }
        if (validPorts[0] !== initialPrimaryRef.current && isRunning) { sileo.error({ title: t('common.error') || 'Error', description: t('servers.settings.portsRequireOffline' as any) || 'The server must be stopped to change the primary port.' }); return; }
      }
      const portsChanged = validPorts.length !== savedPortsRef.current.length || validPorts.some((p, i) => p !== savedPortsRef.current[i]);
      await saveSettings(localSettings, hasScope('server.data.edit.admin'));
      localStorage.setItem(`pluginsEnabled_${serverId}`, pluginsEnabled.toString());
      window.dispatchEvent(new CustomEvent('server:plugins-enabled-changed', { detail: pluginsEnabled }));
      if (canEditAdminData && validPorts.length > 0) {
        await api.put(`/api/servers/${serverId}/data`, { ports: validPorts, primaryPort: validPorts[0] });
      }
      if (canViewPorts && Object.keys(portNotes).length > 0) {
        await api.put(`/api/servers/${serverId}/port-settings`, { primaryPort: validPorts[0], portNotes });
      }
      initialPrimaryRef.current = validPorts[0] ?? initialPrimaryRef.current;
      savedPortsRef.current = [...validPorts];
      onPortsSaved?.();
      if (canEditAdminData && isRunning && portsChanged) {
        sileo.success({ title: t('common.success') || 'Success', description: t('servers.settings.portsApplyOnRestart' as any) || 'Port changes will take effect when the server restarts.' });
      } else {
        sileo.success({ title: t('common.success') || 'Success', description: t('servers.settings.saveSuccess' as any) || 'Settings saved successfully' });
      }
    } catch {
      sileo.error({ title: t('common.error') || 'Error', description: t('servers.settings.saveError' as any) || 'Failed to save settings' });
    } finally { setSaving(false); }
  };

  const renderVariable = (name: string, variable?: SettingVariable) => {
    if (!variable || variable.internal || isSkipped(name)) return null;
    const displayName = variable.display || name;
    const description = variable.desc;
    const isBinaryOption = variable.type === 'option' && variable.options?.length === 2 &&
      variable.options.every((o: any) => ['true', 'false', true, false].includes(o.value));
    if (isBinaryOption) {
      return (
        <div key={name} className="space-y-2">
          <Label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">{displayName}</Label>
          <RadioGroup value={String(variable.value)} onValueChange={(val) => handleVariableChange(name, val)} className="flex gap-2">
            {variable.options?.map((opt: any) => (
              <div key={String(opt.value)} className={cn(
                'flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-all',
                String(variable.value) === String(opt.value) ? 'border-primary bg-primary/10 text-primary' : 'border-border/60 bg-accent/5 text-muted-foreground hover:bg-accent/10'
              )}>
                <RadioGroupItem value={String(opt.value)} id={`${name}-${String(opt.value)}`} className="border-muted-foreground/40" />
                <Label htmlFor={`${name}-${String(opt.value)}`} className="cursor-pointer">{opt.display}</Label>
              </div>
            ))}
          </RadioGroup>
          {description && <p className="text-xs text-muted-foreground">{description}</p>}
        </div>
      );
    }
    switch (variable.type) {
      case 'boolean':
        return (
          <div key={name} className="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-accent/5 px-4 py-3 transition-colors hover:bg-accent/10">
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">{displayName}</p>
              {description && <p className="mt-0.5 text-xs text-muted-foreground truncate">{description}</p>}
            </div>
            <Switch checked={!!variable.value} onCheckedChange={(val) => handleVariableChange(name, val)} />
          </div>
        );
      case 'option':
        return (
          <div key={name} className="space-y-1.5">
            <Label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">{displayName}</Label>
            <Select value={String(variable.value)} onValueChange={(val) => handleVariableChange(name, val)}>
              <SelectTrigger className="h-9 bg-accent/5 border-border/60 text-sm">
                <SelectValue placeholder="Select" />
              </SelectTrigger>
              <SelectContent>
                {variable.options?.map((opt: any) => (
                  <SelectItem key={opt.value} value={String(opt.value)}>{opt.display}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            {description && <p className="text-xs text-muted-foreground">{description}</p>}
          </div>
        );
      case 'integer':
        return (
          <div key={name} className="space-y-1.5">
            <Label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">{displayName}</Label>
            <Input type="number" value={(variable.value as number) ?? 0} onChange={(e) => handleVariableChange(name, parseInt(e.target.value) || 0)} className="h-9 bg-accent/5 border-border/60 text-sm font-mono" />
            {description && <p className="text-xs text-muted-foreground">{description}</p>}
          </div>
        );
      default:
        return (
          <div key={name} className="space-y-1.5">
            <Label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">{displayName}</Label>
            <Input value={(variable.value as string) ?? ""} onChange={(e) => handleVariableChange(name, e.target.value)} className="h-9 bg-accent/5 border-border/60 text-sm" />
            {description && <p className="text-xs text-muted-foreground">{description}</p>}
          </div>
        );
    }
  };

  const groups = localSettings.groups || [];

  const grouplessVars = Object.keys(localSettings.variables).filter(name =>
    !groups.some(g => g.variables.includes(name)) && !isSkipped(name) && !localSettings.variables[name]?.internal
  );

  const adminMetaVars = Object.entries(localSettings.variables).filter(([name, v]) =>
    v.userEdit === false && !v.internal && !isInternalMeta(name) && !groups.some(g => g.variables.includes(name))
  );

  return (
    <div className="mt-6 space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
      <div className="flex items-center gap-4">
        <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl border border-primary/30 bg-gradient-to-br from-primary/25 via-accent/15 to-transparent text-primary shadow-lg shadow-primary/5">
          <Settings className="h-5 w-5" />
        </div>
        <div>
          <h1 className="font-headline text-xl font-bold tracking-tight">{t('servers.settings.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('servers.settings.description')}</p>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-[400px_1fr]">

        <div className="space-y-6">

          <Card className="border-border/60">
            <CardHeader className="pb-3">
              <div className="flex items-center gap-3">
                <div className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
                  <Settings className="h-4 w-4" />
                </div>
                <CardTitle className="text-sm font-bold uppercase tracking-wider">
                  {t('servers.settings.generalTitle') || 'Información General'}
                </CardTitle>
              </div>
            </CardHeader>
            <CardContent className="space-y-3 pt-0">
              <div className="space-y-1.5">
                <Label className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                  {t('servers.settings.nameLabel') || 'Server Name'}
                </Label>
                <Input
                  value={localSettings.definition?.display || ''}
                  onChange={(e) => setLocalSettings(prev => prev ? { ...prev, definition: { ...prev.definition, display: e.target.value } } : prev)}
                  className="h-9 bg-accent/5 border-border/60 text-sm"
                />
              </div>
              {localSettings.definition?.type && (
                <div className="space-y-1.5">
                  <Label className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                    {t('servers.settings.serverTypeLabel')}
                  </Label>
                  <div className="flex h-9 items-center gap-2 rounded-lg border border-border/60 bg-accent/5 px-3">
                    <Box className="h-3.5 w-3.5 text-muted-foreground" />
                    <span className="font-mono text-sm">{localSettings.definition.type}</span>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {canViewResources && (
            <Card className="overflow-hidden border-cyan-500/20">
              <div className="h-1 w-full bg-gradient-to-r from-cyan-500 via-cyan-400 to-cyan-300/0" />
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-cyan-500/10 text-cyan-500">
                      <BarChart3 className="h-4 w-4" />
                    </div>
                    <div>
                      <CardTitle className="text-sm font-bold uppercase tracking-wider">{t('servers.settings.resourcesTitle')}</CardTitle>
                      <CardDescription className="text-[11px]">{t('servers.settings.resourcesDescription')}</CardDescription>
                    </div>
                  </div>
                  <span className="flex shrink-0 items-center gap-1 rounded-full border border-amber-500/25 bg-amber-500/10 px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-wider text-amber-500">
                    <ShieldCheck className="h-3 w-3" />
                    Admin
                  </span>
                </div>
              </CardHeader>
              <CardContent className="space-y-5 pt-0">
                {[
                  { key: 'cpu', icon: Cpu, label: 'CPU', unit: '%', defaultVal: 100, hint: t('servers.settings.cpuHint'), max: 200, color: 'cyan' as const },
                  { key: 'memory', icon: MemoryStick, label: 'RAM', unit: 'MB', defaultVal: 1024, hint: t('servers.settings.memoryHint'), max: 8192, color: 'emerald' as const },
                  { key: 'disk', icon: HardDrive, label: 'Disco', unit: 'MB', defaultVal: 10240, hint: t('servers.settings.diskHint'), max: 51200, color: 'violet' as const },
                ].map(({ key, icon: Icon, label, unit, defaultVal, hint, max, color }) => {
                  const val = (localSettings.variables[key]?.value as number) ?? defaultVal;
                  const pct = Math.min((val / max) * 100, 100);
                  const colorMap = { cyan: 'text-cyan-500', emerald: 'text-emerald-500', violet: 'text-violet-500' };
                  const barMap = { cyan: 'from-cyan-500 to-cyan-400', emerald: 'from-emerald-500 to-emerald-400', violet: 'from-violet-500 to-violet-400' };
                  const bgMap = { cyan: 'bg-cyan-500/10', emerald: 'bg-emerald-500/10', violet: 'bg-violet-500/10' };
                  return (
                    <div key={key} className="rounded-xl border border-border/40 bg-accent/3 p-4 space-y-3">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2.5">
                          <div className={cn("grid h-7 w-7 shrink-0 place-items-center rounded-lg", bgMap[color], colorMap[color])}>
                            <Icon className="h-3.5 w-3.5" />
                          </div>
                          <span className="text-xs font-bold uppercase tracking-wider text-muted-foreground">{label}</span>
                        </div>
                        <div className="relative">
                          <Input type="number" value={val} onChange={(e) => handleVariableChange(key, parseInt(e.target.value) || 0)}
                            className="h-8 w-28 bg-card border-border/60 pr-9 text-right font-mono text-sm font-semibold focus:bg-accent/10 transition-colors" />
                          <span className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-[10px] font-medium text-muted-foreground/60">{unit}</span>
                        </div>
                      </div>
                      <Progress value={pct} className={cn("h-1.5 bg-muted/30 [&>div]:bg-gradient-to-r [&>div]:to-transparent", `[&>div]:${barMap[color]}`)} />
                      <p className="text-[11px] text-muted-foreground/50">{hint}</p>
                    </div>
                  );
                })}
              </CardContent>
            </Card>
          )}

          {canViewPorts && (
            <Card className="overflow-hidden border-primary/20">
              <div className="h-1 w-full bg-gradient-to-r from-primary via-primary/70 to-transparent" />
              <CardHeader className="pb-3">
                <div className="flex items-center gap-3">
                  <div className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
                    <Key className="h-4 w-4" />
                  </div>
                  <div>
                    <CardTitle className="text-sm font-bold uppercase tracking-wider">{t('servers.settings.portsTitle')}</CardTitle>
                    <CardDescription className="text-[11px]">{t('servers.settings.portsDescription')}</CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-3 pt-0">
                {isRunning && canManagePorts && (
                  <div className="flex items-center gap-2 rounded-lg border border-amber-500/25 bg-amber-500/10 px-3 py-2 text-xs text-amber-500">
                    <Power className="h-3.5 w-3.5 shrink-0" />
                    <span>{t('servers.settings.portsRunningHint' as any) || 'The server is running. Stop it to change which port is primary.'}</span>
                  </div>
                )}
                <div className={cn("gap-2 px-1 text-[10px] font-bold uppercase tracking-wider text-muted-foreground/50",
                  canManagePorts ? "grid grid-cols-[80px_1fr_140px_36px]" : "grid grid-cols-[80px_1fr_140px]"
                )}>
                  <span>{t('servers.settings.portsTypeLabel') || 'Tipo'}</span>
                  <span>{t('servers.settings.portPlaceholder') || 'Puerto'}</span>
                  <span>Nota</span>
                  {canManagePorts && <span className="text-right">{t('servers.settings.portsActionsLabel') || 'Acción'}</span>}
                </div>
                <div className="space-y-1.5">
                  {ports.map((p, idx) => (
                    <div key={idx} className={cn("items-center gap-2 rounded-lg border border-border/60 bg-accent/5 px-1 py-1.5 transition-colors hover:bg-accent/10",
                      canManagePorts ? "grid grid-cols-[80px_1fr_140px_36px]" : "grid grid-cols-[80px_1fr_140px]"
                    )}>
                      <span className={cn("ml-1 rounded px-1.5 py-0.5 text-center text-[10px] font-bold uppercase",
                        idx === 0 ? "bg-primary/15 text-primary" : "bg-muted/50 text-muted-foreground"
                      )}>
                        {idx === 0 ? 'Principal' : `#${idx + 1}`}
                      </span>
                      {canManagePorts ? (
                        <Input type="number" value={p || ''} placeholder="25565"
                          onChange={(e) => { const next = [...ports]; next[idx] = parseInt(e.target.value) || 0; setPorts(next); }}
                          className="h-8 bg-card border-border/60 font-mono text-xs focus:bg-accent/10 transition-colors" />
                      ) : (
                        <div className="flex h-8 items-center rounded-md border border-border/60 bg-card px-3 font-mono text-xs">
                          {p}
                        </div>
                      )}
                      <Input
                        value={portNotes[String(p)] || ''}
                        placeholder="Ej: Game, RCON, Query"
                        onChange={(e) => setPortNotes(prev => ({ ...prev, [String(p)]: e.target.value }))}
                        className="h-8 bg-card border-border/60 text-xs"
                      />
                      {canManagePorts && (
                        <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0 text-muted-foreground hover:text-red-500"
                          onClick={() => removePort(idx)} title={t('common.delete') || 'Delete'}>
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      )}
                    </div>
                  ))}
                </div>
                {canManagePorts && (
                  <Button variant="outline" size="sm" className="w-full" onClick={() => setPorts(prev => [...prev, 0])}>
                    <Plus className="mr-1.5 h-3.5 w-3.5" />
                    {t('servers.settings.addPortLabel')}
                  </Button>
                )}
              </CardContent>
            </Card>
          )}
        </div>

        <div className="space-y-6">

          {(hasScope('admin') || hasScope('server.admin')) && adminMetaVars.length > 0 && (
            <Card className="overflow-hidden border-amber-500/20">
              <div className="h-1 w-full bg-gradient-to-r from-amber-500 via-amber-400 to-amber-300/0" />
              <CardHeader className="pb-3">
                <div className="flex items-center gap-3">
                  <div className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-amber-500/10 text-amber-500">
                    <Wrench className="h-4 w-4" />
                  </div>
                  <div>
                    <CardTitle className="text-sm font-bold uppercase tracking-wider">
                      {t('servers.settings.metadataTitle') || 'Metadatos del Servidor'}
                    </CardTitle>
                    <CardDescription className="text-[11px]">Solo lectura</CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="pt-0">
                <div className="rounded-lg border border-border/60 divide-y divide-border/60">
                  {adminMetaVars.map(([name, variable]) => (
                    <div key={name} className="flex items-center justify-between gap-4 px-4 py-2.5 bg-accent/3 hover:bg-accent/8 transition-colors">
                      <span className="text-xs font-medium text-muted-foreground">{variable.display || name}</span>
                      <span className="font-mono text-sm text-foreground/80 truncate max-w-[60%] text-right">
                        {String(variable.value ?? '—')}
                      </span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}

          <Card className="overflow-hidden border-emerald-500/20">
            <div className="h-1 w-full bg-gradient-to-r from-emerald-500 via-emerald-400 to-emerald-300/0" />
            <CardHeader className="pb-3">
              <div className="flex items-center gap-3">
                <div className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-emerald-500/10 text-emerald-500">
                  <Clock className="h-4 w-4" />
                </div>
                <CardTitle className="text-sm font-bold uppercase tracking-wider">{t('servers.settings.autoStartTitle')}</CardTitle>
              </div>
            </CardHeader>
            <CardContent className="pt-0">
              <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
                {[
                  { flag: 'autoStart' as const, label: t('servers.settings.startOnBootLabel'), desc: t('servers.settings.startOnBootDescription') },
                  { flag: 'autoRestartOnCrash' as const, label: t('servers.settings.restartOnCrashLabel'), desc: t('servers.settings.restartOnCrashDescription') },
                  { flag: 'autoRestartOnGraceful' as const, label: t('servers.settings.restartOnStopLabel'), desc: t('servers.settings.restartOnStopDescription') },
                ].map(({ flag, label, desc }) => (
                  <div key={flag} className="flex flex-col items-center gap-2 rounded-xl border border-emerald-500/15 bg-emerald-500/5 px-3 py-4 text-center transition-colors hover:bg-emerald-500/10">
                    <Switch checked={localSettings.flags[flag]} onCheckedChange={(val) => handleFlagChange(flag, val)} />
                    <p className="text-xs font-medium leading-tight">{label}</p>
                    <p className="text-[10px] text-muted-foreground leading-tight">{desc}</p>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>

          {isMinecraftJava && (
            <Card className="overflow-hidden border-violet-500/20">
              <div className="h-1 w-full bg-gradient-to-r from-violet-500 via-violet-400 to-violet-300/0" />
              <CardHeader className="pb-3">
                <div className="flex items-center gap-3">
                  <div className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-violet-500/10 text-violet-500">
                    <Cog className="h-4 w-4" />
                  </div>
                  <CardTitle className="text-sm font-bold uppercase tracking-wider">
                    {t('servers.settings.pluginsTitle') || 'Configuración de Plugins'}
                  </CardTitle>
                </div>
              </CardHeader>
              <CardContent className="pt-0">
                <div className="flex items-center justify-between gap-3 rounded-xl border border-violet-500/15 bg-violet-500/5 px-4 py-4 transition-colors hover:bg-violet-500/10">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium">{t('servers.settings.enablePluginsLabel')}</p>
                    <p className="text-xs text-muted-foreground truncate">{t('servers.settings.enablePluginsDescription')}</p>
                  </div>
                  <Switch checked={pluginsEnabled} onCheckedChange={setPluginsEnabled} />
                </div>
              </CardContent>
            </Card>
          )}

          {groups.filter(g => g.variables.some(vn => {
            const v = localSettings.variables[vn];
            return v && !v.internal && !isSkipped(vn);
          })).map((group, idx) => (
            <Card key={idx} className="border-border/60">
              <CardHeader className="pb-3">
                <div className="flex items-center gap-3">
                  <div className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
                    <Box className="h-4 w-4" />
                  </div>
                  <div>
                    <CardTitle className="text-sm font-bold uppercase tracking-wider">{group.display}</CardTitle>
                    {group.description && <CardDescription className="text-xs">{group.description}</CardDescription>}
                  </div>
                </div>
              </CardHeader>
              <CardContent className="pt-0">
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  {group.variables.map(varName => renderVariable(varName, localSettings.variables[varName]))}
                </div>
              </CardContent>
            </Card>
          ))}

          {grouplessVars.length > 0 && (
            <Card className="border-border/60">
              <CardHeader className="pb-3">
                <div className="flex items-center gap-3">
                  <div className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
                    <Puzzle className="h-4 w-4" />
                  </div>
                  <CardTitle className="text-sm font-bold uppercase tracking-wider">
                    {t('templates.categories.NoGroup') || 'Variables'}
                  </CardTitle>
                </div>
              </CardHeader>
              <CardContent className="pt-0">
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  {grouplessVars.map(varName => renderVariable(varName, localSettings.variables[varName]))}
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      </div>

      <div className="sticky bottom-4 z-20 flex items-center justify-end rounded-xl border border-border/80 bg-card/85 px-5 py-4 shadow-[0_-8px_30px_rgb(0_0_0/0.25)] backdrop-blur-md">
        <Button size="lg" onClick={onSave} disabled={saving} className="px-10 font-bold">
          {saving ? (<><Loader2 className="mr-2 h-4 w-4 animate-spin" />{t('common.saving') || 'Saving...'}</>) : t('servers.settings.saveButton')}
        </Button>
      </div>
    </div>
  );
}
