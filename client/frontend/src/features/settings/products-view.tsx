import { useState, useEffect } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Trash2, Plus, Edit } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import { api } from '@/lib/api-client';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

export function ProductsView() {
    const [products, setProducts] = useState<any[]>([]);
    const [nodes, setNodes] = useState<any[]>([]);
    const [isDialogOpen, setIsDialogOpen] = useState(false);
    const [editingId, setEditingId] = useState<number | null>(null);
    const { toast } = useToast();

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
        fetchData();
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

            fetchData();
            setIsDialogOpen(false);
            toast({ title: 'Success', description: 'Product saved successfully' });
        } catch (e) {
            console.error(e);
            toast({ title: 'Error', description: 'Failed to save product', variant: 'destructive' });
        }
    };

    const handleDelete = async (id: number) => {
        if (!confirm('Are you sure you want to delete this product?')) return;
        try {
            await api.delete(`/api/provision/products/${id}`);
            fetchData();
            toast({ title: 'Success', description: 'Product deleted' });
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

    return (
        <div className="space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                <div>
                    <h2 className="text-xl font-semibold tracking-tight">Provision Products</h2>
                    <p className="text-sm text-muted-foreground mt-1">Define products that can be automatically provisioned by external systems.</p>
                </div>
                <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
                    <DialogTrigger asChild>
                        <Button onClick={openCreate}><Plus className="w-4 h-4 mr-2"/> Add Product</Button>
                    </DialogTrigger>
                    <DialogContent className="max-w-xl">
                        <DialogHeader>
                            <DialogTitle>{editingId ? 'Edit Product' : 'Add New Product'}</DialogTitle>
                        </DialogHeader>
                        <div className="grid grid-cols-2 gap-4 py-4">
                            <div className="space-y-2 col-span-2 sm:col-span-1">
                                <Label>Product ID (e.g. minecraft_2gb)</Label>
                                <Input value={form.product_id} onChange={e => setForm({...form, product_id: e.target.value})} />
                            </div>
                            <div className="space-y-2 col-span-2 sm:col-span-1">
                                <Label>Display Name</Label>
                                <Input value={form.display_name} onChange={e => setForm({...form, display_name: e.target.value})} />
                            </div>
                            <div className="space-y-2 col-span-2">
                                <Label>Template Name</Label>
                                <Input value={form.template} onChange={e => setForm({...form, template: e.target.value})} placeholder="e.g. minecraft" />
                            </div>
                            <div className="space-y-2">
                                <Label>CPU Limit (%)</Label>
                                <Input type="number" value={form.cpu} onChange={e => setForm({...form, cpu: Number(e.target.value)})} />
                            </div>
                            <div className="space-y-2">
                                <Label>Memory Limit (MB)</Label>
                                <Input type="number" value={form.memory} onChange={e => setForm({...form, memory: Number(e.target.value)})} />
                            </div>
                            <div className="space-y-2">
                                <Label>Disk Limit (MB)</Label>
                                <Input type="number" value={form.disk} onChange={e => setForm({...form, disk: Number(e.target.value)})} />
                            </div>
                            <div className="space-y-2">
                                <Label>Default Node</Label>
                                <Select value={form.default_node.toString()} onValueChange={v => setForm({...form, default_node: Number(v)})}>
                                    <SelectTrigger>
                                        <SelectValue placeholder="Select a node" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {nodes.map(node => (
                                            <SelectItem key={node.id} value={node.id.toString()}>{node.name}</SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>
                            <div className="col-span-2 space-y-3 border border-border/60 rounded-xl p-4 bg-muted/20">
                                <div className="flex items-center justify-between">
                                    <div>
                                        <Label className="text-sm font-medium">Assign Port from Range</Label>
                                        <p className="text-xs text-muted-foreground mt-0.5">
                                            A random free port from this range will be assigned on provisioning.
                                        </p>
                                    </div>
                                    <Switch
                                        checked={usePortRange}
                                        onCheckedChange={setUsePortRange}
                                    />
                                </div>
                                {usePortRange && (
                                    <div className="grid grid-cols-2 gap-4 pt-2">
                                        <div className="space-y-1.5">
                                            <Label className="text-xs text-muted-foreground">Min Port</Label>
                                            <Input
                                                type="number"
                                                min={1024}
                                                max={65535}
                                                placeholder="e.g. 25000"
                                                value={form.port_range_min || ''}
                                                onChange={e => setForm({...form, port_range_min: Number(e.target.value)})}
                                                className="bg-background"
                                            />
                                        </div>
                                        <div className="space-y-1.5">
                                            <Label className="text-xs text-muted-foreground">Max Port</Label>
                                            <Input
                                                type="number"
                                                min={1024}
                                                max={65535}
                                                placeholder="e.g. 25999"
                                                value={form.port_range_max || ''}
                                                onChange={e => setForm({...form, port_range_max: Number(e.target.value)})}
                                                className="bg-background"
                                            />
                                        </div>
                                    </div>
                                )}
                            </div>
                        </div>
                        <DialogFooter>
                            <Button variant="outline" onClick={() => setIsDialogOpen(false)}>Cancel</Button>
                            <Button onClick={handleSubmit}>Save Product</Button>
                        </DialogFooter>
                    </DialogContent>
                </Dialog>
            </div>

            <div className="rounded-xl border border-border/60 bg-card shadow-sm overflow-hidden">
                <Table>
                    <TableHeader className="bg-muted/30">
                        <TableRow>
                            <TableHead className="font-medium">Product ID</TableHead>
                            <TableHead className="font-medium">Name</TableHead>
                            <TableHead className="font-medium">Template</TableHead>
                            <TableHead className="font-medium">Limits</TableHead>
                            <TableHead className="w-[100px] text-right"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {products.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={5} className="text-center py-8 text-muted-foreground">No products defined.</TableCell>
                            </TableRow>
                        ) : products.map(prod => (
                            <TableRow key={prod.id}>
                                <TableCell className="font-mono text-[13px]">{prod.product_id}</TableCell>
                                <TableCell className="font-medium">{prod.display_name}</TableCell>
                                <TableCell className="text-muted-foreground">{prod.template}</TableCell>
                                <TableCell className="text-sm text-muted-foreground">
                                    CPU: {prod.cpu}%, RAM: {prod.memory}MB, Disk: {prod.disk}MB
                                </TableCell>
                                <TableCell className="text-right">
                                    <div className="flex justify-end gap-1">
                                        <Button variant="ghost" size="icon" onClick={() => openEdit(prod)} className="text-primary hover:bg-primary/10">
                                            <Edit className="w-4 h-4" />
                                        </Button>
                                        <Button variant="ghost" size="icon" onClick={() => handleDelete(prod.id)} className="text-red-500 hover:text-red-600 hover:bg-red-500/10">
                                            <Trash2 className="w-4 h-4" />
                                        </Button>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </div>
        </div>
    );
}
