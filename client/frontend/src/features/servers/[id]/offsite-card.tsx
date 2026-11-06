'use client';
import { useCallback, useEffect, useId, useState } from 'react';
import { Check, CloudUpload, History, Loader2, Lock, RotateCcw, TriangleAlert } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useAuth } from '@/contexts/providers';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';

/** The intervals the node accepts for the periodic copy, in hours (0 = off). */
export const PERIODIC_HOURS = [0, 12, 24, 48, 72, 168, 336, 720] as const;
const MIN_KEEP = 1;
const MAX_KEEP = 50;

type Translate = (key: string, options?: Record<string, string | number>) => string;

/** An interval as people say it: off, hours up to three days, then days. */
export function periodicHoursLabel(hours: number, t: Translate): string {
  if (hours === 0) return t('servers.backups.offsite.intervalOff');
  if (hours <= 72) return t('servers.backups.offsite.intervalHours', { count: hours });
  return t('servers.backups.offsite.intervalDays', { count: hours / 24 });
}

/** A copies-to-keep value typed by hand, held to what the node accepts. */
export function clampKeep(raw: string): number {
  return Math.min(MAX_KEEP, Math.max(MIN_KEEP, Number.parseInt(raw, 10) || MIN_KEEP));
}

type OffsiteStatus = {
  periodicHours: number | null;
  periodicKeep: number | null;
  effectiveHours: number;
  effectiveKeep: number;
  configured: boolean;
  lastPeriodic: string | null;
};

/** The server's own choice; null follows the panel's setting. */
type Draft = { hours: number | null; keep: number | null };

const DEFAULT_VALUE = 'default';

/**
 * The server's periodic copy to S3: how often it goes and how many stay in the bucket, or the
 * panel's defaults. Changing it takes the right to create backups; without it the card only
 * shows the setting.
 */
export default function OffsiteCard({ serverId }: Readonly<{ serverId: string }>) {
  const { t, language } = useTranslations();
  const { hasScope } = useAuth();
  const ids = useId();
  const [status, setStatus] = useState<OffsiteStatus | null>(null);
  const [draft, setDraft] = useState<Draft>({ hours: null, keep: null });
  const [hidden, setHidden] = useState(false);
  const [saving, setSaving] = useState(false);
  const [justSaved, setJustSaved] = useState(false);
  // the panel's defaults are only seen while the server follows them; remember them after
  const [defaults, setDefaults] = useState<{ hours: number | null; keep: number | null }>({ hours: null, keep: null });
  const [serverScopes, setServerScopes] = useState<string[]>([]);

  const load = useCallback(async () => {
    const data = await api.get<OffsiteStatus>(`/api/servers/${serverId}/offsite`);
    setStatus(data);
    setDraft({ hours: data.periodicHours, keep: data.periodicKeep });
    setDefaults((known) => ({
      hours: data.periodicHours === null ? data.effectiveHours : known.hours,
      keep: data.periodicKeep === null ? data.effectiveKeep : known.keep,
    }));
  }, [serverId]);

  useEffect(() => {
    // no permission, or a node without the periodic copy: the card is simply not offered
    load().catch(() => setHidden(true));
    api.get<{ permissions?: { scopes?: unknown } }>(`/api/servers/${serverId}?perms`)
      .then((data) => {
        const raw = Array.isArray(data?.permissions?.scopes) ? data.permissions.scopes : [];
        setServerScopes(raw.map((s) => (typeof s === 'string' ? s : (s as { value?: string })?.value ?? '')));
      })
      .catch(() => setServerScopes([]));
  }, [load, serverId]);

  if (hidden || !status) return null;

  // a person's rights on this server come from their role and from the server's own permissions
  const canChange = hasScope('server.backup.create') || serverScopes.includes('server.backup.create') || serverScopes.includes('server.admin');
  const disabled = !canChange || !status.configured || saving;
  const dirty = draft.hours !== status.periodicHours || draft.keep !== status.periodicKeep;
  const locale = language === 'es' ? 'es-ES' : 'en-US';

  const save = async () => {
    setSaving(true);
    try {
      await api.put(`/api/servers/${serverId}/offsite`, { periodicHours: draft.hours, periodicKeep: draft.keep });
      await load();
      setJustSaved(true);
      setTimeout(() => setJustSaved(false), 1800);
    } catch (err) {
      sileo.error({ title: t('common.error'), description: err instanceof ApiError && err.message ? err.message : t('servers.backups.offsite.saveError') });
    } finally {
      setSaving(false);
    }
  };

  let saveIcon: React.ReactNode = null;
  if (saving) saveIcon = <Loader2 className="animate-spin" aria-hidden="true" />;
  else if (justSaved) saveIcon = <Check className="animate-in zoom-in-50 duration-300" aria-hidden="true" />;

  let note: React.ReactNode;
  if (!status.configured) {
    note = (
      <p className="flex items-start gap-2 text-xs text-warning">
        <TriangleAlert className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        {t('servers.backups.offsite.notConfigured')}
      </p>
    );
  } else if (status.lastPeriodic) {
    note = (
      <p className="flex items-start gap-2 text-xs text-muted-foreground">
        <History className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        {t('servers.backups.offsite.last', { date: new Date(status.lastPeriodic).toLocaleString(locale) })}
      </p>
    );
  } else {
    note = <p className="text-xs text-muted-foreground">{t('servers.backups.offsite.none')}</p>;
  }

  return (
    <Card className="overflow-hidden">
      <section className="grid gap-5 p-5 md:p-6 lg:grid-cols-[minmax(0,16rem)_minmax(0,1fr)] lg:gap-10">
        <div className="space-y-3">
          <div className="space-y-1.5">
            <h2 className="flex items-center gap-2 text-sm font-semibold text-foreground">
              <CloudUpload className="h-4 w-4 shrink-0 text-primary-bright" aria-hidden="true" />
              {t('servers.backups.offsite.title')}
            </h2>
            <p className="text-pretty text-sm text-muted-foreground">{t('servers.backups.offsite.description')}</p>
          </div>
          {note}
        </div>

        <div className="min-w-0 space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="min-w-0 space-y-2">
              <Label htmlFor={`${ids}-hours`}>{t('servers.backups.offsite.interval')}</Label>
              <Select
                value={draft.hours === null ? DEFAULT_VALUE : String(draft.hours)}
                onValueChange={(value) => setDraft({ ...draft, hours: value === DEFAULT_VALUE ? null : Number(value) })}
                disabled={disabled}
              >
                <SelectTrigger id={`${ids}-hours`} className="h-9">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={DEFAULT_VALUE}>
                    {defaults.hours === null
                      ? t('servers.backups.offsite.defaultPlain')
                      : t('servers.backups.offsite.defaultOption', { value: periodicHoursLabel(defaults.hours, t) })}
                  </SelectItem>
                  {PERIODIC_HOURS.map((hours) => (
                    <SelectItem key={hours} value={String(hours)}>{periodicHoursLabel(hours, t)}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="min-w-0 space-y-2">
              <Label htmlFor={`${ids}-keep`}>{t('servers.backups.offsite.keep')}</Label>
              <div className="flex items-center gap-2">
                <Input
                  id={`${ids}-keep`}
                  type="number"
                  inputMode="numeric"
                  min={MIN_KEEP}
                  max={MAX_KEEP}
                  className="h-9 w-24 tabular-nums"
                  // empty follows the panel's number, shown as the placeholder
                  value={draft.keep === null ? '' : String(draft.keep)}
                  placeholder={defaults.keep === null ? '' : String(defaults.keep)}
                  aria-describedby={`${ids}-keep-help`}
                  disabled={disabled}
                  onChange={(e) => setDraft({ ...draft, keep: e.target.value === '' ? null : clampKeep(e.target.value) })}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="min-w-0 text-muted-foreground"
                  disabled={disabled || draft.keep === null}
                  onClick={() => setDraft({ ...draft, keep: null })}
                >
                  <RotateCcw aria-hidden="true" />
                  <span className="truncate">{t('servers.backups.offsite.useDefault')}</span>
                </Button>
              </div>
              <p id={`${ids}-keep-help`} className="text-pretty text-xs text-muted-foreground">{t('servers.backups.offsite.keepHint')}</p>
            </div>
          </div>

          {canChange ? (
            <div className="flex justify-end">
              <Button size="sm" onClick={save} disabled={disabled || !dirty} className="w-full sm:w-auto">
                {saveIcon}
                {justSaved ? t('servers.backups.offsite.saved') : t('servers.backups.offsite.save')}
              </Button>
            </div>
          ) : (
            <p className="flex items-center gap-2 text-xs text-muted-foreground">
              <Lock className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              {t('servers.backups.offsite.readOnly')}
            </p>
          )}
        </div>
      </section>
    </Card>
  );
}
