'use client';
import { useState, useEffect, useRef } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useTranslations } from '@/contexts/translations-context';
import { useServerSettings, type ServerSettings, type SettingVariable } from '@/hooks/use-server-settings';
import { sileo } from "@/lib/toast";
import { api } from '@/lib/api-client';
import { useAuth } from '@/contexts/providers';
import { cn } from '@/lib/utils';
import { Box, Loader2, Lock, Plus, X } from 'lucide-react';
import type { ReactNode } from 'react';
import NotificationsCard from './notifications-card';
import AddressCard from './address-card';
import VersionCard from './version-card';
import { useExperimental } from '@/contexts/config-context';

type SettingsViewProps = {
  serverId: string;
  serverStatus?: 'online' | 'offline' | 'pending';
  onPortsSaved?: () => void;
};

const isPort = (n: string) => n === 'port' || /^port\d+$/.test(n);
const isSkipped = (n: string) => n === 'cpu' || n === 'memory' || n === 'disk' || isPort(n);
const isInternalMeta = (n: string) => isSkipped(n) || /^resolved|^forge|^javaVersion|^build|^git/i.test(n);
// Values that are real data (versions, addresses, builds, paths) read best in mono.
const isMonoVar = (n: string) => /version|^ip$|address|build|template|path|repo|^entry$|^branch$/i.test(n);
// Only plain identifiers can be looked up as translation keys.
const isTranslatableKey = (n: string) => /^[A-Za-z0-9_-]+$/.test(n);
const toSlug = (s: string) => s.toLowerCase().replace(/[^a-z0-9]+/g, '');

export default function SettingsView({ serverId, serverStatus, onPortsSaved }: Readonly<SettingsViewProps>) {
  const { t } = useTranslations();
  const { features: experimental } = useExperimental();
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

  // Last saved state of everything on this page, to know when there is
  // something to save (the save bar only shows then) and to discard edits.
  const baseSettingsRef = useRef('');
  const baseNotesRef = useRef('{}');
  const basePluginsRef = useRef(true);

  useEffect(() => {
    if (settings) {
      setLocalSettings(JSON.parse(JSON.stringify(settings)));
      baseSettingsRef.current = JSON.stringify(settings);
      const stored = localStorage.getItem(`pluginsEnabled_${serverId}`);
      if (stored !== null) setPluginsEnabled(stored === 'true');
      basePluginsRef.current = stored === null ? true : stored === 'true';
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
            if (data?.portNotes) {
              baseNotesRef.current = JSON.stringify(data.portNotes);
              setPortNotes(data.portNotes);
            }
          }).catch(() => {});
        }
      }
    }
  }, [settings, serverId, canViewPorts]);

  if (loading) return <div className="flex h-[400px] items-center justify-center"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-label={t('common.loading')} /></div>;
  if (error || !localSettings) return (
    <div className="flex h-[400px] flex-col items-center justify-center gap-4">
      <p className="text-destructive font-medium">{t('common.error')}: {error?.message || t('servers.settings.loadError')}</p>
      <Button onClick={() => window.location.reload()}>{t('common.retry')}</Button>
    </div>
  );

  const handleVariableChange = (name: string, value: any) => {
    setLocalSettings(prev => { if (!prev) return prev; return { ...prev, variables: { ...prev.variables, [name]: { ...prev.variables[name], value } } }; });
  };
  const handleFlagChange = (flag: keyof ServerSettings['flags'], value: boolean) => {
    setLocalSettings(prev => { if (!prev) return prev; return { ...prev, flags: { ...prev.flags, [flag]: value } }; });
  };
  const removePort = (idx: number) => {
    if (idx === 0 && isRunning) { sileo.error({ title: t('common.error'), description: t('servers.settings.portsRequireOffline') }); return; }
    setPorts(prev => prev.filter((_, i) => i !== idx));
  };

  const onSave = async () => {
    try {
      setSaving(true);
      const validPorts = ports.filter(p => p > 0);
      if (canEditAdminData) {
        if (validPorts.length === 0) { sileo.error({ title: t('common.error'), description: t('servers.settings.portRequired') }); return; }
        const seen = new Set<number>();
        for (const p of validPorts) {
          if (p < 1024 || p > 65535) { sileo.error({ title: t('common.error'), description: t('servers.settings.portInvalid', { port: p }) }); return; }
          if (seen.has(p)) { sileo.error({ title: t('common.error'), description: t('servers.settings.portDuplicate', { port: p }) }); return; }
          seen.add(p);
        }
        if (validPorts[0] !== initialPrimaryRef.current && isRunning) { sileo.error({ title: t('common.error'), description: t('servers.settings.portsRequireOffline') }); return; }
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
      setPorts(validPorts);
      baseSettingsRef.current = JSON.stringify(localSettings);
      baseNotesRef.current = JSON.stringify(portNotes);
      basePluginsRef.current = pluginsEnabled;
      onPortsSaved?.();
      if (canEditAdminData && isRunning && portsChanged) {
        sileo.success({ title: t('common.success'), description: t('servers.settings.portsApplyOnRestart') });
      } else {
        sileo.success({ title: t('common.success'), description: t('servers.settings.saveSuccess') });
      }
    } catch {
      sileo.error({ title: t('common.error'), description: t('servers.settings.saveError') });
    } finally { setSaving(false); }
  };

  const isDirty =
    JSON.stringify(localSettings) !== baseSettingsRef.current ||
    ports.length !== savedPortsRef.current.length ||
    ports.some((p, i) => p !== savedPortsRef.current[i]) ||
    JSON.stringify(portNotes) !== baseNotesRef.current ||
    pluginsEnabled !== basePluginsRef.current;

  const onDiscard = () => {
    setLocalSettings(JSON.parse(baseSettingsRef.current));
    setPorts([...savedPortsRef.current]);
    setPortNotes(JSON.parse(baseNotesRef.current));
    setPluginsEnabled(basePluginsRef.current);
  };

  // Template variables and groups come from the template definition, usually
  // in English. Known ones have a translation; anything else keeps the
  // template's own text so unknown templates still read fine.
  const translateOr = (key: string, fallback: string) => {
    const translated = t(key);
    return translated === key ? fallback : translated;
  };
  const varLabel = (name: string, variable: SettingVariable) =>
    isTranslatableKey(name) ? translateOr(`servers.settings.vars.${name}`, variable.display || name) : (variable.display || name);
  const varDescription = (name: string, variable: SettingVariable) =>
    variable.desc && isTranslatableKey(name) ? translateOr(`servers.settings.varDescriptions.${name}`, variable.desc) : variable.desc;
  const optionLabel = (display: string) => {
    const slug = toSlug(display);
    return slug ? translateOr(`servers.settings.optionValues.${slug}`, display) : display;
  };
  const groupTitle = (display: string) => {
    const slug = toSlug(display);
    return slug ? translateOr(`servers.settings.groups.${slug}`, display) : display;
  };
  const groupDescription = (display: string, description?: string) => {
    const slug = toSlug(display);
    return description && slug ? translateOr(`servers.settings.groupDescriptions.${slug}`, description) : description;
  };

  const isVisibleVar = (name: string) => {
    const v = localSettings.variables[name];
    return !!v && !v.internal && !isSkipped(name);
  };
  // Booleans and two-state true/false options both render as a switch row.
  const isSwitchVar = (variable: SettingVariable) =>
    variable.type === 'boolean' || (variable.type === 'option' && variable.options?.length === 2 &&
      variable.options.every((o: any) => ['true', 'false', true, false].includes(o.value)));

  const renderSwitchRow = (name: string, variable: SettingVariable) => {
    const fieldId = `var-${name}`;
    const checked = variable.type === 'boolean' ? !!variable.value : String(variable.value) === 'true';
    return (
      <SettingRow key={name} label={varLabel(name, variable)} description={varDescription(name, variable)} htmlFor={fieldId} className="px-4 py-3.5">
        <Switch
          id={fieldId}
          checked={checked}
          onCheckedChange={(val) => handleVariableChange(name, variable.type === 'boolean' ? val : String(val))}
        />
      </SettingRow>
    );
  };

  const renderField = (name: string, variable: SettingVariable) => {
    const displayName = varLabel(name, variable);
    const description = varDescription(name, variable);
    const fieldId = `var-${name}`;
    const mono = isMonoVar(name);

    switch (variable.type) {
      case 'option':
        return (
          <div key={name} className="space-y-2">
            <Label htmlFor={fieldId} className="text-sm font-medium">{displayName}</Label>
            <Select value={String(variable.value)} onValueChange={(val) => handleVariableChange(name, val)}>
              <SelectTrigger id={fieldId} className="h-9 text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {variable.options?.map((opt: any) => (
                  <SelectItem key={opt.value} value={String(opt.value)}>{optionLabel(String(opt.display ?? opt.value))}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            {description && <p className="text-xs text-muted-foreground">{description}</p>}
          </div>
        );
      case 'integer':
        return (
          <div key={name} className="space-y-2">
            <Label htmlFor={fieldId} className="text-sm font-medium">{displayName}</Label>
            <Input id={fieldId} type="number" value={(variable.value as number) ?? 0} onChange={(e) => handleVariableChange(name, Number.parseInt(e.target.value) || 0)} className={cn('h-9 text-sm tabular-nums', mono && 'font-mono')} />
            {description && <p className="text-xs text-muted-foreground">{description}</p>}
          </div>
        );
      default:
        return (
          <div key={name} className="space-y-2">
            <Label htmlFor={fieldId} className="text-sm font-medium">{displayName}</Label>
            <Input id={fieldId} value={(variable.value as string) ?? ""} onChange={(e) => handleVariableChange(name, e.target.value)} className={cn('h-9 text-sm', mono && 'font-mono')} />
            {description && <p className="text-xs text-muted-foreground">{description}</p>}
          </div>
        );
    }
  };

  /** Fields in a two-column grid; consecutive switches share one bordered list. */
  const renderVariables = (names: string[]) => {
    const blocks: ReactNode[] = [];
    let switchRun: string[] = [];
    const flushSwitches = () => {
      if (switchRun.length === 0) return;
      blocks.push(
        <div key={`switches-${switchRun[0]}`} className="divide-y divide-border/50 rounded-lg border border-border/60 sm:col-span-2">
          {switchRun.map(n => renderSwitchRow(n, localSettings.variables[n]))}
        </div>
      );
      switchRun = [];
    };
    for (const name of names) {
      if (!isVisibleVar(name)) continue;
      const variable = localSettings.variables[name];
      if (isSwitchVar(variable)) {
        switchRun.push(name);
      } else {
        flushSwitches();
        blocks.push(renderField(name, variable));
      }
    }
    flushSwitches();
    return <div className="grid gap-5 sm:grid-cols-2">{blocks}</div>;
  };

  const groups = localSettings.groups || [];

  const grouplessVars = Object.keys(localSettings.variables).filter(name =>
    !groups.some(g => g.variables.includes(name)) && !isSkipped(name) && !localSettings.variables[name]?.internal
  );

  const adminMetaVars = Object.entries(localSettings.variables).filter(([name, v]) =>
    v.userEdit === false && !v.internal && !isInternalMeta(name) && !groups.some(g => g.variables.includes(name))
  );
  const showMetadata = (hasScope('admin') || hasScope('server.admin')) && adminMetaVars.length > 0;
  // Read-only installation values are listed once, under metadata, when that section shows.
  const editableGrouplessVars = showMetadata
    ? grouplessVars.filter(name => !adminMetaVars.some(([metaName]) => metaName === name))
    : grouplessVars;

  const visibleGroups = groups.filter(g => g.variables.some(vn => {
    const v = localSettings.variables[vn];
    return v && !v.internal && !isSkipped(vn);
  }));

  const resourceFields = [
    { key: 'cpu', label: t('servers.settings.cpuLabel'), unit: '%', defaultVal: 100, hint: t('servers.settings.cpuHint') },
    { key: 'memory', label: t('servers.settings.memoryLabel'), unit: 'MB', defaultVal: 1024, hint: t('servers.settings.memoryHint') },
    { key: 'disk', label: t('servers.settings.diskLabel'), unit: 'MB', defaultVal: 10240, hint: t('servers.settings.diskHint') },
  ];

  const autoStartFlags = [
    { flag: 'autoStart' as const, label: t('servers.settings.startOnBootLabel'), desc: t('servers.settings.startOnBootDescription') },
    { flag: 'autoRestartOnCrash' as const, label: t('servers.settings.restartOnCrashLabel'), desc: t('servers.settings.restartOnCrashDescription') },
    { flag: 'autoRestartOnGraceful' as const, label: t('servers.settings.restartOnStopLabel'), desc: t('servers.settings.restartOnStopDescription') },
  ];

  // Phones: type, port and remove on one line with the note underneath.
  const portGrid = canManagePorts
    ? 'grid-cols-[3.5rem_minmax(0,1fr)_2rem] sm:grid-cols-[4.5rem_minmax(0,7rem)_minmax(0,1fr)_2rem]'
    : 'grid-cols-[3.5rem_minmax(0,1fr)] sm:grid-cols-[4.5rem_minmax(0,7rem)_minmax(0,1fr)]';

  return (
    <div className="mt-6 space-y-4">
      <Card className="divide-y divide-border/50">
        <SettingsSection title={t('servers.settings.generalTitle')} description={t('servers.settings.generalDescription')}>
          <div className="grid gap-5 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="server-display-name" className="text-sm font-medium">{t('servers.settings.nameLabel')}</Label>
              <Input
                id="server-display-name"
                value={localSettings.definition?.display || ''}
                onChange={(e) => setLocalSettings(prev => prev ? { ...prev, definition: { ...prev.definition, display: e.target.value } } : prev)}
                className="h-9 text-sm"
              />
              <p className="text-xs text-muted-foreground">{t('servers.settings.nameDescription')}</p>
            </div>
            {localSettings.definition?.type && (
              <div className="space-y-2">
                <span className="block text-sm font-medium">{t('servers.settings.serverTypeLabel')}</span>
                <div className="flex h-9 items-center gap-2 text-sm text-muted-foreground">
                  <Box className="h-4 w-4" aria-hidden="true" />
                  <span className="font-mono text-foreground">{localSettings.definition.type}</span>
                </div>
              </div>
            )}
          </div>
        </SettingsSection>

        {canViewResources && (
          <SettingsSection
            title={t('servers.settings.resourcesTitle')}
            description={t('servers.settings.resourcesDescription')}
            aside={
              <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                <Lock className="h-3 w-3" aria-hidden="true" />
                {t('servers.settings.resourcesAdminBadge')}
              </span>
            }
          >
            <div className="grid gap-5 sm:grid-cols-3">
              {resourceFields.map(({ key, label, unit, defaultVal, hint }) => {
                const val = (localSettings.variables[key]?.value as number) ?? defaultVal;
                const id = `resource-${key}`;
                return (
                  <div key={key} className="space-y-2">
                    <Label htmlFor={id} className="text-sm font-medium">{label}</Label>
                    <div className="relative">
                      <Input
                        id={id}
                        type="number"
                        value={val}
                        onChange={(e) => handleVariableChange(key, Number.parseInt(e.target.value) || 0)}
                        className="h-9 pr-12 text-sm tabular-nums"
                      />
                      <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">{unit}</span>
                    </div>
                    <p className="text-xs text-muted-foreground">{hint}</p>
                  </div>
                );
              })}
            </div>
          </SettingsSection>
        )}

        {canViewPorts && (
          <SettingsSection title={t('servers.settings.portsTitle')} description={t('servers.settings.portsDescription')}>
            <div className="space-y-3">
              {isRunning && canManagePorts && (
                <p className="flex items-start gap-2 text-xs text-muted-foreground">
                  <span className="mt-1 h-1.5 w-1.5 shrink-0 rounded-full bg-warning" aria-hidden="true" />
                  {t('servers.settings.portsRunningHint')}
                </p>
              )}
              <div className="overflow-hidden rounded-lg border border-border/60">
                <div className={cn('hidden items-center gap-3 border-b border-border/60 bg-background/40 px-3 py-2 text-xs text-muted-foreground sm:grid', portGrid)}>
                  <span>{t('servers.settings.portsTypeLabel')}</span>
                  <span>{t('servers.settings.portLabel')}</span>
                  <span>{t('servers.settings.portNoteLabel')}</span>
                  {canManagePorts && <span className="sr-only">{t('servers.settings.portsActionsLabel')}</span>}
                </div>
                <ul className="divide-y divide-border/50">
                  {ports.map((p, idx) => (
                    <li key={idx} className={cn('grid items-center gap-x-3 gap-y-2 px-3 py-2.5 sm:py-2', portGrid)}>
                      <span className={cn('text-xs', idx === 0 ? 'font-medium text-primary-bright' : 'text-muted-foreground')}>
                        {idx === 0 ? t('servers.settings.primaryPortLabel') : `#${idx + 1}`}
                      </span>
                      {canManagePorts ? (
                        <Input
                          type="number"
                          value={p || ''}
                          placeholder="25565"
                          aria-label={`${t('servers.settings.portLabel')} ${idx + 1}`}
                          onChange={(e) => { const next = [...ports]; next[idx] = Number.parseInt(e.target.value) || 0; setPorts(next); }}
                          className="h-8 font-mono text-sm tabular-nums"
                        />
                      ) : (
                        <span className="font-mono text-sm tabular-nums">{p}</span>
                      )}
                      <Input
                        value={portNotes[String(p)] || ''}
                        placeholder={t('servers.settings.portNotePlaceholder')}
                        aria-label={`${t('servers.settings.portNoteLabel')} ${idx + 1}`}
                        onChange={(e) => setPortNotes(prev => ({ ...prev, [String(p)]: e.target.value }))}
                        className={cn('h-8 text-sm max-sm:col-start-2 max-sm:row-start-2', canManagePorts && 'max-sm:col-span-2')}
                      />
                      {canManagePorts && (
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright max-sm:col-start-3 max-sm:row-start-1"
                          onClick={() => removePort(idx)}
                          aria-label={t('servers.settings.removePort')}
                        >
                          <X className="h-4 w-4" />
                        </Button>
                      )}
                    </li>
                  ))}
                </ul>
              </div>
              {canManagePorts && (
                <Button variant="ghost" size="sm" className="h-8 px-2 text-muted-foreground hover:text-foreground" onClick={() => setPorts(prev => [...prev, 0])}>
                  <Plus className="h-4 w-4" />
                  {t('servers.settings.addPortLabel')}
                </Button>
              )}
            </div>
          </SettingsSection>
        )}

        <SettingsSection title={t('servers.settings.autoStartTitle')} description={t('servers.settings.autoStartDescription')}>
          <div className="divide-y divide-border/50 rounded-lg border border-border/60">
            {autoStartFlags.map(({ flag, label, desc }) => (
              <SettingRow key={flag} label={label} description={desc} htmlFor={`flag-${flag}`} className="px-4 py-3.5">
                <Switch id={`flag-${flag}`} checked={localSettings.flags[flag]} onCheckedChange={(val) => handleFlagChange(flag, val)} />
              </SettingRow>
            ))}
          </div>
        </SettingsSection>

        {isMinecraftJava && (
          <SettingsSection title={t('servers.settings.pluginsTitle')} description={t('servers.settings.pluginsDescription')}>
            <div className="rounded-lg border border-border/60">
              <SettingRow
                label={t('servers.settings.enablePluginsLabel')}
                description={t('servers.settings.enablePluginsDescription')}
                htmlFor="plugins-enabled"
                className="px-4 py-3.5"
              >
                <Switch id="plugins-enabled" checked={pluginsEnabled} onCheckedChange={setPluginsEnabled} />
              </SettingRow>
            </div>
          </SettingsSection>
        )}

        {visibleGroups.map((group, idx) => (
          <SettingsSection key={idx} title={groupTitle(group.display)} description={groupDescription(group.display, group.description)}>
            {renderVariables(group.variables)}
          </SettingsSection>
        ))}

        {editableGrouplessVars.length > 0 && (
          <SettingsSection title={t('servers.settings.variablesTitle')} description={t('servers.settings.variablesDescription')}>
            {renderVariables(editableGrouplessVars)}
          </SettingsSection>
        )}

        {showMetadata && (
          <SettingsSection title={t('servers.settings.metadataTitle')} description={t('servers.settings.metadataDescription')}>
            <dl className="divide-y divide-border/50 rounded-lg border border-border/60">
              {adminMetaVars.map(([name, variable]) => (
                <div key={name} className="flex items-center justify-between gap-4 px-4 py-2.5">
                  <dt className="min-w-0 text-sm text-muted-foreground">{varLabel(name, variable)}</dt>
                  <dd className={cn('max-w-[60%] truncate text-right text-sm text-foreground', isMonoVar(name) && 'font-mono')}>{String(variable.value ?? '—')}</dd>
                </div>
              ))}
            </dl>
          </SettingsSection>
        )}
      </Card>

      {experimental.versionSwitch && <VersionCard serverId={serverId} />}

      <AddressCard serverId={serverId} />

      <NotificationsCard serverId={serverId} />

      {isDirty && (
        <div
          role="region"
          aria-label={t('servers.settings.unsavedChanges')}
          className="sticky bottom-4 z-20 flex items-center justify-between gap-3 rounded-xl border border-border/70 bg-popover/95 px-4 py-3 shadow-elevation-3 backdrop-blur-md animate-in fade-in slide-in-from-bottom-2 duration-200 sm:px-5"
        >
          <span className="text-sm text-muted-foreground">{t('servers.settings.unsavedChanges')}</span>
          <div className="flex gap-2">
            <Button variant="ghost" size="sm" onClick={onDiscard} disabled={saving}>
              {t('servers.settings.discard')}
            </Button>
            <Button size="sm" onClick={onSave} disabled={saving}>
              {saving && <Loader2 className="animate-spin" />}
              {saving ? t('common.saving') : t('servers.settings.saveButton')}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * One block of the settings page: what it is and why on the left, the
 * controls on the right; stacked on narrow screens.
 */
function SettingsSection({
  title,
  description,
  aside,
  children,
}: Readonly<{
  title: string;
  description?: string;
  aside?: ReactNode;
  children: ReactNode;
}>) {
  return (
    <section className="grid gap-5 p-5 md:p-6 lg:grid-cols-[minmax(0,16rem)_minmax(0,1fr)] lg:gap-10">
      <div className="space-y-1.5">
        <h2 className="text-sm font-semibold text-foreground">{title}</h2>
        {description && <p className="text-pretty text-sm text-muted-foreground">{description}</p>}
        {aside && <div className="pt-1">{aside}</div>}
      </div>
      <div className="min-w-0">{children}</div>
    </section>
  );
}

/** A label + description on the left and a control (usually a switch) on the right. */
function SettingRow({
  label,
  description,
  htmlFor,
  className,
  children,
}: Readonly<{
  label: string;
  description?: string;
  htmlFor: string;
  className?: string;
  children: ReactNode;
}>) {
  return (
    <div className={cn('flex items-center justify-between gap-4', className)}>
      <div className="min-w-0">
        <Label htmlFor={htmlFor} className="text-sm font-medium">{label}</Label>
        {description && <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>}
      </div>
      {children}
    </div>
  );
}
