'use client';
import { useState } from 'react';
import { CheckCircle2, ExternalLink, Globe, Loader2, PlugZap, XCircle } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ApiError, api } from '@/lib/api-client';
import { cn } from '@/lib/utils';
import { SecretInput, SettingRow, SettingRows, SettingsCard, SettingsCardHeader, SettingsField, SettingsSection } from './general-tab';

const KEYS = {
  token: 'panel.dns.cloudflare.token',
  zoneId: 'panel.dns.cloudflare.zoneId',
  baseDomain: 'panel.dns.baseDomain',
  target: 'panel.dns.target',
  autoAssign: 'panel.dns.autoAssign',
} as const;

const ZONE_ID = /^[a-f0-9]{32}$/i;
const DOMAIN = /^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z0-9]([a-z0-9-]*[a-z0-9])?$/i;
const TOKEN_PAGE = 'https://dash.cloudflare.com/profile/api-tokens';

/** Drops leading and trailing dots (".example.com." -> "example.com") without a backtracking regex. */
function trimDots(text: string) {
  let start = 0;
  let end = text.length;
  while (start < end && text[start] === '.') start++;
  while (end > start && text[end - 1] === '.') end--;
  return text.slice(start, end);
}

type DomainsTabProps = {
  localSettings: Record<string, any>;
  handleUpdate: (key: string, value: any) => void;
  t: (key: string, options?: Record<string, string | number>) => string;
};

/**
 * Automatic subdomains: with a Cloudflare token and zone, servers get name.<base domain>
 * instead of an IP and a port. The test checks the values on screen, saved or not.
 */
export function DomainsTab({ localSettings, handleUpdate, t }: Readonly<DomainsTabProps>) {
  const [testing, setTesting] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null);
  const value = (key: string) => String(localSettings[key] ?? '');
  const domain = trimDots(value(KEYS.baseDomain).trim().toLowerCase());
  const zoneId = value(KEYS.zoneId).trim();
  const token = value(KEYS.token).trim();
  const configured = token !== '' && ZONE_ID.test(zoneId) && DOMAIN.test(domain);
  const autoAssign = Boolean(localSettings[KEYS.autoAssign]);
  const zoneInvalid = zoneId !== '' && !ZONE_ID.test(zoneId);
  const domainInvalid = domain !== '' && !DOMAIN.test(domain);

  const runTest = async () => {
    setTesting(true);
    setResult(null);
    try {
      const res = await api.post<{ zone: string }>('/api/settings/test/cloudflare', {
        [KEYS.token]: localSettings[KEYS.token] ?? '',
        [KEYS.zoneId]: localSettings[KEYS.zoneId] ?? '',
        [KEYS.baseDomain]: localSettings[KEYS.baseDomain] ?? '',
      });
      setResult({ ok: true, message: t('settings.domains.testOk', { zone: res?.zone ?? '' }) });
    } catch (err) {
      setResult({ ok: false, message: err instanceof ApiError && err.message ? err.message : t('settings.domains.testError') });
    } finally {
      setTesting(false);
    }
  };

  return (
    <SettingsCard>
      <SettingsCardHeader
        title={t('settings.domains.title')}
        description={t('settings.domains.description')}
        action={
          <Button variant="outline" size="sm" onClick={runTest} disabled={testing || !token || !zoneId}>
            {testing ? <Loader2 className="animate-spin" aria-hidden="true" /> : <PlugZap aria-hidden="true" />}
            {t('settings.domains.test')}
          </Button>
        }
      />

      {result && (
        <output
          className={cn(
            'flex items-start gap-2.5 border-b px-5 py-3 text-sm animate-in fade-in-0 slide-in-from-top-1 duration-300 md:px-6',
            result.ok ? 'border-success/30 bg-success/[0.07] text-success' : 'border-destructive/30 bg-destructive/[0.07] text-destructive-bright',
          )}
        >
          {result.ok ? <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" /> : <XCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />}
          <span className="min-w-0 break-words">{result.message}</span>
        </output>
      )}

      <div className="divide-y divide-border/50">
        <SettingsSection title={t('settings.domains.statusTitle')} description={t('settings.domains.statusDescription')}>
          <SettingRows>
            <SettingRow
              id="domains-auto"
              label={t('settings.domains.autoAssign')}
              description={t('settings.domains.autoAssignDescription')}
              checked={autoAssign}
              onCheckedChange={(v) => handleUpdate(KEYS.autoAssign, v)}
            />
          </SettingRows>
          <p className={cn('mt-3 flex items-start gap-2 text-xs transition-colors duration-300', configured ? 'text-success' : 'text-muted-foreground')}>
            <Globe className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
            <span className="min-w-0 break-words">
              {configured ? t('settings.domains.onHint', { example: `survival.${domain}` }) : t('settings.domains.offHint')}
            </span>
          </p>
        </SettingsSection>

        <SettingsSection title={t('settings.domains.domainTitle')} description={t('settings.domains.domainDescription')}>
          <div className="grid gap-4">
            <SettingsField
              id="domains-base"
              label={t('settings.domains.baseDomain')}
              hint={domainInvalid ? <span className="text-destructive-bright">{t('settings.domains.baseDomainInvalid')}</span> : t('settings.domains.baseDomainHint')}
            >
              <Input
                id="domains-base"
                value={value(KEYS.baseDomain)}
                onChange={(e) => handleUpdate(KEYS.baseDomain, e.target.value)}
                placeholder="play.example.com"
                spellCheck={false}
                autoComplete="off"
                autoCapitalize="none"
                aria-invalid={domainInvalid}
                className="font-mono text-[13px] aria-[invalid=true]:border-destructive/70"
              />
            </SettingsField>
            <SettingsField id="domains-target" label={t('settings.domains.target')} hint={t('settings.domains.targetHint')}>
              <Input
                id="domains-target"
                value={value(KEYS.target)}
                onChange={(e) => handleUpdate(KEYS.target, e.target.value)}
                placeholder="203.0.113.5"
                spellCheck={false}
                autoComplete="off"
                autoCapitalize="none"
                className="font-mono text-[13px]"
              />
            </SettingsField>
            <p className="rounded-xl border border-border/60 bg-background/30 px-4 py-3 text-pretty text-xs text-muted-foreground">
              {t('settings.domains.dnsOnlyNote')}
            </p>
          </div>
        </SettingsSection>

        <SettingsSection
          title={t('settings.domains.credentialsTitle')}
          description={
            <div className="space-y-3">
              <p>{t('settings.domains.credentialsDescription')}</p>
              <a
                href={TOKEN_PAGE}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 rounded text-xs font-medium text-primary transition-colors duration-200 hover:text-primary/80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
              >
                {t('settings.domains.openCloudflare')}
                <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
              </a>
            </div>
          }
        >
          <div className="space-y-4">
            <ol className="space-y-2 rounded-xl border border-border/60 bg-background/30 p-4 text-xs text-muted-foreground">
              {(['step1', 'step2', 'step3'] as const).map((step, index) => (
                <li key={step} className="flex gap-3">
                  <span
                    aria-hidden="true"
                    className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-border/60 font-mono text-[11px] text-foreground"
                  >
                    {index + 1}
                  </span>
                  <span className="min-w-0 text-pretty pt-0.5">{t(`settings.domains.${step}`)}</span>
                </li>
              ))}
            </ol>
            <div className="grid gap-4 sm:grid-cols-2">
              <SettingsField id="domains-token" label={t('settings.domains.token')} hint={t('settings.domains.tokenHint')}>
                <SecretInput id="domains-token" value={value(KEYS.token)} onChange={(v) => handleUpdate(KEYS.token, v)} t={t} />
              </SettingsField>
              <SettingsField
                id="domains-zone"
                label={t('settings.domains.zoneId')}
                hint={zoneInvalid ? <span className="text-destructive-bright">{t('settings.domains.zoneIdInvalid')}</span> : t('settings.domains.zoneIdHint')}
              >
                <Input
                  id="domains-zone"
                  value={value(KEYS.zoneId)}
                  onChange={(e) => handleUpdate(KEYS.zoneId, e.target.value.trim())}
                  placeholder="023e105f4ecef8ad9ca31a8372d0c353"
                  spellCheck={false}
                  autoComplete="off"
                  autoCapitalize="none"
                  maxLength={32}
                  aria-invalid={zoneInvalid}
                  className="font-mono text-[13px] aria-[invalid=true]:border-destructive/70"
                />
              </SettingsField>
            </div>
          </div>
        </SettingsSection>
      </div>
    </SettingsCard>
  );
}
