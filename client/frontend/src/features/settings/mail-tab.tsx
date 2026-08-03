import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Loader2 } from 'lucide-react';
import { SettingsSection } from './general-tab';

interface MailTabProps {
    localSettings: Record<string, any>;
    handleUpdate: (key: string, value: any) => void;
    handleSave: () => void;
    handleTestEmail: () => void;
    saving: boolean;
    t: (key: string) => string;
}

export function MailTab({ localSettings, handleUpdate, handleSave, handleTestEmail, saving, t }: MailTabProps) {
    const provider = localSettings['panel.email.provider'];

    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-xl font-semibold tracking-tight">{t('settings.mail.title')}</h2>
                <p className="text-sm text-muted-foreground mt-1">{t('settings.mail.description')}</p>
            </div>
            
            <div className="rounded-xl border border-border/60 bg-card shadow-sm">
                <div className="px-6">
                    <SettingsSection
                        id="mail-provider"
                        title={t('settings.mail.providerLabel')}
                        description={t('settings.mail.providerDescription')}
                    >
                        <Select
                            value={provider || ''}
                            onValueChange={(v) => handleUpdate('panel.email.provider', v)}
                        >
                            <SelectTrigger id="mail-provider" className="bg-background/50">
                                <SelectValue placeholder={t('settings.mail.providerPlaceholder')} />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="smtp">SMTP</SelectItem>
                                <SelectItem value="mailgun">Mailgun</SelectItem>
                                <SelectItem value="mailjet">Mailjet</SelectItem>
                            </SelectContent>
                        </Select>
                    </SettingsSection>

                    {!provider && (
                        <div className="py-12 text-center text-sm text-muted-foreground">
                            {t('settings.mail.selectProviderPrompt')}
                        </div>
                    )}

                    {provider === 'smtp' && (
                        <div className="py-6 animate-in fade-in slide-in-from-top-2 duration-300">
                            <h3 className="text-sm font-medium text-foreground mb-4">SMTP Configuration</h3>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                <div className="space-y-1.5">
                                    <label htmlFor="smtp-from" className="text-[13px] font-medium text-foreground">{t('settings.mail.smtp.fromLabel')}</label>
                                    <Input
                                        id="smtp-from"
                                        value={localSettings['panel.email.from'] || ''}
                                        onChange={(e) => handleUpdate('panel.email.from', e.target.value)}
                                        placeholder={t('settings.mail.smtp.fromPlaceholder')}
                                        className="bg-background/50"
                                    />
                                </div>
                                <div className="space-y-1.5">
                                    <label htmlFor="smtp-host" className="text-[13px] font-medium text-foreground">{t('settings.mail.smtp.hostLabel')}</label>
                                    <Input
                                        id="smtp-host"
                                        value={localSettings['panel.email.host'] || ''}
                                        onChange={(e) => handleUpdate('panel.email.host', e.target.value)}
                                        placeholder={t('settings.mail.smtp.hostPlaceholder')}
                                        className="bg-background/50"
                                    />
                                </div>
                                <div className="space-y-1.5">
                                    <label htmlFor="smtp-user" className="text-[13px] font-medium text-foreground">{t('settings.mail.smtp.userLabel')}</label>
                                    <Input
                                        id="smtp-user"
                                        value={localSettings['panel.email.username'] || ''}
                                        onChange={(e) => handleUpdate('panel.email.username', e.target.value)}
                                        placeholder={t('settings.mail.smtp.userPlaceholder')}
                                        className="bg-background/50"
                                    />
                                </div>
                                <div className="space-y-1.5">
                                    <label htmlFor="smtp-pass" className="text-[13px] font-medium text-foreground">{t('settings.mail.smtp.passLabel')}</label>
                                    <Input
                                        id="smtp-pass"
                                        type="password"
                                        value={localSettings['panel.email.password'] || ''}
                                        onChange={(e) => handleUpdate('panel.email.password', e.target.value)}
                                        placeholder={t('settings.mail.smtp.passPlaceholder')}
                                        className="bg-background/50"
                                    />
                                </div>
                            </div>
                        </div>
                    )}

                    {provider === 'mailgun' && (
                        <div className="py-6 animate-in fade-in slide-in-from-top-2 duration-300">
                            <h3 className="text-sm font-medium text-foreground mb-4">Mailgun Configuration</h3>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                <div className="space-y-1.5">
                                    <label htmlFor="mailgun-domain" className="text-[13px] font-medium text-foreground">{t('settings.mail.mailgun.domainLabel')}</label>
                                    <Input
                                        id="mailgun-domain"
                                        value={localSettings['panel.email.domain'] || ''}
                                        onChange={(e) => handleUpdate('panel.email.domain', e.target.value)}
                                        placeholder={t('settings.mail.mailgun.domainPlaceholder')}
                                        className="bg-background/50"
                                    />
                                </div>
                                <div className="space-y-1.5">
                                    <label htmlFor="mailgun-from" className="text-[13px] font-medium text-foreground">{t('settings.mail.mailgun.fromLabel')}</label>
                                    <Input
                                        id="mailgun-from"
                                        value={localSettings['panel.email.from'] || ''}
                                        onChange={(e) => handleUpdate('panel.email.from', e.target.value)}
                                        placeholder={t('settings.mail.mailgun.fromPlaceholder')}
                                        className="bg-background/50"
                                    />
                                </div>
                                <div className="space-y-1.5 md:col-span-2">
                                    <label htmlFor="mailgun-key" className="text-[13px] font-medium text-foreground">{t('settings.mail.mailgun.keyLabel')}</label>
                                    <Input
                                        id="mailgun-key"
                                        type="password"
                                        value={localSettings['panel.email.key'] || ''}
                                        onChange={(e) => handleUpdate('panel.email.key', e.target.value)}
                                        placeholder={t('settings.mail.mailgun.keyPlaceholder')}
                                        className="bg-background/50"
                                    />
                                </div>
                            </div>
                        </div>
                    )}

                    {provider === 'mailjet' && (
                        <div className="py-6 animate-in fade-in slide-in-from-top-2 duration-300">
                            <h3 className="text-sm font-medium text-foreground mb-4">Mailjet Configuration</h3>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                <div className="space-y-1.5">
                                    <label htmlFor="mailjet-domain" className="text-[13px] font-medium text-foreground">{t('settings.mail.mailjet.domainLabel')}</label>
                                    <Input
                                        id="mailjet-domain"
                                        value={localSettings['panel.email.domain'] || ''}
                                        onChange={(e) => handleUpdate('panel.email.domain', e.target.value)}
                                        placeholder={t('settings.mail.mailjet.domainPlaceholder')}
                                        className="bg-background/50"
                                    />
                                </div>
                                <div className="space-y-1.5">
                                    <label htmlFor="mailjet-from" className="text-[13px] font-medium text-foreground">{t('settings.mail.mailjet.fromLabel')}</label>
                                    <Input
                                        id="mailjet-from"
                                        value={localSettings['panel.email.from'] || ''}
                                        onChange={(e) => handleUpdate('panel.email.from', e.target.value)}
                                        placeholder={t('settings.mail.mailjet.fromPlaceholder')}
                                        className="bg-background/50"
                                    />
                                </div>
                                <div className="space-y-1.5 md:col-span-2">
                                    <label htmlFor="mailjet-key" className="text-[13px] font-medium text-foreground">{t('settings.mail.mailjet.keyLabel')}</label>
                                    <Input
                                        id="mailjet-key"
                                        type="password"
                                        value={localSettings['panel.email.key'] || ''}
                                        onChange={(e) => handleUpdate('panel.email.key', e.target.value)}
                                        placeholder={t('settings.mail.mailjet.keyPlaceholder')}
                                        className="bg-background/50"
                                    />
                                </div>
                            </div>
                        </div>
                    )}
                </div>

                <div className="flex items-center justify-between px-6 py-4 bg-muted/30 border-t border-border/60 rounded-b-xl">
                    <Button variant="outline" onClick={handleTestEmail} disabled={saving || !provider}>
                        {t('settings.mail.testButton')}
                    </Button>
                    <Button onClick={handleSave} disabled={saving} className="min-w-[120px]">
                        {saving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        {t('settings.mail.saveButton')}
                    </Button>
                </div>
            </div>
        </div>
    );
}
