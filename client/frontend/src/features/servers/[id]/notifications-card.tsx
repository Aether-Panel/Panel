'use client';
import { useEffect, useId, useState } from 'react';
import { AlertOctagon, Archive, Check, Gauge, Loader2, Power, Send, type LucideIcon } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';

type Notifications = {
  discordWebhook: string;
  crash: boolean;
  backup: boolean;
  resources: boolean;
  power: boolean;
};

type EventKey = Exclude<keyof Notifications, 'discordWebhook'>;

const EVENTS: { key: EventKey; icon: LucideIcon; color: string }[] = [
  { key: 'crash', icon: AlertOctagon, color: '#F2726B' },
  { key: 'backup', icon: Archive, color: '#34D399' },
  { key: 'resources', icon: Gauge, color: '#F5A524' },
  { key: 'power', icon: Power, color: '#3B9EF5' },
];

const WEBHOOK_PATTERN = /^https:\/\/(?:(?:ptb|canary)\.)?discord(?:app)?\.com\/api\/webhooks\/\d+\/[\w-]+$/;

/** Discord's own dark theme, so the preview reads as a real message. */
const DISCORD_BG = '#313338';

/**
 * A server's own Discord alerts: the owner pastes a channel webhook and picks which events
 * reach it. A live preview shows what the messages look like.
 */
export default function NotificationsCard({ serverId }: { serverId: string }) {
  const { t, language } = useTranslations();
  const ids = useId();
  const [saved, setSaved] = useState<Notifications | null>(null);
  const [draft, setDraft] = useState<Notifications | null>(null);
  const [hidden, setHidden] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [justSaved, setJustSaved] = useState(false);
  const [previewIndex, setPreviewIndex] = useState(0);

  useEffect(() => {
    api.get<Notifications>(`/api/servers/${serverId}/notifications`)
      .then((data) => {
        setSaved(data);
        setDraft(data);
      })
      // no permission, or a node without notifications: the card is simply not offered
      .catch(() => setHidden(true));
  }, [serverId]);

  const enabled = draft ? EVENTS.filter((e) => draft[e.key]) : [];
  // the preview walks through the events that are switched on
  useEffect(() => {
    if (enabled.length < 2) return;
    const timer = setInterval(() => setPreviewIndex((i) => i + 1), 3200);
    return () => clearInterval(timer);
  }, [enabled.length]);

  if (hidden || !draft || !saved) return null;

  const webhook = draft.discordWebhook.trim();
  const webhookValid = webhook === '' || WEBHOOK_PATTERN.test(webhook);
  const dirty = JSON.stringify({ ...draft, discordWebhook: webhook }) !== JSON.stringify(saved);
  const preview = enabled.length > 0 ? enabled[previewIndex % enabled.length] : null;
  const now = new Date().toLocaleTimeString(language === 'es' ? 'es-ES' : 'en-US', { hour: '2-digit', minute: '2-digit' });

  const save = async () => {
    if (!webhookValid) return;
    setSaving(true);
    try {
      const next = { ...draft, discordWebhook: webhook };
      await api.put(`/api/servers/${serverId}/notifications`, next);
      setSaved(next);
      setDraft(next);
      setJustSaved(true);
      setTimeout(() => setJustSaved(false), 1800);
    } catch (err) {
      sileo.error({ title: t('common.error'), description: err instanceof ApiError && err.message ? err.message : t('servers.notifications.saveError') });
    } finally {
      setSaving(false);
    }
  };

  const sendTest = async () => {
    setTesting(true);
    try {
      await api.post(`/api/servers/${serverId}/notifications/test`, {});
      sileo.success({ title: t('common.success'), description: t('servers.notifications.testSent') });
    } catch (err) {
      const description = err instanceof ApiError && err.status === 429 ? t('servers.notifications.testWait') : t('servers.notifications.testError');
      sileo.error({ title: t('common.error'), description });
    } finally {
      setTesting(false);
    }
  };

  return (
    <Card className="overflow-hidden">
      <section className="grid gap-5 p-5 md:p-6 lg:grid-cols-[minmax(0,16rem)_minmax(0,1fr)] lg:gap-10">
        <div className="space-y-3">
          <span
            aria-hidden="true"
            className="flex h-10 w-10 items-center justify-center rounded-xl text-white shadow-[0_0_24px_-4px_rgba(88,101,242,0.7)]"
            style={{ background: 'linear-gradient(135deg, #5865F2, #7983F5)' }}
          >
            <DiscordIcon className="h-5 w-5" />
          </span>
          <div className="space-y-1.5">
            <h2 className="text-sm font-semibold text-foreground">{t('servers.notifications.title')}</h2>
            <p className="text-pretty text-sm text-muted-foreground">{t('servers.notifications.description')}</p>
          </div>
        </div>

        <div className="min-w-0 space-y-6">
          <div className="space-y-2">
            <Label htmlFor={`${ids}-webhook`}>{t('servers.notifications.webhookLabel')}</Label>
            <Input
              id={`${ids}-webhook`}
              type="url"
              inputMode="url"
              autoComplete="off"
              spellCheck={false}
              placeholder="https://discord.com/api/webhooks/…"
              value={draft.discordWebhook}
              onChange={(e) => setDraft({ ...draft, discordWebhook: e.target.value })}
              aria-invalid={!webhookValid}
              aria-describedby={`${ids}-webhook-help`}
              className="h-9 font-mono text-[13px] aria-[invalid=true]:border-destructive/70 aria-[invalid=true]:focus-visible:ring-destructive/40"
            />
            <p id={`${ids}-webhook-help`} className={cn('text-xs', webhookValid ? 'text-muted-foreground' : 'text-destructive-bright')}>
              {webhookValid ? t('servers.notifications.webhookHelp') : t('servers.notifications.webhookInvalid')}
            </p>
          </div>

          <fieldset>
            <legend className="mb-3 text-sm font-medium">{t('servers.notifications.eventsLabel')}</legend>
            <div className="grid gap-2 sm:grid-cols-2">
              {EVENTS.map(({ key, icon: Icon, color }) => (
                <label
                  key={key}
                  htmlFor={`${ids}-${key}`}
                  className={cn(
                    'flex cursor-pointer items-start gap-3 rounded-xl border p-3 transition-colors duration-200',
                    draft[key] ? 'border-border bg-secondary/40' : 'border-border/50 bg-background/20 hover:border-border',
                  )}
                >
                  <span
                    aria-hidden="true"
                    className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-border/60 transition-colors duration-200"
                    style={draft[key] ? { color, background: `${color}1f`, borderColor: `${color}55` } : undefined}
                  >
                    <Icon className={cn('h-4 w-4', !draft[key] && 'text-muted-foreground')} />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block text-sm font-medium">{t(`servers.notifications.events.${key}.label`)}</span>
                    <span className="mt-0.5 block text-xs text-muted-foreground">{t(`servers.notifications.events.${key}.description`)}</span>
                  </span>
                  <Switch id={`${ids}-${key}`} checked={draft[key]} onCheckedChange={(checked) => setDraft({ ...draft, [key]: checked })} />
                </label>
              ))}
            </div>
          </fieldset>

          {/* What a message looks like in the channel, in Discord's own colours */}
          <div>
            <p className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">{t('servers.notifications.preview')}</p>
            <div aria-hidden="true" className="rounded-xl border border-white/5 p-4" style={{ background: DISCORD_BG }}>
              {preview ? (
                <div key={`${preview.key}-${previewIndex}`} className="flex gap-3 animate-in fade-in-0 slide-in-from-bottom-1 duration-500 ease-panel">
                  <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-white" style={{ background: 'linear-gradient(135deg, #5865F2, #7983F5)' }}>
                    <DiscordIcon className="h-4 w-4" />
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="flex items-center gap-1.5 text-sm">
                      <span className="font-semibold text-white">Aether Panel</span>
                      <span className="rounded bg-[#5865F2] px-1 py-px text-[10px] font-semibold uppercase text-white">bot</span>
                      <span className="text-xs text-[#949BA4]">{now}</span>
                    </p>
                    <div className="mt-1.5 max-w-md rounded border-l-4 bg-[#2B2D31] px-3 py-2.5" style={{ borderColor: preview.color }}>
                      <p className="text-sm font-semibold text-white">{t(`servers.notifications.events.${preview.key}.sampleTitle`)}</p>
                      <p className="mt-1 text-[13px] leading-snug text-[#DBDEE1]">{t(`servers.notifications.events.${preview.key}.sampleText`)}</p>
                      {preview.key === 'crash' && (
                        <pre className="mt-2 overflow-hidden rounded bg-[#1E1F22] px-2 py-1.5 font-mono text-[11px] leading-4 text-[#B5BAC1]">java.lang.OutOfMemoryError: Java heap space</pre>
                      )}
                      <p className="mt-2 text-[11px] text-[#949BA4]">Aether Panel · {serverId}</p>
                    </div>
                  </div>
                </div>
              ) : (
                <p className="py-3 text-center text-sm text-[#949BA4]">{t('servers.notifications.previewEmpty')}</p>
              )}
            </div>
          </div>

          <div className="flex flex-wrap items-center justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={sendTest}
              disabled={testing || dirty || saved.discordWebhook === ''}
              title={dirty ? t('servers.notifications.saveFirst') : undefined}
            >
              {testing ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Send aria-hidden="true" />}
              {t('servers.notifications.test')}
            </Button>
            <Button size="sm" onClick={save} disabled={saving || !dirty || !webhookValid}>
              {saving ? <Loader2 className="animate-spin" aria-hidden="true" /> : justSaved ? <Check className="animate-in zoom-in-50 duration-300" aria-hidden="true" /> : null}
              {justSaved ? t('servers.notifications.saved') : t('servers.notifications.save')}
            </Button>
          </div>
        </div>
      </section>
    </Card>
  );
}

/** Discord's mark, drawn inline (no external requests). */
function DiscordIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" className={className} aria-hidden="true">
      <path d="M20.32 4.37a19.8 19.8 0 0 0-4.89-1.52.07.07 0 0 0-.08.04c-.21.38-.44.87-.61 1.25a18.27 18.27 0 0 0-5.49 0 12.64 12.64 0 0 0-.62-1.25.08.08 0 0 0-.08-.04 19.74 19.74 0 0 0-4.88 1.52.07.07 0 0 0-.03.03C.53 9.05-.32 13.58.1 18.06a.08.08 0 0 0 .03.06 19.9 19.9 0 0 0 5.99 3.03.08.08 0 0 0 .08-.03c.46-.63.87-1.3 1.22-2a.08.08 0 0 0-.04-.1 13.1 13.1 0 0 1-1.87-.9.08.08 0 0 1 0-.13l.37-.29a.07.07 0 0 1 .08-.01c3.93 1.8 8.18 1.8 12.07 0a.07.07 0 0 1 .08.01l.37.29a.08.08 0 0 1 0 .13 12.3 12.3 0 0 1-1.87.9.08.08 0 0 0-.04.1c.36.7.77 1.37 1.22 2a.08.08 0 0 0 .08.03 19.84 19.84 0 0 0 6-3.03.08.08 0 0 0 .03-.06c.5-5.18-.84-9.67-3.55-13.66a.06.06 0 0 0-.03-.03ZM8.02 15.33c-1.18 0-2.16-1.08-2.16-2.42s.96-2.42 2.16-2.42c1.21 0 2.18 1.1 2.16 2.42 0 1.34-.96 2.42-2.16 2.42Zm7.97 0c-1.18 0-2.15-1.08-2.15-2.42s.95-2.42 2.15-2.42c1.21 0 2.18 1.1 2.16 2.42 0 1.34-.95 2.42-2.16 2.42Z" />
    </svg>
  );
}
