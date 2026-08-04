import { useAuth } from '@/contexts/providers';
import { useEffect, useState } from 'react';
import { PageHeader } from '@/components/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Settings, Bell, Mail, Loader2, Key, Package } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';
import { useSettings } from '@/hooks/use-settings';
import { useConfig } from '@/contexts/config-context';
import { useToast } from '@/hooks/use-toast';
import { ApiKeysView } from './api-keys-view';
import { ProductsView } from './products-view';
import { GeneralTab } from './general-tab';
import { DiscordTab } from './discord-tab';
import { MailTab } from './mail-tab';

export default function SettingsPage() {
    const { role, hasScope } = useAuth();
    const [isMounted, setIsMounted] = useState(false);
    const { t } = useTranslations();
    const { settings, loading, saving, saveSettings, sendTestEmail, sendTestDiscord } = useSettings();
    const { refresh: refreshConfig } = useConfig();
    const { toast } = useToast();

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
        { value: 'apikeys', label: 'API Keys', icon: Key },
        { value: 'products', label: 'Provision Products', icon: Package },
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

            <div className="flex flex-col lg:flex-row lg:gap-6 w-full">
                <div className="lg:hidden mb-4">
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

                <Tabs value={activeTab} onValueChange={setActiveTab} className="hidden lg:block lg:w-64 lg:shrink-0">
                    <TabsList className="flex flex-col w-full bg-transparent p-0 h-auto gap-1">
                        {settingsTabs.map((tab) => (
                            <TabsTrigger key={tab.value} value={tab.value} className="justify-start gap-3 py-2 px-3 rounded-md border-0 data-[state=active]:bg-muted/50 data-[state=active]:text-foreground text-muted-foreground hover:text-foreground transition-colors font-medium text-sm">
                                <tab.icon className="h-4 w-4" />
                                {tab.label}
                            </TabsTrigger>
                        ))}
                    </TabsList>
                </Tabs>

                <div className="flex-1 min-w-0">
                    <Tabs value={activeTab} onValueChange={setActiveTab}>
                        <TabsContent value="general" className="mt-0">
                            <GeneralTab
                                localSettings={localSettings}
                                handleUpdate={handleUpdate}
                                handleSave={handleSave}
                                saving={saving}
                                t={t}
                            />
                        </TabsContent>

                        <TabsContent value="discord" className="mt-0">
                            <DiscordTab
                                localSettings={localSettings}
                                handleUpdate={handleUpdate}
                                handleSave={handleSave}
                                handleTestDiscord={handleTestDiscord}
                                saving={saving}
                                t={t}
                            />
                        </TabsContent>

                        <TabsContent value="mail" className="mt-0">
                            <MailTab
                                localSettings={localSettings}
                                handleUpdate={handleUpdate}
                                handleSave={handleSave}
                                handleTestEmail={handleTestEmail}
                                saving={saving}
                                t={t}
                            />
                        </TabsContent>

                        <TabsContent value="apikeys" className="mt-0">
                            <ApiKeysView />
                        </TabsContent>

                        <TabsContent value="products" className="mt-0">
                            <ProductsView />
                        </TabsContent>
                    </Tabs>
                </div>
            </div>
        </div>
    );
}
