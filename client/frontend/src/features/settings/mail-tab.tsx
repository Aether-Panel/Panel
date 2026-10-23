import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Mail, Send } from 'lucide-react';
import { SecretInput, SettingsCard, SettingsCardHeader, SettingsField, SettingsSection } from './general-tab';

interface MailTabProps {
    localSettings: Record<string, any>;
    handleUpdate: (key: string, value: any) => void;
    handleTestEmail: () => void;
    saving: boolean;
    t: (key: string, options?: Record<string, string | number>) => string;
}

const PROVIDER_NAMES: Record<string, string> = { smtp: 'SMTP', mailgun: 'Mailgun', mailjet: 'Mailjet' };

export function MailTab({ localSettings, handleUpdate, handleTestEmail, saving, t }: MailTabProps) {
    const provider: string | undefined = localSettings['panel.email.provider'];
    const providerName = provider ? PROVIDER_NAMES[provider] ?? provider : '';

    const text = (key: string, id: string, label: string, placeholder: string, mono = false) => (
        <SettingsField id={id} label={label}>
            <Input
                id={id}
                spellCheck={false}
                value={localSettings[key] || ''}
                onChange={(e) => handleUpdate(key, e.target.value)}
                placeholder={placeholder}
                className={mono ? '[&:not(:placeholder-shown)]:font-mono [&:not(:placeholder-shown)]:text-[13px]' : undefined}
            />
        </SettingsField>
    );

    const secret = (key: string, id: string, label: string, placeholder: string) => (
        <SettingsField id={id} label={label} className="md:col-span-2">
            <SecretInput id={id} value={localSettings[key] || ''} onChange={(v) => handleUpdate(key, v)} placeholder={placeholder} t={t} />
        </SettingsField>
    );

    return (
        <SettingsCard>
            <SettingsCardHeader
                title={t('settings.mail.title')}
                description={t('settings.mail.description')}
                action={
                    <Button variant="outline" size="sm" onClick={handleTestEmail} disabled={saving || !provider}>
                        <Send aria-hidden="true" />
                        {t('settings.mail.testButton')}
                    </Button>
                }
            />
            <div className="divide-y divide-border/50">
                <SettingsSection title={t('settings.mail.providerLabel')} description={t('settings.mail.providerDescription')}>
                    <div className="max-w-sm space-y-2">
                        <Select value={provider || ''} onValueChange={(v) => handleUpdate('panel.email.provider', v)}>
                            <SelectTrigger id="mail-provider" aria-label={t('settings.mail.providerLabel')}>
                                <SelectValue placeholder={t('settings.mail.providerPlaceholder')} />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="smtp">SMTP</SelectItem>
                                <SelectItem value="mailgun">Mailgun</SelectItem>
                                <SelectItem value="mailjet">Mailjet</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                </SettingsSection>

                {!provider && (
                    <div className="p-5 md:p-6">
                        <div className="rounded-xl border border-dashed border-border/60 px-6 py-10 text-center">
                            <Mail className="mx-auto h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
                            <p className="mx-auto mt-3 max-w-sm text-sm text-muted-foreground">{t('settings.mail.selectProviderPrompt')}</p>
                        </div>
                    </div>
                )}

                {provider && (
                    <SettingsSection
                        title={t('settings.mail.configTitle', { provider: providerName })}
                        description={t('settings.mail.configDescription')}
                    >
                        <div className="grid gap-5 md:grid-cols-2">
                            {provider === 'smtp' && (
                                <>
                                    {text('panel.email.from', 'smtp-from', t('settings.mail.smtp.fromLabel'), t('settings.mail.smtp.fromPlaceholder'))}
                                    {text('panel.email.host', 'smtp-host', t('settings.mail.smtp.hostLabel'), t('settings.mail.smtp.hostPlaceholder'), true)}
                                    {text('panel.email.username', 'smtp-user', t('settings.mail.smtp.userLabel'), t('settings.mail.smtp.userPlaceholder'))}
                                    <SettingsField id="smtp-pass" label={t('settings.mail.smtp.passLabel')}>
                                        <SecretInput
                                            id="smtp-pass"
                                            value={localSettings['panel.email.password'] || ''}
                                            onChange={(v) => handleUpdate('panel.email.password', v)}
                                            placeholder={t('settings.mail.smtp.passPlaceholder')}
                                            t={t}
                                        />
                                    </SettingsField>
                                </>
                            )}
                            {provider === 'mailgun' && (
                                <>
                                    {text('panel.email.domain', 'mailgun-domain', t('settings.mail.mailgun.domainLabel'), t('settings.mail.mailgun.domainPlaceholder'), true)}
                                    {text('panel.email.from', 'mailgun-from', t('settings.mail.mailgun.fromLabel'), t('settings.mail.mailgun.fromPlaceholder'))}
                                    {secret('panel.email.key', 'mailgun-key', t('settings.mail.mailgun.keyLabel'), t('settings.mail.mailgun.keyPlaceholder'))}
                                </>
                            )}
                            {provider === 'mailjet' && (
                                <>
                                    {text('panel.email.domain', 'mailjet-domain', t('settings.mail.mailjet.domainLabel'), t('settings.mail.mailjet.domainPlaceholder'), true)}
                                    {text('panel.email.from', 'mailjet-from', t('settings.mail.mailjet.fromLabel'), t('settings.mail.mailjet.fromPlaceholder'))}
                                    {secret('panel.email.key', 'mailjet-key', t('settings.mail.mailjet.keyLabel'), t('settings.mail.mailjet.keyPlaceholder'))}
                                </>
                            )}
                        </div>
                    </SettingsSection>
                )}
            </div>
        </SettingsCard>
    );
}
