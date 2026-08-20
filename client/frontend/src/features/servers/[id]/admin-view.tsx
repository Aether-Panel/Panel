'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Shield, Loader2, Settings, ArrowRightLeft, Trash2, HardDrive, Flag } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from "@/lib/toast";
import { useState, lazy, Suspense } from 'react';
import { useServerSettings } from '@/hooks/use-server-settings';
import { useAuth } from '@/contexts/providers';

const CodeEditor = lazy(() => import('./code-editor'));

export default function AdminView({ serverId }: { serverId: string }) {
    const { t } = useTranslations();
    
    const { hasScope } = useAuth();
    const { settings, loading, saveSettings } = useServerSettings(serverId);
    const [isInstalling, setIsInstalling] = useState(false);
    const [isDeleting, setIsDeleting] = useState(false);
    const [isSuspending, setIsSuspending] = useState(false);
    const [isSaving, setIsSaving] = useState(false);
    const [editorOpen, setEditorOpen] = useState(false);
    const [editContent, setEditContent] = useState('');

    const [nodes, setNodes] = useState<any[]>([]);
    const [selectedNode, setSelectedNode] = useState<string>('');
    const [isTransferring, setIsTransferring] = useState(false);
    const [transferOpen, setTransferOpen] = useState(false);
    const [nodesLoaded, setNodesLoaded] = useState(false);

    const [adminTab, setAdminTab] = useState('install');

    const fetchNodes = async () => {
        if (nodesLoaded) return;
        try {
            const res = await api.get('/api/nodes');
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
            }, hasScope('server.data.edit.admin'));
            sileo.success({ title: t('common.success'), description: t('servers.admin.editDefinition.success') || 'Definition saved.' });
            setEditorOpen(false);
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e.message || 'Invalid JSON or save failed.' });
        } finally {
            setIsSaving(false);
        }
    };

    const handleInstall = async () => {
        setIsInstalling(true);
        try {
            await api.post(`/api/servers/${serverId}/install`, {});
            sileo.success({ title: t('common.success'), description: t('servers.admin.installStatus.success') || 'Installation started.' });
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e.message || 'Installation failed.' });
        } finally {
            setIsInstalling(false);
        }
    };

    const handleDelete = async () => {
        setIsDeleting(true);
        try {
            await api.delete(`/api/servers/${serverId}`);
            sileo.success({ title: t('common.success'), description: t('servers.admin.delete.success') || 'Server deleted.' });
            window.location.href = '/servers';
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e.message || 'Delete failed.' });
        } finally {
            setIsDeleting(false);
        }
    };

    const handleSuspend = async () => {
        setIsSuspending(true);
        try {
            await api.post(`/api/servers/${serverId}/suspend`, {});
            sileo.success({ title: t('common.success'), description: 'Server suspension state toggled.' });
            window.location.reload();
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e.message || 'Suspension failed.' });
        } finally {
            setIsSuspending(false);
        }
    };

    const handleTransfer = async () => {
        if (!selectedNode) {
            sileo.error({ title: t('common.error'), description: 'Please select a target node.' });
            return;
        }
        setIsTransferring(true);
        const startedAt = Date.now();
        try {
            await api.post(`/api/servers/${serverId}/transfer`, { nodeId: parseInt(selectedNode) });
            sileo.success({ title: t('common.success'), description: 'Transfer started successfully.' });
            setSelectedNode('');
            setTransferOpen(false);
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e.message || 'Transfer failed.' });
        } finally {
            const remaining = Math.max(0, 1200 - (Date.now() - startedAt));
            setTimeout(() => setIsTransferring(false), remaining);
        }
    };

    const canViewAdmin = hasScope('server.admin.view') || hasScope('server.admin') || hasScope('admin');
    const canViewInstall = hasScope('server.admin.install.view') || hasScope('server.install') || hasScope('server.admin') || hasScope('admin');
    const canManageInstall = hasScope('server.admin.install.manage') || hasScope('server.install') || hasScope('server.admin') || hasScope('admin');
    const canViewTransfer = hasScope('server.admin.transfer.view') || hasScope('server.data.edit.admin') || hasScope('server.admin') || hasScope('admin');
    const canManageTransfer = hasScope('server.admin.transfer.manage') || hasScope('server.data.edit.admin') || hasScope('server.admin') || hasScope('admin');
    const canViewConfig = hasScope('server.admin.config.view') || hasScope('server.definition.edit') || hasScope('server.admin') || hasScope('admin');
    const canManageConfig = hasScope('server.admin.config.manage') || hasScope('server.definition.edit') || hasScope('server.admin') || hasScope('admin');
    const canViewAssignments = hasScope('server.admin.assignments.view') || hasScope('server.flags.view') || hasScope('server.admin') || hasScope('admin');
    const canManageAssignments = hasScope('server.admin.assignments.manage') || hasScope('server.flags.edit') || hasScope('server.admin') || hasScope('admin');
    const canViewDelete = hasScope('server.delete') || hasScope('server.admin') || hasScope('admin');

    if (loading) {
        return (
            <div className="flex h-[400px] items-center justify-center">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
            </div>
        );
    }

    const adminTabs = [
        ...(canViewInstall ? [{ value: 'install', label: 'Installation', icon: HardDrive }] : []),
        ...(canViewTransfer ? [{ value: 'transfer', label: 'Transfer', icon: ArrowRightLeft }] : []),
        ...(canViewConfig ? [{ value: 'config', label: 'Configuration', icon: Settings }] : []),
        ...(canViewAssignments ? [{ value: 'assignments', label: 'Assignments', icon: Flag }] : []),
        ...(canViewDelete ? [{ value: 'delete', label: 'Danger Zone', icon: Trash2 }] : []),
    ];

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
                <CardContent>
                    <Tabs value={adminTab} onValueChange={setAdminTab}>
                        <TabsList className="mb-6">
                            {adminTabs.map(tab => (
                                <TabsTrigger key={tab.value} value={tab.value} className="flex items-center gap-2">
                                    <tab.icon className="h-4 w-4" />
                                    {tab.label}
                                </TabsTrigger>
                            ))}
                        </TabsList>

                        <TabsContent value="install" className="space-y-4">
                            <div className="flex items-center justify-between rounded-lg border p-4">
                                <div>
                                    <h3 className="font-medium">{t('servers.admin.installStatus.title')}</h3>
                                    <p className="text-sm text-muted-foreground">
                                        {t('servers.admin.installStatus.description')}
                                    </p>
                                </div>
                                {canManageInstall && (
                                    <Button onClick={handleInstall} disabled={isInstalling}>
                                        {isInstalling && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                        {t('servers.admin.installStatus.button')}
                                    </Button>
                                )}
                            </div>
                        </TabsContent>

                        <TabsContent value="transfer" className="space-y-4">
                            <div className="flex items-center justify-between rounded-lg border p-4">
                                <div>
                                    <h3 className="font-medium">Transfer Server</h3>
                                    <p className="text-sm text-muted-foreground">
                                        Move this server to a different node.
                                    </p>
                                </div>
                                {canManageTransfer && (
                                    <Dialog open={transferOpen} onOpenChange={setTransferOpen}>
                                        <DialogTrigger asChild>
                                            <Button variant="outline" onClick={() => {
                                                setSelectedNode('');
                                                fetchNodes();
                                                setTransferOpen(true);
                                            }}>Transfer</Button>
                                        </DialogTrigger>
                                        <DialogContent>
                                            <DialogHeader>
                                                <DialogTitle>Transfer Server</DialogTitle>
                                                <DialogDescription>
                                                    Select the target node to move this server to.
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
                                )}
                            </div>
                        </TabsContent>

                        <TabsContent value="config" className="space-y-4">
                            <div className="flex items-center justify-between rounded-lg border p-4">
                                <div>
                                    <h3 className="font-medium">{t('servers.admin.editDefinition.title')}</h3>
                                    <p className="text-sm text-muted-foreground">
                                        {t('servers.admin.editDefinition.description')}
                                    </p>
                                </div>
                                {canManageConfig && (
                                    <Button variant="outline" onClick={handleEditDefinition}>{t('servers.admin.editDefinition.button')}</Button>
                                )}
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
                                                onSave={handleSaveDefinition}
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
                        </TabsContent>

                        <TabsContent value="assignments" className="space-y-4">
                            <div className="flex items-center justify-between rounded-lg border p-4">
                                <div>
                                    <h3 className="font-medium">Server Flags</h3>
                                    <p className="text-sm text-muted-foreground">
                                        View and manage server flags and assignments.
                                    </p>
                                </div>
                                <Button variant="outline" disabled>
                                    {canManageAssignments ? 'Manage Flags' : 'View Flags'}
                                </Button>
                            </div>
                        </TabsContent>

                        <TabsContent value="delete" className="space-y-4">
                            <div className="flex items-center justify-between rounded-lg border border-destructive/50 bg-destructive/5 p-4">
                                <div>
                                    <h3 className="font-medium text-destructive">Suspend Server</h3>
                                    <p className="text-sm text-muted-foreground">
                                        Suspending a server will stop it and prevent users from starting or modifying it.
                                    </p>
                                </div>
                                <AlertDialog open={isSuspending ? true : undefined}>
                                    <AlertDialogTrigger asChild>
                                        <Button variant="destructive" disabled={isSuspending}>
                                            {isSuspending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                            Suspend / Unsuspend
                                        </Button>
                                    </AlertDialogTrigger>
                                    <AlertDialogContent>
                                        <AlertDialogHeader>
                                            <AlertDialogTitle>Toggle Suspension</AlertDialogTitle>
                                            <AlertDialogDescription>
                                                Are you sure you want to toggle the suspension state of this server? If suspended, the server will be forcibly stopped.
                                            </AlertDialogDescription>
                                        </AlertDialogHeader>
                                        <AlertDialogFooter>
                                            <AlertDialogCancel disabled={isSuspending}>Cancel</AlertDialogCancel>
                                            <Button
                                                variant="destructive"
                                                disabled={isSuspending}
                                                onClick={(e) => {
                                                    e.preventDefault();
                                                    handleSuspend();
                                                }}
                                            >
                                                {isSuspending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                                Confirm Toggle
                                            </Button>
                                        </AlertDialogFooter>
                                    </AlertDialogContent>
                                </AlertDialog>
                            </div>

                            <div className="flex items-center justify-between rounded-lg border border-destructive/50 bg-destructive/5 p-4 mt-4">
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
                        </TabsContent>
                    </Tabs>
                </CardContent>
            </Card>
        </div>
    );
}
