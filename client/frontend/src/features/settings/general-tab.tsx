import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';
import { Loader2 } from 'lucide-react';

interface GeneralTabProps {
    localSettings: Record<string, any>;
    handleUpdate: (key: string, value: any) => void;
    handleSave: () => void;
    saving: boolean;
    t: (key: string) => string;
}

export function SettingsSection({ id, title, description, children }: { id?: string, title: string, description: string | React.ReactNode, children: React.ReactNode }) {
    return (
        <div className="flex flex-col md:flex-row gap-6 py-6 border-b border-border/40 last:border-0">
            <div className="md:w-1/3 shrink-0">
                <Label htmlFor={id} className="text-sm font-medium text-foreground mb-1.5 block">{title}</Label>
                <div className="text-[13px] text-muted-foreground leading-relaxed pr-4">{description}</div>
            </div>
            <div className="md:w-2/3 max-w-2xl flex flex-col justify-center">
                {children}
            </div>
        </div>
    );
}

export function GeneralTab({ localSettings, handleUpdate, handleSave, saving, t }: GeneralTabProps) {
    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-xl font-semibold tracking-tight">{t('settings.general.title')}</h2>
                <p className="text-sm text-muted-foreground mt-1">{t('settings.general.description')}</p>
            </div>
            
            <div className="rounded-xl border border-border/60 bg-card shadow-sm">
                <div className="px-6">
                    <SettingsSection 
                        id="base-url" 
                        title={t('settings.general.baseUrlLabel')} 
                        description={t('settings.general.baseUrlDescription')}
                    >
                        <Input
                            id="base-url"
                            value={localSettings['panel.settings.masterUrl'] || ''}
                            onChange={(e) => handleUpdate('panel.settings.masterUrl', e.target.value)}
                            className="bg-background/50"
                        />
                    </SettingsSection>

                    <SettingsSection 
                        id="master-node-ip" 
                        title={t('settings.general.masterNodeIpLabel')} 
                        description={t('settings.general.masterNodeIpDescription')}
                    >
                        <Input
                            id="master-node-ip"
                            value={localSettings['panel.settings.masterNodeIp'] || ''}
                            onChange={(e) => handleUpdate('panel.settings.masterNodeIp', e.target.value)}
                            placeholder="panel.example.com"
                            className="bg-background/50"
                        />
                    </SettingsSection>

                    <SettingsSection 
                        id="company-name" 
                        title={t('settings.general.companyNameLabel')} 
                        description={t('settings.general.companyNameDescription')}
                    >
                        <Input
                            id="company-name"
                            value={localSettings['panel.settings.companyName'] || ''}
                            onChange={(e) => handleUpdate('panel.settings.companyName', e.target.value)}
                            className="bg-background/50"
                        />
                    </SettingsSection>

                    {!localSettings['panel.settings.hideAiAnalysis'] && (
                        <SettingsSection 
                            id="gemini-api-key" 
                            title={t('settings.general.geminiApiKeyLabel')} 
                            description={t('settings.general.geminiApiKeyDescription')}
                        >
                            <Input
                                id="gemini-api-key"
                                type="password"
                                value={localSettings['panel.settings.geminiApiKey'] || ''}
                                onChange={(e) => handleUpdate('panel.settings.geminiApiKey', e.target.value)}
                                placeholder={t('settings.general.geminiApiKeyPlaceholder')}
                                className="bg-background/50"
                            />
                        </SettingsSection>
                    )}

                    <SettingsSection 
                        id="hide-ai-analysis" 
                        title={t('settings.general.hideAiAnalysisLabel')} 
                        description={t('settings.general.hideAiAnalysisDescription')}
                    >
                        <div className="flex items-center h-10">
                            <Switch
                                id="hide-ai-analysis"
                                checked={!!localSettings['panel.settings.hideAiAnalysis']}
                                onCheckedChange={(v) => handleUpdate('panel.settings.hideAiAnalysis', v)}
                            />
                        </div>
                    </SettingsSection>

                    <SettingsSection 
                        id="header-decorations" 
                        title={t('settings.general.headerDecorationsLabel')} 
                        description={t('settings.general.headerDecorationsDescription')}
                    >
                        <div className="flex items-center h-10">
                            <Switch
                                id="header-decorations"
                                checked={localSettings['panel.settings.headerDecorations'] !== false}
                                onCheckedChange={(v) => handleUpdate('panel.settings.headerDecorations', v)}
                            />
                        </div>
                    </SettingsSection>

                    <SettingsSection 
                        id="allow-registration" 
                        title={t('settings.general.allowRegistrationLabel')} 
                        description={t('settings.general.allowRegistrationDescription')}
                    >
                        <div className="flex items-center h-10">
                            <Switch
                                id="allow-registration"
                                checked={!!localSettings['panel.registrationEnabled']}
                                onCheckedChange={(v) => handleUpdate('panel.registrationEnabled', v)}
                            />
                        </div>
                    </SettingsSection>
                </div>
                <div className="flex items-center justify-end px-6 py-4 bg-muted/30 border-t border-border/60 rounded-b-xl">
                    <Button onClick={handleSave} disabled={saving} className="min-w-[120px]">
                        {saving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        {t('settings.general.saveButton')}
                    </Button>
                </div>
            </div>
        </div>
    );
}
