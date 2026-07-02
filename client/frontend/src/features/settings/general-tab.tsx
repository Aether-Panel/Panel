import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
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

export function GeneralTab({ localSettings, handleUpdate, handleSave, saving, t }: GeneralTabProps) {
    return (
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
                            placeholder="panel.example.com"
                        />
                        <p className="text-sm text-muted-foreground">If multiple panels share a database, set the true Master IP here (e.g. panel.example.com). ALL panels will sync this and route LocalNode transfers properly.</p>
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
    );
}
