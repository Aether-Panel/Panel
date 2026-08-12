import { useState, useEffect, useCallback, type ReactNode } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { DownloadCloud, Loader2, GitCommit, LifeBuoy, CheckCircle2, XCircle, Info, RefreshCw } from 'lucide-react';
import { sileo } from '@/lib/toast';
import { api } from '@/lib/api-client';

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

export function UpdatesTab() {
    const [check, setCheck] = useState<UpdateCheckResult | null>(null);
    const [checking, setChecking] = useState(true);
    const [updating, setUpdating] = useState(false);
    const [updateFailed, setUpdateFailed] = useState(false);
    const [reconnecting, setReconnecting] = useState(false);
    const [updateStatus, setUpdateStatus] = useState<UpdateStatusResult | null>(null);

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
        checkUpdates();
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
                        title: 'Update Finished With Issues',
                        description: unreachable
                            ? 'Some nodes could not be reached. Check the node list below.'
                            : 'One or more nodes failed to update. Check the node list below.',
                    });
                } else {
                    setUpdateFailed(false);
                    sileo.success({
                        title: 'Update Complete',
                        description: 'The panel and all nodes have been updated successfully.',
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
    }, [updating]);

    const handleUpdate = async () => {
        setUpdating(true);
        setUpdateFailed(false);
        setUpdateStatus(null);
        try {
            const data = await api.post<{ nodes?: { name: string; error?: string }[] }>('/api/settings/update-panel', {});
            const failures = (data.nodes ?? []).filter((n) => n.error);
            if (failures.length > 0) {
                sileo.warning({
                    title: 'Update Incomplete',
                    description: `Could not trigger the update on: ${failures.map((n) => n.name).join(', ')}.`,
                });
            }
        } catch (err: any) {
            setUpdating(false);
            setUpdateFailed(true);
            sileo.error({
                title: 'Update Failed',
                description: err.message || 'There was a problem initiating the update.',
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

    // Current shows the panel version with the deployed commit between parens,
    // while Latest shows the most recent commit available.
    const currentDisplay = current
        ? panelVersion
            ? `${panelVersion} (${current.substring(0, 7)})`
            : current.substring(0, 7)
        : 'Unknown';
    const latestDisplay = latest ? latest.substring(0, 7) : 'Unavailable';

    const nodes = updateStatus?.nodes ?? [];
    const anyRunning = nodes.some((n) => n.running);
    const anyFailed = nodes.some((n) => n.online && n.exitCode !== 0);
    const anyUnreachable = nodes.some((n) => !n.online);

    let statusTitle = 'Checking for updates...';
    let statusIcon = <Loader2 className="h-6 w-6 animate-spin" />;
    let statusColor = 'text-muted-foreground';
    let statusDescription = '';

    if (updating) {
        if (reconnecting) {
            statusTitle = 'Panel is restarting...';
            statusIcon = <Loader2 className="h-6 w-6 animate-spin" />;
            statusColor = 'text-blue-500';
            statusDescription = 'Waiting for the panel to come back online. This can take a minute while containers rebuild.';
        } else if (anyRunning) {
            statusTitle = `Updating ${nodes.filter((n) => n.running).length} of ${nodes.length} node(s)...`;
            statusIcon = <Loader2 className="h-6 w-6 animate-spin" />;
            statusColor = 'text-blue-500';
            statusDescription = 'Pulling the latest code and rebuilding images. This usually takes a few minutes.';
        } else if (anyFailed) {
            statusTitle = 'Update Finished With Errors';
            statusIcon = <XCircle className="h-6 w-6 text-red-500" />;
            statusColor = 'text-red-500';
        } else if (anyUnreachable) {
            statusTitle = 'Some Nodes Unreachable';
            statusIcon = <Info className="h-6 w-6 text-amber-500" />;
            statusColor = 'text-amber-500';
        } else {
            statusTitle = 'Update Complete';
            statusIcon = <CheckCircle2 className="h-6 w-6 text-green-500" />;
            statusColor = 'text-green-500';
        }
    } else if (!checking) {
        if (updateFailed) {
            statusTitle = 'Update Failed';
            statusIcon = <XCircle className="h-6 w-6 text-red-500" />;
            statusColor = 'text-red-500';
        } else if (checkFailed) {
            statusTitle = 'Couldn\'t check for updates';
            statusIcon = <XCircle className="h-6 w-6 text-red-500" />;
            statusColor = 'text-red-500';
            statusDescription = 'The panel could not reach GitHub to look for new versions.';
        } else if (unknownCurrent) {
            statusTitle = 'Unable to verify current version';
            statusIcon = <Info className="h-6 w-6 text-amber-500" />;
            statusColor = 'text-amber-500';
            statusDescription = 'The deployed commit could not be determined. You can still update manually.';
        } else if (updateAvailable) {
            statusTitle = 'Update Available';
            statusIcon = <LifeBuoy className="h-6 w-6 text-blue-500 animate-pulse" />;
            statusColor = 'text-blue-500';
            statusDescription = 'A new version of the panel is available.';
        } else {
            statusTitle = 'System is up to date';
            statusIcon = <CheckCircle2 className="h-6 w-6 text-green-500" />;
            statusColor = 'text-green-500';
        }
    }

    return (
        <div className="flex flex-col gap-6">
            <Card>
                <CardHeader>
                    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                        <div>
                            <CardTitle>System Updates</CardTitle>
                            <CardDescription>
                                Update the panel and every registered node to the latest version.
                            </CardDescription>
                        </div>
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={checkUpdates}
                            disabled={checking || updating}
                            className="self-start sm:self-auto"
                        >
                            <RefreshCw className={`h-4 w-4 mr-2 ${checking ? 'animate-spin' : ''}`} />
                            Check for Updates
                        </Button>
                    </div>
                </CardHeader>
                <CardContent className="space-y-6">
                    <div className="flex flex-col md:flex-row md:items-center justify-between p-4 border rounded-lg bg-card">
                        <div className="flex items-center gap-4 mb-4 md:mb-0">
                            <div className={`p-3 rounded-full bg-primary/10 ${statusColor}`}>
                                {statusIcon}
                            </div>
                            <div>
                                <h3 className="font-semibold text-lg">{statusTitle}</h3>
                                {statusDescription && (
                                    <p className="text-sm text-muted-foreground mt-1">{statusDescription}</p>
                                )}
                                <div className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4 text-sm text-muted-foreground mt-1">
                                    <div className="flex items-center gap-1">
                                        <GitCommit className="h-4 w-4" />
                                        <span>Current: {currentDisplay}</span>
                                    </div>
                                    <div className="hidden sm:block text-border">•</div>
                                    <div className="flex items-center gap-1">
                                        <DownloadCloud className="h-4 w-4" />
                                        <span>Latest: {latestDisplay}</span>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <Button
                            onClick={handleUpdate}
                            disabled={checking || updating || isUpToDate}
                            className="w-full md:w-auto"
                        >
                            {updating ? (
                                <>
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    Updating...
                                </>
                            ) : (
                                <>
                                    <DownloadCloud className="mr-2 h-4 w-4" />
                                    Update Panel
                                </>
                            )}
                        </Button>
                    </div>

                    {(updating || updateFailed) && nodes.length > 0 && (
                        <div className="space-y-3">
                            <h4 className="font-semibold text-sm text-muted-foreground">Nodes</h4>
                            {nodes.map((node) => {
                                let label: string;
                                let icon: ReactNode;
                                if (node.running) {
                                    label = 'Updating...';
                                    icon = <Loader2 className="h-4 w-4 animate-spin text-blue-500" />;
                                } else if (!node.online) {
                                    label = node.error || 'Unreachable';
                                    icon = <XCircle className="h-4 w-4 text-red-500" />;
                                } else if (node.exitCode !== 0) {
                                    label = `Failed (exit ${node.exitCode})`;
                                    icon = <XCircle className="h-4 w-4 text-red-500" />;
                                } else {
                                    label = 'Completed';
                                    icon = <CheckCircle2 className="h-4 w-4 text-green-500" />;
                                }

                                return (
                                    <div key={node.name} className="rounded-lg border bg-card p-3">
                                        <div className="flex items-center gap-3">
                                            {icon}
                                            <span className="font-medium">{node.name}</span>
                                            <span className="text-sm text-muted-foreground">{node.address}</span>
                                            {node.local && (
                                                <span className="text-xs rounded bg-primary/10 text-primary px-2 py-0.5">
                                                    This Panel
                                                </span>
                                            )}
                                            <span className={`ml-auto text-sm ${label === 'Completed' ? 'text-green-500' : 'text-muted-foreground'}`}>
                                                {label}
                                            </span>
                                        </div>
                                        {node.log && (
                                            <details className="mt-2">
                                                <summary className="text-xs text-muted-foreground cursor-pointer">
                                                    View logs
                                                </summary>
                                                <pre className="mt-2 text-xs font-mono whitespace-pre-wrap max-h-64 overflow-y-auto rounded bg-black text-green-400 p-3">
                                                    {node.log}
                                                </pre>
                                            </details>
                                        )}
                                    </div>
                                );
                            })}
                        </div>
                    )}

                    <div className="text-sm text-muted-foreground">
                        <p><strong>Note:</strong> The update pulls the latest code from the repository, rebuilds the panel image and restarts the services, including on every registered node. The panel may be unreachable for a few minutes during the process. Thank You!</p>
                    </div>
                </CardContent>
            </Card>
        </div>
    );
}