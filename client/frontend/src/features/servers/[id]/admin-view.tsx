'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { Shield, Loader2 } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { useToast } from '@/hooks/use-toast';
import { useState, lazy, Suspense } from 'react';
import { useServerSettings } from '@/hooks/use-server-settings';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { useAuth } from '@/contexts/providers';

const CodeEditor = lazy(() => import('./code-editor'));

export default function AdminView({ serverId }: { serverId: string }) {
    const { t } = useTranslations();
    const { toast } = useToast();
    const { hasScope } = useAuth();
    const { settings, loading, saveSettings } = useServerSettings(serverId);
    const [isInstalling, setIsInstalling] = useState(false);
    const [isDeleting, setIsDeleting] = useState(false);
    const [isSaving, setIsSaving] = useState(false);
    const [editorOpen, setEditorOpen] = useState(false);
    const [editContent, setEditContent] = useState('');

    // Transfer states
    const [nodes, setNodes] = useState<any[]>([]);
    const [selectedNode, setSelectedNode] = useState<string>('');
    const [isTransferring, setIsTransferring] = useState(false);
    const [nodesLoaded, setNodesLoaded] = useState(false);

    // Fetch nodes for transfer
    const fetchNodes = async () => {
        if (nodesLoaded) return;
        try {
            const res = await api.get('/api/nodes');
            // if (res.data) {
            //     setNodes(res.data);
            // }
            if (Array.isArray(res)) {
                setNodes(res);
            }
            setNodesLoaded(true);
        } catch (e) {
            console.error('Failed to fetch nodes', e);
        }
    };

    const handleEditDefinition = () => {
        if (settings?.definition) {
            setEditContent(JSON.stringify(settings.definition, null, 4));
            setEditorOpen(true);
        }
    };

    const handleSaveDefinition = async () => {
        if (!settings) return;
        setIsSaving(true);
        try {
            const parsed = JSON.parse(editContent);
            await saveSettings({
                ...settings,
                definition: parsed
            });
            toast({ title: t('common.success'), description: t('servers.admin.editDefinition.success') || 'Definition saved.' });
            setEditorOpen(false);
        } catch (e: any) {
            toast({ title: t('common.error'), description: e.message || 'Invalid JSON or save failed.', variant: 'destructive' });
        } finally {
            setIsSaving(false);
        }
    };

    if (loading) {
        return (
            <div className="flex h-[400px] items-center justify-center">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
            </div>
        );
    }

    const handleInstall = async () => {
        setIsInstalling(true);
        try {
            await api.post(`/api/servers/${serverId}/install`, {});
            toast({ title: t('common.success'), description: t('servers.admin.installStatus.success') || 'Installation started.' });
        } catch (e: any) {
            toast({ title: t('common.error'), description: e.message || 'Installation failed.', variant: 'destructive' });
        } finally {
            setIsInstalling(false);
        }
    };

    const handleDelete = async () => {
        setIsDeleting(true);
        try {
            await api.delete(`/api/servers/${serverId}`);
            toast({ title: t('common.success'), description: t('servers.admin.delete.success') || 'Server deleted.' });
            window.location.href = '/servers';
        } catch (e: any) {
            toast({ title: t('common.error'), description: e.message || 'Delete failed.', variant: 'destructive' });
        } finally {
            setIsDeleting(false);
        }
    };

    const handleTransfer = async () => {
        if (!selectedNode) {
            toast({ title: t('common.error'), description: 'Please select a target node.', variant: 'destructive' });
            return;
        }
        setIsTransferring(true);
        try {
            await api.post(`/api/servers/${serverId}/transfer`, { nodeId: parseInt(selectedNode) });
            toast({ title: t('common.success'), description: 'Transfer started successfully.' });
        } catch (e: any) {
            toast({ title: t('common.error'), description: e.message || 'Transfer failed.', variant: 'destructive' });
        } finally {
            setIsTransferring(false);
        }
    };

    return (
        <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
            <Card className="border-0">
                <CardHeader>
                    <div className="flex items-center gap-2">
                        <Shield className="h-6 w-6" />
                        <CardTitle>{t('servers.admin.title')}</CardTitle>
                    </div>
                    <CardDescription>
                        {t('servers.admin.description')}
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    {(hasScope('server.admin') || hasScope('server.data.edit.admin') || hasScope('admin')) && (
                        <div className="flex items-center justify-between rounded-lg border p-4">
                            <div>
                                <h3 className="font-medium">Transfer Server</h3>
                                <p className="text-sm text-muted-foreground">
                                    Move this server to a different node.
                                </p>
                            </div>
                            <Dialog>
                                <DialogTrigger asChild>
                                    <Button variant="outline" onClick={fetchNodes}>Transfer</Button>
                                </DialogTrigger>
                                <DialogContent>
                                    <DialogHeader>
                                        <DialogTitle>Transfer Server</DialogTitle>
                                        <DialogDescription>
                                            Select the target node to move this server to. This operation will move all files.
                                        </DialogDescription>
                                    </DialogHeader>
                                    <div className="space-y-4 py-4">
                                        <select
                                            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                                            value={selectedNode}
                                            onChange={(e) => setSelectedNode(e.target.value)}
                                            disabled={nodes.length === 0}
                                        >
                                            <option value="" disabled>Select Target Node...</option>
                                            {nodes.map(node => (
                                                <option key={node.id} value={node.id}>
                                                    {node.name}
                                                </option>
                                            ))}
                                        </select>
                                    </div>
                                    <DialogFooter>
                                        <Button onClick={handleTransfer} disabled={!selectedNode || isTransferring || nodes.length === 0}>
                                            {isTransferring && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                            Start Transfer
                                        </Button>
                                    </DialogFooter>
                                </DialogContent>
                            </Dialog>
                        </div>
                    )}

                    <div className="flex items-center justify-between rounded-lg border p-4">
                        <div>
                            <h3 className="font-medium">{t('servers.admin.editDefinition.title')}</h3>
                            <p className="text-sm text-muted-foreground">
                                {t('servers.admin.editDefinition.description')}
                            </p>
                        </div>
                        <Button variant="outline" onClick={handleEditDefinition}>{t('servers.admin.editDefinition.button')}</Button>
                    </div>

                    <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
                        <DialogContent className="w-[90vw] max-w-[90vw] h-[80vh] flex flex-col">
                            <DialogHeader>
                                <DialogTitle>{t('servers.admin.editDefinition.title')}</DialogTitle>
                                <DialogDescription>
                                    {t('servers.admin.editDefinition.description')}
                                </DialogDescription>
                            </DialogHeader>
                            <div className="flex-grow min-h-0 border rounded-md">
                                <Suspense fallback={<div className="flex items-center justify-center h-full"><Loader2 className="h-8 w-8 animate-spin" /></div>}>
                                    <CodeEditor
                                        language="json"
                                        value={editContent}
                                        onChange={(val) => setEditContent(val || '')}
                                    />
                                </Suspense>
                            </div>
                            <DialogFooter className="mt-4 gap-2">
                                <Button variant="outline" onClick={() => setEditorOpen(false)}>
                                    {t('common.cancel')}
                                </Button>
                                <Button onClick={handleSaveDefinition} disabled={isSaving}>
                                    {isSaving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                    {t('common.save')}
                                </Button>
                            </DialogFooter>
                        </DialogContent>
                    </Dialog>

                    <div className="flex items-center justify-between rounded-lg border p-4">
                        <div>
                            <h3 className="font-medium">{t('servers.admin.installStatus.title')}</h3>
                            <p className="text-sm text-muted-foreground">
                                {t('servers.admin.installStatus.description')}
                            </p>
                        </div>
                        <Button onClick={handleInstall} disabled={isInstalling}>
                            {isInstalling && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                            {t('servers.admin.installStatus.button')}
                        </Button>
                    </div>

                    <div className="flex items-center justify-between rounded-lg border border-destructive/50 bg-destructive/5 p-4">
                        <div>
                            <h3 className="font-medium text-destructive">{t('servers.admin.delete.title')}</h3>
                            <p className="text-sm text-muted-foreground">
                                {t('servers.admin.delete.description')}
                            </p>
                        </div>
                        <AlertDialog open={isDeleting ? true : undefined}>
                            <AlertDialogTrigger asChild>
                                <Button variant="destructive" disabled={isDeleting}>
                                    {isDeleting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                    {t('servers.admin.delete.button')}
                                </Button>
                            </AlertDialogTrigger>
                            <AlertDialogContent>
                                <AlertDialogHeader>
                                    <AlertDialogTitle>{t('servers.admin.deleteDialog.title')}</AlertDialogTitle>
                                    <AlertDialogDescription>
                                        {t('servers.admin.deleteDialog.description')}
                                    </AlertDialogDescription>
                                </AlertDialogHeader>
                                <AlertDialogFooter>
                                    <AlertDialogCancel disabled={isDeleting}>{t('servers.admin.deleteDialog.cancel')}</AlertDialogCancel>
                                    <Button
                                        variant="destructive"
                                        disabled={isDeleting}
                                        onClick={(e) => {
                                            e.preventDefault();
                                            handleDelete();
                                        }}
                                    >
                                        {isDeleting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                        {t('servers.admin.deleteDialog.confirm')}
                                    </Button>
                                </AlertDialogFooter>
                            </AlertDialogContent>
                        </AlertDialog>
                    </div>
                </CardContent>
            </Card>
        </div>
    );
}
