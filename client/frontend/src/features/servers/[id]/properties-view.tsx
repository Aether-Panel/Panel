'use client';
import { useCallback, useEffect, useId, useMemo, useRef, useState, type ReactNode } from 'react';
import {
  AlertTriangle, Box, ChevronDown, FileQuestion, Gamepad2, Globe2, Loader2, Lock, MonitorPlay, Network,
  RotateCcw, RotateCw, Save, Search, ShieldAlert, SignalHigh, SlidersHorizontal, Users, Wrench, X, type AppIcon,
} from '@/components/icons';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { EmptyState } from '@/components/empty-state';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';
import { keyedByContent } from '@/lib/keyed';
import { MC_COLORS, parseMotdLine, parseProperties, serializeProperties, type MotdSegment, type ParsedProperties } from './server-properties';
import {
  KNOWN_KEYS, PROPERTY_FIELDS, PROPERTY_SECTIONS, fieldId, validateField, type PropertyField, type PropertySection,
} from './server-properties-catalog';

const FILE = 'server.properties';

/** A MOTD run is identified by its text and its styling together. */
const motdSegmentText = (segment: MotdSegment) => JSON.stringify(segment);

const SECTION_ICONS: Record<PropertySection, AppIcon> = {
  game: Gamepad2,
  players: Users,
  world: Globe2,
  display: MonitorPlay,
  network: Network,
  advanced: Wrench,
};

type LoadState = 'loading' | 'ready' | 'missing' | 'denied' | 'error';

/** Which empty state a failed read of the file shows. */
function loadFailure(status: number): LoadState {
  if (status === 404) return 'missing';
  if (status === 403) return 'denied';
  return 'error';
}

type Props = {
  serverId: string;
  serverName?: string;
  serverStatus?: 'online' | 'offline' | 'pending';
};

/**
 * server.properties as a form: every option with a plain explanation, grouped by what it is
 * about, with a live preview of how the server looks in the game's server list. Saving
 * rewrites only the keys that changed, so comments and unknown options survive.
 */
export default function PropertiesView({ serverId, serverName, serverStatus }: Readonly<Props>) {
  const { t } = useTranslations();
  const [state, setState] = useState<LoadState>('loading');
  const [parsed, setParsed] = useState<ParsedProperties | null>(null);
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [query, setQuery] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [saving, setSaving] = useState(false);
  const [savedWhile, setSavedWhile] = useState<'online' | 'offline' | null>(null);
  const [restarting, setRestarting] = useState(false);

  const fileUrl = `/api/servers/${serverId}/file/${FILE}`;

  const fetchText = useCallback(async () => {
    const res = await fetch(fileUrl, { credentials: 'include', headers: { Accept: 'text/plain' } });
    if (!res.ok) throw new ApiError(res.status, res.statusText);
    return res.text();
  }, [fileUrl]);

  const load = useCallback(async () => {
    setState('loading');
    try {
      const text = await fetchText();
      setParsed(parseProperties(text));
      setDraft({});
      setState('ready');
    } catch (err) {
      const status = err instanceof ApiError ? err.status : 0;
      setState(loadFailure(status));
    }
  }, [fetchText]);

  useEffect(() => {
    void load();
  }, [load]);

  const fieldsByKey = useMemo(() => new Map(PROPERTY_FIELDS.map((f) => [f.key, f])), []);

  /** What the file says now, or Minecraft's default when the key is missing */
  const original = useCallback(
    (key: string) => parsed?.values[key] ?? fieldsByKey.get(key)?.def ?? '',
    [parsed, fieldsByKey],
  );
  const valueOf = (key: string) => draft[key] ?? original(key);

  const setValue = (key: string, value: string) => {
    setSavedWhile(null);
    setDraft((prev) => {
      const next = { ...prev };
      if (value === original(key)) delete next[key];
      else next[key] = value;
      return next;
    });
  };
  const resetValue = (key: string) => setDraft(({ [key]: _, ...rest }) => rest);

  const changedKeys = Object.keys(draft);
  const errors = useMemo(() => {
    const found: Record<string, string> = {};
    for (const [key, value] of Object.entries(draft)) {
      const field = fieldsByKey.get(key);
      const problem = field ? validateField(field, value) : null;
      if (problem) found[key] = problem;
    }
    return found;
  }, [draft, fieldsByKey]);
  const hasErrors = Object.keys(errors).length > 0;
  const dirty = changedKeys.length > 0;

  const save = useCallback(async () => {
    if (!dirty || hasErrors || saving) return;
    setSaving(true);
    try {
      // apply the changes on top of the file as it is now, so an edit made meanwhile in the
      // file manager or by the game is not thrown away
      const fresh = parseProperties(await fetchText());
      const text = serializeProperties(fresh, draft);
      const res = await fetch(fileUrl, { method: 'PUT', body: text, credentials: 'include' });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(typeof data?.error === 'string' ? data.error : data?.error?.msg || t('servers.properties.saveError'));
      }
      setParsed(parseProperties(text));
      setDraft({});
      setSavedWhile(serverStatus === 'offline' ? 'offline' : 'online');
    } catch (err) {
      sileo.error({ title: t('common.error'), description: err instanceof Error && err.message ? err.message : t('servers.properties.saveError') });
    } finally {
      setSaving(false);
    }
  }, [dirty, hasErrors, saving, fetchText, draft, fileUrl, serverStatus, t]);

  // Ctrl/Cmd+S saves, like in the file editor
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's' && dirty) {
        e.preventDefault();
        void save();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [dirty, save]);

  // leaving the page with unsaved changes asks first
  useEffect(() => {
    if (!dirty) return;
    const onLeave = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener('beforeunload', onLeave);
    return () => window.removeEventListener('beforeunload', onLeave);
  }, [dirty]);

  const restart = async () => {
    setRestarting(true);
    try {
      await api.post(`/api/servers/${serverId}/restart`, {});
      sileo.success({ title: t('servers.properties.restartedTitle'), description: t('servers.properties.restarted') });
      setSavedWhile(null);
    } catch (err) {
      sileo.error({ title: t('common.error'), description: err instanceof ApiError && err.message ? err.message : t('servers.properties.restartError') });
    } finally {
      setRestarting(false);
    }
  };

  if (state === 'loading') return <PropertiesSkeleton />;
  if (state !== 'ready' || !parsed) {
    const copy = {
      missing: { icon: FileQuestion, title: t('servers.properties.missingTitle'), description: t('servers.properties.missing'), tone: 'default' as const },
      denied: { icon: ShieldAlert, title: t('servers.properties.deniedTitle'), description: t('servers.properties.denied'), tone: 'default' as const },
      error: { icon: AlertTriangle, title: t('servers.properties.errorTitle'), description: t('servers.properties.error'), tone: 'error' as const },
    }[state === 'ready' ? 'error' : state];
    return (
      <div className="mt-6">
        <EmptyState
          icon={copy.icon}
          title={copy.title}
          description={copy.description}
          tone={copy.tone}
          action={state !== 'denied' ? <Button variant="outline" onClick={() => void load()}><RotateCw aria-hidden="true" />{t('common.retry')}</Button> : undefined}
        />
      </div>
    );
  }

  const needle = query.trim().toLowerCase();
  const matches = (key: string) => {
    if (!needle) return true;
    const id = fieldId(key);
    return [key, t(`servers.properties.fields.${id}.label`), t(`servers.properties.fields.${id}.desc`)]
      .some((text) => text.toLowerCase().includes(needle));
  };
  const visible = (field: PropertyField) => (field.core || field.key in parsed.values) && matches(field.key);
  const otherKeys = Object.keys(parsed.values).filter((key) => !KNOWN_KEYS.has(key) && matches(key));
  const advancedOpen = showAdvanced || needle !== '';
  const motdVisible = matches('motd');
  const sections = PROPERTY_SECTIONS
    .filter((section) => section !== 'advanced' || advancedOpen)
    .map((section) => ({ section, fields: PROPERTY_FIELDS.filter((f) => f.section === section && visible(f)) }))
    .filter(({ section, fields }) => fields.length > 0 || (section === 'advanced' && otherKeys.length > 0));
  const advancedCount = PROPERTY_FIELDS.filter((f) => f.section === 'advanced' && (f.core || f.key in parsed.values)).length
    + Object.keys(parsed.values).filter((key) => !KNOWN_KEYS.has(key)).length;
  const nothingFound = needle !== '' && sections.length === 0 && !motdVisible;

  const row = (field: PropertyField) => (
    <PropertyRow
      key={field.key}
      field={field}
      value={valueOf(field.key)}
      changed={field.key in draft}
      inFile={field.key in parsed.values}
      error={errors[field.key]}
      onChange={(value) => setValue(field.key, value)}
      onReset={() => resetValue(field.key)}
    />
  );

  return (
    <div className="mt-6 space-y-6">
      <Card className="p-5 md:p-6">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex min-w-0 items-start gap-3">
            <span aria-hidden="true" className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-border/60 bg-card text-primary-bright">
              <SlidersHorizontal className="h-5 w-5" />
            </span>
            <div className="min-w-0 space-y-1">
              <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('servers.properties.title')}</h2>
              <p className="text-pretty text-sm text-muted-foreground">{t('servers.properties.description')}</p>
            </div>
          </div>
          <div className="relative w-full lg:w-72">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
            <Input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t('servers.properties.search')}
              aria-label={t('servers.properties.search')}
              className="h-9 pl-9"
            />
          </div>
        </div>
      </Card>

      {savedWhile && (
        <output className="flex flex-col gap-3 rounded-xl border border-success/30 bg-success/[0.06] px-4 py-3 text-sm animate-in fade-in-0 slide-in-from-top-1 duration-300 ease-panel sm:flex-row sm:items-center sm:justify-between">
          <p className="flex items-start gap-2.5">
            <RotateCw className="mt-0.5 h-4 w-4 shrink-0 text-success" aria-hidden="true" />
            <span>
              <span className="font-medium text-foreground">{t('servers.properties.savedTitle')}</span>{' '}
              <span className="text-muted-foreground">{t(savedWhile === 'online' ? 'servers.properties.savedOnline' : 'servers.properties.savedOffline')}</span>
            </span>
          </p>
          <div className="flex shrink-0 items-center gap-2 self-end sm:self-auto">
            {savedWhile === 'online' && (
              <Button size="sm" onClick={() => void restart()} disabled={restarting}>
                {restarting ? <Loader2 className="animate-spin" aria-hidden="true" /> : <RotateCw aria-hidden="true" />}
                {t('servers.properties.restartNow')}
              </Button>
            )}
            <Button size="icon" variant="ghost" className="h-8 w-8" onClick={() => setSavedWhile(null)} aria-label={t('common.close')}>
              <X aria-hidden="true" />
            </Button>
          </div>
        </output>
      )}

      {motdVisible && (
        <MotdCard
          serverId={serverId}
          serverName={serverName}
          value={valueOf('motd')}
          changed={'motd' in draft}
          maxPlayers={valueOf('max-players')}
          hidePlayers={valueOf('hide-online-players') === 'true'}
          onChange={(value) => setValue('motd', value)}
          onReset={() => resetValue('motd')}
        />
      )}

      {nothingFound ? (
        <EmptyState icon={Search} title={t('servers.properties.noMatchTitle')} description={t('servers.properties.noMatch', { query: query.trim() })} />
      ) : (
        <div className="grid gap-6 xl:grid-cols-2">
          {sections.map(({ section, fields }) => {
            const Icon = SECTION_ICONS[section];
            return (
              <Card key={section} className={cn('overflow-hidden', section === 'advanced' && 'xl:col-span-2')}>
                <div className="flex items-start gap-3 border-b border-border/50 px-5 py-4">
                  <Icon className="mt-0.5 h-4 w-4 shrink-0 text-primary-bright" aria-hidden="true" />
                  <div className="min-w-0">
                    <h3 className="text-sm font-semibold">{t(`servers.properties.sections.${section}.title`)}</h3>
                    <p className="text-xs text-muted-foreground">{t(`servers.properties.sections.${section}.description`)}</p>
                  </div>
                </div>
                <div className={cn('grid gap-1 p-2 sm:p-3', section === 'advanced' && 'md:grid-cols-2')}>
                  {fields.map(row)}
                  {section === 'advanced' && otherKeys.map((key) => (
                    <PropertyRow
                      key={key}
                      field={{ key, section: 'advanced', type: 'text', def: '' }}
                      unknown
                      value={valueOf(key)}
                      changed={key in draft}
                      inFile
                      onChange={(value) => setValue(key, value)}
                      onReset={() => resetValue(key)}
                    />
                  ))}
                </div>
              </Card>
            );
          })}
        </div>
      )}

      {!needle && (
        <div className="flex justify-center">
          <Button variant="ghost" size="sm" onClick={() => setShowAdvanced((v) => !v)} aria-expanded={showAdvanced} className="text-muted-foreground">
            <ChevronDown className={cn('transition-transform duration-200 ease-panel', showAdvanced && 'rotate-180')} aria-hidden="true" />
            {t(showAdvanced ? 'servers.properties.hideAdvanced' : 'servers.properties.showAdvanced', { count: advancedCount })}
          </Button>
        </div>
      )}

      {dirty && (
        <div className="sticky bottom-4 z-20 animate-in fade-in-0 slide-in-from-bottom-3 duration-300 ease-panel">
          <div className="mx-auto flex max-w-3xl flex-col gap-3 rounded-xl border border-border/70 bg-card/95 px-4 py-3 shadow-lg backdrop-blur supports-[backdrop-filter]:bg-card/80 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm" role="status" aria-live="polite">
              <span className="font-medium text-foreground">{t('servers.properties.unsaved', { count: changedKeys.length })}</span>
              {hasErrors ? (
                <span className="ml-2 text-destructive-bright">{t('servers.properties.fixErrors')}</span>
              ) : (
                <span className="ml-2 hidden text-muted-foreground md:inline">{t('servers.properties.saveHint')}</span>
              )}
            </p>
            <div className="flex items-center justify-end gap-2">
              <Button variant="ghost" onClick={() => setDraft({})} disabled={saving}>
                {t('servers.properties.discard')}
              </Button>
              <Button onClick={() => void save()} disabled={saving || hasErrors}>
                {saving ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Save aria-hidden="true" />}
                {t('servers.properties.save')}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/** The input that edits a field, by its type; null for a switch, which sits beside the label. */
function propertyControl({
  field, id, value, error, unknown, onChange, optionLabel,
}: {
  field: PropertyField;
  id: string;
  value: string;
  error?: string;
  unknown?: boolean;
  onChange: (value: string) => void;
  optionLabel: (option: string) => string;
}): ReactNode {
  if (field.locked) {
    return (
      <p className="flex h-9 items-center gap-2 rounded-md border border-dashed border-border/60 px-3 font-mono text-[13px] text-muted-foreground">
        <Lock className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        <span className="truncate">{value || '—'}</span>
      </p>
    );
  }
  if (field.type === 'bool') return null;
  if (field.type === 'enum') {
    const options = field.options ?? [];
    // a value from an older version (e.g. "0" for survival) stays selectable
    const all = value && !options.includes(value) ? [...options, value] : options;
    return (
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger id={id} className="h-9">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {all.map((option) => (
            <SelectItem key={option} value={option}>{optionLabel(option)}</SelectItem>
          ))}
        </SelectContent>
      </Select>
    );
  }
  return (
    <Input
      id={id}
      type={field.type === 'secret' ? 'password' : 'text'}
      inputMode={field.type === 'number' ? 'numeric' : undefined}
      autoComplete="off"
      spellCheck={false}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      aria-invalid={!!error}
      aria-describedby={`${id}-desc`}
      className={cn('h-9', (field.type === 'number' || unknown) && 'font-mono text-[13px] tabular-nums', error && 'border-destructive focus-visible:ring-destructive')}
    />
  );
}

function PropertyRow({
  field, value, changed, inFile, error, unknown, onChange, onReset,
}: Readonly<{
  field: PropertyField;
  value: string;
  changed: boolean;
  inFile: boolean;
  error?: string;
  unknown?: boolean;
  onChange: (value: string) => void;
  onReset: () => void;
}>) {
  const { t } = useTranslations();
  const id = useId();
  const fid = fieldId(field.key);
  const label = unknown ? field.key : t(`servers.properties.fields.${fid}.label`);
  const desc = unknown ? t('servers.properties.unknownField') : t(`servers.properties.fields.${fid}.desc`);
  const warning = field.warnWhen !== undefined && value === field.warnWhen ? t(`servers.properties.fields.${fid}.warning`) : null;
  const optionLabel = (option: string) => {
    const key = `servers.properties.options.${fid}.${option.replace(/[.:]/g, '_')}`;
    const text = t(key);
    return text === key ? option : text;
  };

  const control = propertyControl({ field, id, value, error, unknown, onChange, optionLabel });

  return (
    <div className={cn('group rounded-lg px-3 py-3 transition-colors duration-200 hover:bg-secondary/30', changed && 'bg-primary/[0.04]')}>
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0 space-y-0.5">
          <div className="flex items-center gap-2">
            {changed && <span aria-hidden="true" className="h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />}
            <Label htmlFor={id} className={cn('text-sm font-medium', unknown && 'font-mono text-[13px]')}>{label}</Label>
            {changed && <span className="sr-only">{t('servers.properties.changed')}</span>}
          </div>
          <p id={`${id}-desc`} className="text-pretty text-xs leading-relaxed text-muted-foreground">
            {field.locked ? t('servers.properties.locked') : desc}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {changed && (
            <Button
              size="icon"
              variant="ghost"
              className="h-7 w-7 text-muted-foreground"
              onClick={onReset}
              aria-label={t('servers.properties.undo', { name: label })}
              title={t('servers.properties.undo', { name: label })}
            >
              <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
            </Button>
          )}
          {field.type === 'bool' && !field.locked && (
            <Switch id={id} checked={value === 'true'} onCheckedChange={(checked) => onChange(checked ? 'true' : 'false')} aria-describedby={`${id}-desc`} />
          )}
        </div>
      </div>
      {control && <div className="mt-2.5">{control}</div>}
      <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1">
        {!unknown && (
          <code className="font-mono text-[11px] text-muted-foreground/70">
            {field.key}
            {!inFile && !changed && <span className="ml-1.5 font-sans">· {t('servers.properties.default')}</span>}
          </code>
        )}
        {field.newWorldsOnly && <span className="text-[11px] text-muted-foreground">{t('servers.properties.newWorldsOnly')}</span>}
      </div>
      {error && (
        <p className="mt-1.5 text-xs text-destructive-bright" role="alert">
          {t(error, { min: field.min ?? '', max: field.max ?? '' })}
        </p>
      )}
      {warning && (
        <p className="mt-1.5 flex items-start gap-1.5 text-xs text-warning">
          <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          {warning}
        </p>
      )}
    </div>
  );
}

const FORMAT_CODES = [
  { code: 'l', className: 'font-bold' },
  { code: 'o', className: 'italic' },
  { code: 'n', className: 'underline' },
  { code: 'm', className: 'line-through' },
  { code: 'r', className: '' },
];

function MotdCard({
  serverId, serverName, value, changed, maxPlayers, hidePlayers, onChange, onReset,
}: Readonly<{
  serverId: string;
  serverName?: string;
  value: string;
  changed: boolean;
  maxPlayers: string;
  hidePlayers: boolean;
  onChange: (value: string) => void;
  onReset: () => void;
}>) {
  const { t } = useTranslations();
  const id = useId();
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [iconFailed, setIconFailed] = useState(false);
  const lines = value.split('\n').slice(0, 2);

  // the server list shows two lines; a third would be cut off by the game
  const update = (next: string) => onChange(next.split('\n').slice(0, 2).join('\n'));

  const insert = (code: string) => {
    const el = textareaRef.current;
    const token = `§${code}`;
    if (!el) {
      update(value + token);
      return;
    }
    const start = el.selectionStart ?? value.length;
    const end = el.selectionEnd ?? value.length;
    update(value.slice(0, start) + token + value.slice(end));
    requestAnimationFrame(() => {
      el.focus();
      el.setSelectionRange(start + token.length, start + token.length);
    });
  };

  return (
    <Card className="overflow-hidden">
      <section className="grid gap-5 p-5 md:p-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] lg:gap-8">
        <div className="min-w-0 space-y-3">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 space-y-1">
              <div className="flex items-center gap-2">
                {changed && <span aria-hidden="true" className="h-1.5 w-1.5 rounded-full bg-primary" />}
                <Label htmlFor={id} className="text-sm font-semibold">{t('servers.properties.fields.motd.label')}</Label>
              </div>
              <p className="text-pretty text-xs leading-relaxed text-muted-foreground">{t('servers.properties.fields.motd.desc')}</p>
            </div>
            {changed && (
              <Button size="icon" variant="ghost" className="h-7 w-7 shrink-0 text-muted-foreground" onClick={onReset} aria-label={t('servers.properties.undo', { name: t('servers.properties.fields.motd.label') })}>
                <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
              </Button>
            )}
          </div>
          <Textarea
            id={id}
            ref={textareaRef}
            rows={2}
            value={value}
            onChange={(e) => update(e.target.value)}
            spellCheck={false}
            className="min-h-[4.25rem] resize-none font-mono text-[13px] leading-relaxed"
          />
          <fieldset className="flex min-w-0 flex-wrap items-center gap-1.5" aria-label={t('servers.properties.motd.palette')}>
            {Object.entries(MC_COLORS).map(([code, color]) => (
              <button
                key={code}
                type="button"
                onClick={() => insert(code)}
                title={`§${code}`}
                aria-label={t('servers.properties.motd.color', { code })}
                className="h-6 w-6 rounded-md border border-border/60 transition-transform duration-150 ease-panel hover:scale-110 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                style={{ backgroundColor: color }}
              />
            ))}
            <span aria-hidden="true" className="mx-1 h-5 w-px bg-border/70" />
            <span className="flex items-center gap-1.5">
            {FORMAT_CODES.map(({ code, className }) => (
              <button
                key={code}
                type="button"
                onClick={() => insert(code)}
                aria-label={t(`servers.properties.motd.format.${code}`)}
                title={t(`servers.properties.motd.format.${code}`)}
                className={cn('flex h-6 min-w-6 items-center justify-center rounded-md border border-border/60 px-1.5 text-xs text-foreground transition-colors hover:bg-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring', className)}
              >
                {code === 'r' ? <X className="h-3 w-3" aria-hidden="true" /> : 'A'}
              </button>
            ))}
            </span>
          </fieldset>
        </div>

        <div className="min-w-0 space-y-2">
          <p className="text-xs font-medium text-muted-foreground">{t('servers.properties.motd.preview')}</p>
          {/* the game's server list, roughly: icon, name, player count and the two MOTD lines */}
          <div className="flex gap-3 rounded-lg border border-black/40 bg-[#101012] p-3 shadow-inner">
            <span className="flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-[3px] bg-[#2a2a2e]">
              {iconFailed ? (
                <Box className="h-7 w-7 text-[#8b8b90]" aria-hidden="true" />
              ) : (
                <img
                  src={`/api/servers/${serverId}/file/server-icon.png`}
                  alt=""
                  className="h-full w-full [image-rendering:pixelated]"
                  onError={() => setIconFailed(true)}
                />
              )}
            </span>
            <div className="min-w-0 flex-1 font-mono text-[13px] leading-[1.35]">
              <div className="flex items-baseline justify-between gap-3">
                <span className="truncate text-white">{serverName || t('servers.properties.motd.serverName')}</span>
                <span className="flex shrink-0 items-center gap-1.5 text-[#aaaaaa]">
                  <span className="tabular-nums">{hidePlayers ? '???' : `0/${maxPlayers || '20'}`}</span>
                  <SignalHigh className="h-3.5 w-3.5 text-[#55ff55]" aria-hidden="true" />
                </span>
              </div>
              {keyedByContent(lines).map(({ key: lineKey, item: line }) => (
                <p key={lineKey} className="truncate text-[#aaaaaa]">
                  {keyedByContent(parseMotdLine(line), motdSegmentText).map(({ key: segmentKey, item: segment }) => (
                    <span
                      key={segmentKey}
                      style={{ color: segment.color }}
                      className={cn(segment.bold && 'font-bold', segment.italic && 'italic', segment.underline && 'underline', segment.strike && 'line-through', segment.obfuscated && 'blur-[1.5px]')}
                    >
                      {segment.text}
                    </span>
                  ))}
                  {line === '' && ' '}
                </p>
              ))}
            </div>
          </div>
          <p className="text-[11px] text-muted-foreground">{t('servers.properties.motd.previewHint')}</p>
        </div>
      </section>
    </Card>
  );
}

function PropertiesSkeleton() {
  return (
    <div className="mt-6 space-y-6" aria-busy="true">
      <Skeleton className="h-[88px] w-full rounded-xl" />
      <Skeleton className="h-44 w-full rounded-xl" />
      <div className="grid gap-6 xl:grid-cols-2">
        {[0, 1].map((i) => (
          <div key={i} className="space-y-3 rounded-xl border border-border/60 p-5">
            <Skeleton className="h-4 w-40" />
            {[0, 1, 2, 3].map((j) => <Skeleton key={j} className="h-14 w-full" />)}
          </div>
        ))}
      </div>
    </div>
  );
}
