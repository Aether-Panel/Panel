import { useAuth } from '@/contexts/providers';
import { useEffect, useState } from 'react';
import { PageHeader } from '@/components/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Settings, Bell, Mail, Loader2, Key, Package, DownloadCloud, CloudUpload, Globe } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';
import { useSettings } from '@/hooks/use-settings';
import { useConfig } from '@/contexts/config-context';
import { sileo } from "@/lib/toast";
import { ApiKeysView } from './api-keys-view';
import { ProductsView } from './products-view';
import { GeneralTab } from './general-tab';
import { DiscordTab } from './discord-tab';
import { MailTab } from './mail-tab';
import { UpdatesTab } from './updates-tab';
import { OffsiteTab } from './offsite-tab';
import { DomainsTab } from './domains-tab';

/** Underline tab style shared with the server page tab bar. */
const triggerClass =
    "relative h-11 shrink-0 gap-2 rounded-none bg-transparent px-3 text-[13px] font-medium text-muted-foreground shadow-none transition-colors duration-200 hover:text-foreground focus-visible:ring-inset focus-visible:ring-offset-0 data-[state=active]:bg-transparent data-[state=active]:text-foreground data-[state=active]:shadow-none after:absolute after:inset-x-3 after:-bottom-px after:h-0.5 after:rounded-full after:bg-primary after:opacity-0 after:transition-opacity after:duration-200 data-[state=active]:after:opacity-100 [&_svg]:size-4 [&_svg]:shrink-0";

const empty = (v: unknown) => v ?? '';

/** Whether two settings maps hold the same values, ignoring key order. */
function sameSettings(a: Record<string, any>, b: Record<string, any>) {
    const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
    for (const k of keys) {
        if (JSON.stringify(empty(a[k])) !== JSON.stringify(empty(b[k]))) return false;
    }
    return true;
}

export default function SettingsPage() {
    const { role, hasScope } = useAuth();
    const [isMounted, setIsMounted] = useState(false);
    const { t } = useTranslations();
    const { settings, loading, saving, saveSettings, sendTestEmail, sendTestDiscord } = useSettings();
    const { refresh: refreshConfig } = useConfig();

    const [localSettings, setLocalSettings] = useState<Record<string, any>>({});
    const [activeTab, setActiveTab] = useState('general');

    useEffect(() => {
        if (settings) {
            setLocalSettings(settings);
        }
    }, [settings]);

    const settingsTabs = [
        { value: 'general', label: t('settings.tabs.general'), icon: Settings },
        { value: 'discord', label: t('settings.tabs.discordShort'), icon: Bell },
        { value: 'mail', label: t('settings.tabs.mail'), icon: Mail },
        { value: 'offsite', label: t('settings.tabs.offsite'), icon: CloudUpload },
        { value: 'domains', label: t('settings.tabs.domains'), icon: Globe },
        { value: 'apikeys', label: t('settings.tabs.apiKeys'), icon: Key },
        { value: 'products', label: t('settings.tabs.products'), icon: Package },
        { value: 'updates', label: t('settings.tabs.updates'), icon: DownloadCloud },
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
            sileo.success({
                title: t('common.success'),
                description: t('settings.saveSuccess'),
            });
        } catch (e: any) {
            sileo.error({
                title: t('common.error'),
                description: e.message || t('settings.saveError'),
            });
        }
    };

    const handleTestEmail = async () => {
        try {
            await sendTestEmail(localSettings);
            sileo.success({
                title: t('common.success'),
                description: t('settings.mail.testSuccess'),
            });
        } catch (e: any) {
            sileo.error({
                title: t('common.error'),
                description: e.message || t('settings.mail.testError'),
            });
        }
    };

    const handleTestDiscord = async () => {
        try {
            await sendTestDiscord();
            sileo.success({
                title: t('common.success'),
                description: t('settings.discord.testSuccess'),
            });
        } catch (e: any) {
            sileo.error({
                title: t('common.error'),
                description: e.message || t('settings.discord.testError'),
            });
        }
    };

    if (!isMounted || !hasScope('settings.edit') || loading) {
        return (
            <div className="flex flex-col gap-6 pb-12" aria-busy="true">
                <PageHeader title={t('settings.title')} description={t('settings.description')} />
                <span className="sr-only">{t('common.loading')}</span>
                <Skeleton className="h-11 w-full max-w-2xl" />
                <Card className="divide-y divide-border/50">
                    <div className="space-y-2 p-5 md:p-6">
                        <Skeleton className="h-5 w-48" />
                        <Skeleton className="h-4 w-72 max-w-full" />
                    </div>
                    {[0, 1, 2].map((i) => (
                        <div key={i} className="grid gap-5 p-5 md:p-6 lg:grid-cols-[minmax(0,16rem)_minmax(0,1fr)] lg:gap-10">
                            <div className="space-y-2">
                                <Skeleton className="h-4 w-32" />
                                <Skeleton className="h-4 w-48 max-w-full" />
                            </div>
                            <div className="space-y-3">
                                <Skeleton className="h-4 w-24" />
                                <Skeleton className="h-10 w-full" />
                            </div>
                        </div>
                    ))}
                </Card>
            </div>
        );
    }

    const isDirty = !sameSettings(localSettings, settings);
    const onDiscard = () => setLocalSettings(settings);

    return (
        <div className="stagger flex flex-col gap-6 pb-12">
            <PageHeader title={t('settings.title')} description={t('settings.description')} />

            <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
                {/* Phones and tablets: a select; desktop: one row of underline tabs. */}
                <div className="lg:hidden">
                    <Select value={activeTab} onValueChange={setActiveTab}>
                        <SelectTrigger aria-label={t('settings.tabs.sectionLabel')}>
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {settingsTabs.map((tab) => (
                                <SelectItem key={tab.value} value={tab.value}>
                                    <span className="flex items-center gap-2">
                                        <tab.icon className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                                        {tab.label}
                                    </span>
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>

                <div className="hidden lg:block">
                    <TabsList className="flex h-auto w-full items-stretch justify-start gap-1 overflow-x-auto rounded-none border-b border-border/60 bg-transparent p-0 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                        {settingsTabs.map((tab) => (
                            <TabsTrigger key={tab.value} value={tab.value} className={triggerClass}>
                                <tab.icon aria-hidden="true" />
                                {tab.label}
                            </TabsTrigger>
                        ))}
                    </TabsList>
                </div>

                <TabsContent value="general" className="mt-6">
                    <GeneralTab localSettings={localSettings} handleUpdate={handleUpdate} t={t} />
                </TabsContent>

                <TabsContent value="discord" className="mt-6">
                    <DiscordTab
                        localSettings={localSettings}
                        handleUpdate={handleUpdate}
                        handleTestDiscord={handleTestDiscord}
                        saving={saving}
                        t={t}
                    />
                </TabsContent>

                <TabsContent value="mail" className="mt-6">
                    <MailTab
                        localSettings={localSettings}
                        handleUpdate={handleUpdate}
                        handleTestEmail={handleTestEmail}
                        saving={saving}
                        t={t}
                    />
                </TabsContent>

                <TabsContent value="offsite" className="mt-6">
                    <OffsiteTab localSettings={localSettings} handleUpdate={handleUpdate} t={t} />
                </TabsContent>

                <TabsContent value="domains" className="mt-6">
                    <DomainsTab localSettings={localSettings} handleUpdate={handleUpdate} t={t} />
                </TabsContent>

                <TabsContent value="apikeys" className="mt-6">
                    <ApiKeysView />
                </TabsContent>

                <TabsContent value="products" className="mt-6">
                    <ProductsView />
                </TabsContent>

                <TabsContent value="updates" className="mt-6">
                    <UpdatesTab />
                </TabsContent>
            </Tabs>

            {isDirty && (
                <div
                    role="region"
                    aria-label={t('servers.settings.unsavedChanges')}
                    className="sticky bottom-4 z-20 flex items-center justify-between gap-3 rounded-xl border border-border/70 bg-popover/95 px-4 py-3 shadow-elevation-3 backdrop-blur-md animate-in fade-in slide-in-from-bottom-2 duration-200 sm:px-5"
                >
                    <span className="text-sm text-muted-foreground">{t('servers.settings.unsavedChanges')}</span>
                    <div className="flex gap-2">
                        <Button variant="ghost" size="sm" onClick={onDiscard} disabled={saving}>
                            {t('servers.settings.discard')}
                        </Button>
                        <Button size="sm" onClick={handleSave} disabled={saving}>
                            {saving && <Loader2 className="animate-spin" aria-hidden="true" />}
                            {saving ? t('common.saving') : t('servers.settings.saveButton')}
                        </Button>
                    </div>
                </div>
            )}
        </div>
    );
}
