import { useDatabaseHosts } from '@/hooks/use-database-hosts';
import type { DatabaseHost } from '@/hooks/use-database-hosts';
import { useAuth } from '@/contexts/providers';
import { useNodes } from '@/hooks/use-nodes';
import { useEffect, useState, Fragment } from 'react';
import { PageHeader } from '@/components/page-header';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Check, Copy, Info, PlusCircle, Trash2, Edit2, Loader2 } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';
import { useTranslations } from '@/contexts/translations-context';

export default function DatabaseHostsPage() {
    const { role, hasScope } = useAuth();
    const [isMounted, setIsMounted] = useState(false);
    const { hosts, loading, createHost, updateHost, deleteHost, refresh } = useDatabaseHosts();
    const { nodes } = useNodes();
    const [isAddDialogOpen, setIsAddDialogOpen] = useState(false);
    const [currentStep, setCurrentStep] = useState(1);
    const [isEditing, setIsEditing] = useState(false);
    const [editingId, setEditingId] = useState<number | null>(null);
    const { toast } = useToast();
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
                toast({ title: t('common.success'), description: "Database host updated successfully" });
            } else {
                await createHost(formData);
                toast({ title: t('common.success'), description: "Database host created successfully" });
            }
            handleDialogChange(false);
        } catch (e: any) {
            toast({ title: t('common.error'), description: e.message, variant: 'destructive' });
        }
    };

    const handleDeleteHost = async (id: number) => {
        if (!confirm(t('common.confirmDelete') || "Are you sure you want to delete this database host?")) return;
        try {
            await deleteHost(id);
            toast({ title: t('common.success'), description: "Database host deleted successfully" });
        } catch (e: any) {
            toast({ title: t('common.error'), description: e.message, variant: 'destructive' });
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

    const CopyableCode = ({ command }: { command: string }) => {
        const handleCopy = () => {
            navigator.clipboard.writeText(command);
            toast({ title: t('databaseHosts.addDialog.step2.copyToast') });
        };

        return (
            <div className="flex items-center gap-2 rounded-md bg-muted p-2 font-mono text-sm">
                <pre className="flex-grow overflow-x-auto"><code>{command}</code></pre>
                <Button variant="ghost" size="icon" className="h-8 w-8" onClick={handleCopy}>
                    <Copy className="h-4 w-4" />
                    <span className="sr-only">{t('common.copy')}</span>
                </Button>
            </div>
        );
    };

    const Stepper = ({ currentStep, steps }: { currentStep: number, steps: string[] }) => (
        <div className="flex items-start justify-between px-4 py-2">
            {steps.map((step, index) => (
                <Fragment key={step}>
                    <div className="flex flex-col items-center gap-2 w-24 text-center">
                        <div
                            className={cn(
                                "flex h-8 w-8 items-center justify-center rounded-full text-sm font-bold transition-all duration-300",
                                currentStep > index + 1 ? "bg-primary text-primary-foreground" :
                                    currentStep === index + 1 ? "border-2 border-primary text-primary scale-110" : "bg-muted text-muted-foreground"
                            )}
                        >
                            {currentStep > index + 1 ? <Check className="h-5 w-5" /> : index + 1}
                        </div>
                        <p className={cn(
                            "text-xs transition-colors duration-300",
                            currentStep === index + 1 ? "text-foreground font-semibold" : "text-muted-foreground"
                        )}>{step}</p>
                    </div>
                    {index < steps.length - 1 && (
                        <div className="relative flex-1 mt-4">
                            <div className="h-0.5 bg-border" />
                            <div
                                className={cn(
                                    "absolute top-0 left-0 h-0.5 bg-primary transition-all duration-500 ease-in-out",
                                    currentStep > index + 1 ? "w-full" : "w-0"
                                )}
                            />
                        </div>
                    )}
                </Fragment>
            ))}
        </div>
    );

    if (!isMounted) {
        return (
            <div className="flex h-[80vh] items-center justify-center">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
            </div>
        );
    }

    if (!hasScope('admin')) {
        return (
            <div className="flex h-full items-center justify-center">
                <p>Not authorized</p>
            </div>
        );
    }

    return (
        <div className="flex flex-col gap-8">
            <PageHeader title={t('databaseHosts.title')} description={t('databaseHosts.description')}>
                <Dialog open={isAddDialogOpen} onOpenChange={handleDialogChange}>
                    <DialogTrigger asChild>
                        <Button>
                            <PlusCircle className="mr-2" />
                            {t('databaseHosts.addHost')}
                        </Button>
                    </DialogTrigger>
                    <DialogContent className="sm:max-w-3xl">
                        <DialogHeader>
                            <DialogTitle>{isEditing ? "Edit Database Host" : t('databaseHosts.addDialog.title')}</DialogTitle>
                            <DialogDescription>
                                {t('databaseHosts.addDialog.description')}
                            </DialogDescription>
                        </DialogHeader>

                        {!isEditing && <Stepper currentStep={currentStep} steps={stepTitles} />}

                        <div key={currentStep} className="py-4 max-h-[60vh] overflow-y-auto px-1 animate-in fade-in-50 duration-500">
                            {currentStep === 1 && (
                                <div className="space-y-6 px-4">
                                    <Alert>
                                        <Info className="h-4 w-4" />
                                        <AlertTitle>{t('databaseHosts.addDialog.step1.alertTitle')}</AlertTitle>
                                        <AlertDescription>
                                            {t('databaseHosts.addDialog.step1.alertDescription1')}
                                            <br />
                                            {t('databaseHosts.addDialog.step1.alertDescription2')}
                                        </AlertDescription>
                                    </Alert>

                                    <div className="space-y-2">
                                        <Label htmlFor="host-user">{t('databaseHosts.addDialog.step1.userLabel')}</Label>
                                        <Input
                                            id="host-user"
                                            value={formData.username}
                                            onChange={(e) => setFormData(prev => ({ ...prev, username: e.target.value }))}
                                        />
                                        <p className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step1.userDescription')}</p>
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="host-password">{t('databaseHosts.addDialog.step1.passwordLabel')}</Label>
                                        <Input
                                            id="host-password"
                                            type="password"
                                            value={formData.password}
                                            onChange={(e) => setFormData(prev => ({ ...prev, password: e.target.value }))}
                                        />
                                        <p className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step1.passwordDescription')}</p>
                                    </div>
                                </div>
                            )}
                            {currentStep === 2 && (
                                <div className="space-y-6 px-4">
                                    <Card>
                                        <CardHeader>
                                            <CardTitle>{t('databaseHosts.addDialog.step2.instructionsTitle')}</CardTitle>
                                        </CardHeader>
                                        <CardContent className="space-y-4">
                                            <div>
                                                <h4 className="font-semibold">{t('databaseHosts.addDialog.step2.userTitle')}</h4>
                                                <p className="text-sm text-muted-foreground" dangerouslySetInnerHTML={{ __html: t('databaseHosts.addDialog.step2.userDescription') }} />
                                            </div>
                                            <div>
                                                <h4 className="font-semibold">{t('databaseHosts.addDialog.step2.createUserCommandTitle')}</h4>
                                                <CopyableCode command={"CREATE USER 'aetheruser'@'127.0.0.1' IDENTIFIED BY 'password';"} />
                                            </div>
                                            <div>
                                                <h4 className="font-semibold">{t('databaseHosts.addDialog.step2.grantPermissionsCommandTitle')}</h4>
                                                <CopyableCode command={"GRANT ALL PRIVILEGES ON *.* TO 'aetheruser'@'127.0.0.1' WITH GRANT OPTION;"} />
                                            </div>
                                            <p className="text-sm text-muted-foreground" dangerouslySetInnerHTML={{ __html: t('databaseHosts.addDialog.step2.exitMysql') }} />
                                        </CardContent>
                                    </Card>

                                    <Card>
                                        <CardHeader>
                                            <CardTitle>{t('databaseHosts.addDialog.step2.externalAccessTitle')}</CardTitle>
                                        </CardHeader>
                                        <CardContent className="space-y-4">
                                            <p className="text-sm text-muted-foreground" dangerouslySetInnerHTML={{ __html: t('databaseHosts.addDialog.step2.externalAccessDescription1') }} />
                                            <p className="text-sm text-muted-foreground" dangerouslySetInnerHTML={{ __html: t('databaseHosts.addDialog.step2.externalAccessDescription2') }} />
                                            <CopyableCode command={"[mysqld]\nbind-address=0.0.0.0"} />
                                            <p className="text-sm text-muted-foreground">
                                                {t('databaseHosts.addDialog.step2.externalAccessDescription3')}
                                            </p>
                                        </CardContent>
                                    </Card>
                                </div>
                            )}
                            {currentStep === 3 && (
                                <div className="grid grid-cols-2 gap-6 px-4">
                                    {isEditing && (
                                        <div className="col-span-2">
                                            <Alert className="mb-4">
                                                <Info className="h-4 w-4" />
                                                <AlertTitle>Edit Mode</AlertTitle>
                                                <AlertDescription>
                                                    Leave user and password empty if you don't want to update them.
                                                </AlertDescription>
                                            </Alert>
                                            <div className="grid grid-cols-2 gap-4">
                                                <div className="space-y-2">
                                                    <Label htmlFor="edit-username">Update Username</Label>
                                                    <Input id="edit-username" value={formData.username} onChange={e => setFormData(p => ({ ...p, username: e.target.value }))} />
                                                </div>
                                                <div className="space-y-2">
                                                    <Label htmlFor="edit-password">Update Password</Label>
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
                                        />
                                        <p className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step3.hostNameDescription')}</p>
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="host-host">{t('databaseHosts.addDialog.step3.connectionHostLabel')}</Label>
                                        <Input
                                            id="host-host"
                                            value={formData.host}
                                            onChange={(e) => setFormData(prev => ({ ...prev, host: e.target.value }))}
                                        />
                                        <p className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step3.hostNameDescription')}</p>
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="host-port">{t('databaseHosts.addDialog.step3.portLabel')}</Label>
                                        <Input
                                            id="host-port"
                                            type="number"
                                            value={formData.port}
                                            onChange={(e) => setFormData(prev => ({ ...prev, port: parseInt(e.target.value) || 0 }))}
                                        />
                                        <p className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step3.portDescription')}</p>
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="host-max-db">{t('databaseHosts.addDialog.step3.dbLimitLabel')}</Label>
                                        <Input
                                            id="host-max-db"
                                            type="number"
                                            value={formData.max_databases}
                                            onChange={(e) => setFormData(prev => ({ ...prev, max_databases: parseInt(e.target.value) || 0 }))}
                                        />
                                        <p className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step3.dbLimitDescription')}</p>
                                    </div>
                                    <div className="space-y-2">
                                        <Label>{t('databaseHosts.addDialog.step3.linkedNodesLabel')}</Label>
                                        <Select
                                            value={formData.node_id?.toString() || "none"}
                                            onValueChange={(v) => setFormData(p => ({ ...p, node_id: v === "none" ? null : parseInt(v) }))}
                                        >
                                            <SelectTrigger>
                                                <SelectValue placeholder={t('databaseHosts.addDialog.step3.linkedNodesPlaceholder')} />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="none">None</SelectItem>
                                                {nodes.map(node => (
                                                    <SelectItem key={node.id} value={node.id.toString()}>{node.name}</SelectItem>
                                                ))}
                                            </SelectContent>
                                        </Select>
                                        <p className="text-sm text-muted-foreground">{t('databaseHosts.addDialog.step3.linkedNodesDescription')}</p>
                                    </div>
                                </div>
                            )}
                        </div>
                        <DialogFooter>
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
                                    {isEditing && <Button variant="outline" onClick={() => handleDialogChange(false)}>{t('common.cancel') || "Cancel"}</Button>}
                                    <Button type="submit" onClick={handleSaveHost} disabled={!formData.name || !formData.host || loading}>
                                        {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                        {isEditing ? (t('common.save') || "Save") : t('databaseHosts.addDialog.buttons.create')}
                                    </Button>
                                </>
                            )}
                        </DialogFooter>
                    </DialogContent>
                </Dialog>
            </PageHeader>

            <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
                <Card className="border-0">
                    <CardHeader>
                        <CardTitle>{t('databaseHosts.availableHosts')}</CardTitle>
                    </CardHeader>
                    <CardContent>
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>{t('databaseHosts.table.name')}</TableHead>
                                    <TableHead>{t('databaseHosts.table.host')}</TableHead>
                                    <TableHead>{t('databaseHosts.table.port')}</TableHead>
                                    <TableHead>{t('databaseHosts.table.username')}</TableHead>
                                    <TableHead>{t('databaseHosts.table.linkedNode')}</TableHead>
                                    <TableHead className="text-right">{t('databaseHosts.table.actions')}</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {loading && hosts.length === 0 ? (
                                    <TableRow>
                                        <TableCell colSpan={6} className="text-center py-8">
                                            <Loader2 className="h-8 w-8 animate-spin mx-auto text-primary" />
                                        </TableCell>
                                    </TableRow>
                                ) : hosts.length === 0 ? (
                                    <TableRow>
                                        <TableCell colSpan={6} className="text-center py-8 text-muted-foreground">
                                            No database hosts found.
                                        </TableCell>
                                    </TableRow>
                                ) : hosts.map((host) => (
                                    <TableRow key={host.id}>
                                        <TableCell className="font-medium">{host.name}</TableCell>
                                        <TableCell>{host.host}</TableCell>
                                        <TableCell>{host.port}</TableCell>
                                        <TableCell>{host.username}</TableCell>
                                        <TableCell>
                                            {nodes.find(n => n.id === host.node_id)?.name || "—"}
                                        </TableCell>
                                        <TableCell className="text-right">
                                            <div className="flex justify-end gap-2">
                                                <Button variant="ghost" size="icon" onClick={() => handleEditHost(host)}>
                                                    <Edit2 className="h-4 w-4" />
                                                </Button>
                                                <Button variant="ghost" size="icon" onClick={() => handleDeleteHost(host.id)}>
                                                    <Trash2 className="h-4 w-4 text-destructive" />
                                                </Button>
                                            </div>
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </CardContent>
                </Card>
            </div>
        </div>
    );
}

