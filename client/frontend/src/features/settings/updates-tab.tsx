import { useState, useEffect, useCallback } from 'react';
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

interface UpdateStatusResult {
    running: boolean;
    containerId: string;
    startedAt: string;
    finishedAt: string;
    exitCode: number;
    log: string;
    error: string;
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

    // Poll the update status while an update is running.
    useEffect(() => {
        if (!updating) return;

        let cancelled = false;
        let timer: ReturnType<typeof setTimeout>;

        const poll = async () => {
            try {
                const data = await api.get<UpdateStatusResult>('/api/settings/update-status');
                setReconnecting(false);
                if (!cancelled) setUpdateStatus(data);

                if (!data.running) {
                    if (!cancelled) setUpdating(false);

                    if (data.exitCode === 0) {
                        setUpdateFailed(false);
                        sileo.success({
                            title: 'Update Complete',
                            description: 'The panel has been updated successfully.',
                        });
                        // Reload so the freshly built frontend is served.
                        setTimeout(() => window.location.reload(), 2500);
                    } else {
                        setUpdateFailed(true);
                        sileo.error({
                            title: 'Update Failed',
                            description: data.error || `The update exited with code ${data.exitCode}.`,
                        });
                    }
                    return;
                }
            } catch {
                // The panel may be restarting. Keep polling until it comes back.
                setReconnecting(true);
            }

            if (!cancelled) {
                timer = setTimeout(poll, 3000);
            }
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
            await api.post('/api/settings/update-panel', {});
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

    let statusTitle = 'Checking for updates...';
    let statusIcon = <Loader2 className="h-6 w-6 animate-spin" />;
    let statusColor = 'text-muted-foreground';
    let statusDescription = '';

    if (updating) {
        statusTitle = reconnecting ? 'Panel is restarting...' : 'Updating...';
        statusIcon = <Loader2 className="h-6 w-6 animate-spin" />;
        statusColor = 'text-blue-500';
        statusDescription = reconnecting
            ? 'Waiting for the panel to come back online. This can take a minute while migrations run.'
            : 'Rebuilding the panel image and restarting the system. This usually takes 1-3 minutes.';
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

    const showLog = updating || updateFailed;
    const logText = updateStatus?.log || '';

    return (
        <div className="flex flex-col gap-6">
            <Card>
                <CardHeader>
                    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                        <div>
                            <CardTitle>System Updates</CardTitle>
                            <CardDescription>
                                Keep your Aether Panel up to date with the latest features and security patches.
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

                    {showLog && logText && (
                        <div className="rounded-lg border bg-black text-green-400 p-3">
                            <pre className="text-xs font-mono whitespace-pre-wrap max-h-64 overflow-y-auto">
                                {logText}
                            </pre>
                        </div>
                    )}

                    <div className="text-sm text-muted-foreground">
                        <p><strong>Note:</strong> The update pulls the latest code from the repository, rebuilds the panel image and restarts the services. The panel may be unreachable for a few minutes during the process.</p>
                    </div>
                </CardContent>
            </Card>
        </div>
    );
}
