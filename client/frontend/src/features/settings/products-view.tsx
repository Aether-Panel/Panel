import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
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
    });

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
                default_node: Number(form.default_node)
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
        });
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
        });
        setEditingId(product.id);
        setIsDialogOpen(true);
    };

    return (
        <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50 animate-in slide-in-from-bottom-4 duration-500">
            <Card className="border-0 shadow-lg">
                <CardHeader className="flex flex-row items-center justify-between">
                    <div>
                        <CardTitle>Provision Products</CardTitle>
                        <CardDescription>Define products that can be automatically provisioned by external systems.</CardDescription>
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
                            </div>
                            <DialogFooter>
                                <Button variant="outline" onClick={() => setIsDialogOpen(false)}>Cancel</Button>
                                <Button onClick={handleSubmit}>Save Product</Button>
                            </DialogFooter>
                        </DialogContent>
                    </Dialog>
                </CardHeader>
                <CardContent>
                    <div className="rounded-md border">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>Product ID</TableHead>
                                    <TableHead>Name</TableHead>
                                    <TableHead>Template</TableHead>
                                    <TableHead>Limits</TableHead>
                                    <TableHead className="w-[100px] text-right">Actions</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {products.length === 0 ? (
                                    <TableRow>
                                        <TableCell colSpan={5} className="text-center py-6 text-muted-foreground">No products defined.</TableCell>
                                    </TableRow>
                                ) : products.map(prod => (
                                    <TableRow key={prod.id}>
                                        <TableCell className="font-mono text-sm">{prod.product_id}</TableCell>
                                        <TableCell>{prod.display_name}</TableCell>
                                        <TableCell>{prod.template}</TableCell>
                                        <TableCell className="text-sm text-muted-foreground">
                                            CPU: {prod.cpu}%, RAM: {prod.memory}MB, Disk: {prod.disk}MB
                                        </TableCell>
                                        <TableCell className="text-right">
                                            <Button variant="ghost" size="icon" onClick={() => openEdit(prod)}>
                                                <Edit className="w-4 h-4" />
                                            </Button>
                                            <Button variant="ghost" size="icon" onClick={() => handleDelete(prod.id)} className="text-red-500 hover:text-red-600 hover:bg-red-500/10">
                                                <Trash2 className="w-4 h-4" />
                                            </Button>
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </div>
                </CardContent>
            </Card>
        </div>
    );
}
