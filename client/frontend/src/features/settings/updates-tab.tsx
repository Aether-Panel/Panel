import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { DownloadCloud, Loader2, GitCommit, LifeBuoy, CheckCircle2, XCircle } from 'lucide-react';
import { useConfig } from '@/contexts/config-context';
import { sileo } from '@/lib/toast';
import { api } from '@/lib/api-client';

export function UpdatesTab() {
    const { config } = useConfig();
    const [latestCommit, setLatestCommit] = useState<string | null>(null);
    const [checking, setChecking] = useState(true);
    const [updating, setUpdating] = useState(false);
    const [updateFailed, setUpdateFailed] = useState(false);

    useEffect(() => {
        const checkUpdates = async () => {
            try {
                // Fetch the latest commit from the dev2.0 branch (or main depending on deployment)
                const res = await fetch('https://api.github.com/repos/Aether-Panel/Panel/commits/dev2.0');
                if (res.ok) {
                    const data = await res.json();
                    setLatestCommit(data.sha);
                }
            } catch (err) {
                console.error("Failed to check for updates:", err);
            } finally {
                setChecking(false);
            }
        };
        
        checkUpdates();
    }, [config?.version]);

    const handleUpdate = async () => {
        setUpdating(true);
        try {
            setUpdateFailed(false);
            await api.post('/api/settings/update-panel');
            sileo.success({
                title: 'Update Initiated',
                description: 'The update process has started. The panel will restart shortly.',
            });
        } catch (err: any) {
            sileo.error({
                title: 'Update Failed',
                description: err.message || 'There was a problem initiating the update.',
            });
            setUpdating(false);
            setUpdateFailed(true);
        }
    };

    // Calculate if we need an update
    // We assume an update is needed if the version doesn't contain the first 7 chars of the latest commit
    // Development builds usually have 'dev-docker' or 'devel' as version.
    const isDevBuild = config?.version === 'dev-docker' || config?.version === 'devel' || !config?.version;
    const updateAvailable = latestCommit && config?.version && !config.version.includes(latestCommit.substring(0, 7));
    
    // Display version formatting
    const currentVerDisplay = isDevBuild ? 'Development Build' : (config?.version || 'Unknown');
    const latestVerDisplay = latestCommit ? latestCommit.substring(0, 7) : 'Checking...';

    return (
        <div className="flex flex-col gap-6">
            <Card>
                <CardHeader>
                    <CardTitle>System Updates</CardTitle>
                    <CardDescription>
                        Keep your Aether Panel up to date with the latest features and security patches.
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-6">
                    <div className="flex flex-col md:flex-row md:items-center justify-between p-4 border rounded-lg bg-card">
                        <div className="flex items-center gap-4 mb-4 md:mb-0">
                            <div className="p-3 rounded-full bg-primary/10 text-primary">
                                {checking ? (
                                    <Loader2 className="h-6 w-6 animate-spin" />
                                ) : updateFailed ? (
                                    <XCircle className="h-6 w-6 text-red-500" />
                                ) : updateAvailable ? (
                                    <LifeBuoy className="h-6 w-6 text-blue-500 animate-pulse" />
                                ) : (
                                    <CheckCircle2 className="h-6 w-6 text-green-500" />
                                )}
                            </div>
                            <div>
                                <h3 className="font-semibold text-lg">
                                    {checking ? 'Checking for updates...' : updateFailed ? 'Update Failed' : updateAvailable ? 'Update Available' : 'System is up to date'}
                                </h3>
                                <div className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4 text-sm text-muted-foreground mt-1">
                                    <div className="flex items-center gap-1">
                                        <GitCommit className="h-4 w-4" />
                                        <span>Current: {currentVerDisplay}</span>
                                    </div>
                                    <div className="hidden sm:block text-border">•</div>
                                    <div className="flex items-center gap-1">
                                        <DownloadCloud className="h-4 w-4" />
                                        <span>Latest: {latestVerDisplay}</span>
                                    </div>
                                </div>
                            </div>
                        </div>
                        
                        <Button 
                            onClick={handleUpdate} 
                            disabled={checking || updating || !updateAvailable}
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

                    <div className="text-sm text-muted-foreground">
                        <p><strong>Note:</strong> Starting an update will automatically put the panel into maintenance mode, download the latest files, and restart the system. This process usually takes 1-3 minutes.</p>
                    </div>
                </CardContent>
            </Card>
        </div>
    );
}
