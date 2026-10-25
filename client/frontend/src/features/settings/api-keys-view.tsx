import { useState, useEffect, useRef } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
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
import { Trash2, Key, Copy, Check, Plus, TriangleAlert } from 'lucide-react';
import { sileo } from "@/lib/toast";
import { api } from '@/lib/api-client';
import { useTranslations } from '@/contexts/translations-context';
import { SettingsCard, SettingsCardHeader } from './general-tab';
import { EmptyState } from '@/components/empty-state';

export function ApiKeysView() {
    const { t, language } = useTranslations();
    const [keys, setKeys] = useState<any[]>([]);
    const [name, setName] = useState('');
    const [isCreating, setIsCreating] = useState(false);
    const [newKeyData, setNewKeyData] = useState<{token: string, key: any} | null>(null);
    const [pendingDelete, setPendingDelete] = useState<any>(null);
    const [copied, setCopied] = useState(false);
    const copyTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

    useEffect(() => () => clearTimeout(copyTimer.current), []);

    const fetchKeys = async () => {
        try {
            const data = await api.get('/api/settings/apikeys');
            setKeys(data);
        } catch (e) {
            console.error(e);
        }
    };

    useEffect(() => {
        void fetchKeys();
    }, []);

    const handleCreate = async () => {
        try {
            const data = await api.post('/api/settings/apikeys', { name, permissions: ['provision'] });
            setNewKeyData(data);
            setName('');
            setIsCreating(false);
            void fetchKeys();
            sileo.success({ title: t('common.success'), description: t('settings.apiKeys.toast.created') });
        } catch (e) {
            console.error(e);
            sileo.error({ title: t('common.error'), description: t('settings.apiKeys.toast.createFailed') });
        }
    };

    const handleDelete = async (id: number) => {
        try {
            await api.delete(`/api/settings/apikeys/${id}`);
            void fetchKeys();
            sileo.success({ title: t('common.success'), description: t('settings.apiKeys.toast.revoked') });
        } catch (e) {
            console.error(e);
        }
    };

    const copyToClipboard = (text: string) => {
        void navigator.clipboard.writeText(text);
        setCopied(true);
        clearTimeout(copyTimer.current);
        copyTimer.current = setTimeout(() => setCopied(false), 1600);
        sileo.success({ title: t('common.copied'), description: t('settings.apiKeys.toast.copied') });
    };

    const formatDate = (value: string) => {
        const d = new Date(value);
        if (Number.isNaN(d.getTime())) return '—';
        return d.toLocaleDateString(language === 'es' ? 'es-ES' : 'en-US', { dateStyle: 'medium' });
    };

    // One dialog: the name form first, then the generated key (shown once).
    const dialogOpen = isCreating || !!newKeyData;
    const onDialogOpenChange = (open: boolean) => {
        if (open) return;
        setIsCreating(false);
        setNewKeyData(null);
        setCopied(false);
    };

    const generateButton = (
        <Button onClick={() => setIsCreating(true)}>
            <Plus aria-hidden="true" />
            {t('settings.apiKeys.generate')}
        </Button>
    );

    return (
        <SettingsCard>
            <SettingsCardHeader
                title={t('settings.apiKeys.title')}
                description={t('settings.apiKeys.description')}
                action={keys.length > 0 ? generateButton : undefined}
            />
            <div className="p-5 md:p-6">
                {keys.length === 0 ? (
                    <EmptyState
                        icon={Key}
                        title={t('settings.apiKeys.emptyTitle')}
                        description={t('settings.apiKeys.emptyDescription')}
                        action={generateButton}
                    />
                ) : (
                    <div className="overflow-hidden rounded-xl border border-border/60 bg-background/30">
                        <Table>
                            <TableHeader className="bg-transparent">
                                <TableRow className="hover:bg-transparent">
                                    <TableHead className="font-medium text-muted-foreground">{t('settings.apiKeys.table.name')}</TableHead>
                                    <TableHead className="font-medium text-muted-foreground">{t('settings.apiKeys.table.prefix')}</TableHead>
                                    <TableHead className="hidden font-medium text-muted-foreground sm:table-cell">{t('settings.apiKeys.table.created')}</TableHead>
                                    <TableHead className="w-14"><span className="sr-only">{t('common.actions')}</span></TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {keys.map(key => (
                                    <TableRow key={key.id} className="hover:bg-accent/30">
                                        <TableCell>
                                            <div className="font-medium">{key.name}</div>
                                            <div className="mt-0.5 text-xs text-muted-foreground sm:hidden">{formatDate(key.created_at)}</div>
                                        </TableCell>
                                        <TableCell className="whitespace-nowrap font-mono text-[13px] text-muted-foreground">{key.prefix}…</TableCell>
                                        <TableCell className="hidden whitespace-nowrap text-sm text-muted-foreground sm:table-cell">{formatDate(key.created_at)}</TableCell>
                                        <TableCell className="text-right">
                                            <Button
                                                variant="ghost"
                                                size="icon"
                                                onClick={() => setPendingDelete(key)}
                                                className="h-9 w-9 text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright"
                                                aria-label={t('settings.apiKeys.revokeAria', { name: key.name })}
                                            >
                                                <Trash2 aria-hidden="true" />
                                            </Button>
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </div>
                )}
            </div>

            <Dialog open={dialogOpen} onOpenChange={onDialogOpenChange}>
                <DialogContent className="sm:max-w-md">
                    {newKeyData ? (
                        <>
                            <DialogHeader>
                                <DialogTitle>{t('settings.apiKeys.createdTitle')}</DialogTitle>
                                <DialogDescription>{t('settings.apiKeys.createdDescription')}</DialogDescription>
                            </DialogHeader>
                            <div className="space-y-3">
                                <div className="flex items-start gap-2 rounded-lg border border-warning/30 bg-warning/10 px-3 py-2.5 text-sm text-warning">
                                    <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
                                    <span>{t('settings.apiKeys.createdWarning')}</span>
                                </div>
                                <Label htmlFor="new-api-key" className="sr-only">{t('settings.apiKeys.title')}</Label>
                                <div className="flex gap-2">
                                    <Input
                                        id="new-api-key"
                                        readOnly
                                        value={newKeyData.token}
                                        onFocus={(e) => e.currentTarget.select()}
                                        className="font-mono text-[13px] md:text-[13px]"
                                    />
                                    <Button variant="secondary" onClick={() => copyToClipboard(newKeyData.token)} className="shrink-0">
                                        {copied ? <Check className="text-success" aria-hidden="true" /> : <Copy aria-hidden="true" />}
                                        {copied ? t('common.copied') : t('common.copy')}
                                    </Button>
                                </div>
                            </div>
                            <DialogFooter>
                                <Button onClick={() => onDialogOpenChange(false)}>{t('settings.apiKeys.done')}</Button>
                            </DialogFooter>
                        </>
                    ) : (
                        <form
                            className="grid gap-4"
                            onSubmit={(e) => {
                                e.preventDefault();
                                if (name) void handleCreate();
                            }}
                        >
                            <DialogHeader>
                                <DialogTitle>{t('settings.apiKeys.createTitle')}</DialogTitle>
                                <DialogDescription>{t('settings.apiKeys.createDescription')}</DialogDescription>
                            </DialogHeader>
                            <div className="space-y-2">
                                <Label htmlFor="api-key-name">{t('settings.apiKeys.nameLabel')}</Label>
                                <Input id="api-key-name" value={name} onChange={e => setName(e.target.value)} placeholder={t('settings.apiKeys.namePlaceholder')} autoComplete="off" />
                            </div>
                            <DialogFooter>
                                <Button type="button" variant="outline" onClick={() => setIsCreating(false)}>{t('common.cancel')}</Button>
                                <Button type="submit" disabled={!name}>{t('settings.apiKeys.generateAction')}</Button>
                            </DialogFooter>
                        </form>
                    )}
                </DialogContent>
            </Dialog>

            <AlertDialog open={!!pendingDelete} onOpenChange={(open) => !open && setPendingDelete(null)}>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>{t('settings.apiKeys.revokeTitle')}</AlertDialogTitle>
                        <AlertDialogDescription>
                            {t('settings.apiKeys.revokeDescription', { name: pendingDelete?.name ?? '' })}
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                        <AlertDialogAction
                            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                            onClick={() => pendingDelete && void handleDelete(pendingDelete.id)}
                        >
                            {t('settings.apiKeys.revoke')}
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </SettingsCard>
    );
}
