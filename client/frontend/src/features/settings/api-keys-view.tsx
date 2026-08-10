import { useState, useEffect } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Trash2, Key, Copy, Plus } from 'lucide-react';
import { sileo } from "@/lib/toast";
import { api } from '@/lib/api-client';

export function ApiKeysView() {
    const [keys, setKeys] = useState<any[]>([]);
    const [name, setName] = useState('');
    const [isCreating, setIsCreating] = useState(false);
    const [newKeyData, setNewKeyData] = useState<{token: string, key: any} | null>(null);
    

    const fetchKeys = async () => {
        try {
            const data = await api.get('/api/settings/apikeys');
            setKeys(data);
        } catch (e) {
            console.error(e);
        }
    };

    useEffect(() => {
        fetchKeys();
    }, []);

    const handleCreate = async () => {
        try {
            const data = await api.post('/api/settings/apikeys', { name, permissions: ['provision'] });
            setNewKeyData(data);
            setName('');
            setIsCreating(false);
            fetchKeys();
            sileo.success({ title: 'Success', description: 'API Key generated successfully' });
        } catch (e) {
            console.error(e);
            sileo.error({ title: 'Error', description: 'Failed to generate key' });
        }
    };

    const handleDelete = async (id: number) => {
        if (!confirm('Are you sure you want to delete this key?')) return;
        try {
            await api.delete(`/api/settings/apikeys/${id}`);
            fetchKeys();
            sileo.success({ title: 'Success', description: 'API Key deleted' });
        } catch (e) {
            console.error(e);
        }
    };

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text);
        sileo.success({ title: 'Copied', description: 'API Key copied to clipboard' });
    };

    return (
        <div className="space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                <div>
                    <h2 className="text-xl font-semibold tracking-tight">API Keys</h2>
                    <p className="text-sm text-muted-foreground mt-1">Manage API Keys for external integrations (WHMCS, Paymenter)</p>
                </div>
                <Dialog open={isCreating} onOpenChange={setIsCreating}>
                    <DialogTrigger asChild>
                        <Button><Plus className="w-4 h-4 mr-2"/> Generate Key</Button>
                    </DialogTrigger>
                    <DialogContent>
                        <DialogHeader>
                            <DialogTitle>Generate New API Key</DialogTitle>
                            <DialogDescription>This key will be used to authenticate external systems.</DialogDescription>
                        </DialogHeader>
                        <div className="space-y-4 py-4">
                            <div className="space-y-2">
                                <Label>Name / Description</Label>
                                <Input value={name} onChange={e => setName(e.target.value)} placeholder="e.g. WHMCS Server 1" />
                            </div>
                        </div>
                        <DialogFooter>
                            <Button variant="outline" onClick={() => setIsCreating(false)}>Cancel</Button>
                            <Button onClick={handleCreate} disabled={!name}>Generate</Button>
                        </DialogFooter>
                    </DialogContent>
                </Dialog>
            </div>

            {newKeyData && (
                <div className="p-4 rounded-xl bg-success/10 border border-success/20 text-success">
                    <h4 className="font-semibold mb-2 flex items-center">
                        <Key className="w-4 h-4 mr-2"/> Key Generated Successfully
                    </h4>
                    <p className="text-sm mb-4">Please copy this key now. You won't be able to see it again.</p>
                    <div className="flex gap-2">
                        <Input readOnly value={newKeyData.token} className="bg-background/80 font-mono" />
                        <Button variant="secondary" onClick={() => copyToClipboard(newKeyData.token)}>
                            <Copy className="w-4 h-4" />
                        </Button>
                    </div>
                </div>
            )}

            <div className="rounded-xl border border-border/60 bg-card shadow-sm overflow-hidden">
                <Table>
                    <TableHeader className="bg-muted/30">
                        <TableRow>
                            <TableHead className="font-medium">Prefix</TableHead>
                            <TableHead className="font-medium">Name</TableHead>
                            <TableHead className="font-medium">Created At</TableHead>
                            <TableHead className="w-[80px]"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {keys.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={4} className="text-center py-8 text-muted-foreground">
                                    No API Keys generated yet.
                                </TableCell>
                            </TableRow>
                        ) : keys.map(key => (
                            <TableRow key={key.id}>
                                <TableCell className="font-mono text-[13px] text-muted-foreground">{key.prefix}***</TableCell>
                                <TableCell className="font-medium">{key.name}</TableCell>
                                <TableCell className="text-muted-foreground text-sm">{new Date(key.created_at).toLocaleDateString()}</TableCell>
                                <TableCell className="text-right">
                                    <Button variant="ghost" size="icon" onClick={() => handleDelete(key.id)} className="text-red-500 hover:text-red-600 hover:bg-red-500/10">
                                        <Trash2 className="w-4 h-4" />
                                    </Button>
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </div>
        </div>
    );
}
