import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Loader2 } from 'lucide-react';
import { SettingsSection } from './general-tab';

interface DiscordTabProps {
    localSettings: Record<string, any>;
    handleUpdate: (key: string, value: any) => void;
    handleSave: () => void;
    handleTestDiscord: () => void;
    saving: boolean;
    t: (key: string) => string;
}

export function DiscordTab({ localSettings, handleUpdate, handleSave, handleTestDiscord, saving, t }: DiscordTabProps) {
    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-xl font-semibold tracking-tight">{t('settings.discord.title')}</h2>
                <p className="text-sm text-muted-foreground mt-1">{t('settings.discord.description')}</p>
            </div>
            
            <div className="rounded-xl border border-border/60 bg-card shadow-sm">
                <div className="px-6">
                    <SettingsSection
                        id="webhook-alerts"
                        title={t('settings.discord.alertsWebhookLabel')}
                        description={t('settings.discord.alertsWebhookDescription')}
                    >
                        <Input
                            id="webhook-alerts"
                            value={localSettings['panel.notifications.discordWebhook'] || ''}
                            onChange={(e) => handleUpdate('panel.notifications.discordWebhook', e.target.value)}
                            placeholder={t('settings.discord.alertsWebhookPlaceholder')}
                            className="bg-background/50"
                        />
                    </SettingsSection>
                    
                    <SettingsSection
                        id="webhook-reports"
                        title={t('settings.discord.reportsWebhookLabel')}
                        description={t('settings.discord.reportsWebhookDescription')}
                    >
                        <Input
                            id="webhook-reports"
                            value={localSettings['panel.notifications.discordWebhookSystem'] || ''}
                            onChange={(e) => handleUpdate('panel.notifications.discordWebhookSystem', e.target.value)}
                            placeholder={t('settings.discord.reportsWebhookPlaceholder')}
                            className="bg-background/50"
                        />
                    </SettingsSection>
                    
                    <SettingsSection
                        id="webhook-status"
                        title={t('settings.discord.statusWebhookLabel')}
                        description={t('settings.discord.statusWebhookDescription')}
                    >
                        <Input
                            id="webhook-status"
                            value={localSettings['panel.notifications.discordWebhookNode'] || ''}
                            onChange={(e) => handleUpdate('panel.notifications.discordWebhookNode', e.target.value)}
                            placeholder={t('settings.discord.statusWebhookPlaceholder')}
                            className="bg-background/50"
                        />
                    </SettingsSection>
                    
                    <SettingsSection
                        id="webhook-extransfer"
                        title={t('settings.discord.extransferWebhookLabel')}
                        description={t('settings.discord.extransferWebhookDescription')}
                    >
                        <Input
                            id="webhook-extransfer"
                            value={localSettings['panel.notifications.discordWebhookExTransfer'] || ''}
                            onChange={(e) => handleUpdate('panel.notifications.discordWebhookExTransfer', e.target.value)}
                            placeholder={t('settings.discord.extransferWebhookPlaceholder')}
                            className="bg-background/50"
                        />
                    </SettingsSection>
                </div>
                
                <div className="flex items-center justify-between px-6 py-4 bg-muted/30 border-t border-border/60 rounded-b-xl">
                    <Button variant="outline" onClick={handleTestDiscord} disabled={saving}>
                        {t('settings.discord.testButton')}
                    </Button>
                    <Button onClick={handleSave} disabled={saving} className="min-w-[120px]">
                        {saving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        {t('settings.discord.saveButton')}
                    </Button>
                </div>
            </div>
        </div>
    );
}
