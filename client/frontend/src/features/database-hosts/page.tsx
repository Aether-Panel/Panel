import { useDatabaseHosts } from '@/hooks/use-database-hosts';
import type { DatabaseHost } from '@/hooks/use-database-hosts';
import { api } from '@/lib/api-client';
import { useAuth } from '@/contexts/providers';
import { useNodes } from '@/hooks/use-nodes';
import { useEffect, useState, Fragment } from 'react';
import { PageHeader } from '@/components/page-header';
import { Card } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { AlertTriangle, Check, Copy, Database, Info, Lock, MoreHorizontal, Plus, Trash2, Loader2 } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { sileo } from "@/lib/toast";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';
import { useTranslations } from '@/contexts/translations-context';
import { EmptyState } from '@/components/empty-state';

const richText = '[&_code]:rounded [&_code]:bg-secondary [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-xs [&_code]:text-foreground';

function CopyableCode({ command }: { command: string }) {
    const { t } = useTranslations();
    const handleCopy = () => {
        void navigator.clipboard.writeText(command);
        sileo.success({ title: t('databaseHosts.addDialog.step2.copyToast') });
    };

    return (
        <div className="relative rounded-lg border border-border/60 bg-background/50">
            <pre className="custom-scrollbar overflow-x-auto p-3 pr-12 font-mono text-xs leading-relaxed text-foreground"><code>{command}</code></pre>
            <Button
                variant="ghost"
                size="icon"
                className="absolute right-1 top-1 h-9 w-9 text-muted-foreground hover:text-foreground"
                onClick={handleCopy}
                aria-label={t('common.copy')}
            >
                <Copy aria-hidden="true" />
            </Button>
        </div>
    );
}

function Stepper({ currentStep, steps }: { currentStep: number; steps: string[] }) {
    return (
        <ol className="flex items-center gap-3">
            {steps.map((step, index) => {
                const done = currentStep > index + 1;
                const current = currentStep === index + 1;
                return (
                    <Fragment key={step}>
                        <li className="flex min-w-0 items-center gap-2" aria-current={current ? 'step' : undefined}>
                            <span
                                className={cn(
                                    'grid h-7 w-7 shrink-0 place-items-center rounded-full border text-xs font-semibold tabular-nums transition-colors duration-200',
                                    done && 'border-primary bg-primary text-primary-foreground',
                                    current && 'border-primary bg-primary/10 text-primary-bright',
                                    !done && !current && 'border-border/60 bg-secondary text-muted-foreground',
                                )}
                            >
                                {done ? <Check className="h-3.5 w-3.5" aria-hidden="true" /> : index + 1}
                            </span>
                            <span
                                className={cn(
                                    'truncate text-sm transition-colors duration-200',
                                    current ? 'font-medium text-foreground' : 'hidden text-muted-foreground sm:inline',
                                )}
                            >
                                {step}
                            </span>
                        </li>
                        {index < steps.length - 1 && (
                            <li aria-hidden="true" className="h-px min-w-4 flex-1 bg-border">
                                <div className={cn('h-px bg-primary transition-[width] duration-300 ease-panel', done ? 'w-full' : 'w-0')} />
                            </li>
                        )}
                    </Fragment>
                );
            })}
        </ol>
    );
}

function HostsSkeleton() {
    return (
        <div className="flex flex-col gap-8" aria-busy="true">
            <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                <div className="space-y-2">
                    <Skeleton className="h-9 w-72 max-w-full" />
                    <Skeleton className="h-4 w-80 max-w-full" />
                </div>
                <Skeleton className="h-10 w-36 rounded-lg" />
            </div>
            <Card className="overflow-hidden">
                <div className="space-y-2 border-b border-border/50 p-5 md:p-6">
                    <Skeleton className="h-5 w-40" />
                    <Skeleton className="h-4 w-48 max-w-full" />
                </div>
                <div className="space-y-2 p-5 md:p-6">
                    {Array.from({ length: 3 }, (_, i) => <Skeleton key={i} className="h-14 rounded-xl" />)}
                </div>
            </Card>
        </div>
    );
}

export default function DatabaseHostsPage() {
    const { role, hasScope } = useAuth();
    const [isMounted, setIsMounted] = useState(false);
    const { hosts, loading, error, createHost, updateHost, deleteHost, refresh } = useDatabaseHosts();
    const { nodes } = useNodes();
    const [isAddDialogOpen, setIsAddDialogOpen] = useState(false);
    const [currentStep, setCurrentStep] = useState(1);
    const [isEditing, setIsEditing] = useState(false);
    const [editingId, setEditingId] = useState<number | null>(null);
    const [isTesting, setIsTesting] = useState(false);
    const [deletingHost, setDeletingHost] = useState<DatabaseHost | null>(null);

    const { t } = useTranslations();

    const stepTitles = [
        t('databaseHosts.addDialog.stepper.credentials'),
        t('databaseHosts.addDialog.stepper.instructions'),
        t('databaseHosts.addDialog.stepper.config')
    ];

    // New host form state
    const [formData, setFormData] = useState({
        name: '',
        host: '0.0.0.0',
        port: 3306,
        username: 'root',
        password: '',
        max_databases: 0,
        node_id: null as number | null
    });

    useEffect(() => {
        setIsMounted(true);
        if (role && !hasScope('admin')) {
            window.location.href = '/dashboard';
        }
    }, [role, hasScope]);

    const handleSaveHost = async () => {
        if (!formData.name || !formData.host || !formData.username) return;

        try {
            if (isEditing && editingId !== null) {
                await updateHost(editingId, formData);
                sileo.success({ title: t('common.success'), description: t('databaseHosts.toast.updated') });
            } else {
                await createHost(formData);
                sileo.success({ title: t('common.success'), description: t('databaseHosts.toast.created') });
            }
            handleDialogChange(false);
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e.message });
        }
    };

    const handleTestConnection = async () => {
        if (!formData.host || !formData.username || !formData.password) {
            sileo.error({ title: t('common.error'), description: t('databaseHosts.toast.testMissingFields') });
            return;
        }
        setIsTesting(true);
        const startedAt = Date.now();
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 15000);
        try {
            await api.post('/api/databasehosts/test', {
                host: formData.host,
                port: formData.port,
                username: formData.username,
                password: formData.password
            }, controller.signal);
            clearTimeout(timeoutId);
            sileo.success({ title: t('databaseHosts.toast.testSuccessTitle'), description: t('databaseHosts.toast.testSuccess') });
        } catch (e: any) {
            clearTimeout(timeoutId);
            sileo.error({
                title: t('databaseHosts.toast.testFailedTitle'),
                description: e?.name === 'AbortError'
                    ? t('databaseHosts.toast.testTimeout')
                    : (e?.message || t('databaseHosts.toast.testFailed'))
            });
        } finally {
            clearTimeout(timeoutId);
            const remaining = Math.max(0, 1200 - (Date.now() - startedAt));
            setTimeout(() => setIsTesting(false), remaining);
        }
    };

    const handleDeleteHost = async (id: number) => {
        try {
            await deleteHost(id);
            sileo.success({ title: t('common.success'), description: t('databaseHosts.toast.deleted') });
        } catch (e: any) {
            sileo.error({ title: t('common.error'), description: e.message });
        }
    };

    const handleEditHost = (host: DatabaseHost) => {
        setFormData({
            name: host.name,
            host: host.host,
            port: host.port,
            username: host.username,
            password: '',
            max_databases: host.max_databases,
            node_id: host.node_id || null
        });
        setEditingId(host.id);
        setIsEditing(true);
        setCurrentStep(3); // Go straight to config in edit mode
        setIsAddDialogOpen(true);
    };

    const handleDialogChange = (open: boolean) => {
        setIsAddDialogOpen(open);
        if (!open) {
            setTimeout(() => {
                setCurrentStep(1);
                setIsEditing(false);
                setEditingId(null);
                setFormData({
                    name: '',
                    host: '0.0.0.0',
                    port: 3306,
                    username: 'root',
                    password: '',
                    max_databases: 0,
                    node_id: null
                });
            }, 300);
        }
    }

    if (!isMounted) {
        return <HostsSkeleton />;
    }

    if (!hasScope('admin')) {
        return (
            <Card className="flex flex-col items-center px-6 py-16 text-center">
                <Lock className="mb-3 h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
                <p className="text-sm text-muted-foreground">{t('databaseHosts.notAuthorized')}</p>
            </Card>
        );
    }

    const nodeName = (host: DatabaseHost) => nodes.find(n => n.id === host.node_id)?.name || '—';

    const rowActions = (host: DatabaseHost) => (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                <Button
                    variant="ghost"
                    size="icon"
                    className="h-9 w-9 shrink-0 text-muted-foreground hover:text-foreground"
                    aria-label={t('databaseHosts.actions.openMenu', { name: host.name })}
                >
                    <MoreHorizontal aria-hidden="true" />
                </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
                <DropdownMenuLabel>{t('common.actions')}</DropdownMenuLabel>
                <DropdownMenuItem onClick={() => setTimeout(() => handleEditHost(host))}>
                    {t('common.edit')}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                    className="text-destructive-bright focus:bg-destructive/10 focus:text-destructive-bright"
                    onClick={() => setTimeout(() => setDeletingHost(host), 50)}
                >
                    <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
                    {t('common.delete')}
                </DropdownMenuItem>
            </DropdownMenuContent>
        </DropdownMenu>
    );

    return (
        <div className="stagger flex flex-col gap-8">
            <PageHeader title={t('databaseHosts.title')} description={t('databaseHosts.description')}>
                <Dialog open={isAddDialogOpen} onOpenChange={handleDialogChange}>
                    <DialogTrigger asChild>
                        <Button>
                            <Plus aria-hidden="true" />
                            {t('databaseHosts.addHost')}
                        </Button>
                    </DialogTrigger>
                    <DialogContent className="flex max-h-[min(90svh,52rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-3xl">
                        <DialogHeader className="space-y-4 border-b border-border/60 p-6 pb-5 text-left">
                            <div className="space-y-1.5">
                                <DialogTitle className="text-xl font-semibold tracking-[-0.02em]">
                                    {isEditing ? t('databaseHosts.editDialog.title') : t('databaseHosts.addDialog.title')}
                                </DialogTitle>
                                <DialogDescription>
                                    {isEditing ? t('databaseHosts.editDialog.description') : t('databaseHosts.addDialog.description')}
                                </DialogDescription>
                            </div>
                            {!isEditing && <Stepper currentStep={currentStep} steps={stepTitles} />}
                        </DialogHeader>

                        <div className="custom-scrollbar min-h-0 flex-1 overflow-y-auto p-6">
                            {currentStep === 1 && (
                                <div className="space-y-6">
                                    <div className="flex gap-3 rounded-xl border border-border/60 bg-background/30 p-4">
                                        <Info className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                                        <div className="space-y-1 text-sm">
                                            <p className="font-medium">{t('databaseHosts.addDialog.step1.alertTitle')}</p>
                                            <p className="text-muted-foreground">{t('databaseHosts.addDialog.step1.alertDescription1')}</p>
                                            <p className="text-muted-foreground">{t('databaseHosts.addDialog.step1.alertDescription2')}</p>
                                        </div>
                                    </div>

                                    <div className="space-y-2">
                                        <Label htmlFor="host-user">{t('databaseHosts.addDialog.step1.userLabel')}</Label>
                                        <Input
                                            id="host-user"
                                            className="font-mono"
                                            value={formData.username}
                                            onChange={(e) => setFormData(prev => ({ ...prev, username: e.target.value }))}
                                            aria-describedby="host-user-help"
                                        />
                                        <p id="host-user-help" className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step1.userDescription')}</p>
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="host-password">{t('databaseHosts.addDialog.step1.passwordLabel')}</Label>
                                        <Input
                                            id="host-password"
                                            type="password"
                                            value={formData.password}
                                            onChange={(e) => setFormData(prev => ({ ...prev, password: e.target.value }))}
                                            aria-describedby="host-password-help"
                                        />
                                        <p id="host-password-help" className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step1.passwordDescription')}</p>
                                    </div>
                                </div>
                            )}
                            {currentStep === 2 && (
                                <div className="space-y-4">
                                    <section className="space-y-4 rounded-xl border border-border/60 bg-background/30 p-4 md:p-5">
                                        <h3 className="text-base font-semibold">{t('databaseHosts.addDialog.step2.instructionsTitle')}</h3>
                                        <div className="space-y-1">
                                            <h4 className="text-sm font-semibold">{t('databaseHosts.addDialog.step2.userTitle')}</h4>
                                            <p className={cn('text-sm text-muted-foreground', richText)} dangerouslySetInnerHTML={{ __html: t('databaseHosts.addDialog.step2.userDescription') }} />
                                        </div>
                                        <div className="space-y-2">
                                            <h4 className="text-sm font-semibold">{t('databaseHosts.addDialog.step2.createUserCommandTitle')}</h4>
                                            <CopyableCode command={"CREATE USER 'aetheruser'@'%' IDENTIFIED BY 'your_strong_password';"} />
                                        </div>
                                        <div className="space-y-2">
                                            <h4 className="text-sm font-semibold">{t('databaseHosts.addDialog.step2.grantPermissionsCommandTitle')}</h4>
                                            <CopyableCode command={"GRANT ALL PRIVILEGES ON *.* TO 'aetheruser'@'%' WITH GRANT OPTION;"} />
                                        </div>
                                        <p className={cn('text-sm text-muted-foreground', richText)} dangerouslySetInnerHTML={{ __html: t('databaseHosts.addDialog.step2.passwordNote') }} />
                                        <p className={cn('text-sm text-muted-foreground', richText)} dangerouslySetInnerHTML={{ __html: t('databaseHosts.addDialog.step2.exitMysql') }} />
                                    </section>

                                    <section className="space-y-4 rounded-xl border border-border/60 bg-background/30 p-4 md:p-5">
                                        <h3 className="text-base font-semibold">{t('databaseHosts.addDialog.step2.externalAccessTitle')}</h3>
                                        <p className={cn('text-sm text-muted-foreground', richText)} dangerouslySetInnerHTML={{ __html: t('databaseHosts.addDialog.step2.externalAccessDescription1') }} />
                                        <p className={cn('text-sm text-muted-foreground', richText)} dangerouslySetInnerHTML={{ __html: t('databaseHosts.addDialog.step2.externalAccessDescription2') }} />
                                        <CopyableCode command={"[mysqld]\nbind-address=0.0.0.0"} />
                                        <p className="text-sm text-muted-foreground">
                                            {t('databaseHosts.addDialog.step2.externalAccessDescription3')}
                                        </p>
                                    </section>
                                </div>
                            )}
                            {currentStep === 3 && (
                                <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
                                    {isEditing && (
                                        <div className="space-y-4 rounded-xl border border-border/60 bg-background/30 p-4 sm:col-span-2">
                                            <div className="flex gap-3">
                                                <Info className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                                                <div className="space-y-1 text-sm">
                                                    <p className="font-medium">{t('databaseHosts.editMode.title')}</p>
                                                    <p className="text-muted-foreground">{t('databaseHosts.editMode.description')}</p>
                                                </div>
                                            </div>
                                            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                                                <div className="space-y-2">
                                                    <Label htmlFor="edit-username">{t('databaseHosts.editMode.usernameLabel')}</Label>
                                                    <Input id="edit-username" className="font-mono" value={formData.username} onChange={e => setFormData(p => ({ ...p, username: e.target.value }))} />
                                                </div>
                                                <div className="space-y-2">
                                                    <Label htmlFor="edit-password">{t('databaseHosts.editMode.passwordLabel')}</Label>
                                                    <Input id="edit-password" type="password" value={formData.password} onChange={e => setFormData(p => ({ ...p, password: e.target.value }))} />
                                                </div>
                                            </div>
                                        </div>
                                    )}
                                    <div className="space-y-2">
                                        <Label htmlFor="host-name">{t('databaseHosts.addDialog.step3.hostNameLabel')}</Label>
                                        <Input
                                            id="host-name"
                                            value={formData.name}
                                            onChange={(e) => setFormData(prev => ({ ...prev, name: e.target.value }))}
                                            placeholder={t('databaseHosts.addDialog.step3.hostNamePlaceholder')}
                                            aria-describedby="host-name-help"
                                        />
                                        <p id="host-name-help" className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step3.hostNameDescription')}</p>
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="host-host">{t('databaseHosts.addDialog.step3.connectionHostLabel')}</Label>
                                        <Input
                                            id="host-host"
                                            className="font-mono"
                                            value={formData.host}
                                            onChange={(e) => setFormData(prev => ({ ...prev, host: e.target.value }))}
                                            aria-describedby="host-host-help"
                                        />
                                        <p id="host-host-help" className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step3.connectionHostDescription')}</p>
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="host-port">{t('databaseHosts.addDialog.step3.portLabel')}</Label>
                                        <Input
                                            id="host-port"
                                            type="number"
                                            className="font-mono"
                                            value={formData.port}
                                            onChange={(e) => setFormData(prev => ({ ...prev, port: Number.parseInt(e.target.value) || 0 }))}
                                            aria-describedby="host-port-help"
                                        />
                                        <p id="host-port-help" className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step3.portDescription')}</p>
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="host-max-db">{t('databaseHosts.addDialog.step3.dbLimitLabel')}</Label>
                                        <Input
                                            id="host-max-db"
                                            type="number"
                                            value={formData.max_databases}
                                            onChange={(e) => setFormData(prev => ({ ...prev, max_databases: Number.parseInt(e.target.value) || 0 }))}
                                            aria-describedby="host-max-db-help"
                                        />
                                        <p id="host-max-db-help" className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step3.dbLimitDescription')}</p>
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="host-node">{t('databaseHosts.addDialog.step3.linkedNodesLabel')}</Label>
                                        <Select
                                            value={formData.node_id?.toString() || "none"}
                                            onValueChange={(v) => setFormData(p => ({ ...p, node_id: v === "none" ? null : Number.parseInt(v) }))}
                                        >
                                            <SelectTrigger id="host-node" aria-describedby="host-node-help">
                                                <SelectValue placeholder={t('databaseHosts.addDialog.step3.linkedNodesPlaceholder')} />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="none">{t('databaseHosts.addDialog.step3.linkedNodesPlaceholder')}</SelectItem>
                                                {nodes.map(node => (
                                                    <SelectItem key={node.id} value={node.id.toString()}>{node.name}</SelectItem>
                                                ))}
                                            </SelectContent>
                                        </Select>
                                        <p id="host-node-help" className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step3.linkedNodesDescription')}</p>
                                    </div>
                                </div>
                            )}
                        </div>
                        <DialogFooter className="gap-2 border-t border-border/60 px-6 py-4 sm:gap-0">
                            {currentStep === 1 && !isEditing && (
                                <>
                                    <Button variant="outline" onClick={() => handleDialogChange(false)}>{t('databaseHosts.addDialog.buttons.cancel')}</Button>
                                    <Button onClick={() => setCurrentStep(2)} disabled={!formData.username}>{t('databaseHosts.addDialog.buttons.next')}</Button>
                                </>
                            )}
                            {currentStep === 2 && !isEditing && (
                                <>
                                    <Button variant="outline" onClick={() => setCurrentStep(1)}>{t('databaseHosts.addDialog.buttons.back')}</Button>
                                    <Button onClick={() => setCurrentStep(3)}>{t('databaseHosts.addDialog.buttons.next')}</Button>
                                </>
                            )}
                            {currentStep === 3 && (
                                <>
                                    {!isEditing && <Button variant="outline" onClick={() => setCurrentStep(2)}>{t('databaseHosts.addDialog.buttons.back')}</Button>}
                                    {isEditing && <Button variant="outline" onClick={() => handleDialogChange(false)}>{t('common.cancel')}</Button>}
                                    <Button variant="outline" onClick={handleTestConnection} disabled={isTesting || !formData.host || !formData.username || !formData.password}>
                                        {isTesting && <Loader2 className="animate-spin" aria-hidden="true" />}
                                        {isTesting ? t('databaseHosts.testing') : t('databaseHosts.testConnection')}
                                    </Button>
                                    <Button type="submit" onClick={handleSaveHost} disabled={!formData.name || !formData.host || loading}>
                                        {loading && <Loader2 className="animate-spin" aria-hidden="true" />}
                                        {isEditing ? t('common.save') : t('databaseHosts.addDialog.buttons.create')}
                                    </Button>
                                </>
                            )}
                        </DialogFooter>
                    </DialogContent>
                </Dialog>
            </PageHeader>

            <Card className="overflow-hidden">
                <div className="flex flex-col gap-1 border-b border-border/50 p-5 md:p-6">
                    <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('databaseHosts.availableHosts')}</h2>
                    <p className="text-sm text-muted-foreground">{t('databaseHosts.listDescription', { count: hosts.length })}</p>
                </div>

                <div className="p-5 md:p-6">
                    {loading && hosts.length === 0 ? (
                        <div className="space-y-2" aria-busy="true">
                            {Array.from({ length: 3 }, (_, i) => <Skeleton key={i} className="h-14 rounded-xl" />)}
                        </div>
                    ) : error && hosts.length === 0 ? (
                        <EmptyState
                            icon={AlertTriangle}
                            tone="error"
                            title={t('databaseHosts.error.title')}
                            description={t('databaseHosts.error.description')}
                            action={
                                <Button variant="outline" onClick={() => void refresh()}>
                                    {t('common.retry')}
                                </Button>
                            }
                        />
                    ) : hosts.length === 0 ? (
                        <EmptyState
                            icon={Database}
                            title={t('databaseHosts.empty.title')}
                            description={t('databaseHosts.empty.description')}
                            action={
                                <Button onClick={() => setIsAddDialogOpen(true)}>
                                    <Plus aria-hidden="true" />
                                    {t('databaseHosts.addHost')}
                                </Button>
                            }
                        />
                    ) : (
                        <>
                            {/* Phones and tablets (the sidebar leaves too little room for the table): stacked rows. */}
                            <ul className="divide-y divide-border/50 overflow-hidden rounded-xl border border-border/60 bg-background/30 lg:hidden">
                                {hosts.map((host) => (
                                    <li key={host.id} className="flex items-center gap-3 py-3 pl-4 pr-2">
                                        <div className="min-w-0 flex-1">
                                            <p className="truncate text-sm font-semibold">{host.name}</p>
                                            <p className="mt-0.5 break-all font-mono text-xs text-muted-foreground">{host.username}@{host.host}:{host.port}</p>
                                            <p className="mt-0.5 truncate text-xs text-muted-foreground">
                                                {t('databaseHosts.table.linkedNode')}: <span className="text-foreground">{nodeName(host)}</span>
                                            </p>
                                        </div>
                                        {rowActions(host)}
                                    </li>
                                ))}
                            </ul>

                            {/* Desktop: the full table. */}
                            <div className="hidden overflow-hidden rounded-xl border border-border/60 bg-background/30 lg:block">
                                <Table>
                                    <TableHeader>
                                        <TableRow className="hover:bg-transparent">
                                            <TableHead>{t('databaseHosts.table.name')}</TableHead>
                                            <TableHead>{t('databaseHosts.table.host')}</TableHead>
                                            <TableHead className="hidden xl:table-cell">{t('databaseHosts.table.port')}</TableHead>
                                            <TableHead className="hidden xl:table-cell">{t('databaseHosts.table.username')}</TableHead>
                                            <TableHead>{t('databaseHosts.table.linkedNode')}</TableHead>
                                            <TableHead className="w-14"><span className="sr-only">{t('databaseHosts.table.actions')}</span></TableHead>
                                        </TableRow>
                                    </TableHeader>
                                    <TableBody>
                                        {hosts.map((host) => (
                                            <TableRow key={host.id} className="hover:bg-accent/30">
                                                <TableCell className="max-w-[14rem] truncate font-medium">{host.name}</TableCell>
                                                <TableCell className="max-w-[16rem] break-all font-mono text-xs text-muted-foreground">
                                                    {host.host}<span className="xl:hidden">:{host.port}</span>
                                                </TableCell>
                                                <TableCell className="hidden font-mono text-xs tabular-nums text-muted-foreground xl:table-cell">{host.port}</TableCell>
                                                <TableCell className="hidden font-mono text-xs text-muted-foreground xl:table-cell">{host.username}</TableCell>
                                                <TableCell className="max-w-[12rem] truncate">{nodeName(host)}</TableCell>
                                                <TableCell className="py-2 text-right">{rowActions(host)}</TableCell>
                                            </TableRow>
                                        ))}
                                    </TableBody>
                                </Table>
                            </div>
                        </>
                    )}
                </div>
            </Card>

            <AlertDialog open={!!deletingHost} onOpenChange={(open) => !open && setDeletingHost(null)}>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>{t('databaseHosts.deleteDialog.title')}</AlertDialogTitle>
                        <AlertDialogDescription>
                            {t('databaseHosts.deleteDialog.description', { name: deletingHost?.name || '' })}
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                        <AlertDialogAction
                            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                            onClick={() => { if (deletingHost) void handleDeleteHost(deletingHost.id); }}
                        >
                            {t('common.delete')}
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </div>
    );
}
