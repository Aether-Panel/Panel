import { useState, useEffect } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
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
import { Trash2, Plus, Pencil, Package } from '@/components/icons';
import { sileo } from "@/lib/toast";
import { api } from '@/lib/api-client';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useTranslations } from '@/contexts/translations-context';
import { SettingsCard, SettingsCardHeader } from './general-tab';
import { EmptyState } from '@/components/empty-state';

export function ProductsView() {
    const { t } = useTranslations();
    const [products, setProducts] = useState<any[]>([]);
    const [nodes, setNodes] = useState<any[]>([]);
    const [isDialogOpen, setIsDialogOpen] = useState(false);
    const [editingId, setEditingId] = useState<number | null>(null);
    const [pendingDelete, setPendingDelete] = useState<any>(null);

    const [form, setForm] = useState({
        product_id: '',
        display_name: '',
        template: '',
        cpu: 0,
        memory: 0,
        disk: 0,
        default_node: 0,
        port_range_min: 0,
        port_range_max: 0,
    });
    const [usePortRange, setUsePortRange] = useState(false);

    const fetchData = async () => {
        try {
            const [prodRes, nodesRes] = await Promise.all([
                api.get('/api/provision/products'),
                api.get('/api/nodes')
            ]);
            setProducts(prodRes || []);
            setNodes(nodesRes || []);
        } catch (e) {
            console.error(e);
        }
    };

    useEffect(() => {
        void fetchData();
    }, []);

    const handleSubmit = async () => {
        try {
            const url = editingId ? `/api/provision/products/${editingId}` : '/api/provision/products';

            const payload = {
                ...form,
                cpu: Number(form.cpu),
                memory: Number(form.memory),
                disk: Number(form.disk),
                default_node: Number(form.default_node),
                port_range_min: usePortRange ? Number(form.port_range_min) : 0,
                port_range_max: usePortRange ? Number(form.port_range_max) : 0,
            };

            if (editingId) {
                await api.put(url, payload);
            } else {
                await api.post(url, payload);
            }

            void fetchData();
            setIsDialogOpen(false);
            sileo.success({ title: t('common.success'), description: t('settings.products.toast.saved') });
        } catch (e) {
            console.error(e);
            sileo.error({ title: t('common.error'), description: t('settings.products.toast.saveFailed') });
        }
    };

    const handleDelete = async (id: number) => {
        try {
            await api.delete(`/api/provision/products/${id}`);
            void fetchData();
            sileo.success({ title: t('common.success'), description: t('settings.products.toast.deleted') });
        } catch (e) {
            console.error(e);
        }
    };

    const openCreate = () => {
        setForm({
            product_id: '',
            display_name: '',
            template: '',
            cpu: 0,
            memory: 0,
            disk: 0,
            default_node: nodes[0]?.id || 0,
            port_range_min: 0,
            port_range_max: 0,
        });
        setUsePortRange(false);
        setEditingId(null);
        setIsDialogOpen(true);
    };

    const openEdit = (product: any) => {
        setForm({
            product_id: product.product_id,
            display_name: product.display_name,
            template: product.template,
            cpu: product.cpu,
            memory: product.memory,
            disk: product.disk,
            default_node: product.default_node,
            port_range_min: product.port_range_min || 0,
            port_range_max: product.port_range_max || 0,
        });
        setUsePortRange(!!(product.port_range_min && product.port_range_max));
        setEditingId(product.id);
        setIsDialogOpen(true);
    };

    const addButton = (
        <Button onClick={openCreate}>
            <Plus aria-hidden="true" />
            {t('settings.products.add')}
        </Button>
    );

    const rowActions = (prod: any) => (
        <div className="flex shrink-0 justify-end gap-1">
            <Button
                variant="ghost"
                size="icon"
                onClick={() => openEdit(prod)}
                className="h-9 w-9 text-muted-foreground hover:text-foreground"
                aria-label={t('settings.products.editAria', { name: prod.display_name })}
            >
                <Pencil aria-hidden="true" />
            </Button>
            <Button
                variant="ghost"
                size="icon"
                onClick={() => setPendingDelete(prod)}
                className="h-9 w-9 text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright"
                aria-label={t('settings.products.deleteAria', { name: prod.display_name })}
            >
                <Trash2 aria-hidden="true" />
            </Button>
        </div>
    );

    const numberField = (id: string, label: string, unit: string, key: 'cpu' | 'memory' | 'disk') => (
        <div className="space-y-2">
            <Label htmlFor={id}>{label}</Label>
            <div className="relative">
                <Input
                    id={id}
                    type="number"
                    inputMode="numeric"
                    value={form[key]}
                    onChange={e => setForm({ ...form, [key]: Number(e.target.value) })}
                    className="pr-12 tabular-nums md:text-[13px]"
                />
                <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">{unit}</span>
            </div>
        </div>
    );

    return (
        <SettingsCard>
            <SettingsCardHeader
                title={t('settings.products.title')}
                description={t('settings.products.description')}
                action={products.length > 0 ? addButton : undefined}
            />
            <div className="p-5 md:p-6">
                {products.length === 0 ? (
                    <EmptyState
                        icon={Package}
                        title={t('settings.products.emptyTitle')}
                        description={t('settings.products.emptyDescription')}
                        action={addButton}
                    />
                ) : (
                    <>
                    {/* Phones: stacked rows. */}
                    <ul className="divide-y divide-border/50 rounded-xl border border-border/60 bg-background/30 sm:hidden">
                        {products.map(prod => (
                            <li key={prod.id} className="flex items-start justify-between gap-3 px-4 py-3">
                                <div className="min-w-0 space-y-1">
                                    <p className="font-medium">{prod.display_name}</p>
                                    <p className="truncate font-mono text-xs text-muted-foreground">{prod.product_id} · {prod.template}</p>
                                    <p className="text-xs tabular-nums text-muted-foreground">
                                        {t('settings.products.limitsSummary', { cpu: prod.cpu, memory: prod.memory, disk: prod.disk })}
                                    </p>
                                    {prod.port_range_min > 0 && prod.port_range_max > 0 && (
                                        <p className="font-mono text-xs tabular-nums text-muted-foreground">
                                            {t('settings.products.portsSummary', { min: prod.port_range_min, max: prod.port_range_max })}
                                        </p>
                                    )}
                                </div>
                                {rowActions(prod)}
                            </li>
                        ))}
                    </ul>
                    <div className="hidden overflow-hidden rounded-xl border border-border/60 bg-background/30 sm:block">
                        <Table>
                            <TableHeader className="bg-transparent">
                                <TableRow className="hover:bg-transparent">
                                    <TableHead className="font-medium text-muted-foreground">{t('settings.products.table.product')}</TableHead>
                                    <TableHead className="font-medium text-muted-foreground">{t('settings.products.table.template')}</TableHead>
                                    <TableHead className="font-medium text-muted-foreground">{t('settings.products.table.limits')}</TableHead>
                                    <TableHead className="w-24"><span className="sr-only">{t('common.actions')}</span></TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {products.map(prod => (
                                    <TableRow key={prod.id} className="hover:bg-accent/30">
                                        <TableCell>
                                            <div className="font-medium">{prod.display_name}</div>
                                            <div className="mt-0.5 font-mono text-xs text-muted-foreground">{prod.product_id}</div>
                                        </TableCell>
                                        <TableCell className="font-mono text-[13px] text-muted-foreground">{prod.template}</TableCell>
                                        <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
                                            <span className="tabular-nums">
                                                {t('settings.products.limitsSummary', { cpu: prod.cpu, memory: prod.memory, disk: prod.disk })}
                                            </span>
                                            {prod.port_range_min > 0 && prod.port_range_max > 0 && (
                                                <div className="mt-0.5 font-mono tabular-nums">
                                                    {t('settings.products.portsSummary', { min: prod.port_range_min, max: prod.port_range_max })}
                                                </div>
                                            )}
                                        </TableCell>
                                        <TableCell className="text-right">{rowActions(prod)}</TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </div>
                    </>
                )}
            </div>

            <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
                <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
                    <DialogHeader>
                        <DialogTitle>{editingId ? t('settings.products.editTitle') : t('settings.products.createTitle')}</DialogTitle>
                        <DialogDescription>{t('settings.products.dialogDescription')}</DialogDescription>
                    </DialogHeader>
                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                        <div className="space-y-2">
                            <Label htmlFor="product-id">{t('settings.products.form.productId')}</Label>
                            <Input id="product-id" spellCheck={false} value={form.product_id} onChange={e => setForm({...form, product_id: e.target.value})} placeholder="minecraft_2gb" className="font-mono md:text-[13px]" />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="product-name">{t('settings.products.form.displayName')}</Label>
                            <Input id="product-name" value={form.display_name} onChange={e => setForm({...form, display_name: e.target.value})} />
                        </div>
                        <div className="space-y-2 sm:col-span-2">
                            <Label htmlFor="product-template">{t('settings.products.form.template')}</Label>
                            <Input id="product-template" spellCheck={false} value={form.template} onChange={e => setForm({...form, template: e.target.value})} placeholder="minecraft" className="font-mono md:text-[13px]" />
                        </div>
                        {numberField('product-cpu', t('settings.products.form.cpu'), '%', 'cpu')}
                        {numberField('product-memory', t('settings.products.form.memory'), 'MB', 'memory')}
                        {numberField('product-disk', t('settings.products.form.disk'), 'MB', 'disk')}
                        <div className="space-y-2">
                            <Label htmlFor="product-node">{t('settings.products.form.defaultNode')}</Label>
                            <Select value={form.default_node.toString()} onValueChange={v => setForm({...form, default_node: Number(v)})}>
                                <SelectTrigger id="product-node">
                                    <SelectValue placeholder={t('settings.products.form.selectNode')} />
                                </SelectTrigger>
                                <SelectContent>
                                    {nodes.map(node => (
                                        <SelectItem key={node.id} value={node.id.toString()}>{node.name}</SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="rounded-xl border border-border/60 bg-background/30 sm:col-span-2">
                            <div className="flex items-center justify-between gap-4 px-4 py-3.5">
                                <div className="min-w-0">
                                    <Label htmlFor="product-port-range" className="text-sm font-medium">{t('settings.products.form.portRange')}</Label>
                                    <p className="mt-0.5 text-xs text-muted-foreground">{t('settings.products.form.portRangeDescription')}</p>
                                </div>
                                <Switch id="product-port-range" checked={usePortRange} onCheckedChange={setUsePortRange} />
                            </div>
                            {usePortRange && (
                                <div className="grid grid-cols-2 gap-4 border-t border-border/50 px-4 py-3.5">
                                    <div className="space-y-2">
                                        <Label htmlFor="product-port-min">{t('settings.products.form.minPort')}</Label>
                                        <Input
                                            id="product-port-min"
                                            type="number"
                                            inputMode="numeric"
                                            min={1024}
                                            max={65535}
                                            placeholder="25000"
                                            value={form.port_range_min || ''}
                                            onChange={e => setForm({...form, port_range_min: Number(e.target.value)})}
                                            className="font-mono tabular-nums md:text-[13px]"
                                        />
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="product-port-max">{t('settings.products.form.maxPort')}</Label>
                                        <Input
                                            id="product-port-max"
                                            type="number"
                                            inputMode="numeric"
                                            min={1024}
                                            max={65535}
                                            placeholder="25999"
                                            value={form.port_range_max || ''}
                                            onChange={e => setForm({...form, port_range_max: Number(e.target.value)})}
                                            className="font-mono tabular-nums md:text-[13px]"
                                        />
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setIsDialogOpen(false)}>{t('common.cancel')}</Button>
                        <Button onClick={handleSubmit}>{t('settings.products.save')}</Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            <AlertDialog open={!!pendingDelete} onOpenChange={(open) => !open && setPendingDelete(null)}>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>{t('settings.products.deleteTitle')}</AlertDialogTitle>
                        <AlertDialogDescription>
                            {t('settings.products.deleteDescription', { name: pendingDelete?.display_name ?? '' })}
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                        <AlertDialogAction
                            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                            onClick={() => pendingDelete && void handleDelete(pendingDelete.id)}
                        >
                            {t('common.delete')}
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </SettingsCard>
    );
}
