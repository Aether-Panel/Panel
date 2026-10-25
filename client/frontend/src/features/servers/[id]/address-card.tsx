'use client';
import { useEffect, useId, useRef, useState } from 'react';
import { AlertCircle, Check, CheckCircle2, Copy, Globe, Info, Loader2, Trash2, XCircle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';
import { announceSubdomain, fetchSubdomain, playerAddress, type SubdomainInfo } from './server-address';

/** Same rule as the panel: one DNS label of 3 to 32 lowercase letters, digits and inner hyphens. */
const NAME_PATTERN = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

type Status = 'idle' | 'checking' | 'available' | 'current' | 'taken' | 'invalid' | 'reserved' | 'error';

function localStatus(name: string): Status | null {
  if (name === '') return 'idle';
  if (name.length < 3 || name.length > 32 || !NAME_PATTERN.test(name)) return 'invalid';
  return null;
}

/** The panel's error code for a refused change, mapped to the text the owner reads. */
function errorKey(err: unknown): string | null {
  if (!(err instanceof ApiError)) return null;
  const code = err.data?.error?.code as string | undefined;
  if (err.status === 429) return 'servers.subdomain.wait';
  if (code === 'ErrSubdomainTaken' || err.status === 409) return 'servers.subdomain.status.taken';
  if (code === 'ErrSubdomainReserved') return 'servers.subdomain.status.reserved';
  if (code === 'ErrSubdomainInvalid') return 'servers.subdomain.status.invalid';
  if (code === 'ErrSubdomainNoTarget') return 'servers.subdomain.noTarget';
  return null;
}

/** What the owner reads when a change is refused: the mapped code, the panel's message, or the fallback. */
function failureMessage(err: unknown, t: (key: string) => string, fallbackKey: string): string {
  const key = errorKey(err);
  if (key) return t(key);
  if (err instanceof ApiError && err.message) return err.message;
  return t(fallbackKey);
}

/**
 * The server's own address under the panel's domain (survival.play.example.com), kept in
 * Cloudflare by the panel. Hidden when the admin has not set Cloudflare up, or when the user
 * cannot change the server's settings.
 */
/** What the panel's availability check says about a name. */
function checkedStatus(res: { available: boolean; reason: string }): Status {
  if (res.available) return res.reason === 'current' ? 'current' : 'available';
  return res.reason === 'taken' || res.reason === 'reserved' ? res.reason : 'invalid';
}

/**
 * Live availability of the name being typed: local rules right away, the panel ~400 ms after
 * the last keystroke. A save that finds the name taken can set the status directly.
 */
function useNameStatus(serverId: string, info: SubdomainInfo | null, draft: string) {
  const [status, setStatus] = useState<Status>('idle');
  const checkSeq = useRef(0);

  useEffect(() => {
    if (!info) return;
    const seq = ++checkSeq.current;
    if (draft === info.subdomain && draft !== '') {
      setStatus('current');
      return;
    }
    const local = localStatus(draft);
    if (local === 'idle') {
      setStatus('idle');
      return;
    }
    if (local) {
      // a name half typed ("my-") is not flagged until the owner pauses
      setStatus('idle');
      const timer = setTimeout(() => setStatus(local), 400);
      return () => clearTimeout(timer);
    }
    setStatus('checking');
    const timer = setTimeout(() => {
      api.get<{ available: boolean; reason: string }>(`/api/servers/${serverId}/subdomain/check?name=${encodeURIComponent(draft)}`)
        .then((res) => {
          if (seq === checkSeq.current) setStatus(checkedStatus(res));
        })
        .catch(() => seq === checkSeq.current && setStatus('error'));
    }, 400);
    return () => clearTimeout(timer);
  }, [draft, info, serverId]);

  return [status, setStatus] as const;
}

export default function AddressCard({ serverId }: Readonly<{ serverId: string }>) {
  const { t } = useTranslations();
  const ids = useId();
  const [info, setInfo] = useState<SubdomainInfo | null>(null);
  const [hidden, setHidden] = useState(false);
  const [draft, setDraft] = useState('');
  const [saving, setSaving] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState(false);
  const [propagating, setPropagating] = useState(false);
  const [copied, setCopied] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [status, setStatus] = useNameStatus(serverId, info, draft);

  useEffect(() => {
    let alive = true;
    fetchSubdomain(serverId)
      .then((data) => {
        if (!alive) return;
        if (!data.enabled) {
          setHidden(true);
          return;
        }
        setInfo(data);
        setDraft(data.subdomain);
      })
      // 403 (no permission) or an older panel: the card is simply not offered
      .catch(() => alive && setHidden(true));
    return () => {
      alive = false;
    };
  }, [serverId]);


  useEffect(() => {
    if (!confirmRemove) return;
    const timer = setTimeout(() => setConfirmRemove(false), 4000);
    return () => clearTimeout(timer);
  }, [confirmRemove]);

  if (hidden || !info) return null;

  const hasAddress = info.subdomain !== '' && info.address !== '';
  const address = hasAddress ? playerAddress(info) : '';
  const busy = saving || removing;
  const canSave = !busy && status === 'available' && draft !== info.subdomain;

  const save = async () => {
    if (!canSave) return;
    setSaving(true);
    setSaveError(null);
    try {
      const next = await api.put<SubdomainInfo>(`/api/servers/${serverId}/subdomain`, { subdomain: draft });
      setInfo(next);
      setDraft(next.subdomain);
      setPropagating(true);
      announceSubdomain(serverId, next);
      sileo.success({ title: t('servers.subdomain.savedTitle'), description: t('servers.subdomain.propagation') });
    } catch (err) {
      if (errorKey(err) === 'servers.subdomain.status.taken') setStatus('taken');
      setSaveError(failureMessage(err, t, 'servers.subdomain.saveError'));
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    if (!confirmRemove) {
      setConfirmRemove(true);
      return;
    }
    setConfirmRemove(false);
    setRemoving(true);
    setSaveError(null);
    try {
      await api.delete(`/api/servers/${serverId}/subdomain`);
      const next = { ...info, subdomain: '', address: '', records: [] };
      setInfo(next);
      setDraft('');
      setPropagating(false);
      announceSubdomain(serverId, null);
      sileo.success({ title: t('servers.subdomain.removedTitle'), description: t('servers.subdomain.removedDescription') });
    } catch (err) {
      setSaveError(failureMessage(err, t, 'servers.subdomain.removeError'));
    } finally {
      setRemoving(false);
    }
  };

  const copy = () => {
    void navigator.clipboard.writeText(address);
    setCopied(true);
    setTimeout(() => setCopied(false), 1600);
    sileo.success({ title: t('servers.address.copyToastTitle'), description: t('servers.address.copyToastDescription', { address }) });
  };

  const invalidInput = status === 'invalid' || status === 'reserved' || status === 'taken';

  return (
    <Card className="overflow-hidden">
      <section className="grid gap-5 p-5 md:p-6 lg:grid-cols-[minmax(0,16rem)_minmax(0,1fr)] lg:gap-10">
        <div className="space-y-3">
          <span
            aria-hidden="true"
            className="flex h-10 w-10 items-center justify-center rounded-xl text-white shadow-[0_0_24px_-4px_hsl(var(--primary)/0.7)]"
            style={{ background: 'linear-gradient(135deg, hsl(var(--primary)), #34D399)' }}
          >
            <Globe className="h-5 w-5" />
          </span>
          <div className="space-y-1.5">
            <h2 className="text-sm font-semibold text-foreground">{t('servers.subdomain.title')}</h2>
            <p className="text-pretty text-sm text-muted-foreground">{t('servers.subdomain.description')}</p>
          </div>
        </div>

        <div className="min-w-0 space-y-5">
          {hasAddress ? (
            <div key={info.address} className="space-y-3 rounded-xl border border-border/60 bg-background/30 p-4 animate-in fade-in-0 duration-300">
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{t('servers.subdomain.currentLabel')}</p>
                  <p className="mt-1 break-all font-mono text-[15px] text-foreground">{address}</p>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  onClick={copy}
                  aria-label={t('servers.address.copyButtonTitle')}
                  title={t('servers.address.copyButtonTitle')}
                  className="h-8 w-8 shrink-0"
                >
                  {copied ? <Check className="text-success animate-in zoom-in-50 duration-200" aria-hidden="true" /> : <Copy aria-hidden="true" />}
                </Button>
              </div>
              <p className="flex items-start gap-2 text-xs text-muted-foreground">
                <Info className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                <span>{info.srv ? t('servers.subdomain.srvNote') : t('servers.subdomain.portNote', { port: info.port })}</span>
              </p>
              {propagating && (
                <output className="flex items-start gap-2 text-xs text-success animate-in fade-in-0 duration-300">
                  <CheckCircle2 className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                  <span>{t('servers.subdomain.propagation')}</span>
                </output>
              )}
              {info.records.length > 0 && (
                <details className="group text-xs">
                  <summary className="cursor-pointer select-none rounded text-muted-foreground transition-colors duration-200 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70">
                    {t('servers.subdomain.recordsLabel')}
                  </summary>
                  <ul className="mt-2 divide-y divide-border/50 rounded-lg border border-border/60">
                    {info.records.map((record) => (
                      <li key={`${record.type}-${record.name}`} className="flex flex-col gap-1 px-3 py-2 sm:flex-row sm:items-baseline sm:gap-3">
                        <span className="w-14 shrink-0 font-mono font-semibold text-foreground">{record.type}</span>
                        <span className="min-w-0 break-all font-mono text-muted-foreground">
                          {record.name} → {record.content}
                        </span>
                      </li>
                    ))}
                  </ul>
                </details>
              )}
            </div>
          ) : (
            <p className="rounded-xl border border-dashed border-border/60 bg-background/30 px-4 py-3 text-sm text-muted-foreground">
              {t('servers.subdomain.empty')}
            </p>
          )}

          <div className="space-y-2">
            <Label htmlFor={`${ids}-name`}>{hasAddress ? t('servers.subdomain.changeLabel') : t('servers.subdomain.nameLabel')}</Label>
            <div
              className={cn(
                'flex h-10 min-w-0 items-center rounded-lg border border-input bg-background transition-all duration-200 ease-panel focus-within:border-primary focus-within:ring-2 focus-within:ring-ring/70',
                invalidInput && 'border-destructive/70 focus-within:border-destructive/70 focus-within:ring-destructive/40',
                busy && 'opacity-60',
              )}
            >
              <input
                id={`${ids}-name`}
                value={draft}
                onChange={(e) => {
                  setSaveError(null);
                  setDraft(e.target.value.toLowerCase().replace(/\s+/g, '-'));
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') void save();
                }}
                maxLength={32}
                disabled={busy}
                placeholder="survival"
                autoComplete="off"
                autoCapitalize="none"
                spellCheck={false}
                aria-invalid={invalidInput}
                aria-describedby={`${ids}-status`}
                className="h-full min-w-[5rem] flex-1 rounded-l-lg bg-transparent pl-3 font-mono text-[13px] text-foreground outline-none placeholder:text-muted-foreground/60 disabled:cursor-not-allowed"
              />
              <span title={`.${info.domain}`} className="max-w-[60%] shrink truncate pr-3 font-mono text-[13px] text-muted-foreground">
                .{info.domain}
              </span>
            </div>
            <StatusLine id={`${ids}-status`} status={status} t={t} />
          </div>

          {saveError && (
            <div role="alert" className="flex items-start gap-2.5 rounded-xl border border-destructive/30 bg-destructive/[0.07] px-3 py-2.5 text-sm text-destructive-bright animate-in fade-in-0 slide-in-from-top-1 duration-300">
              <XCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
              <span className="min-w-0 break-words">{saveError}</span>
            </div>
          )}

          <div className="flex flex-wrap items-center justify-end gap-2">
            {hasAddress && (
              <Button
                variant="outline"
                size="sm"
                onClick={remove}
                disabled={busy}
                className={cn(confirmRemove && 'border-destructive/50 text-destructive-bright hover:bg-destructive/10 hover:text-destructive-bright')}
              >
                {removing ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Trash2 aria-hidden="true" />}
                {confirmRemove ? t('servers.subdomain.confirmRemove') : t('servers.subdomain.remove')}
              </Button>
            )}
            <Button size="sm" onClick={save} disabled={!canSave}>
              {saving && <Loader2 className="animate-spin" aria-hidden="true" />}
              {saving ? t('common.saving') : t('servers.subdomain.save')}
            </Button>
          </div>
        </div>
      </section>
    </Card>
  );
}

function StatusLine({ id, status, t }: Readonly<{ id: string; status: Status; t: (key: string) => string }>) {
  const content: Record<Status, { icon: React.ReactNode; text: string; className: string }> = {
    idle: { icon: null, text: t('servers.subdomain.hint'), className: 'text-muted-foreground' },
    checking: { icon: <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />, text: t('servers.subdomain.status.checking'), className: 'text-muted-foreground' },
    available: { icon: <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />, text: t('servers.subdomain.status.available'), className: 'text-success' },
    current: { icon: <Check className="h-3.5 w-3.5" aria-hidden="true" />, text: t('servers.subdomain.status.current'), className: 'text-muted-foreground' },
    taken: { icon: <XCircle className="h-3.5 w-3.5" aria-hidden="true" />, text: t('servers.subdomain.status.taken'), className: 'text-destructive-bright' },
    invalid: { icon: <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />, text: t('servers.subdomain.status.invalid'), className: 'text-destructive-bright' },
    reserved: { icon: <XCircle className="h-3.5 w-3.5" aria-hidden="true" />, text: t('servers.subdomain.status.reserved'), className: 'text-destructive-bright' },
    error: { icon: <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />, text: t('servers.subdomain.status.error'), className: 'text-muted-foreground' },
  };
  const { icon, text, className } = content[status];
  return (
    <p id={id} aria-live="polite" className={cn('flex min-h-[1.25rem] items-center gap-1.5 text-xs transition-colors duration-200', className)}>
      {icon}
      <span key={status} className="animate-in fade-in-0 duration-200">{text}</span>
    </p>
  );
}
