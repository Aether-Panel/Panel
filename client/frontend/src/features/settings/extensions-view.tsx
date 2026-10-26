import { useEffect, useRef, useState } from 'react';
import { Blocks, Check, Copy, ExternalLink, Loader2, Send, Trash2, Upload } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Badge } from '@/components/ui/badge';
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { EmptyState } from '@/components/empty-state';
import { api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { useTranslations } from '@/contexts/translations-context';
import { SettingsCard, SettingsCardHeader } from './general-tab';

type InstalledExtension = {
    id: string;
    name: string;
    version: string;
    author: string;
    description: string;
    enabled: boolean;
    webhookUrl: string;
    manifestWebhook: string;
    secret: string;
    pages: { id: string; title: string; placement: 'server' | 'admin'; url: string }[];
    events: string[];
};

/** Install, switch on and off, and uninstall panel extensions. */
export function ExtensionsView() {
    const { t } = useTranslations();
    const [list, setList] = useState<InstalledExtension[] | null>(null);
    const [installing, setInstalling] = useState(false);
    const [pendingRemove, setPendingRemove] = useState<InstalledExtension | null>(null);
    const fileRef = useRef<HTMLInputElement>(null);

    const load = async () => {
        try {
            setList(await api.get<InstalledExtension[]>('/api/extensions'));
        } catch {
            setList([]);
            sileo.error({ title: t('common.error'), description: t('extensions.loadError') });
        }
    };

    useEffect(() => {
        void load();
    }, []);

    const handleFile = async (file: File | undefined) => {
        if (!file) return;
        setInstalling(true);
        try {
            const form = new FormData();
            form.append('file', file);
            const ext = await api.postForm<InstalledExtension>('/api/extensions', form);
            sileo.success({ title: t('extensions.installed'), description: t('extensions.installedDescription', { name: ext.name }) });
            await load();
        } catch (e: any) {
            sileo.error({ title: t('extensions.installFailed'), description: e?.message || t('extensions.installFailed') });
        } finally {
            setInstalling(false);
            if (fileRef.current) fileRef.current.value = '';
        }
    };

    const update = async (ext: InstalledExtension, body: Partial<Pick<InstalledExtension, 'enabled' | 'webhookUrl'>>) => {
        try {
            await api.put(`/api/extensions/${ext.id}`, body);
            await load();
            return true;
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e?.message || t('extensions.saveError') });
            return false;
        }
    };

    const remove = async () => {
        if (!pendingRemove) return;
        try {
            await api.delete(`/api/extensions/${pendingRemove.id}`);
            sileo.success({ title: t('common.success'), description: t('extensions.removed', { name: pendingRemove.name }) });
            await load();
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e?.message || t('extensions.removeError') });
        } finally {
            setPendingRemove(null);
        }
    };

    const installButton = (
        <Button onClick={() => fileRef.current?.click()} disabled={installing}>
            {installing ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Upload aria-hidden="true" />}
            {t('extensions.install')}
        </Button>
    );

    return (
        <SettingsCard>
            <SettingsCardHeader
                title={t('extensions.title')}
                description={t('extensions.description')}
                action={list && list.length > 0 ? installButton : undefined}
            />
            <input
                ref={fileRef}
                type="file"
                accept=".zip,application/zip"
                className="sr-only"
                tabIndex={-1}
                aria-hidden="true"
                onChange={(e) => void handleFile(e.target.files?.[0])}
            />
            <div className="p-5 md:p-6">
                {list === null ? (
                    <div className="flex h-24 items-center justify-center text-muted-foreground">
                        <Loader2 className="h-5 w-5 animate-spin" aria-label={t('common.loading')} />
                    </div>
                ) : list.length === 0 ? (
                    <EmptyState
                        icon={Blocks}
                        title={t('extensions.emptyTitle')}
                        description={t('extensions.emptyDescription')}
                        action={installButton}
                    />
                ) : (
                    <ul className="space-y-4">
                        {list.map((ext) => (
                            <ExtensionItem key={ext.id} ext={ext} onUpdate={update} onRemove={() => setPendingRemove(ext)} />
                        ))}
                    </ul>
                )}
            </div>

            <AlertDialog open={pendingRemove !== null} onOpenChange={(open) => !open && setPendingRemove(null)}>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>{t('extensions.removeTitle')}</AlertDialogTitle>
                        <AlertDialogDescription>{t('extensions.removeDescription', { name: pendingRemove?.name ?? '' })}</AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                        <AlertDialogAction onClick={() => void remove()} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                            {t('extensions.removeConfirm')}
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </SettingsCard>
    );
}

function ExtensionItem({
    ext,
    onUpdate,
    onRemove,
}: Readonly<{
    ext: InstalledExtension;
    onUpdate: (ext: InstalledExtension, body: Partial<Pick<InstalledExtension, 'enabled' | 'webhookUrl'>>) => Promise<boolean>;
    onRemove: () => void;
}>) {
    const { t } = useTranslations();
    const [webhook, setWebhook] = useState(ext.webhookUrl);
    const [saving, setSaving] = useState(false);
    const [testing, setTesting] = useState(false);
    const [copied, setCopied] = useState(false);
    const copyTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

    useEffect(() => setWebhook(ext.webhookUrl), [ext.webhookUrl]);
    useEffect(() => () => clearTimeout(copyTimer.current), []);

    const switchId = `extension-enabled-${ext.id}`;
    const webhookId = `extension-webhook-${ext.id}`;
    const webhookDirty = webhook.trim() !== ext.webhookUrl;
    const usesEvents = ext.events.length > 0;

    const saveWebhook = async () => {
        setSaving(true);
        if (await onUpdate(ext, { webhookUrl: webhook.trim() })) {
            sileo.success({ title: t('common.success'), description: t('extensions.webhookSaved') });
        }
        setSaving(false);
    };

    const test = async () => {
        setTesting(true);
        try {
            await api.post(`/api/extensions/${ext.id}/test`, {});
            sileo.success({ title: t('extensions.testOk'), description: t('extensions.testOkDescription') });
        } catch (e: any) {
            sileo.error({ title: t('extensions.testFailed'), description: e?.message || t('extensions.testFailed') });
        } finally {
            setTesting(false);
        }
    };

    const copySecret = () => {
        void navigator.clipboard.writeText(ext.secret);
        setCopied(true);
        clearTimeout(copyTimer.current);
        copyTimer.current = setTimeout(() => setCopied(false), 1600);
    };

    return (
        <li className="rounded-xl border border-border/60 bg-background/30">
            <div className="flex flex-col gap-4 p-4 sm:flex-row sm:items-start sm:justify-between">
                <div className="min-w-0 space-y-1.5">
                    <div className="flex flex-wrap items-center gap-2">
                        <h3 className="text-sm font-semibold">{ext.name}</h3>
                        <Badge variant="outline" className="font-mono text-[11px]">v{ext.version}</Badge>
                        <code className="font-mono text-xs text-muted-foreground">{ext.id}</code>
                    </div>
                    {ext.author && <p className="text-xs text-muted-foreground">{t('extensions.by', { author: ext.author })}</p>}
                    {ext.description && <p className="text-pretty text-sm text-muted-foreground">{ext.description}</p>}
                </div>
                <div className="flex shrink-0 items-center gap-3">
                    <Label htmlFor={switchId} className="text-sm text-muted-foreground">
                        {ext.enabled ? t('extensions.enabled') : t('extensions.disabled')}
                    </Label>
                    <Switch id={switchId} checked={ext.enabled} onCheckedChange={(enabled) => void onUpdate(ext, { enabled })} />
                </div>
            </div>

            <div className="grid gap-4 border-t border-border/50 p-4 lg:grid-cols-2">
                <div className="min-w-0 space-y-2">
                    <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{t('extensions.pages')}</p>
                    {ext.pages.length === 0 ? (
                        <p className="text-sm text-muted-foreground">{t('extensions.noPages')}</p>
                    ) : (
                        <ul className="space-y-1.5">
                            {ext.pages.map((page) => (
                                <li key={page.id} className="flex flex-wrap items-center gap-2 text-sm">
                                    <Badge variant="secondary" className="text-[11px]">
                                        {page.placement === 'server' ? t('extensions.placementServer') : t('extensions.placementAdmin')}
                                    </Badge>
                                    {page.placement === 'admin' && ext.enabled ? (
                                        <a
                                            href={`/extension/?id=${encodeURIComponent(ext.id)}&page=${encodeURIComponent(page.id)}`}
                                            className="inline-flex items-center gap-1 rounded text-primary-bright hover:underline focus-ring"
                                        >
                                            {page.title}
                                            <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
                                        </a>
                                    ) : (
                                        <span>{page.title}</span>
                                    )}
                                </li>
                            ))}
                        </ul>
                    )}
                </div>

                <div className="min-w-0 space-y-2">
                    <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{t('extensions.events')}</p>
                    {usesEvents ? (
                        <div className="flex flex-wrap gap-1.5">
                            {ext.events.map((event) => (
                                <code key={event} className="rounded-md border border-border/60 bg-surface-2 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
                                    {event}
                                </code>
                            ))}
                        </div>
                    ) : (
                        <p className="text-sm text-muted-foreground">{t('extensions.noEvents')}</p>
                    )}
                </div>
            </div>

            {(usesEvents || ext.webhookUrl) && (
                <div className="space-y-4 border-t border-border/50 p-4">
                    <div className="space-y-2">
                        <Label htmlFor={webhookId} className="text-sm font-medium">{t('extensions.webhook')}</Label>
                        <div className="flex flex-col gap-2 sm:flex-row">
                            <Input
                                id={webhookId}
                                type="url"
                                inputMode="url"
                                value={webhook}
                                placeholder="https://"
                                onChange={(e) => setWebhook(e.target.value)}
                                className="font-mono text-[13px] md:text-[13px]"
                            />
                            {webhookDirty && (
                                <Button variant="secondary" onClick={() => void saveWebhook()} disabled={saving} className="shrink-0">
                                    {saving && <Loader2 className="animate-spin" aria-hidden="true" />}
                                    {t('common.save')}
                                </Button>
                            )}
                        </div>
                        <p className="text-xs text-muted-foreground">{t('extensions.webhookHint')}</p>
                    </div>
                    <div className="space-y-2">
                        <span className="block text-sm font-medium">{t('extensions.secret')}</span>
                        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                            <code className="min-w-0 flex-1 truncate rounded-lg border border-border/60 bg-surface-2 px-3 py-2 font-mono text-xs text-muted-foreground">
                                {ext.secret}
                            </code>
                            <div className="flex shrink-0 gap-2">
                                <Button variant="outline" size="sm" onClick={copySecret}>
                                    {copied ? <Check className="text-success" aria-hidden="true" /> : <Copy aria-hidden="true" />}
                                    {copied ? t('common.copied') : t('common.copy')}
                                </Button>
                                <Button variant="outline" size="sm" onClick={() => void test()} disabled={testing || !ext.webhookUrl || webhookDirty}>
                                    {testing ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Send aria-hidden="true" />}
                                    {t('extensions.test')}
                                </Button>
                            </div>
                        </div>
                        <p className="text-xs text-muted-foreground">{t('extensions.secretHint')}</p>
                    </div>
                </div>
            )}

            <div className="flex justify-end border-t border-border/50 px-4 py-3">
                <Button
                    variant="ghost"
                    size="sm"
                    onClick={onRemove}
                    className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright"
                >
                    <Trash2 aria-hidden="true" />
                    {t('extensions.remove')}
                </Button>
            </div>
        </li>
    );
}
