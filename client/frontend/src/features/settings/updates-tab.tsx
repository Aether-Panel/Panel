import { useState, useEffect, useCallback, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
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
import { DownloadCloud, Loader2, CheckCircle2, XCircle, Info, RefreshCw, ArrowUpCircle } from 'lucide-react';
import { sileo } from '@/lib/toast';
import { api } from '@/lib/api-client';
import { cn } from '@/lib/utils';
import { useTranslations } from '@/contexts/translations-context';
import { SettingsCard, SettingsCardHeader } from './general-tab';

interface UpdateCheckResult {
    current: string;
    latest: string;
    version: string;
    updateAvailable: boolean;
}

interface NodeUpdateStatus {
    name: string;
    address: string;
    local: boolean;
    online: boolean;
    running: boolean;
    containerId: string;
    startedAt: string;
    finishedAt: string;
    exitCode: number;
    log: string;
    error: string;
}

interface UpdateStatusResult {
    nodes: NodeUpdateStatus[];
}

type Tone = 'muted' | 'primary' | 'success' | 'warning' | 'destructive';

const toneText: Record<Tone, string> = {
    muted: 'text-muted-foreground',
    primary: 'text-primary-bright',
    success: 'text-success',
    warning: 'text-warning',
    destructive: 'text-destructive-bright',
};

const toneBadge: Record<Tone, string> = {
    muted: 'bg-muted/60 text-muted-foreground',
    primary: 'bg-primary/10 text-primary-bright',
    success: 'bg-success/10 text-success',
    warning: 'bg-warning/10 text-warning',
    destructive: 'bg-destructive/10 text-destructive-bright',
};

const toneDot: Record<Tone, string> = {
    muted: 'bg-muted-foreground',
    primary: 'bg-primary-bright',
    success: 'bg-success',
    warning: 'bg-warning',
    destructive: 'bg-destructive-bright',
};

type Translate = ReturnType<typeof useTranslations>['t'];

interface StatusView {
    statusTitle: string;
    StatusIcon: typeof Loader2;
    spin: boolean;
    tone: Tone;
    statusDescription: string;
}

/** Status headline while an update is running across the fleet. */
function updatingStatus(nodes: NodeUpdateStatus[], reconnecting: boolean, t: Translate): StatusView {
    const base = { StatusIcon: Loader2, spin: true, tone: 'primary' as Tone, statusDescription: '' };
    if (reconnecting) {
        return {
            ...base,
            statusTitle: t('settings.updates.status.restarting'),
            statusDescription: t('settings.updates.status.restartingDescription'),
        };
    }
    if (nodes.some((n) => n.running)) {
        return {
            ...base,
            statusTitle: t('settings.updates.status.updatingNodes', { running: nodes.filter((n) => n.running).length, total: nodes.length }),
            statusDescription: t('settings.updates.status.updatingDescription'),
        };
    }
    if (nodes.some((n) => n.online && n.exitCode !== 0)) {
        return { ...base, statusTitle: t('settings.updates.status.finishedWithErrors'), StatusIcon: XCircle, spin: false, tone: 'destructive' };
    }
    if (nodes.some((n) => !n.online)) {
        return { ...base, statusTitle: t('settings.updates.status.nodesUnreachable'), StatusIcon: Info, spin: false, tone: 'warning' };
    }
    return { ...base, statusTitle: t('settings.updates.status.complete'), StatusIcon: CheckCircle2, spin: false, tone: 'success' };
}

/** Status headline once the version check has answered and nothing is running. */
function idleStatus(
    flags: { updateFailed: boolean; checkFailed: boolean; unknownCurrent: boolean; updateAvailable: boolean },
    t: Translate,
): StatusView {
    const base = { spin: false, statusDescription: '' };
    if (flags.updateFailed) {
        return { ...base, statusTitle: t('settings.updates.status.failed'), StatusIcon: XCircle, tone: 'destructive' };
    }
    if (flags.checkFailed) {
        return {
            ...base,
            statusTitle: t('settings.updates.status.checkFailed'),
            StatusIcon: XCircle,
            tone: 'destructive',
            statusDescription: t('settings.updates.status.checkFailedDescription'),
        };
    }
    if (flags.unknownCurrent) {
        return {
            ...base,
            statusTitle: t('settings.updates.status.unknownCurrent'),
            StatusIcon: Info,
            tone: 'warning',
            statusDescription: t('settings.updates.status.unknownCurrentDescription'),
        };
    }
    if (flags.updateAvailable) {
        return {
            ...base,
            statusTitle: t('settings.updates.status.available'),
            StatusIcon: ArrowUpCircle,
            tone: 'primary',
            statusDescription: t('settings.updates.status.availableDescription'),
        };
    }
    return { ...base, statusTitle: t('settings.updates.status.upToDate'), StatusIcon: CheckCircle2, tone: 'success' };
}

/** Label, tone and icon for one node's row in the fleet list. */
function nodeRowState(node: NodeUpdateStatus, t: Translate): { label: string; nodeTone: Tone; icon: ReactNode } {
    if (node.running) {
        return {
            label: t('settings.updates.node.updating'),
            nodeTone: 'primary',
            icon: <Loader2 className="h-4 w-4 animate-spin text-primary-bright" aria-hidden="true" />,
        };
    }
    if (!node.online) {
        return {
            label: node.error || t('settings.updates.node.unreachable'),
            nodeTone: 'destructive',
            icon: <XCircle className="h-4 w-4 text-destructive-bright" aria-hidden="true" />,
        };
    }
    if (node.exitCode !== 0) {
        return {
            label: t('settings.updates.node.failed', { code: node.exitCode }),
            nodeTone: 'destructive',
            icon: <XCircle className="h-4 w-4 text-destructive-bright" aria-hidden="true" />,
        };
    }
    return {
        label: t('settings.updates.node.completed'),
        nodeTone: 'success',
        icon: <CheckCircle2 className="h-4 w-4 text-success" aria-hidden="true" />,
    };
}

/** Current shows the panel version with the deployed commit between parens. */
function formatCurrent(current: string, panelVersion: string): string | null {
    if (!current) return null;
    if (panelVersion) return `${panelVersion} (${current.substring(0, 7)})`;
    return current.substring(0, 7);
}

export function UpdatesTab() {
    const { t } = useTranslations();
    const [check, setCheck] = useState<UpdateCheckResult | null>(null);
    const [checking, setChecking] = useState(true);
    const [updating, setUpdating] = useState(false);
    const [updateFailed, setUpdateFailed] = useState(false);
    const [reconnecting, setReconnecting] = useState(false);
    const [updateStatus, setUpdateStatus] = useState<UpdateStatusResult | null>(null);
    const [confirmOpen, setConfirmOpen] = useState(false);

    const checkUpdates = useCallback(async () => {
        setChecking(true);
        try {
            const data = await api.get<UpdateCheckResult>('/api/settings/update-check');
            setCheck(data);
        } catch (err) {
            console.error("Failed to check for updates:", err);
            setCheck({ current: '', latest: '', version: '', updateAvailable: false });
        } finally {
            setChecking(false);
        }
    }, []);

    useEffect(() => {
        void checkUpdates();
    }, [checkUpdates]);

    // Poll the update status of the whole fleet while an update is running.
    useEffect(() => {
        if (!updating) return;

        let cancelled = false;
        let timer: ReturnType<typeof setTimeout>;

        const poll = async () => {
            let data: UpdateStatusResult;
            try {
                data = await api.get<UpdateStatusResult>('/api/settings/update-status');
            } catch {
                // The panel may be restarting. Keep polling until it comes back.
                setReconnecting(true);
                if (!cancelled) timer = setTimeout(poll, 3000);
                return;
            }

            setReconnecting(false);
            if (!cancelled) setUpdateStatus(data);

            const nodes = data.nodes ?? [];
            const anyRunning = nodes.some((n) => n.running);

            if (!anyRunning) {
                if (!cancelled) setUpdating(false);
                const failed = nodes.some((n) => n.online && n.exitCode !== 0);
                const unreachable = nodes.some((n) => !n.online);

                if (failed || unreachable) {
                    setUpdateFailed(true);
                    sileo.error({
                        title: t('settings.updates.toast.issuesTitle'),
                        description: unreachable
                            ? t('settings.updates.toast.unreachable')
                            : t('settings.updates.toast.nodesFailed'),
                    });
                } else {
                    setUpdateFailed(false);
                    sileo.success({
                        title: t('settings.updates.toast.completeTitle'),
                        description: t('settings.updates.toast.complete'),
                    });
                    // Reload so the freshly built frontend is served.
                    setTimeout(() => window.location.reload(), 2500);
                }
                return;
            }

            if (!cancelled) timer = setTimeout(poll, 3000);
        };

        timer = setTimeout(poll, 1000);

        return () => {
            cancelled = true;
            clearTimeout(timer);
        };
    }, [updating, t]);

    const handleUpdate = async () => {
        setUpdating(true);
        setUpdateFailed(false);
        setUpdateStatus(null);
        try {
            const data = await api.post<{ nodes?: { name: string; error?: string }[] }>('/api/settings/update-panel', {});
            const failures = (data.nodes ?? []).filter((n) => n.error);
            if (failures.length > 0) {
                sileo.warning({
                    title: t('settings.updates.toast.incompleteTitle'),
                    description: t('settings.updates.toast.incomplete', { nodes: failures.map((n) => n.name).join(', ') }),
                });
            }
        } catch (err: any) {
            setUpdating(false);
            setUpdateFailed(true);
            sileo.error({
                title: t('settings.updates.toast.failedTitle'),
                description: err.message || t('settings.updates.toast.failed'),
            });
        }
    };

    const current = check?.current || '';
    const latest = check?.latest || '';
    const panelVersion = check?.version || '';
    const checkFailed = !latest;
    const unknownCurrent = latest && !current;
    const isUpToDate = Boolean(current && latest) && !check?.updateAvailable;
    const updateAvailable = Boolean(check?.updateAvailable);

    const nodes = updateStatus?.nodes ?? [];

    let status: StatusView = {
        statusTitle: t('settings.updates.status.checking'),
        StatusIcon: Loader2,
        spin: true,
        tone: 'muted',
        statusDescription: '',
    };
    if (updating) {
        status = updatingStatus(nodes, reconnecting, t);
    } else if (!checking) {
        status = idleStatus({ updateFailed, checkFailed, unknownCurrent: Boolean(unknownCurrent), updateAvailable }, t);
    }
    const { statusTitle, StatusIcon, spin, tone, statusDescription } = status;

    // Current shows the panel version with the deployed commit between parens,
    // while Latest shows the most recent commit available.
    const currentDisplay = formatCurrent(current, panelVersion);
    const latestDisplay = latest ? latest.substring(0, 7) : null;

    return (
        <SettingsCard>
            <SettingsCardHeader
                title={t('settings.updates.title')}
                description={t('settings.updates.description')}
                action={
                    <Button variant="outline" size="sm" onClick={checkUpdates} disabled={checking || updating}>
                        <RefreshCw className={cn(checking && 'animate-spin')} aria-hidden="true" />
                        {t('settings.updates.checkButton')}
                    </Button>
                }
            />
            <div className="space-y-5 p-5 md:p-6">
                <div className="flex flex-col gap-4 rounded-xl border border-border/60 bg-background/30 p-4 sm:p-5 md:flex-row md:items-center md:justify-between">
                    <div className="flex min-w-0 items-start gap-3">
                        <StatusIcon className={cn('mt-0.5 h-5 w-5 shrink-0', toneText[tone], spin && 'animate-spin')} aria-hidden="true" />
                        <div className="min-w-0 space-y-1" aria-live="polite">
                            <h3 className="text-base font-semibold">{statusTitle}</h3>
                            {statusDescription && <p className="text-pretty text-sm text-muted-foreground">{statusDescription}</p>}
                            <dl className="flex flex-col gap-x-6 gap-y-1 pt-1 text-sm sm:flex-row sm:flex-wrap">
                                <div className="flex items-baseline gap-2">
                                    <dt className="text-muted-foreground">{t('settings.updates.current')}</dt>
                                    <dd className={cn('tabular-nums', currentDisplay ? 'font-mono text-[13px] text-foreground' : 'text-muted-foreground')}>
                                        {currentDisplay ?? t('settings.updates.unknown')}
                                    </dd>
                                </div>
                                <div className="flex items-baseline gap-2">
                                    <dt className="text-muted-foreground">{t('settings.updates.latest')}</dt>
                                    <dd className={cn('tabular-nums', latestDisplay ? 'font-mono text-[13px] text-foreground' : 'text-muted-foreground')}>
                                        {latestDisplay ?? t('settings.updates.unavailable')}
                                    </dd>
                                </div>
                            </dl>
                        </div>
                    </div>

                    <Button
                        onClick={() => setConfirmOpen(true)}
                        disabled={checking || updating || isUpToDate}
                        className="w-full shrink-0 md:w-auto"
                    >
                        {updating ? <Loader2 className="animate-spin" aria-hidden="true" /> : <DownloadCloud aria-hidden="true" />}
                        {updating ? t('settings.updates.updating') : t('settings.updates.updateButton')}
                    </Button>
                </div>

                {(updating || updateFailed) && nodes.length > 0 && (
                    <div className="space-y-3">
                        <h4 className="text-sm font-semibold">{t('settings.updates.nodesTitle')}</h4>
                        <ul className="divide-y divide-border/50 rounded-xl border border-border/60 bg-background/30">
                            {nodes.map((node) => {
                                const { label, nodeTone, icon } = nodeRowState(node, t);

                                return (
                                    <li key={node.name} className="px-4 py-3">
                                        <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
                                            {icon}
                                            <span className="font-medium">{node.name}</span>
                                            <span className="font-mono text-xs text-muted-foreground">{node.address}</span>
                                            {node.local && (
                                                <span className="rounded-md bg-primary/10 px-2 py-0.5 text-xs text-primary-bright">
                                                    {t('settings.updates.node.thisPanel')}
                                                </span>
                                            )}
                                            <span className={cn('ml-auto inline-flex max-w-full items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium', toneBadge[nodeTone])}>
                                                <span className={cn('h-1.5 w-1.5 shrink-0 rounded-full', toneDot[nodeTone])} aria-hidden="true" />
                                                <span className="truncate">{label}</span>
                                            </span>
                                        </div>
                                        {node.log && (
                                            <details className="group mt-2">
                                                <summary className="w-fit cursor-pointer rounded text-xs text-muted-foreground transition-colors duration-150 hover:text-foreground focus-ring">
                                                    {t('settings.updates.node.viewLogs')}
                                                </summary>
                                                <pre className="mt-2 max-h-64 overflow-y-auto whitespace-pre-wrap break-words rounded-lg border border-border/60 bg-background/70 p-3 font-mono text-xs text-foreground/80">
                                                    {node.log}
                                                </pre>
                                            </details>
                                        )}
                                    </li>
                                );
                            })}
                        </ul>
                    </div>
                )}

                <p className="flex items-start gap-2 text-pretty text-sm text-muted-foreground">
                    <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
                    {t('settings.updates.note')}
                </p>
            </div>

            <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>{t('settings.updates.confirm.title')}</AlertDialogTitle>
                        <AlertDialogDescription>{t('settings.updates.confirm.description')}</AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                        <AlertDialogAction
                            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                            onClick={() => void handleUpdate()}
                        >
                            {t('settings.updates.confirm.action')}
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </SettingsCard>
    );
}
