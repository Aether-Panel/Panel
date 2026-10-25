'use client';
import { useEffect, useId, useState } from 'react';
import { AlertOctagon, Archive, BellRing, Check, Gauge, Loader2, Power, Send, Webhook, type LucideIcon } from 'lucide-react';
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
  telegramBotToken: string;
  telegramChatId: string;
  webhookUrl: string;
  webhookSecret: string;
  crash: boolean;
  backup: boolean;
  resources: boolean;
  power: boolean;
};

type EventKey = 'crash' | 'backup' | 'resources' | 'power';
type Channel = 'discord' | 'telegram' | 'webhook';

const TELEGRAM_TOKEN = /^\d{5,15}:[\w-]{30,}$/;
const TELEGRAM_CHAT = /^(-?\d{1,20}|@[A-Za-z]\w{4,31})$/;

const EMPTY: Omit<Notifications, EventKey> = { discordWebhook: '', telegramBotToken: '', telegramChatId: '', webhookUrl: '', webhookSecret: '' };

/** Trims what the owner typed, so a stray space does not count as a change or an error. */
function normalize(n: Notifications): Notifications {
  return {
    ...n,
    discordWebhook: n.discordWebhook.trim(),
    telegramBotToken: n.telegramBotToken.trim(),
    telegramChatId: n.telegramChatId.trim(),
    webhookUrl: n.webhookUrl.trim(),
  };
}

function isConfigured(n: Notifications, channel: Channel) {
  if (channel === 'discord') return n.discordWebhook.trim() !== '';
  if (channel === 'telegram') return n.telegramBotToken.trim() !== '';
  return n.webhookUrl.trim() !== '';
}

function channelError(n: Notifications, channel: Channel): string | null {
  if (channel === 'discord') {
    const hook = n.discordWebhook.trim();
    return hook === '' || WEBHOOK_PATTERN.test(hook) ? null : 'servers.notifications.webhookInvalid';
  }
  if (channel === 'telegram') {
    const token = n.telegramBotToken.trim();
    const chat = n.telegramChatId.trim();
    if (token === '' && chat === '') return null;
    return TELEGRAM_TOKEN.test(token) && TELEGRAM_CHAT.test(chat) ? null : 'servers.notifications.telegram.invalid';
  }
  const url = n.webhookUrl.trim();
  return url === '' || /^https:\/\/[^\s/@]+/.test(url) ? null : 'servers.notifications.webhook.invalid';
}

const EVENTS: { key: EventKey; icon: LucideIcon; color: string }[] = [
  { key: 'crash', icon: AlertOctagon, color: '#F2726B' },
  { key: 'backup', icon: Archive, color: '#34D399' },
  { key: 'resources', icon: Gauge, color: '#F5A524' },
  { key: 'power', icon: Power, color: '#3B9EF5' },
];

const CHANNELS: { key: Channel; icon: LucideIcon | typeof DiscordIcon; color: string }[] = [
  { key: 'discord', icon: DiscordIcon, color: '#7983F5' },
  { key: 'telegram', icon: TelegramIcon, color: '#29A9EB' },
  { key: 'webhook', icon: Webhook, color: '#34D399' },
];

const WEBHOOK_PATTERN = /^https:\/\/(?:(?:ptb|canary)\.)?discord(?:app)?\.com\/api\/webhooks\/\d+\/[\w-]+$/;

/** Discord's own dark theme, so the preview reads as a real message. */
const DISCORD_BG = '#313338';

/**
 * A server's own Discord alerts: the owner pastes a channel webhook and picks which events
 * reach it. A live preview shows what the messages look like.
 */
export default function NotificationsCard({ serverId }: Readonly<{ serverId: string }>) {
  const { t, language } = useTranslations();
  const ids = useId();
  const [saved, setSaved] = useState<Notifications | null>(null);
  const [draft, setDraft] = useState<Notifications | null>(null);
  const [hidden, setHidden] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [justSaved, setJustSaved] = useState(false);
  const [previewIndex, setPreviewIndex] = useState(0);
  const [channel, setChannel] = useState<Channel>('discord');

  useEffect(() => {
    api.get<Notifications>(`/api/servers/${serverId}/notifications`)
      .then((data) => {
        const full = { ...EMPTY, ...data };
        setSaved(full);
        setDraft(full);
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

  const errors = (['discord', 'telegram', 'webhook'] as Channel[]).map((c) => channelError(draft, c));
  const valid = errors.every((e) => e === null);
  const currentError = channelError(draft, channel);
  const dirty = JSON.stringify(normalize(draft)) !== JSON.stringify(normalize(saved));
  const anyChannel = (['discord', 'telegram', 'webhook'] as Channel[]).some((c) => isConfigured(saved, c));
  const preview = enabled.length > 0 ? enabled[previewIndex % enabled.length] : null;
  const now = new Date().toLocaleTimeString(language === 'es' ? 'es-ES' : 'en-US', { hour: '2-digit', minute: '2-digit' });

  const save = async () => {
    if (!valid) return;
    setSaving(true);
    try {
      const next = normalize(draft);
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
      const results = await api.post<Record<string, string>>(`/api/servers/${serverId}/notifications/test`, {});
      const failed = Object.entries(results ?? {}).filter(([, result]) => result !== 'ok').map(([name]) => t(`servers.notifications.channels.${name}`));
      if (failed.length === 0) {
        sileo.success({ title: t('common.success'), description: t('servers.notifications.testSent') });
      } else {
        sileo.error({ title: t('common.error'), description: t('servers.notifications.testFailedOn', { channels: failed.join(', ') }) });
      }
    } catch (err) {
      const description = err instanceof ApiError && err.status === 429 ? t('servers.notifications.testWait') : t('servers.notifications.testError');
      sileo.error({ title: t('common.error'), description });
    } finally {
      setTesting(false);
    }
  };

  let saveIcon: React.ReactNode = null;
  if (saving) saveIcon = <Loader2 className="animate-spin" aria-hidden="true" />;
  else if (justSaved) saveIcon = <Check className="animate-in zoom-in-50 duration-300" aria-hidden="true" />;

  return (
    <Card className="overflow-hidden">
      <section className="grid gap-5 p-5 md:p-6 lg:grid-cols-[minmax(0,16rem)_minmax(0,1fr)] lg:gap-10">
        <div className="space-y-3">
          <span
            aria-hidden="true"
            className="flex h-10 w-10 items-center justify-center rounded-xl text-white shadow-[0_0_24px_-4px_hsl(var(--primary)/0.7)]"
            style={{ background: 'linear-gradient(135deg, hsl(var(--primary)), #7983F5)' }}
          >
            <BellRing className="h-5 w-5" />
          </span>
          <div className="space-y-1.5">
            <h2 className="text-sm font-semibold text-foreground">{t('servers.notifications.title')}</h2>
            <p className="text-pretty text-sm text-muted-foreground">{t('servers.notifications.description')}</p>
          </div>
        </div>

        <div className="min-w-0 space-y-6">
          <div className="space-y-3">
            <div role="tablist" aria-label={t('servers.notifications.channelsLabel')} className="grid grid-cols-3 gap-2">
              {CHANNELS.map(({ key, icon: Icon, color }) => {
                const active = channel === key;
                const configured = isConfigured(draft, key);
                return (
                  <button
                    key={key}
                    type="button"
                    role="tab"
                    id={`${ids}-tab-${key}`}
                    aria-selected={active}
                    aria-controls={`${ids}-panel`}
                    onClick={() => setChannel(key)}
                    className={cn(
                      'relative flex min-w-0 items-center justify-center gap-2 rounded-lg border px-2 py-2 text-xs font-medium transition-all duration-200 ease-panel focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70 sm:text-sm',
                      active ? 'border-border bg-secondary text-foreground shadow-elevation-1' : 'border-border/50 bg-background/20 text-muted-foreground hover:border-border hover:text-foreground',
                    )}
                  >
                    <Icon className="h-4 w-4 shrink-0" style={active ? { color } : undefined} />
                    <span className="truncate">{t(`servers.notifications.channels.${key}`)}</span>
                    {configured && (
                      <span aria-label={t('servers.notifications.configured')} className="absolute right-1.5 top-1.5 h-1.5 w-1.5 rounded-full bg-success shadow-[0_0_6px_hsl(var(--success)/0.9)]" />
                    )}
                  </button>
                );
              })}
            </div>

            <div id={`${ids}-panel`} role="tabpanel" aria-labelledby={`${ids}-tab-${channel}`} key={channel} className="space-y-3 animate-in fade-in-0 duration-200">
              {channel === 'discord' && (
                <Field id={`${ids}-discord`} label={t('servers.notifications.webhookLabel')} help={t('servers.notifications.webhookHelp')} error={currentError ? t(currentError) : null}>
                  {(props) => (
                    <Input {...props} type="url" inputMode="url" placeholder="https://discord.com/api/webhooks/…" value={draft.discordWebhook} onChange={(e) => setDraft({ ...draft, discordWebhook: e.target.value })} />
                  )}
                </Field>
              )}
              {channel === 'telegram' && (
                <>
                  <Field id={`${ids}-tg-token`} label={t('servers.notifications.telegram.tokenLabel')} help={t('servers.notifications.telegram.tokenHelp')} error={currentError ? t(currentError) : null}>
                    {(props) => (
                      <Input {...props} type="password" placeholder="123456789:AA…" value={draft.telegramBotToken} onChange={(e) => setDraft({ ...draft, telegramBotToken: e.target.value })} />
                    )}
                  </Field>
                  <Field id={`${ids}-tg-chat`} label={t('servers.notifications.telegram.chatLabel')} help={t('servers.notifications.telegram.chatHelp')}>
                    {(props) => (
                      <Input {...props} placeholder="-1001234567890" value={draft.telegramChatId} onChange={(e) => setDraft({ ...draft, telegramChatId: e.target.value })} />
                    )}
                  </Field>
                </>
              )}
              {channel === 'webhook' && (
                <>
                  <Field id={`${ids}-wh-url`} label={t('servers.notifications.webhook.urlLabel')} help={t('servers.notifications.webhook.urlHelp')} error={currentError ? t(currentError) : null}>
                    {(props) => (
                      <Input {...props} type="url" inputMode="url" placeholder="https://example.com/hooks/aether" value={draft.webhookUrl} onChange={(e) => setDraft({ ...draft, webhookUrl: e.target.value })} />
                    )}
                  </Field>
                  <Field id={`${ids}-wh-secret`} label={t('servers.notifications.webhook.secretLabel')} help={t('servers.notifications.webhook.secretHelp')}>
                    {(props) => (
                      <Input {...props} type="password" value={draft.webhookSecret} onChange={(e) => setDraft({ ...draft, webhookSecret: e.target.value })} />
                    )}
                  </Field>
                </>
              )}
            </div>
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
              disabled={testing || dirty || !anyChannel}
              title={dirty ? t('servers.notifications.saveFirst') : undefined}
            >
              {testing ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Send aria-hidden="true" />}
              {t('servers.notifications.test')}
            </Button>
            <Button size="sm" onClick={save} disabled={saving || !dirty || !valid}>
              {saveIcon}
              {justSaved ? t('servers.notifications.saved') : t('servers.notifications.save')}
            </Button>
          </div>
        </div>
      </section>
    </Card>
  );
}

/** Discord's mark, drawn inline (no external requests). */
function DiscordIcon({ className, style }: Readonly<{ className?: string; style?: React.CSSProperties }>) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" className={className} style={style} aria-hidden="true">
      <path d="M20.32 4.37a19.8 19.8 0 0 0-4.89-1.52.07.07 0 0 0-.08.04c-.21.38-.44.87-.61 1.25a18.27 18.27 0 0 0-5.49 0 12.64 12.64 0 0 0-.62-1.25.08.08 0 0 0-.08-.04 19.74 19.74 0 0 0-4.88 1.52.07.07 0 0 0-.03.03C.53 9.05-.32 13.58.1 18.06a.08.08 0 0 0 .03.06 19.9 19.9 0 0 0 5.99 3.03.08.08 0 0 0 .08-.03c.46-.63.87-1.3 1.22-2a.08.08 0 0 0-.04-.1 13.1 13.1 0 0 1-1.87-.9.08.08 0 0 1 0-.13l.37-.29a.07.07 0 0 1 .08-.01c3.93 1.8 8.18 1.8 12.07 0a.07.07 0 0 1 .08.01l.37.29a.08.08 0 0 1 0 .13 12.3 12.3 0 0 1-1.87.9.08.08 0 0 0-.04.1c.36.7.77 1.37 1.22 2a.08.08 0 0 0 .08.03 19.84 19.84 0 0 0 6-3.03.08.08 0 0 0 .03-.06c.5-5.18-.84-9.67-3.55-13.66a.06.06 0 0 0-.03-.03ZM8.02 15.33c-1.18 0-2.16-1.08-2.16-2.42s.96-2.42 2.16-2.42c1.21 0 2.18 1.1 2.16 2.42 0 1.34-.96 2.42-2.16 2.42Zm7.97 0c-1.18 0-2.15-1.08-2.15-2.42s.95-2.42 2.15-2.42c1.21 0 2.18 1.1 2.16 2.42 0 1.34-.95 2.42-2.16 2.42Z" />
    </svg>
  );
}

/** Telegram's paper plane, drawn inline. */
function TelegramIcon({ className, style }: Readonly<{ className?: string; style?: React.CSSProperties }>) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" className={className} style={style} aria-hidden="true">
      <path d="M21.94 4.3a1.5 1.5 0 0 0-2.04-1.5L2.86 9.38a1.2 1.2 0 0 0 .08 2.26l4.2 1.33 1.6 5.05a1.1 1.1 0 0 0 1.84.45l2.37-2.28 4.15 3.04a1.5 1.5 0 0 0 2.34-.9L21.94 4.3ZM17.2 7.6l-7.56 6.82-.3 3.18-1.26-4.02 9.12-5.98Z" />
    </svg>
  );
}

type FieldProps = { id: string; 'aria-invalid'?: boolean; 'aria-describedby': string; autoComplete: string; spellCheck: boolean; className: string };

/** A labelled input with its help line, which turns into the error when there is one. */
function Field({ id, label, help, error, children }: Readonly<{ id: string; label: string; help: string; error?: string | null; children: (props: FieldProps) => React.ReactNode }>) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      {children({
        id,
        'aria-invalid': Boolean(error),
        'aria-describedby': `${id}-help`,
        autoComplete: 'off',
        spellCheck: false,
        className: 'h-9 font-mono text-[13px] aria-[invalid=true]:border-destructive/70 aria-[invalid=true]:focus-visible:ring-destructive/40',
      } as FieldProps & { id: string })}
      <p id={`${id}-help`} className={cn('text-xs', error ? 'text-destructive-bright' : 'text-muted-foreground')}>
        {error ?? help}
      </p>
    </div>
  );
}
