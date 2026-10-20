import { Button } from '@/components/ui/button';
import { Send } from 'lucide-react';
import { SecretInput, SettingsCard, SettingsCardHeader, SettingsSection } from './general-tab';

interface DiscordTabProps {
    localSettings: Record<string, any>;
    handleUpdate: (key: string, value: any) => void;
    handleTestDiscord: () => void;
    saving: boolean;
    t: (key: string) => string;
}

const WEBHOOKS = [
    { key: 'panel.notifications.discordWebhook', id: 'webhook-alerts', prefix: 'settings.discord.alertsWebhook', title: 'settings.discord.sections.alerts' },
    { key: 'panel.notifications.discordWebhookSystem', id: 'webhook-reports', prefix: 'settings.discord.reportsWebhook', title: 'settings.discord.sections.reports' },
    { key: 'panel.notifications.discordWebhookNode', id: 'webhook-status', prefix: 'settings.discord.statusWebhook', title: 'settings.discord.sections.status' },
    { key: 'panel.notifications.discordWebhookExTransfer', id: 'webhook-extransfer', prefix: 'settings.discord.extransferWebhook', title: 'settings.discord.sections.extransfer' },
] as const;

export function DiscordTab({ localSettings, handleUpdate, handleTestDiscord, saving, t }: DiscordTabProps) {
    return (
        <SettingsCard>
            <SettingsCardHeader
                title={t('settings.discord.title')}
                description={t('settings.discord.description')}
                action={
                    <Button variant="outline" size="sm" onClick={handleTestDiscord} disabled={saving}>
                        <Send aria-hidden="true" />
                        {t('settings.discord.testButton')}
                    </Button>
                }
            />
            <div className="divide-y divide-border/50">
                {WEBHOOKS.map((w) => (
                    <SettingsSection key={w.id} title={t(w.title)} description={t(`${w.prefix}Description`)}>
                        <label htmlFor={w.id} className="sr-only">{t(`${w.prefix}Label`)}</label>
                        <SecretInput
                            id={w.id}
                            value={localSettings[w.key] || ''}
                            onChange={(v) => handleUpdate(w.key, v)}
                            placeholder={t('settings.discord.extransferWebhookPlaceholder')}
                            t={t}
                        />
                    </SettingsSection>
                ))}
            </div>
        </SettingsCard>
    );
}
