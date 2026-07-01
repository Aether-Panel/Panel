import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { Loader2 } from 'lucide-react';

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
        <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
            <Card className="border-0 shadow-lg">
                <CardHeader>
                    <CardTitle>{t('settings.discord.title')}</CardTitle>
                    <CardDescription>{t('settings.discord.description')}</CardDescription>
                </CardHeader>
                <CardContent className="space-y-6">
                    <div className="space-y-2">
                        <Label htmlFor="webhook-alerts">{t('settings.discord.alertsWebhookLabel')}</Label>
                        <Input
                            id="webhook-alerts"
                            value={localSettings['panel.notifications.discordWebhook'] || ''}
                            onChange={(e) => handleUpdate('panel.notifications.discordWebhook', e.target.value)}
                            placeholder={t('settings.discord.alertsWebhookPlaceholder')}
                        />
                        <p className="text-sm text-muted-foreground">{t('settings.discord.alertsWebhookDescription')}</p>
                    </div>
                    <div className="space-y-2">
                        <Label htmlFor="webhook-reports">{t('settings.discord.reportsWebhookLabel')}</Label>
                        <Input
                            id="webhook-reports"
                            value={localSettings['panel.notifications.discordWebhookSystem'] || ''}
                            onChange={(e) => handleUpdate('panel.notifications.discordWebhookSystem', e.target.value)}
                            placeholder={t('settings.discord.reportsWebhookPlaceholder')}
                        />
                        <p className="text-sm text-muted-foreground">{t('settings.discord.reportsWebhookDescription')}</p>
                    </div>
                    <div className="space-y-2">
                        <Label htmlFor="webhook-status">{t('settings.discord.statusWebhookLabel')}</Label>
                        <Input
                            id="webhook-status"
                            value={localSettings['panel.notifications.discordWebhookNode'] || ''}
                            onChange={(e) => handleUpdate('panel.notifications.discordWebhookNode', e.target.value)}
                            placeholder={t('settings.discord.statusWebhookPlaceholder')}
                        />
                        <p className="text-sm text-muted-foreground">{t('settings.discord.statusWebhookDescription')}</p>
                    </div>
                    <div className="flex justify-between items-center pt-4">
                        <Button variant="secondary" onClick={handleTestDiscord} disabled={saving}>
                            {t('settings.discord.testButton')}
                        </Button>
                        <Button onClick={handleSave} disabled={saving}>
                            {saving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                            {t('settings.discord.saveButton')}
                        </Button>
                    </div>
                </CardContent>
            </Card>
        </div>
    );
}
