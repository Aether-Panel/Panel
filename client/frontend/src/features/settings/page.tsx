'use client';
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
import { Settings, Bell, Mail } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';


export default function SettingsPage() {
    const { role } = useAuth();
    const [isMounted, setIsMounted] = useState(false);
    const { t } = useTranslations();

    // State for form fields
    const [companyName, setCompanyName] = useState('Aether Panel');
    const [baseUrl, setBaseUrl] = useState('http://localhost:8080');
    const [allowRegistration, setAllowRegistration] = useState(true);
    const [mailProvider, setMailProvider] = useState('');
    const [geminiApiKey, setGeminiApiKey] = useState('');
    const [hideAIAnalysis, setHideAIAnalysis] = useState(false);
    const [activeTab, setActiveTab] = useState('general');

    const settingsTabs = [
        { value: 'general', label: t('settings.tabs.general'), icon: Settings },
        { value: 'discord', label: t('settings.tabs.discord'), icon: Bell },
        { value: 'mail', label: t('settings.tabs.mail'), icon: Mail },
    ];

    useEffect(() => {
        setIsMounted(true);
        if (role && role !== 'admin') {
            window.location.href = '/dashboard';
        }
    }, [role]);

    if (!isMounted || role !== 'admin') {
        return (
            <div className="flex h-full items-center justify-center">
                <p>{t('common.loading')}</p>
            </div>
        );
    }

    return (
        <div className="flex flex-col gap-8">
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
                        <Card className="border-0">
                            <CardHeader>
                                <CardTitle>{t('settings.general.title')}</CardTitle>
                                <CardDescription>{t('settings.general.description')}</CardDescription>
                            </CardHeader>
                            <CardContent className="space-y-6">
                                <div className="space-y-2">
                                    <Label htmlFor="base-url">{t('settings.general.baseUrlLabel')}</Label>
                                    <Input id="base-url" value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} />
                                    <p className="text-sm text-muted-foreground">{t('settings.general.baseUrlDescription')}</p>
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="company-name">{t('settings.general.companyNameLabel')}</Label>
                                    <Input id="company-name" value={companyName} onChange={(e) => setCompanyName(e.target.value)} />
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="gemini-api-key">{t('settings.general.geminiApiKeyLabel')}</Label>
                                    <Input id="gemini-api-key" type="password" value={geminiApiKey} onChange={(e) => setGeminiApiKey(e.target.value)} placeholder={t('settings.general.geminiApiKeyPlaceholder')} />
                                    <p className="text-sm text-muted-foreground">{t('settings.general.geminiApiKeyDescription')}</p>
                                </div>
                                <div className="flex items-center justify-between rounded-lg border p-4">
                                    <div>
                                        <Label htmlFor="hide-ai-analysis" className="font-medium">{t('settings.general.hideAiAnalysisLabel')}</Label>
                                        <p className="text-sm text-muted-foreground max-w-prose mt-1">
                                            {t('settings.general.hideAiAnalysisDescription')}
                                        </p>
                                    </div>
                                    <Switch id="hide-ai-analysis" checked={hideAIAnalysis} onCheckedChange={setHideAIAnalysis} />
                                </div>
                                <div className="flex items-center justify-between rounded-lg border p-4">
                                    <div>
                                        <Label htmlFor="allow-registration" className="font-medium">{t('settings.general.allowRegistrationLabel')}</Label>
                                        <p className="text-sm text-muted-foreground max-w-prose mt-1">
                                            {t('settings.general.allowRegistrationDescription')}
                                        </p>
                                    </div>
                                    <Switch id="allow-registration" checked={allowRegistration} onCheckedChange={setAllowRegistration} />
                                </div>
                                <div className="flex justify-end pt-4">
                                    <Button>{t('settings.general.saveButton')}</Button>
                                </div>
                            </CardContent>
                        </Card>
                    </div>
                </TabsContent>
                <TabsContent value="discord">
                    <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
                        <Card className="border-0">
                            <CardHeader>
                                <CardTitle>{t('settings.discord.title')}</CardTitle>
                                <CardDescription>{t('settings.discord.description')}</CardDescription>
                            </CardHeader>
                            <CardContent className="space-y-6">
                                <div className="space-y-2">
                                    <Label htmlFor="webhook-alerts">{t('settings.discord.alertsWebhookLabel')}</Label>
                                    <Input id="webhook-alerts" placeholder={t('settings.discord.alertsWebhookPlaceholder')} />
                                    <p className="text-sm text-muted-foreground">{t('settings.discord.alertsWebhookDescription')}</p>
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="webhook-reports">{t('settings.discord.reportsWebhookLabel')}</Label>
                                    <Input id="webhook-reports" placeholder={t('settings.discord.reportsWebhookPlaceholder')} />
                                    <p className="text-sm text-muted-foreground">{t('settings.discord.reportsWebhookDescription')}</p>
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="webhook-status">{t('settings.discord.statusWebhookLabel')}</Label>
                                    <Input id="webhook-status" placeholder={t('settings.discord.statusWebhookPlaceholder')} />
                                    <p className="text-sm text-muted-foreground">{t('settings.discord.statusWebhookDescription')}</p>
                                </div>
                                <div className="flex justify-between items-center pt-4">
                                    <Button variant="secondary">{t('settings.discord.testButton')}</Button>
                                    <Button>{t('settings.discord.saveButton')}</Button>
                                </div>
                            </CardContent>
                        </Card>
                    </div>
                </TabsContent>
                <TabsContent value="mail">
                    <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
                        <Card className="border-0">
                            <CardHeader>
                                <CardTitle>{t('settings.mail.title')}</CardTitle>
                                <CardDescription>{t('settings.mail.description')}</CardDescription>
                            </CardHeader>
                            <CardContent className="space-y-6">
                                <div className="space-y-2">
                                    <Label htmlFor="mail-provider">{t('settings.mail.providerLabel')}</Label>
                                    <Select onValueChange={setMailProvider}>
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

                                {!mailProvider && (
                                    <div className="text-center text-muted-foreground py-6">
                                        <p>{t('settings.mail.selectProviderPrompt')}</p>
                                    </div>
                                )}

                                {mailProvider === 'smtp' && (
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6 pt-6 border-t animate-in fade-in">
                                        <div className="space-y-2">
                                            <Label htmlFor="smtp-from">{t('settings.mail.smtp.fromLabel')}</Label>
                                            <Input id="smtp-from" placeholder={t('settings.mail.smtp.fromPlaceholder')} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="smtp-host">{t('settings.mail.smtp.hostLabel')}</Label>
                                            <Input id="smtp-host" placeholder={t('settings.mail.smtp.hostPlaceholder')} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="smtp-user">{t('settings.mail.smtp.userLabel')}</Label>
                                            <Input id="smtp-user" placeholder={t('settings.mail.smtp.userPlaceholder')} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="smtp-pass">{t('settings.mail.smtp.passLabel')}</Label>
                                            <Input id="smtp-pass" type="password" placeholder={t('settings.mail.smtp.passPlaceholder')} />
                                        </div>
                                    </div>
                                )}

                                {mailProvider === 'mailgun' && (
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6 pt-6 border-t animate-in fade-in">
                                        <div className="space-y-2">
                                            <Label htmlFor="mailgun-domain">{t('settings.mail.mailgun.domainLabel')}</Label>
                                            <Input id="mailgun-domain" placeholder={t('settings.mail.mailgun.domainPlaceholder')} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="mailgun-from">{t('settings.mail.mailgun.fromLabel')}</Label>
                                            <Input id="mailgun-from" placeholder={t('settings.mail.mailgun.fromPlaceholder')} />
                                        </div>
                                        <div className="space-y-2 md:col-span-2">
                                            <Label htmlFor="mailgun-key">{t('settings.mail.mailgun.keyLabel')}</Label>
                                            <Input id="mailgun-key" type="password" placeholder={t('settings.mail.mailgun.keyPlaceholder')} />
                                        </div>
                                    </div>
                                )}

                                {mailProvider === 'mailjet' && (
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6 pt-6 border-t animate-in fade-in">
                                        <div className="space-y-2">
                                            <Label htmlFor="mailjet-domain">{t('settings.mail.mailjet.domainLabel')}</Label>
                                            <Input id="mailjet-domain" placeholder={t('settings.mail.mailjet.domainPlaceholder')} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="mailjet-from">{t('settings.mail.mailjet.fromLabel')}</Label>
                                            <Input id="mailjet-from" placeholder={t('settings.mail.mailjet.fromPlaceholder')} />
                                        </div>
                                        <div className="space-y-2 md:col-span-2">
                                            <Label htmlFor="mailjet-key">{t('settings.mail.mailjet.keyLabel')}</Label>
                                            <Input id="mailjet-key" type="password" placeholder={t('settings.mail.mailjet.keyPlaceholder')} />
                                        </div>
                                    </div>
                                )}

                                {mailProvider && (
                                    <>
                                        <Separator className="mt-6" />
                                        <div className="flex justify-between items-center pt-4">
                                            <Button variant="secondary">{t('settings.mail.testButton')}</Button>
                                            <Button>{t('settings.mail.saveButton')}</Button>
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
