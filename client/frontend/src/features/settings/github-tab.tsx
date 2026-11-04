import { ExternalLink } from '@/components/icons';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { SettingsCard, SettingsCardHeader, SettingsField, SettingsSection } from './general-tab';

const CLIENT_ID_KEY = 'panel.github.clientId';
/** Where an OAuth App is created */
const NEW_APP_PAGE = 'https://github.com/settings/applications/new';
/** An OAuth App's Client ID: "Ov23li…" now, a 20-character hex string on older apps */
const CLIENT_ID = /^[A-Za-z0-9.]{16,64}$/;

interface GitHubTabProps {
    localSettings: Record<string, any>;
    handleUpdate: (key: string, value: any) => void;
    t: (key: string, options?: Record<string, string | number>) => string;
}

/**
 * The OAuth App users connect their GitHub account through, to install repositories into their
 * servers. The device flow needs only the Client ID: no secret and no callback URL.
 */
export function GitHubTab({ localSettings, handleUpdate, t }: Readonly<GitHubTabProps>) {
    const clientId = String(localSettings[CLIENT_ID_KEY] ?? '');
    const invalid = clientId !== '' && !CLIENT_ID.test(clientId);
    const configured = clientId !== '' && !invalid;

    return (
        <SettingsCard>
            <SettingsCardHeader title={t('settings.github.title')} description={t('settings.github.description')} />
            <div className="divide-y divide-border/50">
                <SettingsSection
                    title={t('settings.github.appTitle')}
                    description={
                        <div className="space-y-3">
                            <p>{t('settings.github.appDescription')}</p>
                            <a
                                href={NEW_APP_PAGE}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="inline-flex items-center gap-1.5 rounded text-xs font-medium text-primary transition-colors duration-200 hover:text-primary/80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
                            >
                                {t('settings.github.openGitHub')}
                                <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
                            </a>
                        </div>
                    }
                >
                    <div className="space-y-4">
                        <ol className="space-y-2 rounded-xl border border-border/60 bg-background/30 p-4 text-xs text-muted-foreground">
                            {(['step1', 'step2', 'step3', 'step4'] as const).map((step, index) => (
                                <li key={step} className="flex gap-3">
                                    <span
                                        aria-hidden="true"
                                        className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-border/60 font-mono text-[11px] text-foreground"
                                    >
                                        {index + 1}
                                    </span>
                                    <span className="min-w-0 text-pretty pt-0.5 [overflow-wrap:anywhere]">{t(`settings.github.${step}`)}</span>
                                </li>
                            ))}
                        </ol>
                        <SettingsField
                            id="github-client-id"
                            label={t('settings.github.clientId')}
                            hint={invalid ? <span className="text-destructive-bright">{t('settings.github.clientIdInvalid')}</span> : t('settings.github.clientIdHint')}
                        >
                            <Input
                                id="github-client-id"
                                value={clientId}
                                onChange={(e) => handleUpdate(CLIENT_ID_KEY, e.target.value.trim())}
                                placeholder="Ov23liAbCdEfGhIjKlMn"
                                spellCheck={false}
                                autoComplete="off"
                                autoCapitalize="none"
                                maxLength={64}
                                aria-invalid={invalid}
                                className="font-mono text-[13px] md:text-[13px]"
                            />
                        </SettingsField>
                        <p className={cn('text-xs', configured ? 'text-success' : 'text-muted-foreground')}>
                            {configured ? t('settings.github.onHint') : t('settings.github.offHint')}
                        </p>
                    </div>
                </SettingsSection>
            </div>
        </SettingsCard>
    );
}
