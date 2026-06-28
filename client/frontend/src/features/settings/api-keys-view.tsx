import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Trash2, Key, Copy, Plus } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import { api } from '@/lib/api-client';

export function ApiKeysView() {
    const [keys, setKeys] = useState<any[]>([]);
    const [name, setName] = useState('');
    const [isCreating, setIsCreating] = useState(false);
    const [newKeyData, setNewKeyData] = useState<{token: string, key: any} | null>(null);
    const { toast } = useToast();

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
            toast({ title: 'Success', description: 'API Key generated successfully' });
        } catch (e) {
            console.error(e);
            toast({ title: 'Error', description: 'Failed to generate key', variant: 'destructive' });
        }
    };

    const handleDelete = async (id: number) => {
        if (!confirm('Are you sure you want to delete this key?')) return;
        try {
            await api.delete(`/api/settings/apikeys/${id}`);
            fetchKeys();
            toast({ title: 'Success', description: 'API Key deleted' });
        } catch (e) {
            console.error(e);
        }
    };

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text);
        toast({ title: 'Copied', description: 'API Key copied to clipboard' });
    };

    return (
        <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50 animate-in slide-in-from-bottom-4 duration-500">
            <Card className="border-0 shadow-lg">
                <CardHeader className="flex flex-row items-center justify-between">
                    <div>
                        <CardTitle>API Keys</CardTitle>
                        <CardDescription>Manage API Keys for external integrations (WHMCS, Paymenter)</CardDescription>
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
                </CardHeader>
                <CardContent>
                    {newKeyData && (
                        <div className="mb-6 p-4 rounded-md bg-green-500/10 border border-green-500/20 text-green-700 dark:text-green-400">
                            <h4 className="font-semibold mb-2 flex items-center"><Key className="w-4 h-4 mr-2"/> Key Generated Successfully</h4>
                            <p className="text-sm mb-4">Please copy this key now. You won't be able to see it again.</p>
                            <div className="flex gap-2">
                                <Input readOnly value={newKeyData.token} className="bg-background font-mono" />
                                <Button variant="secondary" onClick={() => copyToClipboard(newKeyData.token)}>
                                    <Copy className="w-4 h-4" />
                                </Button>
                            </div>
                        </div>
                    )}

                    <div className="rounded-md border">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>Prefix</TableHead>
                                    <TableHead>Name</TableHead>
                                    <TableHead>Created At</TableHead>
                                    <TableHead className="w-[100px]"></TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {keys.length === 0 ? (
                                    <TableRow>
                                        <TableCell colSpan={4} className="text-center py-6 text-muted-foreground">No API Keys generated yet.</TableCell>
                                    </TableRow>
                                ) : keys.map(key => (
                                    <TableRow key={key.id}>
                                        <TableCell className="font-mono text-sm">{key.prefix}***</TableCell>
                                        <TableCell>{key.name}</TableCell>
                                        <TableCell>{new Date(key.created_at).toLocaleDateString()}</TableCell>
                                        <TableCell>
                                            <Button variant="ghost" size="icon" onClick={() => handleDelete(key.id)} className="text-red-500 hover:text-red-600 hover:bg-red-500/10">
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
