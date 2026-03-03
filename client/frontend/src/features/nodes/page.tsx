'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useState } from 'react';
import { PageHeader } from '@/components/page-header';
import { Button } from '@/components/ui/button';
import { Copy, MoreHorizontal, PlusCircle, Loader2, Trash2 } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger, DropdownMenuSeparator } from '@/components/ui/dropdown-menu';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import { useToast } from '@/hooks/use-toast';
import { api } from '@/lib/api-client';

import { useTranslations } from '@/contexts/translations-context';
import { useNodes } from '@/hooks/use-dashboard-data';

export default function NodesPage() {
  const { role, hasScope } = useAuth();
  const { t } = useTranslations();
  const { nodes: realNodes, loading: nodesLoading, error, refresh } = useNodes();
  const [isMounted, setIsMounted] = useState(false);
  const { toast } = useToast();

  // ── Create state ──────────────────────────────────────────
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createPublicHost, setCreatePublicHost] = useState('');
  const [createPublicPort, setCreatePublicPort] = useState('8080');
  const [createSftpPort, setCreateSftpPort] = useState('5657');
  const [createUseDifferentHost, setCreateUseDifferentHost] = useState(false);
  const [createPrivateHost, setCreatePrivateHost] = useState('');
  const [createPrivatePort, setCreatePrivatePort] = useState('8080');
  const [isCreating, setIsCreating] = useState(false);

  // ── Edit state ────────────────────────────────────────────
  const [editingNode, setEditingNode] = useState<any | null>(null);
  const [editName, setEditName] = useState('');
  const [editPublicHost, setEditPublicHost] = useState('');
  const [editPublicPort, setEditPublicPort] = useState('8080');
  const [editSftpPort, setEditSftpPort] = useState('5657');
  const [editUseDifferentHost, setEditUseDifferentHost] = useState(false);
  const [editPrivateHost, setEditPrivateHost] = useState('');
  const [editPrivatePort, setEditPrivatePort] = useState('8080');
  const [isSaving, setIsSaving] = useState(false);

  // ── Deploy state ──────────────────────────────────────────
  const [deployingNode, setDeployingNode] = useState<any | null>(null);

  // ── Delete state ──────────────────────────────────────────
  const [deletingNode, setDeletingNode] = useState<any | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  useEffect(() => {
    setIsMounted(true);
    if (role && !hasScope('nodes.view')) {
      window.location.href = '/dashboard';
    }
  }, [role, hasScope]);

  useEffect(() => {
    if (editingNode) {
      setEditName(editingNode.name || '');
      setEditPublicHost(editingNode.publicHost || '');
      setEditPublicPort(String(editingNode.publicPort || '8080'));
      setEditSftpPort(String(editingNode.sftpPort || '5657'));
      setEditUseDifferentHost(!!editingNode.privateHost);
      setEditPrivateHost(editingNode.privateHost || '');
      setEditPrivatePort(String(editingNode.privatePort || '8080'));
    }
  }, [editingNode]);

  // ── Handlers ──────────────────────────────────────────────

  const handleCreateNode = async () => {
    if (!createName.trim()) {
      toast({ title: t('nodes.addDialog.nameLabel'), description: 'El nombre es obligatorio.', variant: 'destructive' });
      return;
    }
    if (!createPublicHost.trim()) {
      toast({ title: t('nodes.addDialog.publicHostLabel'), description: 'La dirección pública es obligatoria.', variant: 'destructive' });
      return;
    }
    setIsCreating(true);
    try {
      const pHost = createUseDifferentHost ? createPrivateHost.trim() : createPublicHost.trim();
      const pPort = createUseDifferentHost ? (Number(createPrivatePort) || 8080) : (Number(createPublicPort) || 8080);

      await api.post('/api/nodes', {
        name: createName.trim(),
        publicHost: createPublicHost.trim(),
        publicPort: Number(createPublicPort) || 8080,
        sftpPort: Number(createSftpPort) || 5657,
        privateHost: pHost,
        privatePort: pPort,
      });
      toast({ title: t('nodes.addDialog.createButton'), description: `Nodo "${createName}" creado correctamente.` });
      // reset form
      setCreateName(''); setCreatePublicHost(''); setCreatePublicPort('8080');
      setCreateSftpPort('5657'); setCreateUseDifferentHost(false);
      setCreatePrivateHost(''); setCreatePrivatePort('8080');
      setIsCreateOpen(false);
      refresh();
    } catch (e: any) {
      toast({ title: 'Error al crear nodo', description: e?.message || 'Error desconocido.', variant: 'destructive' });
    } finally {
      setIsCreating(false);
    }
  };

  const handleUpdateNode = async () => {
    if (!editingNode) return;
    if (!editName.trim()) {
      toast({ title: 'Error', description: 'El nombre es obligatorio.', variant: 'destructive' });
      return;
    }
    setIsSaving(true);
    const pHost = editUseDifferentHost ? editPrivateHost.trim() : editPublicHost.trim();
    const pPort = editUseDifferentHost ? (Number(editPrivatePort) || 8080) : (Number(editPublicPort) || 8080);

    try {
      await api.put(`/api/nodes/${editingNode.id}`, {
        name: editName.trim(),
        publicHost: editPublicHost.trim(),
        publicPort: Number(editPublicPort) || 8080,
        sftpPort: Number(editSftpPort) || 5657,
        privateHost: pHost,
        privatePort: pPort,
      });
      toast({ title: t('nodes.editDialog.save'), description: `Nodo "${editName}" actualizado.` });
      setEditingNode(null);
      refresh();
    } catch (e: any) {
      toast({ title: 'Error al actualizar', description: e?.message || 'Error desconocido.', variant: 'destructive' });
    } finally {
      setIsSaving(false);
    }
  };

  const handleDeleteNode = async () => {
    if (!deletingNode) return;
    setIsDeleting(true);
    try {
      await api.delete(`/api/nodes/${deletingNode.id}`);
      toast({ title: 'Nodo eliminado', description: `"${deletingNode.name}" fue eliminado correctamente.` });
      setDeletingNode(null);
      refresh();
    } catch (e: any) {
      toast({ title: 'Error al eliminar', description: e?.message || 'Error desconocido.', variant: 'destructive' });
    } finally {
      setIsDeleting(false);
    }
  };

  const StatusIndicator = ({ status }: { status: string }) => {
    const statusClasses: Record<string, string> = {
      online: 'bg-green-500',
      offline: 'bg-red-500',
      deploying: 'bg-yellow-500 animate-pulse',
    };
    return <div className={`mr-2 h-2.5 w-2.5 rounded-full ${statusClasses[status] || 'bg-gray-400'}`} />;
  };

  const CopyableCode = ({ command }: { command: string }) => {
    const handleCopy = () => {
      navigator.clipboard.writeText(command);
      toast({ title: t('common.copied') });
    };

    return (
      <div className="flex items-center gap-2 rounded-md bg-muted p-2 my-2 font-mono text-sm">
        <pre className="flex-grow overflow-x-auto"><code>{command}</code></pre>
        <Button variant="ghost" size="icon" className="h-8 w-8 flex-shrink-0" onClick={handleCopy}>
          <Copy className="h-4 w-4" />
          <span className="sr-only">{t('common.copy')}</span>
        </Button>
      </div>
    );
  };

  const deployConfigJson = deployingNode ? JSON.stringify({
    "logs": "/var/log/AetherPanel",
    "web": { "host": "0.0.0.0:8080" },
    "token": { "public": "http://192.168.0.12:8080/auth/publickey" },
    "panel": { "enable": false },
    "daemon": {
      "auth": {
        "url": "http://192.168.0.12:8080/oauth2/token",
        "clientId": `.node_${String(deployingNode.id).includes('-') ? String(deployingNode.id).split('-')[1] : deployingNode.id}`,
        "clientSecret": "7bdb03bbfbd44aeda8e2a4fc52035c38",
        "publicKey": ""
      },
      "data": { "root": "/var/lib/AetherPanel" },
      "sftp": { "host": `0.0.0.0:${deployingNode.sftpPort}` }
    }
  }, null, 2) : '';

  if (!isMounted || !hasScope('nodes.view') || nodesLoading) {
    return (
      <div className="flex h-screen w-full items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    );
  }

  const nodes = realNodes.map((n: any) => ({
    ...n,
    status: n.systemInfo ? 'online' : 'offline',
    location: n.isLocal ? 'Local' : (n.publicHost || 'Unknown')
  }));

  return (
    <div className="flex flex-col gap-8">
      <PageHeader title={t('nodes.title')} description={t('nodes.description')}>
        {/* ── Create Node Dialog ── */}
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <DialogTrigger asChild>
            <Button>
              <PlusCircle className="mr-2" />
              {t('nodes.addNode')}
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-2xl">
            <DialogHeader>
              <DialogTitle>{t('nodes.addDialog.title')}</DialogTitle>
            </DialogHeader>
            <div className="grid gap-6 py-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="node-name">{t('nodes.addDialog.nameLabel')} <span className="text-red-500">*</span></Label>
                  <Input
                    id="node-name"
                    placeholder={t('nodes.addDialog.namePlaceholder')}
                    value={createName}
                    onChange={(e) => setCreateName(e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && handleCreateNode()}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="public-host">{t('nodes.addDialog.publicHostLabel')} <span className="text-red-500">*</span></Label>
                  <Input
                    id="public-host"
                    placeholder={t('nodes.addDialog.publicHostPlaceholder')}
                    value={createPublicHost}
                    onChange={(e) => setCreatePublicHost(e.target.value)}
                  />
                </div>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="public-port">{t('nodes.addDialog.publicPortLabel')}</Label>
                  <Input
                    id="public-port"
                    type="number"
                    value={createPublicPort}
                    onChange={(e) => setCreatePublicPort(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="sftp-port">{t('nodes.addDialog.sftpPortLabel')}</Label>
                  <Input
                    id="sftp-port"
                    type="number"
                    value={createSftpPort}
                    onChange={(e) => setCreateSftpPort(e.target.value)}
                  />
                </div>
              </div>
              <div className="items-top flex space-x-2">
                <Checkbox
                  id="use-different-host"
                  checked={createUseDifferentHost}
                  onCheckedChange={(checked) => setCreateUseDifferentHost(Boolean(checked))}
                />
                <div className="grid gap-1.5 leading-none">
                  <label htmlFor="use-different-host" className="text-sm font-medium leading-none">
                    {t('nodes.addDialog.useDifferentHostLabel')}
                  </label>
                  <p className="text-sm text-muted-foreground">{t('nodes.addDialog.useDifferentHostDescription')}</p>
                </div>
              </div>
              {createUseDifferentHost && (
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 rounded-md border bg-muted/50 p-4 animate-in fade-in">
                  <div className="space-y-2">
                    <Label htmlFor="private-host">{t('nodes.addDialog.privateHostLabel')}</Label>
                    <Input
                      id="private-host"
                      placeholder={t('nodes.addDialog.privateHostPlaceholder')}
                      value={createPrivateHost}
                      onChange={(e) => setCreatePrivateHost(e.target.value)}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="private-port">{t('nodes.addDialog.privatePortLabel')}</Label>
                    <Input
                      id="private-port"
                      type="number"
                      value={createPrivatePort}
                      onChange={(e) => setCreatePrivatePort(e.target.value)}
                    />
                  </div>
                </div>
              )}
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setIsCreateOpen(false)} disabled={isCreating}>
                {t('nodes.editDialog.cancel')}
              </Button>
              <Button onClick={handleCreateNode} disabled={isCreating}>
                {isCreating && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {t('nodes.addDialog.createButton')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </PageHeader>

      <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
        <Card className="border-0">
          <CardHeader>
            <CardTitle>{t('nodes.allNodes')}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('nodes.table.name')}</TableHead>
                  <TableHead className="hidden sm:table-cell">{t('nodes.table.location')}</TableHead>
                  <TableHead>{t('dashboard.table.status')}</TableHead>
                  <TableHead className="text-right">{t('dashboard.table.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {nodes.map((node) => (
                  <TableRow key={node.id}>
                    <TableCell className="font-medium">
                      <a href={`/nodes/view/?id=${node.id}`} className="hover:underline">
                        {node.name}
                      </a>
                    </TableCell>
                    <TableCell className="hidden sm:table-cell">{node.location}</TableCell>
                    <TableCell>
                      <Badge variant={node.status === 'online' ? 'default' : node.status === 'offline' ? 'destructive' : 'secondary'} className="capitalize flex items-center gap-2 w-fit">
                        <StatusIndicator status={node.status} />
                        {t(`dashboard.status.${node.status}`)}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" className="h-8 w-8 p-0">
                            <span className="sr-only">Open menu</span>
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuLabel>{t('servers.actions.menuLabel')}</DropdownMenuLabel>
                          <DropdownMenuItem onClick={() => window.location.href = `/nodes/view/?id=${node.id}`}>
                            {t('nodes.actions.viewDetails')}
                          </DropdownMenuItem>
                          <DropdownMenuItem onClick={() => setTimeout(() => setEditingNode(node))}>{t('nodes.actions.edit')}</DropdownMenuItem>
                          <DropdownMenuItem onClick={() => setDeployingNode(node)}>{t('nodes.actions.deploy')}</DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            className="text-red-500 focus:text-red-500"
                            onClick={() => setDeletingNode(node)}
                          >
                            <Trash2 className="mr-2 h-4 w-4" />
                            {t('nodes.actions.delete')}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                ))}
                {nodes.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="h-24 text-center text-muted-foreground">
                      No hay nodos registrados. Crea uno con el botón de arriba.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>

      {/* ── Edit Node Dialog ── */}
      <Dialog open={!!editingNode} onOpenChange={(isOpen) => !isOpen && setEditingNode(null)}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t('nodes.editDialog.title')}</DialogTitle>
          </DialogHeader>
          <div className="grid gap-6 py-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="edit-node-name">{t('nodes.addDialog.nameLabel')}</Label>
                <Input id="edit-node-name" value={editName} onChange={(e) => setEditName(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-public-host">{t('nodes.addDialog.publicHostLabel')}</Label>
                <Input id="edit-public-host" value={editPublicHost} onChange={(e) => setEditPublicHost(e.target.value)} />
              </div>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="edit-public-port">{t('nodes.addDialog.publicPortLabel')}</Label>
                <Input id="edit-public-port" type="number" value={editPublicPort} onChange={(e) => setEditPublicPort(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-sftp-port">{t('nodes.addDialog.sftpPortLabel')}</Label>
                <Input id="edit-sftp-port" type="number" value={editSftpPort} onChange={(e) => setEditSftpPort(e.target.value)} />
              </div>
            </div>
            <div className="items-top flex space-x-2">
              <Checkbox
                id="edit-use-different-host"
                checked={editUseDifferentHost}
                onCheckedChange={(checked) => setEditUseDifferentHost(Boolean(checked))}
              />
              <div className="grid gap-1.5 leading-none">
                <label htmlFor="edit-use-different-host" className="text-sm font-medium leading-none">
                  {t('nodes.addDialog.useDifferentHostLabel')}
                </label>
                <p className="text-sm text-muted-foreground">{t('nodes.addDialog.useDifferentHostDescription')}</p>
              </div>
            </div>
            {editUseDifferentHost && (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 rounded-md border bg-muted/50 p-4 animate-in fade-in">
                <div className="space-y-2">
                  <Label htmlFor="edit-private-host">{t('nodes.addDialog.privateHostLabel')}</Label>
                  <Input id="edit-private-host" value={editPrivateHost} onChange={(e) => setEditPrivateHost(e.target.value)} />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="edit-private-port">{t('nodes.addDialog.privatePortLabel')}</Label>
                  <Input id="edit-private-port" type="number" value={editPrivatePort} onChange={(e) => setEditPrivatePort(e.target.value)} />
                </div>
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditingNode(null)} disabled={isSaving}>{t('nodes.editDialog.cancel')}</Button>
            <Button type="submit" onClick={handleUpdateNode} disabled={isSaving}>
              {isSaving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t('nodes.editDialog.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── Delete Confirmation Dialog ── */}
      <Dialog open={!!deletingNode} onOpenChange={(isOpen) => !isOpen && setDeletingNode(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Trash2 className="h-5 w-5 text-red-500" />
              Eliminar nodo
            </DialogTitle>
            <DialogDescription>
              ¿Estás seguro de que quieres eliminar el nodo <strong>"{deletingNode?.name}"</strong>?
              Esta acción no se puede deshacer y todos los servidores de este nodo quedarán inaccesibles.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="mt-4">
            <Button variant="outline" onClick={() => setDeletingNode(null)} disabled={isDeleting}>Cancelar</Button>
            <Button variant="destructive" onClick={handleDeleteNode} disabled={isDeleting}>
              {isDeleting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Eliminar nodo
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── Deploy Node Dialog ── */}
      <Dialog open={!!deployingNode} onOpenChange={(isOpen) => !isOpen && setDeployingNode(null)}>
        <DialogContent className="sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle>{t('nodes.deployDialog.title', { name: deployingNode?.name || '' })}</DialogTitle>
            <DialogDescription>{t('nodes.deployDialog.description')}</DialogDescription>
          </DialogHeader>
          <div className="py-4 max-h-[70vh] overflow-y-auto pr-4 space-y-6">
            <div className="flex gap-4 items-start">
              <div className="flex-shrink-0 flex h-8 w-8 items-center justify-center rounded-full bg-primary text-primary-foreground font-bold mt-1">1</div>
              <div>
                <h4 className="font-semibold">{t('nodes.deployDialog.step1')}</h4>
                <p className="text-sm text-muted-foreground">{t('nodes.deployDialog.step1Description')}</p>
              </div>
            </div>
            <div className="flex gap-4 items-start">
              <div className="flex-shrink-0 flex h-8 w-8 items-center justify-center rounded-full bg-primary text-primary-foreground font-bold mt-1">2</div>
              <div>
                <h4 className="font-semibold">{t('nodes.deployDialog.step2')}</h4>
                <p className="text-sm text-muted-foreground">{t('nodes.deployDialog.step2Description')}</p>
                <CopyableCode command="sudo systemctl stop AetherPanel" />
              </div>
            </div>
            <div className="flex gap-4 items-start">
              <div className="flex-shrink-0 flex h-8 w-8 items-center justify-center rounded-full bg-primary text-primary-foreground font-bold mt-1">3</div>
              <div>
                <h4 className="font-semibold">{t('nodes.deployDialog.step3')}</h4>
                <p className="text-sm text-muted-foreground" dangerouslySetInnerHTML={{ __html: t('nodes.deployDialog.step3Description') }}></p>
                <CopyableCode command={deployConfigJson} />
              </div>
            </div>
            <div className="flex gap-4 items-start">
              <div className="flex-shrink-0 flex h-8 w-8 items-center justify-center rounded-full bg-primary text-primary-foreground font-bold mt-1">4</div>
              <div>
                <h4 className="font-semibold">{t('nodes.deployDialog.step4')}</h4>
                <p className="text-sm text-muted-foreground">{t('nodes.deployDialog.step4Description')}</p>
                <p className="mt-4 text-sm font-semibold text-green-500">{t('nodes.deployDialog.successMessage')}</p>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeployingNode(null)}>{t('nodes.deployDialog.close')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
