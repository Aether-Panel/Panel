'use client';
import { useState } from 'react';
import { CheckCircle2, CloudUpload, Loader2, PlugZap, XCircle } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ApiError, api } from '@/lib/api-client';
import { cn } from '@/lib/utils';
import { SecretInput, SettingRow, SettingRows, SettingsCard, SettingsCardHeader, SettingsField, SettingsSection } from './general-tab';

const KEYS = {
  enabled: 'daemon.backups.s3.enabled',
  endpoint: 'daemon.backups.s3.endpoint',
  region: 'daemon.backups.s3.region',
  bucket: 'daemon.backups.s3.bucket',
  accessKey: 'daemon.backups.s3.accessKey',
  secretKey: 'daemon.backups.s3.secretKey',
  prefix: 'daemon.backups.s3.prefix',
  insecure: 'daemon.backups.s3.insecure',
} as const;

/** Well-known providers, to fill the endpoint in one click. */
const PROVIDERS = [
  { name: 'Backblaze B2', endpoint: 's3.us-west-004.backblazeb2.com', region: 'us-west-004' },
  { name: 'Cloudflare R2', endpoint: 'https://<account>.r2.cloudflarestorage.com', region: 'auto' },
  { name: 'AWS S3', endpoint: 's3.eu-west-1.amazonaws.com', region: 'eu-west-1' },
  { name: 'Wasabi', endpoint: 's3.eu-central-1.wasabisys.com', region: 'eu-central-1' },
];

type OffsiteTabProps = {
  localSettings: Record<string, any>;
  handleUpdate: (key: string, value: any) => void;
  t: (key: string, options?: Record<string, string | number>) => string;
};

/**
 * Off-site backups: every finished backup is also copied to an S3-compatible bucket, so a
 * dead disk does not take the backups with it. The test checks the values on screen, saved
 * or not.
 */
export function OffsiteTab({ localSettings, handleUpdate, t }: Readonly<OffsiteTabProps>) {
  const [testing, setTesting] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null);
  const value = (key: string) => (localSettings[key] ?? '') as string;
  const enabled = Boolean(localSettings[KEYS.enabled]);

  const runTest = async () => {
    setTesting(true);
    setResult(null);
    try {
      await api.post('/api/settings/test/s3', Object.fromEntries(Object.values(KEYS).map((key) => [key, localSettings[key]])));
      setResult({ ok: true, message: t('settings.offsite.testOk') });
    } catch (err) {
      setResult({ ok: false, message: err instanceof ApiError && err.message ? err.message : t('settings.offsite.testError') });
    } finally {
      setTesting(false);
    }
  };

  return (
    <SettingsCard>
      <SettingsCardHeader
        title={t('settings.offsite.title')}
        description={t('settings.offsite.description')}
        action={
          <Button variant="outline" size="sm" onClick={runTest} disabled={testing || !value(KEYS.endpoint) || !value(KEYS.bucket)}>
            {testing ? <Loader2 className="animate-spin" aria-hidden="true" /> : <PlugZap aria-hidden="true" />}
            {t('settings.offsite.test')}
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
          <span>{result.message}</span>
        </output>
      )}

      <div className="divide-y divide-border/50">
        <SettingsSection title={t('settings.offsite.statusTitle')} description={t('settings.offsite.statusDescription')}>
          <SettingRows>
            <SettingRow
              id="offsite-enabled"
              label={t('settings.offsite.enabled')}
              description={t('settings.offsite.enabledDescription')}
              checked={enabled}
              onCheckedChange={(v) => handleUpdate(KEYS.enabled, v)}
            />
          </SettingRows>
          <p className={cn('mt-3 flex items-center gap-2 text-xs transition-colors duration-300', enabled ? 'text-success' : 'text-muted-foreground')}>
            <CloudUpload className="h-3.5 w-3.5" aria-hidden="true" />
            {enabled ? t('settings.offsite.onHint') : t('settings.offsite.offHint')}
          </p>
        </SettingsSection>

        <SettingsSection title={t('settings.offsite.bucketTitle')} description={t('settings.offsite.bucketDescription')}>
          <div className="space-y-4">
            <div className="flex flex-wrap gap-2" aria-label={t('settings.offsite.providers')}>
              {PROVIDERS.map((provider) => (
                <button
                  key={provider.name}
                  type="button"
                  onClick={() => {
                    handleUpdate(KEYS.endpoint, provider.endpoint);
                    handleUpdate(KEYS.region, provider.region);
                  }}
                  className="rounded-full border border-border/60 bg-background/30 px-3 py-1 text-xs font-medium text-muted-foreground transition-colors duration-200 hover:border-primary/40 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
                >
                  {provider.name}
                </button>
              ))}
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <SettingsField id="offsite-endpoint" label={t('settings.offsite.endpoint')} hint={t('settings.offsite.endpointHint')} className="sm:col-span-2">
                <Input id="offsite-endpoint" value={value(KEYS.endpoint)} onChange={(e) => handleUpdate(KEYS.endpoint, e.target.value)} placeholder="s3.us-west-004.backblazeb2.com" spellCheck={false} autoComplete="off" className="font-mono text-[13px]" />
              </SettingsField>
              <SettingsField id="offsite-bucket" label={t('settings.offsite.bucket')}>
                <Input id="offsite-bucket" value={value(KEYS.bucket)} onChange={(e) => handleUpdate(KEYS.bucket, e.target.value)} placeholder="mis-copias" spellCheck={false} autoComplete="off" className="font-mono text-[13px]" />
              </SettingsField>
              <SettingsField id="offsite-region" label={t('settings.offsite.region')} hint={t('settings.offsite.regionHint')}>
                <Input id="offsite-region" value={value(KEYS.region)} onChange={(e) => handleUpdate(KEYS.region, e.target.value)} placeholder="us-west-004" spellCheck={false} autoComplete="off" className="font-mono text-[13px]" />
              </SettingsField>
              <SettingsField id="offsite-prefix" label={t('settings.offsite.prefix')} hint={t('settings.offsite.prefixHint')} className="sm:col-span-2">
                <Input id="offsite-prefix" value={value(KEYS.prefix)} onChange={(e) => handleUpdate(KEYS.prefix, e.target.value)} placeholder="aether-backups" spellCheck={false} autoComplete="off" className="font-mono text-[13px]" />
              </SettingsField>
            </div>
          </div>
        </SettingsSection>

        <SettingsSection title={t('settings.offsite.keysTitle')} description={t('settings.offsite.keysDescription')}>
          <div className="grid gap-4 sm:grid-cols-2">
            <SettingsField id="offsite-access" label={t('settings.offsite.accessKey')}>
              <Input id="offsite-access" value={value(KEYS.accessKey)} onChange={(e) => handleUpdate(KEYS.accessKey, e.target.value)} spellCheck={false} autoComplete="off" className="font-mono text-[13px]" />
            </SettingsField>
            <SettingsField id="offsite-secret" label={t('settings.offsite.secretKey')}>
              <SecretInput id="offsite-secret" value={value(KEYS.secretKey)} onChange={(v) => handleUpdate(KEYS.secretKey, v)} t={t} />
            </SettingsField>
          </div>
          <div className="mt-4">
            <SettingRows>
              <SettingRow
                id="offsite-insecure"
                label={t('settings.offsite.insecure')}
                description={t('settings.offsite.insecureDescription')}
                checked={Boolean(localSettings[KEYS.insecure])}
                onCheckedChange={(v) => handleUpdate(KEYS.insecure, v)}
              />
            </SettingRows>
          </div>
        </SettingsSection>
      </div>
    </SettingsCard>
  );
}
