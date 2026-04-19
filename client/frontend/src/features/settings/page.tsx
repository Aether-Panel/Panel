import { useAuth } from '@/contexts/providers';
import { useEffect, useState } from 'react';
import { PageHeader } from '@/components/page-header';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Separator } from '@/components/ui/separator';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Settings, Bell, Mail, Loader2 } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';
import { useSettings } from '@/hooks/use-settings';
import { useConfig } from '@/contexts/config-context';
import { useToast } from '@/hooks/use-toast';


export default function SettingsPage() {
    const { role, hasScope } = useAuth();
    const [isMounted, setIsMounted] = useState(false);
    const { t } = useTranslations();
    const { settings, loading, saving, saveSettings, sendTestEmail, sendTestDiscord } = useSettings();
    const { refresh: refreshConfig } = useConfig();
    const { toast } = useToast();

    // Local form state
    const [localSettings, setLocalSettings] = useState<Record<string, any>>({});
    const [activeTab, setActiveTab] = useState('general');

    useEffect(() => {
        if (settings) {
            setLocalSettings(settings);
        }
    }, [settings]);

    const settingsTabs = [
        { value: 'general', label: t('settings.tabs.general'), icon: Settings },
        { value: 'discord', label: t('settings.tabs.discord'), icon: Bell },
        { value: 'mail', label: t('settings.tabs.mail'), icon: Mail },
    ];

    useEffect(() => {
        setIsMounted(true);
        if (role && !hasScope('settings.edit')) {
            window.location.href = '/dashboard';
        }
    }, [role, hasScope]);

    const handleUpdate = (key: string, value: any) => {
        setLocalSettings(prev => ({ ...prev, [key]: value }));
    };

    const handleSave = async () => {
        try {
            await saveSettings(localSettings);
            await refreshConfig();
            toast({
                title: t('common.success'),
                description: t('settings.saveSuccess') || 'Settings saved successfully',
            });
        } catch (e: any) {
            toast({
                title: t('common.error'),
                description: e.message || 'Failed to save settings',
                variant: 'destructive',
            });
        }
    };

    const handleTestEmail = async () => {
        try {
            await sendTestEmail();
            toast({
                title: t('common.success'),
                description: t('settings.mail.testSuccess') || 'Test email sent successfully',
            });
        } catch (e: any) {
            toast({
                title: t('common.error'),
                description: e.message || 'Failed to send test email',
                variant: 'destructive',
            });
        }
    };

    const handleTestDiscord = async () => {
        try {
            await sendTestDiscord();
            toast({
                title: t('common.success'),
                description: t('settings.discord.testSuccess') || 'Test Discord message sent successfully',
            });
        } catch (e: any) {
            toast({
                title: t('common.error'),
                description: e.message || 'Failed to send test Discord message',
                variant: 'destructive',
            });
        }
    };

    if (!isMounted || !hasScope('settings.edit') || loading) {
        return (
            <div className="flex h-full items-center justify-center p-12">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
                <span className="ml-2 text-lg">{t('common.loading')}</span>
            </div>
        );
    }

    return (
        <div className="flex flex-col gap-8 pb-12">
            <PageHeader title={t('settings.title')} description={t('settings.description')} />

            <Tabs value={activeTab} onValueChange={setActiveTab} defaultValue="general" className="w-full">
                <div className="md:hidden mb-4">
                    <Select value={activeTab} onValueChange={setActiveTab}>
                        <SelectTrigger>
                            <SelectValue placeholder={t('settings.tabs.general')} />
                        </SelectTrigger>
                        <SelectContent>
                            {settingsTabs.map((tab) => (
                                <SelectItem key={tab.value} value={tab.value}>
                                    <div className="flex items-center gap-2">
                                        <tab.icon className="h-4 w-4" />
                                        <span>{tab.label}</span>
                                    </div>
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>

                <TabsList className="hidden md:grid w-full grid-cols-3">
                    {settingsTabs.map((tab) => (
                        <TabsTrigger key={tab.value} value={tab.value}>
                            <tab.icon className="mr-2 h-4 w-4" />
                            {tab.label}
                        </TabsTrigger>
                    ))}
                </TabsList>

                <TabsContent value="general">
                    <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
                        <Card className="border-0 shadow-lg">
                            <CardHeader>
                                <CardTitle>{t('settings.general.title')}</CardTitle>
                                <CardDescription>{t('settings.general.description')}</CardDescription>
                            </CardHeader>
                            <CardContent className="space-y-6">
                                <div className="space-y-2">
                                    <Label htmlFor="base-url">{t('settings.general.baseUrlLabel')}</Label>
                                    <Input
                                        id="base-url"
                                        value={localSettings['panel.settings.masterUrl'] || ''}
                                        onChange={(e) => handleUpdate('panel.settings.masterUrl', e.target.value)}
                                    />
                                    <p className="text-sm text-muted-foreground">{t('settings.general.baseUrlDescription')}</p>
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="master-node-ip">Global Master Node IP</Label>
                                    <Input
                                        id="master-node-ip"
                                        value={localSettings['panel.settings.masterNodeIp'] || ''}
                                        onChange={(e) => handleUpdate('panel.settings.masterNodeIp', e.target.value)}
                                        placeholder="192.168.0.3"
                                    />
                                    <p className="text-sm text-muted-foreground">If multiple panels share a database, set the true Master IP here (e.g. 192.168.0.3). ALL panels will sync this and route LocalNode transfers properly.</p>
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="company-name">{t('settings.general.companyNameLabel')}</Label>
                                    <Input
                                        id="company-name"
                                        value={localSettings['panel.settings.companyName'] || ''}
                                        onChange={(e) => handleUpdate('panel.settings.companyName', e.target.value)}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="gemini-api-key">{t('settings.general.geminiApiKeyLabel')}</Label>
                                    <Input
                                        id="gemini-api-key"
                                        type="password"
                                        value={localSettings['panel.settings.geminiApiKey'] || ''}
                                        onChange={(e) => handleUpdate('panel.settings.geminiApiKey', e.target.value)}
                                        placeholder={t('settings.general.geminiApiKeyPlaceholder')}
                                    />
                                    <p className="text-sm text-muted-foreground">{t('settings.general.geminiApiKeyDescription')}</p>
                                </div>
                                <div className="flex items-center justify-between rounded-lg border p-4 bg-muted/30">
                                    <div>
                                        <Label htmlFor="hide-ai-analysis" className="font-medium">{t('settings.general.hideAiAnalysisLabel')}</Label>
                                        <p className="text-sm text-muted-foreground max-w-prose mt-1">
                                            {t('settings.general.hideAiAnalysisDescription')}
                                        </p>
                                    </div>
                                    <Switch
                                        id="hide-ai-analysis"
                                        checked={!!localSettings['panel.settings.hideAiAnalysis']}
                                        onCheckedChange={(v) => handleUpdate('panel.settings.hideAiAnalysis', v)}
                                    />
                                </div>
                                <div className="flex items-center justify-between rounded-lg border p-4 bg-muted/30">
                                    <div>
                                        <Label htmlFor="allow-registration" className="font-medium">{t('settings.general.allowRegistrationLabel')}</Label>
                                        <p className="text-sm text-muted-foreground max-w-prose mt-1">
                                            {t('settings.general.allowRegistrationDescription')}
                                        </p>
                                    </div>
                                    <Switch
                                        id="allow-registration"
                                        checked={!!localSettings['panel.registrationEnabled']}
                                        onCheckedChange={(v) => handleUpdate('panel.registrationEnabled', v)}
                                    />
                                </div>
                                <div className="flex justify-end pt-4">
                                    <Button onClick={handleSave} disabled={saving}>
                                        {saving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                        {t('settings.general.saveButton')}
                                    </Button>
                                </div>
                            </CardContent>
                        </Card>
                    </div>
                </TabsContent>

                <TabsContent value="discord">
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
                </TabsContent>

                <TabsContent value="mail">
                    <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
                        <Card className="border-0 shadow-lg">
                            <CardHeader>
                                <CardTitle>{t('settings.mail.title')}</CardTitle>
                                <CardDescription>{t('settings.mail.description')}</CardDescription>
                            </CardHeader>
                            <CardContent className="space-y-6">
                                <div className="space-y-2">
                                    <Label htmlFor="mail-provider">{t('settings.mail.providerLabel')}</Label>
                                    <Select
                                        value={localSettings['panel.email.provider'] || ''}
                                        onValueChange={(v) => handleUpdate('panel.email.provider', v)}
                                    >
                                        <SelectTrigger id="mail-provider">
                                            <SelectValue placeholder={t('settings.mail.providerPlaceholder')} />
                                        </SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="smtp">SMTP</SelectItem>
                                            <SelectItem value="mailgun">Mailgun</SelectItem>
                                            <SelectItem value="mailjet">Mailjet</SelectItem>
                                        </SelectContent>
                                    </Select>
                                    <p className="text-sm text-muted-foreground">{t('settings.mail.providerDescription')}</p>
                                </div>

                                {!localSettings['panel.email.provider'] && (
                                    <div className="text-center text-muted-foreground py-6">
                                        <p>{t('settings.mail.selectProviderPrompt')}</p>
                                    </div>
                                )}

                                {localSettings['panel.email.provider'] === 'smtp' && (
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6 pt-6 border-t animate-in slide-in-from-top-2 duration-300">
                                        <div className="space-y-2">
                                            <Label htmlFor="smtp-from">{t('settings.mail.smtp.fromLabel')}</Label>
                                            <Input
                                                id="smtp-from"
                                                value={localSettings['panel.email.from'] || ''}
                                                onChange={(e) => handleUpdate('panel.email.from', e.target.value)}
                                                placeholder={t('settings.mail.smtp.fromPlaceholder')}
                                            />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="smtp-host">{t('settings.mail.smtp.hostLabel')}</Label>
                                            <Input
                                                id="smtp-host"
                                                value={localSettings['panel.email.host'] || ''}
                                                onChange={(e) => handleUpdate('panel.email.host', e.target.value)}
                                                placeholder={t('settings.mail.smtp.hostPlaceholder')}
                                            />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="smtp-user">{t('settings.mail.smtp.userLabel')}</Label>
                                            <Input
                                                id="smtp-user"
                                                value={localSettings['panel.email.username'] || ''}
                                                onChange={(e) => handleUpdate('panel.email.username', e.target.value)}
                                                placeholder={t('settings.mail.smtp.userPlaceholder')}
                                            />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="smtp-pass">{t('settings.mail.smtp.passLabel')}</Label>
                                            <Input
                                                id="smtp-pass"
                                                type="password"
                                                value={localSettings['panel.email.password'] || ''}
                                                onChange={(e) => handleUpdate('panel.email.password', e.target.value)}
                                                placeholder={t('settings.mail.smtp.passPlaceholder')}
                                            />
                                        </div>
                                    </div>
                                )}

                                {localSettings['panel.email.provider'] === 'mailgun' && (
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6 pt-6 border-t animate-in slide-in-from-top-2 duration-300">
                                        <div className="space-y-2">
                                            <Label htmlFor="mailgun-domain">{t('settings.mail.mailgun.domainLabel')}</Label>
                                            <Input
                                                id="mailgun-domain"
                                                value={localSettings['panel.email.domain'] || ''}
                                                onChange={(e) => handleUpdate('panel.email.domain', e.target.value)}
                                                placeholder={t('settings.mail.mailgun.domainPlaceholder')}
                                            />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="mailgun-from">{t('settings.mail.mailgun.fromLabel')}</Label>
                                            <Input
                                                id="mailgun-from"
                                                value={localSettings['panel.email.from'] || ''}
                                                onChange={(e) => handleUpdate('panel.email.from', e.target.value)}
                                                placeholder={t('settings.mail.mailgun.fromPlaceholder')}
                                            />
                                        </div>
                                        <div className="space-y-2 md:col-span-2">
                                            <Label htmlFor="mailgun-key">{t('settings.mail.mailgun.keyLabel')}</Label>
                                            <Input
                                                id="mailgun-key"
                                                type="password"
                                                value={localSettings['panel.email.key'] || ''}
                                                onChange={(e) => handleUpdate('panel.email.key', e.target.value)}
                                                placeholder={t('settings.mail.mailgun.keyPlaceholder')}
                                            />
                                        </div>
                                    </div>
                                )}

                                {localSettings['panel.email.provider'] === 'mailjet' && (
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6 pt-6 border-t animate-in slide-in-from-top-2 duration-300">
                                        <div className="space-y-2">
                                            <Label htmlFor="mailjet-domain">{t('settings.mail.mailjet.domainLabel')}</Label>
                                            <Input
                                                id="mailjet-domain"
                                                value={localSettings['panel.email.domain'] || ''}
                                                onChange={(e) => handleUpdate('panel.email.domain', e.target.value)}
                                                placeholder={t('settings.mail.mailjet.domainPlaceholder')}
                                            />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="mailjet-from">{t('settings.mail.mailjet.fromLabel')}</Label>
                                            <Input
                                                id="mailjet-from"
                                                value={localSettings['panel.email.from'] || ''}
                                                onChange={(e) => handleUpdate('panel.email.from', e.target.value)}
                                                placeholder={t('settings.mail.mailjet.fromPlaceholder')}
                                            />
                                        </div>
                                        <div className="space-y-2 md:col-span-2">
                                            <Label htmlFor="mailjet-key">{t('settings.mail.mailjet.keyLabel')}</Label>
                                            <Input
                                                id="mailjet-key"
                                                type="password"
                                                value={localSettings['panel.email.key'] || ''}
                                                onChange={(e) => handleUpdate('panel.email.key', e.target.value)}
                                                placeholder={t('settings.mail.mailjet.keyPlaceholder')}
                                            />
                                        </div>
                                    </div>
                                )}

                                {localSettings['panel.email.provider'] && (
                                    <>
                                        <Separator className="mt-6" />
                                        <div className="flex justify-between items-center pt-4">
                                            <Button variant="secondary" onClick={handleTestEmail} disabled={saving}>
                                                {t('settings.mail.testButton')}
                                            </Button>
                                            <Button onClick={handleSave} disabled={saving}>
                                                {saving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                                {t('settings.mail.saveButton')}
                                            </Button>
                                        </div>
                                    </>
                                )}
                            </CardContent>
                        </Card>
                    </div>
                </TabsContent>
            </Tabs>
        </div>
    );
}
