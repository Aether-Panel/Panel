'use client';
import { Card } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Loader2, Settings, ArrowRightLeft, Trash2, HardDrive, Flag, TriangleAlert, Download, FileCode, Ban, Copy } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from "@/lib/toast";
import { useState, lazy, Suspense, type ReactNode } from 'react';
import { cn } from '@/lib/utils';
import { useExperimental } from '@/contexts/config-context';
import { BetaBadge } from '@/components/beta-badge';
import { useServerSettings } from '@/hooks/use-server-settings';
import { useAuth } from '@/contexts/providers';

const CodeEditor = lazy(() => import('./code-editor'));
const CloneServerCard = lazy(() => import('./clone-server-card'));

export default function AdminView({ serverId }: Readonly<{ serverId: string }>) {
    const { t } = useTranslations();
    
    const { hasScope } = useAuth();
    const { settings, loading, saveDefinition } = useServerSettings(serverId);
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
    const { features: experimental } = useExperimental();

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
            await saveDefinition(parsed);
            sileo.success({ title: t('common.success'), description: t('servers.admin.editDefinition.success') });
            setEditorOpen(false);
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e.message || t('servers.admin.editDefinition.error') });
        } finally {
            setIsSaving(false);
        }
    };

    const handleInstall = async () => {
        setIsInstalling(true);
        try {
            await api.post(`/api/servers/${serverId}/install`, {});
            sileo.success({ title: t('common.success'), description: t('servers.admin.installStatus.success') });
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e.message || t('servers.admin.installStatus.error') });
        } finally {
            setIsInstalling(false);
        }
    };

    const handleDelete = async () => {
        setIsDeleting(true);
        try {
            await api.delete(`/api/servers/${serverId}`);
            sileo.success({ title: t('common.success'), description: t('servers.admin.delete.success') });
            window.location.href = '/servers';
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e.message || t('servers.admin.delete.error') });
        } finally {
            setIsDeleting(false);
        }
    };

    const handleSuspend = async () => {
        setIsSuspending(true);
        try {
            await api.post(`/api/servers/${serverId}/suspend`, {});
            sileo.success({ title: t('common.success'), description: t('servers.admin.suspend.success') });
            window.location.reload();
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e.message || t('servers.admin.suspend.error') });
        } finally {
            setIsSuspending(false);
        }
    };

    const handleTransfer = async () => {
        if (!selectedNode) {
            sileo.error({ title: t('common.error'), description: t('servers.admin.transfer.selectRequired') });
            return;
        }
        setIsTransferring(true);
        const startedAt = Date.now();
        try {
            await api.post(`/api/servers/${serverId}/transfer`, { nodeId: Number.parseInt(selectedNode) });
            sileo.success({ title: t('common.success'), description: t('servers.admin.transfer.success') });
            setSelectedNode('');
            setTransferOpen(false);
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e.message || t('servers.admin.transfer.error') });
        } finally {
            const remaining = Math.max(0, 1200 - (Date.now() - startedAt));
            setTimeout(() => setIsTransferring(false), remaining);
        }
    };

    const canViewInstall = hasScope('server.admin.install.view') || hasScope('server.install') || hasScope('server.admin') || hasScope('admin');
    // the buttons follow the scopes their routes check, so none shows only to answer 403
    const canManageInstall = hasScope('server.install') || hasScope('admin');
    const canViewTransfer = hasScope('server.admin.transfer.view') || hasScope('server.data.edit.admin') || hasScope('server.admin') || hasScope('admin');
    const canManageTransfer = hasScope('server.data.edit.admin') || hasScope('admin');
    const canViewConfig = hasScope('server.admin.config.view') || hasScope('server.definition.edit') || hasScope('server.admin') || hasScope('admin');
    const canManageConfig = hasScope('server.definition.edit') || hasScope('admin');
    const canViewAssignments = hasScope('server.admin.assignments.view') || hasScope('server.flags.view') || hasScope('server.admin') || hasScope('admin');
    const canManageAssignments = hasScope('server.flags.edit') || hasScope('admin');
    const canSuspend = hasScope('server.data.edit.admin') || hasScope('admin');
    const canDelete = hasScope('server.delete') || hasScope('admin');
    const canViewDelete = canSuspend || canDelete;
    // cloning creates a server, so it needs the same permission as creating one
    const canClone = experimental.serverClone && (hasScope('server.create') || hasScope('admin'));

    if (loading) {
        return (
            <div className="flex h-[400px] items-center justify-center">
                <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-label={t('common.loading')} />
            </div>
        );
    }

    const adminTabs = [
        ...(canViewInstall ? [{ value: 'install', label: t('servers.admin.tabs.install'), icon: HardDrive }] : []),
        ...(canViewTransfer ? [{ value: 'transfer', label: t('servers.admin.tabs.transfer'), icon: ArrowRightLeft }] : []),
        ...(canClone ? [{ value: 'clone', label: t('servers.clone.tab'), icon: Copy, beta: true }] : []),
        ...(canViewConfig ? [{ value: 'config', label: t('servers.admin.tabs.config'), icon: Settings }] : []),
        ...(canViewAssignments ? [{ value: 'assignments', label: t('servers.admin.tabs.assignments'), icon: Flag }] : []),
        ...(canViewDelete ? [{ value: 'delete', label: t('servers.admin.tabs.danger'), icon: TriangleAlert }] : []),
    ];
    // a section that is not offered (no permission, or an experimental feature switched off) falls back to the first one
    const currentTab = adminTabs.some(tab => tab.value === adminTab) ? adminTab : adminTabs[0]?.value ?? adminTab;

    return (
        <Card className="mt-6">
            <div className="space-y-1 border-b border-border/50 p-5 md:p-6">
                <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('servers.admin.title')}</h2>
                <p className="text-sm text-muted-foreground">{t('servers.admin.description')}</p>
            </div>

            <Tabs value={currentTab} onValueChange={setAdminTab}>
                <div className="border-b border-border/50 px-5 py-3 md:px-6">
                    {/* Below lg (sidebar + card padding leave too little room) the sections
                        collapse into a select instead of a clipped or wrapping strip. */}
                    <div className="lg:hidden">
                        <Select value={currentTab} onValueChange={setAdminTab}>
                            <SelectTrigger aria-label={t('servers.admin.sectionLabel')}>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {adminTabs.map(tab => (
                                    <SelectItem key={tab.value} value={tab.value}>
                                        <div className="flex items-center gap-2">
                                            <tab.icon className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                                            <span>{tab.label}</span>
                                            {tab.beta && <BetaBadge />}
                                        </div>
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                    <TabsList className="hidden h-auto w-max max-w-full flex-wrap justify-start lg:inline-flex">
                        {adminTabs.map(tab => (
                            <TabsTrigger
                                key={tab.value}
                                value={tab.value}
                                className={cn(
                                    'h-8',
                                    tab.value === 'delete' && 'data-[state=active]:text-destructive-bright',
                                )}
                            >
                                {tab.label}
                                {tab.beta && <BetaBadge className="ml-2" />}
                            </TabsTrigger>
                        ))}
                    </TabsList>
                </div>

                <div className="p-5 md:p-6">
                    <TabsContent value="install" className="mt-0">
                        <ActionRow
                            title={t('servers.admin.installStatus.title')}
                            description={t('servers.admin.installStatus.description')}
                        >
                            {canManageInstall && (
                                <AlertDialog>
                                    <AlertDialogTrigger asChild>
                                        <Button disabled={isInstalling}>
                                            {isInstalling ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Download aria-hidden="true" />}
                                            {t('servers.admin.installStatus.button')}
                                        </Button>
                                    </AlertDialogTrigger>
                                    <AlertDialogContent>
                                        <AlertDialogHeader>
                                            <AlertDialogTitle>{t('servers.admin.installDialog.title')}</AlertDialogTitle>
                                            <AlertDialogDescription>
                                                {t('servers.admin.installDialog.description')}
                                            </AlertDialogDescription>
                                        </AlertDialogHeader>
                                        <AlertDialogFooter>
                                            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                                            <AlertDialogAction
                                                className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                                                onClick={() => void handleInstall()}
                                            >
                                                {t('servers.admin.installDialog.confirm')}
                                            </AlertDialogAction>
                                        </AlertDialogFooter>
                                    </AlertDialogContent>
                                </AlertDialog>
                            )}
                        </ActionRow>
                    </TabsContent>

                    <TabsContent value="transfer" className="mt-0">
                        <ActionRow
                            title={t('servers.admin.transfer.title')}
                            description={t('servers.admin.transfer.description')}
                        >
                            {canManageTransfer && (
                                <Dialog open={transferOpen} onOpenChange={setTransferOpen}>
                                    <DialogTrigger asChild>
                                        <Button variant="outline" onClick={() => {
                                            setSelectedNode('');
                                            void fetchNodes();
                                            setTransferOpen(true);
                                        }}>
                                            <ArrowRightLeft aria-hidden="true" />
                                            {t('servers.admin.transfer.button')}
                                        </Button>
                                    </DialogTrigger>
                                    <DialogContent>
                                        <DialogHeader>
                                            <DialogTitle>{t('servers.admin.transfer.title')}</DialogTitle>
                                            <DialogDescription>
                                                {t('servers.admin.transfer.dialogDescription')}
                                            </DialogDescription>
                                        </DialogHeader>
                                        <div className="space-y-2 py-4">
                                            <Label htmlFor="admin-transfer-node">{t('servers.admin.transfer.nodeLabel')}</Label>
                                            <select
                                                id="admin-transfer-node"
                                                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                                                value={selectedNode}
                                                onChange={(e) => setSelectedNode(e.target.value)}
                                                disabled={nodes.length === 0}
                                            >
                                                <option value="" disabled>{t('servers.admin.transfer.nodePlaceholder')}</option>
                                                {nodes.map(node => (
                                                    <option key={node.id} value={node.id}>
                                                        {node.name}
                                                    </option>
                                                ))}
                                            </select>
                                        </div>
                                        <DialogFooter>
                                            <Button variant="outline" onClick={() => setTransferOpen(false)}>
                                                {t('common.cancel')}
                                            </Button>
                                            <Button onClick={handleTransfer} disabled={!selectedNode || isTransferring || nodes.length === 0}>
                                                {isTransferring && <Loader2 className="animate-spin" aria-hidden="true" />}
                                                {t('servers.admin.transfer.start')}
                                            </Button>
                                        </DialogFooter>
                                    </DialogContent>
                                </Dialog>
                            )}
                        </ActionRow>
                    </TabsContent>

                    {canClone && (
                        <TabsContent value="clone" className="mt-0">
                            <Suspense fallback={<div className="flex h-40 items-center justify-center"><Loader2 className="h-5 w-5 animate-spin text-muted-foreground" aria-label={t('common.loading')} /></div>}>
                                <CloneServerCard serverId={serverId} />
                            </Suspense>
                        </TabsContent>
                    )}

                    <TabsContent value="config" className="mt-0">
                        <ActionRow
                            title={t('servers.admin.editDefinition.title')}
                            description={t('servers.admin.editDefinition.description')}
                        >
                            {canManageConfig && (
                                <Button variant="outline" onClick={handleEditDefinition}>
                                    <FileCode aria-hidden="true" />
                                    {t('servers.admin.editDefinition.button')}
                                </Button>
                            )}
                        </ActionRow>

                        <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
                            <DialogContent className="flex h-[85dvh] w-[calc(100vw-1rem)] max-w-[calc(100vw-1rem)] flex-col gap-0 overflow-hidden p-0 sm:w-[90vw] sm:max-w-[90vw]">
                                <DialogHeader className="border-b border-border/50 px-5 py-4 pr-12 text-left">
                                    <DialogTitle className="text-base">{t('servers.admin.editDefinition.title')}</DialogTitle>
                                    <DialogDescription>
                                        {t('servers.admin.editDefinition.description')}
                                    </DialogDescription>
                                </DialogHeader>
                                <div className="min-h-0 flex-grow [&_.monaco-editor-host]:rounded-none">
                                    <Suspense fallback={<div className="flex h-full items-center justify-center"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></div>}>
                                        <CodeEditor
                                            language="json"
                                            value={editContent}
                                            onChange={(val) => setEditContent(val || '')}
                                            onSave={handleSaveDefinition}
                                        />
                                    </Suspense>
                                </div>
                                <DialogFooter className="gap-2 border-t border-border/50 px-5 py-3">
                                    <Button variant="outline" onClick={() => setEditorOpen(false)}>
                                        {t('common.cancel')}
                                    </Button>
                                    <Button onClick={handleSaveDefinition} disabled={isSaving}>
                                        {isSaving && <Loader2 className="animate-spin" aria-hidden="true" />}
                                        {t('common.save')}
                                    </Button>
                                </DialogFooter>
                            </DialogContent>
                        </Dialog>
                    </TabsContent>

                    <TabsContent value="assignments" className="mt-0">
                        <ActionRow
                            title={t('servers.admin.assignments.title')}
                            description={t('servers.admin.assignments.description')}
                        >
                            <Button variant="outline" disabled>
                                {canManageAssignments ? t('servers.admin.assignments.manage') : t('servers.admin.assignments.view')}
                            </Button>
                        </ActionRow>
                    </TabsContent>

                    <TabsContent value="delete" className="mt-0">
                        <div className="divide-y divide-destructive/20 overflow-hidden rounded-xl border border-destructive/30 bg-destructive/5">
                            {canSuspend && (
                            <ActionRow
                                bare
                                title={t('servers.admin.suspend.title')}
                                description={t('servers.admin.suspend.description')}
                            >
                                <AlertDialog open={isSuspending ? true : undefined}>
                                    <AlertDialogTrigger asChild>
                                        <Button
                                            variant="outline"
                                            disabled={isSuspending}
                                            className="border-destructive/40 text-destructive-bright hover:border-destructive/60 hover:bg-destructive/10 hover:text-destructive-bright"
                                        >
                                            {isSuspending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Ban aria-hidden="true" />}
                                            {t('servers.admin.suspend.button')}
                                        </Button>
                                    </AlertDialogTrigger>
                                    <AlertDialogContent>
                                        <AlertDialogHeader>
                                            <AlertDialogTitle>{t('servers.admin.suspend.dialogTitle')}</AlertDialogTitle>
                                            <AlertDialogDescription>
                                                {t('servers.admin.suspend.dialogDescription')}
                                            </AlertDialogDescription>
                                        </AlertDialogHeader>
                                        <AlertDialogFooter>
                                            <AlertDialogCancel disabled={isSuspending}>{t('common.cancel')}</AlertDialogCancel>
                                            <Button
                                                variant="destructive"
                                                disabled={isSuspending}
                                                onClick={(e) => {
                                                    e.preventDefault();
                                                    void handleSuspend();
                                                }}
                                            >
                                                {isSuspending && <Loader2 className="animate-spin" aria-hidden="true" />}
                                                {t('servers.admin.suspend.confirm')}
                                            </Button>
                                        </AlertDialogFooter>
                                    </AlertDialogContent>
                                </AlertDialog>
                            </ActionRow>
                            )}

                            {canDelete && (
                            <ActionRow
                                bare
                                title={t('servers.admin.delete.title')}
                                description={t('servers.admin.delete.description')}
                            >
                                <AlertDialog open={isDeleting ? true : undefined}>
                                    <AlertDialogTrigger asChild>
                                        <Button variant="destructive" disabled={isDeleting}>
                                            {isDeleting ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Trash2 aria-hidden="true" />}
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
                                                    void handleDelete();
                                                }}
                                            >
                                                {isDeleting && <Loader2 className="animate-spin" aria-hidden="true" />}
                                                {t('servers.admin.deleteDialog.confirm')}
                                            </Button>
                                        </AlertDialogFooter>
                                    </AlertDialogContent>
                                </AlertDialog>
                            </ActionRow>
                            )}
                        </div>
                    </TabsContent>
                </div>
            </Tabs>
        </Card>
    );
}

/** Title + description on the left, the action on the right (stacked on phones). */
function ActionRow({
    title,
    description,
    bare,
    children,
}: Readonly<{
    title: string;
    description: string;
    /** Skip the item frame when the row already sits inside a framed list. */
    bare?: boolean;
    children?: ReactNode;
}>) {
    return (
        <div
            className={cn(
                'flex flex-col gap-4 p-4 sm:flex-row sm:items-center sm:justify-between sm:p-5',
                !bare && 'rounded-xl border border-border/60 bg-background/30',
            )}
        >
            <div className="min-w-0 space-y-1">
                <h3 className="text-sm font-semibold">{title}</h3>
                <p className="text-pretty text-sm text-muted-foreground">{description}</p>
            </div>
            {children && <div className="shrink-0 self-start sm:self-auto">{children}</div>}
        </div>
    );
}
