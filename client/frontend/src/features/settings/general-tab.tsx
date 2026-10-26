import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { cn } from '@/lib/utils';
import { Check, Copy, CopyPlus, Eye, EyeOff, Layers, MapPin, SlidersHorizontal, type LucideIcon } from 'lucide-react';

interface GeneralTabProps {
    localSettings: Record<string, any>;
    handleUpdate: (key: string, value: any) => void;
    t: (key: string) => string;
}

/* ------------------------------------------------------------------ */
/* Shared building blocks for the panel settings tabs                  */
/* ------------------------------------------------------------------ */

/** Header row of a settings tab card: title + description, optional action on the right. */
export function SettingsCardHeader({ title, description, action }: Readonly<{ title: string; description?: string; action?: ReactNode }>) {
    return (
        <div className="flex flex-col gap-4 border-b border-border/50 p-5 sm:flex-row sm:items-center sm:justify-between md:p-6">
            <div className="min-w-0 space-y-1">
                <h2 className="text-lg font-semibold tracking-[-0.02em]">{title}</h2>
                {description && <p className="text-pretty text-sm text-muted-foreground">{description}</p>}
            </div>
            {action && <div className="flex shrink-0 flex-wrap gap-2 self-start sm:self-auto">{action}</div>}
        </div>
    );
}

/** A settings tab: one card, sections divided by hairlines. */
export function SettingsCard({ children }: Readonly<{ children: ReactNode }>) {
    return <Card className="overflow-hidden">{children}</Card>;
}

/**
 * One block of a settings tab: what it is and why on the left, the
 * controls on the right; stacked on narrow screens.
 */
export function SettingsSection({ title, description, children }: Readonly<{ title: string; description?: ReactNode; children: ReactNode }>) {
    return (
        <section className="grid gap-5 p-5 md:p-6 lg:grid-cols-[minmax(0,16rem)_minmax(0,1fr)] lg:gap-10">
            <div className="space-y-1.5">
                <h3 className="text-sm font-semibold text-foreground">{title}</h3>
                {description && <div className="text-pretty text-sm text-muted-foreground">{description}</div>}
            </div>
            <div className="min-w-0">{children}</div>
        </section>
    );
}

/** Label + control + helper text, stacked. */
export function SettingsField({ id, label, hint, className, children }: Readonly<{ id: string; label: string; hint?: ReactNode; className?: string; children: ReactNode }>) {
    return (
        <div className={cn('space-y-2', className)}>
            <Label htmlFor={id} className="text-sm font-medium">{label}</Label>
            {children}
            {hint && <p className="text-pretty text-xs text-muted-foreground">{hint}</p>}
        </div>
    );
}

/** A label + description on the left and a switch on the right. */
export function SettingRow({ id, label, description, checked, onCheckedChange }: Readonly<{ id: string; label: string; description?: string; checked: boolean; onCheckedChange: (v: boolean) => void }>) {
    return (
        <div className="flex items-center justify-between gap-4 px-4 py-3.5">
            <div className="min-w-0">
                <Label htmlFor={id} className="block text-sm font-medium leading-snug">{label}</Label>
                {description && <p className="mt-1 text-pretty text-xs text-muted-foreground">{description}</p>}
            </div>
            <Switch id={id} checked={checked} onCheckedChange={onCheckedChange} />
        </div>
    );
}

/** Container for a list of switch rows. */
export function SettingRows({ children }: Readonly<{ children: ReactNode }>) {
    return <div className="divide-y divide-border/50 rounded-xl border border-border/60 bg-background/30">{children}</div>;
}

/**
 * Switch row for an experimental feature: what it does and where it shows up
 * once it is on. The icon lights up with the switch.
 */
function ExperimentalRow({ id, icon: Icon, label, description, where, checked, onCheckedChange }: Readonly<{
    id: string;
    icon: LucideIcon;
    label: string;
    description: string;
    where: string;
    checked: boolean;
    onCheckedChange: (v: boolean) => void;
}>) {
    return (
        <div className="flex items-start gap-3.5 px-4 py-3.5">
            <span
                aria-hidden="true"
                className={cn(
                    'mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border transition-colors duration-300',
                    checked ? 'border-primary/40 bg-primary/10 text-primary-bright' : 'border-border/60 bg-card text-muted-foreground',
                )}
            >
                <Icon className="h-4 w-4" />
            </span>
            <div className="min-w-0 flex-1">
                <Label htmlFor={id} className="block text-sm font-medium leading-snug">{label}</Label>
                <p className="mt-1 text-pretty text-xs text-muted-foreground">{description}</p>
                <p className="mt-1.5 flex items-center gap-1 text-[11px] text-muted-foreground/80">
                    <MapPin className="h-3 w-3 shrink-0" aria-hidden="true" />
                    {where}
                </p>
            </div>
            <Switch id={id} checked={checked} onCheckedChange={onCheckedChange} className="mt-1" />
        </div>
    );
}

/**
 * Masked input for credentials (API keys, passwords, webhook URLs) with
 * reveal and copy buttons inside the field.
 */
export function SecretInput({
    id,
    value,
    onChange,
    placeholder,
    t,
}: Readonly<{
    id: string;
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    t: (key: string) => string;
}>) {
    const [revealed, setRevealed] = useState(false);
    const [copied, setCopied] = useState(false);
    const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

    useEffect(() => () => clearTimeout(timer.current), []);

    const copy = async () => {
        if (!value) return;
        try {
            await navigator.clipboard.writeText(value);
            setCopied(true);
            clearTimeout(timer.current);
            timer.current = setTimeout(() => setCopied(false), 1600);
        } catch {
            /* clipboard unavailable */
        }
    };

    return (
        <div className="relative">
            <Input
                id={id}
                type={revealed ? 'text' : 'password'}
                autoComplete="off"
                spellCheck={false}
                value={value}
                onChange={(e) => onChange(e.target.value)}
                placeholder={placeholder}
                className="pr-[5.25rem] [&:not(:placeholder-shown)]:font-mono [&:not(:placeholder-shown)]:text-[13px]"
            />
            <div className="absolute inset-y-0 right-0.5 flex items-center">
                <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="h-9 w-9 text-muted-foreground hover:text-foreground"
                    onClick={() => setRevealed((v) => !v)}
                    aria-label={revealed ? t('settings.secret.hide') : t('settings.secret.show')}
                    aria-pressed={revealed}
                    aria-controls={id}
                >
                    {revealed ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}
                </Button>
                <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="h-9 w-9 text-muted-foreground hover:text-foreground"
                    onClick={copy}
                    disabled={!value}
                    aria-label={copied ? t('common.copied') : t('common.copy')}
                >
                    {copied ? <Check className="text-success" aria-hidden="true" /> : <Copy aria-hidden="true" />}
                </Button>
            </div>
            <span className="sr-only" aria-live="polite">{copied ? t('common.copied') : ''}</span>
        </div>
    );
}

/* ------------------------------------------------------------------ */

/** Experimental features, in the order they are listed; each is a panel setting that starts off. */
const EXPERIMENTAL_FEATURES: { key: string; name: string; icon: LucideIcon }[] = [
    { key: 'panel.experimental.versionSwitch', name: 'versionSwitch', icon: Layers },
    { key: 'panel.experimental.propertiesEditor', name: 'propertiesEditor', icon: SlidersHorizontal },
    { key: 'panel.experimental.serverClone', name: 'serverClone', icon: CopyPlus },
];

export function GeneralTab({ localSettings, handleUpdate, t }: Readonly<GeneralTabProps>) {
    const hideAi = !!localSettings['panel.settings.hideAiAnalysis'];
    const turnstile = !!localSettings['panel.turnstile.enabled'];

    return (
        <SettingsCard>
            <SettingsCardHeader title={t('settings.general.title')} description={t('settings.general.description')} />
            <div className="divide-y divide-border/50">
                <SettingsSection title={t('settings.general.sections.panelTitle')} description={t('settings.general.sections.panelDescription')}>
                    <div className="space-y-5">
                        <SettingsField id="company-name" label={t('settings.general.companyNameLabel')} hint={t('settings.general.companyNameDescription')}>
                            <Input
                                id="company-name"
                                value={localSettings['panel.settings.companyName'] || ''}
                                onChange={(e) => handleUpdate('panel.settings.companyName', e.target.value)}
                            />
                        </SettingsField>
                        <SettingsField id="base-url" label={t('settings.general.baseUrlLabel')} hint={t('settings.general.baseUrlDescription')}>
                            <Input
                                id="base-url"
                                type="url"
                                inputMode="url"
                                spellCheck={false}
                                value={localSettings['panel.settings.masterUrl'] || ''}
                                onChange={(e) => handleUpdate('panel.settings.masterUrl', e.target.value)}
                                className="font-mono text-[13px] md:text-[13px]"
                            />
                        </SettingsField>
                        <SettingsField id="master-node-ip" label={t('settings.general.masterNodeIpLabel')} hint={t('settings.general.masterNodeIpDescription')}>
                            <Input
                                id="master-node-ip"
                                spellCheck={false}
                                value={localSettings['panel.settings.masterNodeIp'] || ''}
                                onChange={(e) => handleUpdate('panel.settings.masterNodeIp', e.target.value)}
                                placeholder="panel.example.com"
                                className="[&:not(:placeholder-shown)]:font-mono [&:not(:placeholder-shown)]:text-[13px]"
                            />
                        </SettingsField>
                    </div>
                </SettingsSection>

                <SettingsSection title={t('settings.general.sections.aiTitle')} description={t('settings.general.sections.aiDescription')}>
                    <div className="space-y-5">
                        <SettingRows>
                            <SettingRow
                                id="hide-ai-analysis"
                                label={t('settings.general.hideAiAnalysisLabel')}
                                description={t('settings.general.hideAiAnalysisDescription')}
                                checked={hideAi}
                                onCheckedChange={(v) => handleUpdate('panel.settings.hideAiAnalysis', v)}
                            />
                        </SettingRows>
                        {!hideAi && (
                            <SettingsField id="gemini-api-key" label={t('settings.general.geminiApiKeyLabel')} hint={t('settings.general.geminiApiKeyDescription')}>
                                <SecretInput
                                    id="gemini-api-key"
                                    value={localSettings['panel.settings.geminiApiKey'] || ''}
                                    onChange={(v) => handleUpdate('panel.settings.geminiApiKey', v)}
                                    placeholder={t('settings.general.geminiApiKeyPlaceholder')}
                                    t={t}
                                />
                            </SettingsField>
                        )}
                    </div>
                </SettingsSection>

                <SettingsSection title={t('settings.general.sections.modpacksTitle')} description={t('settings.general.sections.modpacksDescription')}>
                    <SettingsField id="curseforge-api-key" label={t('settings.general.curseforgeKeyLabel')} hint={t('settings.general.curseforgeKeyDescription')}>
                        <SecretInput
                            id="curseforge-api-key"
                            value={localSettings['daemon.curseforge.key'] || ''}
                            onChange={(v) => handleUpdate('daemon.curseforge.key', v)}
                            placeholder={t('settings.general.curseforgeKeyPlaceholder')}
                            t={t}
                        />
                    </SettingsField>
                </SettingsSection>

                <SettingsSection title={t('settings.general.sections.accessTitle')} description={t('settings.general.sections.accessDescription')}>
                    <div className="space-y-5">
                        <SettingRows>
                            <SettingRow
                                id="allow-registration"
                                label={t('settings.general.allowRegistrationLabel')}
                                description={t('settings.general.allowRegistrationDescription')}
                                checked={!!localSettings['panel.registrationEnabled']}
                                onCheckedChange={(v) => handleUpdate('panel.registrationEnabled', v)}
                            />
                            <SettingRow
                                id="turnstile-enabled"
                                label={t('settings.general.turnstileEnabledLabel')}
                                description={t('settings.general.turnstileEnabledDescription')}
                                checked={turnstile}
                                onCheckedChange={(v) => handleUpdate('panel.turnstile.enabled', v)}
                            />
                        </SettingRows>
                        {turnstile && (
                            <div className="grid gap-5 md:grid-cols-2">
                                <SettingsField id="turnstile-site-key" label={t('settings.general.turnstileSiteKeyLabel')} hint={t('settings.general.turnstileSiteKeyDescription')}>
                                    <Input
                                        id="turnstile-site-key"
                                        spellCheck={false}
                                        value={localSettings['panel.turnstile.siteKey'] || ''}
                                        onChange={(e) => handleUpdate('panel.turnstile.siteKey', e.target.value)}
                                        placeholder={t('settings.general.turnstileSiteKeyPlaceholder')}
                                        className="font-mono text-[13px] md:text-[13px]"
                                    />
                                </SettingsField>
                                <SettingsField id="turnstile-secret-key" label={t('settings.general.turnstileSecretKeyLabel')} hint={t('settings.general.turnstileSecretKeyDescription')}>
                                    <SecretInput
                                        id="turnstile-secret-key"
                                        value={localSettings['panel.turnstile.secretKey'] || ''}
                                        onChange={(v) => handleUpdate('panel.turnstile.secretKey', v)}
                                        placeholder={t('settings.general.turnstileSecretKeyPlaceholder')}
                                        t={t}
                                    />
                                </SettingsField>
                            </div>
                        )}
                    </div>
                </SettingsSection>

                <SettingsSection title={t('settings.general.sections.experimentalTitle')} description={t('settings.general.sections.experimentalDescription')}>
                    <SettingRows>
                        {EXPERIMENTAL_FEATURES.map((feature) => (
                            <ExperimentalRow
                                key={feature.key}
                                id={`experimental-${feature.name}`}
                                icon={feature.icon}
                                label={t(`settings.general.experimental.${feature.name}Label`)}
                                description={t(`settings.general.experimental.${feature.name}Description`)}
                                where={t(`settings.general.experimental.${feature.name}Where`)}
                                checked={!!localSettings[feature.key]}
                                onCheckedChange={(v) => handleUpdate(feature.key, v)}
                            />
                        ))}
                    </SettingRows>
                </SettingsSection>
            </div>
        </SettingsCard>
    );
}
