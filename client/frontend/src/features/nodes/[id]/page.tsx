'use client';
import { useEffect, useState, useRef } from 'react';
import { useNodes } from '@/hooks/use-dashboard-data';
import { useServers } from '@/hooks/use-servers';
import { api } from '@/lib/api-client';
import { PageHeader } from '@/components/page-header';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Cpu, MemoryStick, Server as ServerIcon, CheckCircle, Info, Trash2, Rocket, Copy, Loader2, ChevronLeft, ChevronRight } from 'lucide-react';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import { Separator } from '@/components/ui/separator';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { sileo } from "@/lib/toast";
import { useTranslations } from '@/contexts/translations-context';

function findNode(realNodes: any[], idParam: string) {
  return realNodes.find((n: any) =>
    String(n.id) === idParam ||
    (idParam === 'node-local' && (n.isLocal || n.name === 'Nodo Local')) ||
    n.name.toLowerCase().replace(/\s/g, '') === idParam.toLowerCase() ||
    n.name.toLowerCase() === idParam.toLowerCase()
  );
}

function formatUptime(s: number | undefined) {
  if (!s) return '—';
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  return `${d}d ${h}h ${m}m`;
}

const InfoItem = ({ label, value, children }: { label: string, value?: React.ReactNode, children?: React.ReactNode }) => (
  <div className="flex items-start justify-between text-sm">
    <p className="text-muted-foreground">{label}</p>
    {value ? <p className="font-medium text-right text-wrap">{value}</p> : children}
  </div>
);

const CopyableCode = ({ command, t }: { command: string, t: any }) => {
  const handleCopy = () => {
    navigator.clipboard.writeText(command);
    sileo.success({ title: t('common.copied') });
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

function SystemInfoCard({ node, t }: { node: any; t: any }) {
  return (
    <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
      <Card className="border-0">
        <CardHeader>
          <CardTitle>{t('nodes.detail.systemInfo.title')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <InfoItem label={t('nodes.detail.systemInfo.os')} value={node.systemInfo?.os || '—'} />
            <InfoItem label={t('nodes.detail.systemInfo.arch')} value={node.systemInfo?.arch || '—'} />
            <InfoItem label={t('nodes.detail.systemInfo.version')} value={node.systemInfo?.platformVersion || '—'} />
            <InfoItem label={t('nodes.detail.systemInfo.publicAddress')} value={`${node.publicHost}:${node.publicPort}`} />
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-x-4 gap-y-2 pt-4 border-t">
            <h4 className="md:col-span-2 font-semibold text-base mb-2">{t('nodes.detail.systemInfo.general')}</h4>
            <InfoItem label={t('nodes.detail.systemInfo.hostname')} value={node.systemInfo?.hostname || '—'} />
            <InfoItem label={t('nodes.detail.systemInfo.platform')} value={node.systemInfo?.platform || '—'} />
            <InfoItem label={t('nodes.detail.systemInfo.uptime')} value={formatUptime(node.systemInfo?.uptime)} />
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-x-4 gap-y-2 pt-4 border-t">
            <h4 className="md:col-span-2 font-semibold text-base mb-2">{t('nodes.detail.systemInfo.cpu')}</h4>
            <InfoItem label={t('nodes.detail.systemInfo.model')} value={node.systemInfo?.cpuModel || '—'} />
            <InfoItem label={t('nodes.detail.systemInfo.physicalCores')} value={node.systemInfo?.cpuCores != null ? String(node.systemInfo.cpuCores) : '—'} />
            <InfoItem label={t('nodes.detail.systemInfo.logicalCores')} value={node.systemInfo?.cpuThreads != null ? String(node.systemInfo.cpuThreads) : '—'} />
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-x-4 gap-y-2 pt-4 border-t">
            <h4 className="md:col-span-2 font-semibold text-base mb-2">{t('nodes.detail.systemInfo.memory')}</h4>
            <InfoItem label={t('nodes.detail.systemInfo.total')} value={node.systemInfo?.memoryTotal ? `${(node.systemInfo.memoryTotal / 1024 / 1024 / 1024).toFixed(1)} GB` : '—'} />
            <InfoItem label={t('nodes.detail.systemInfo.used')} value={node.systemInfo?.memoryUsed ? `${(node.systemInfo.memoryUsed / 1024 / 1024).toFixed(0)} MB` : '—'} />
            <InfoItem label={t('nodes.detail.systemInfo.free')} value={node.systemInfo?.memoryFree ? `${(node.systemInfo.memoryFree / 1024 / 1024 / 1024).toFixed(1)} GB` : '—'} />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function DisksCard({ disks, t }: { disks: any[]; t: any }) {
  return (
    <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
      <Card className="border-0">
        <CardHeader>
          <CardTitle>{t('nodes.detail.disks.title')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {disks.map((disk: any, index: number) => (
            <div key={index} className="space-y-2">
              <div className="flex justify-between text-sm">
                <p className="font-mono">{disk.path}</p>
                <p className="text-muted-foreground">
                  {disk.used != null ? `${(disk.used / 1024 / 1024 / 1024).toFixed(1)} GB` : '?'}
                  {' / '}
                  {disk.total != null ? `${(disk.total / 1024 / 1024 / 1024).toFixed(1)} GB` : '?'}
                  {disk.usedPercent != null ? ` (${disk.usedPercent.toFixed(1)}%)` : ''}
                </p>
              </div>
              <Progress value={disk.usedPercent || 0} />
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}

function ServersCard({ serversOnNode, t }: { serversOnNode: any[]; t: any }) {
  const [currentPage, setCurrentPage] = useState(1);
  const serversPerPage = 5;
  const indexOfLastServer = currentPage * serversPerPage;
  const indexOfFirstServer = indexOfLastServer - serversPerPage;
  const currentServers = serversOnNode.slice(indexOfFirstServer, indexOfLastServer);
  const totalPages = Math.ceil(serversOnNode.length / serversPerPage);

  return (
    <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
      <Card className="border-0">
        <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
          <CardTitle>{t('nodes.detail.serversOnNode.title')}</CardTitle>
          <Badge variant="secondary" className="font-semibold">
            {serversOnNode.length}
          </Badge>
        </CardHeader>
        <CardContent>
          {serversOnNode.length > 0 ? (
            <>
              <ul className="space-y-2">
                {currentServers.map((server: any) => (
                  <li key={server.id} className="flex items-center justify-between rounded-md border p-3 text-sm hover:bg-muted/50 transition-colors">
                    <div className="overflow-hidden mr-2">
                      <p className="font-medium truncate">{server.name}</p>
                      <p className="text-xs text-muted-foreground font-mono truncate">{server.id}</p>
                    </div>
                    <a href={`/servers/view/?id=${server.id}`} className="flex-shrink-0">
                      <Button variant="ghost" size="sm">{t('nodes.detail.serversOnNode.view')}</Button>
                    </a>
                  </li>
                ))}
              </ul>
              {totalPages > 1 && (
                <div className="flex items-center justify-between pt-4 border-t mt-4">
                  <Button
                    variant="outline" size="sm"
                    onClick={() => setCurrentPage((prev) => Math.max(prev - 1, 1))}
                    disabled={currentPage === 1} className="h-8 w-8 p-0"
                  >
                    <ChevronLeft className="h-4 w-4" />
                  </Button>
                  <span className="text-xs text-muted-foreground font-medium">Pág. {currentPage} de {totalPages}</span>
                  <Button
                    variant="outline" size="sm"
                    onClick={() => setCurrentPage((prev) => Math.min(prev + 1, totalPages))}
                    disabled={currentPage === totalPages} className="h-8 w-8 p-0"
                  >
                    <ChevronRight className="h-4 w-4" />
                  </Button>
                </div>
              )}
            </>
          ) : (
            <p className="text-sm text-muted-foreground text-center py-4">{t('nodes.detail.serversOnNode.empty')}</p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function DeployDialog({ isDeployDialogOpen, setIsDeployDialogOpen, node, t }: any) {
  const origin = typeof window !== 'undefined' ? window.location.origin : 'https://192.168.0.12:8080';
  const clientId = node ? `.node_${String(node.id).includes('-') ? String(node.id).split('-')[1] : node.id}` : '';
  const deployCompose = node ? [
    'services:',
    '  skypanel:',
    '    image: ghcr.io/aether-panel/panel',
    '    container_name: skypanel',
    '    restart: unless-stopped',
    '    ports:',
    '      - "8080:8080"',
    `      - "${node.sftpPort}:${node.sftpPort}"`,
    '    volumes:',
    '      - ./storage/skypanel-data:/var/lib/SkyPanel',
    '      - ./storage/skypanel-logs:/var/log/SkyPanel',
    '      - //var/run/docker.sock:/var/run/docker.sock',
    '    environment:',
    '      - GIN_MODE=release',
    '      - SKYPANEL_PLATFORM=docker',
    '      - SKYPANEL_WEB_HOST=0.0.0.0:8080',
    '      - SKYPANEL_PANEL_ENABLE=false',
    `      - SKYPANEL_PANEL_SETTINGS_MASTERURL=${origin}`,
    `      - SKYPANEL_TOKEN_PUBLIC=${origin}/auth/publickey`,
    `      - SKYPANEL_DAEMON_AUTH_URL=${origin}/oauth2/token`,
    `      - SKYPANEL_DAEMON_AUTH_CLIENTID=${clientId}`,
    '      - SKYPANEL_DAEMON_AUTH_CLIENTSECRET=7bdb03bbfbd44aeda8e2a4fc52035c38',
    `      - SKYPANEL_DAEMON_SFTP_HOST=0.0.0.0:${node.sftpPort}`,
    '    user: "0:0"'
  ].join('\n') : '';

  return (
    <Dialog open={isDeployDialogOpen} onOpenChange={setIsDeployDialogOpen}>
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>{t('nodes.deployDialog.title', { name: node?.name || '' })}</DialogTitle>
          <DialogDescription>
            {t('nodes.deployDialog.description')}
          </DialogDescription>
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
              <CopyableCode command="mkdir -p /opt/AetherPanel && cd /opt/AetherPanel" t={t}  />
            </div>
          </div>
          <div className="flex gap-4 items-start">
            <div className="flex-shrink-0 flex h-8 w-8 items-center justify-center rounded-full bg-primary text-primary-foreground font-bold mt-1">3</div>
            <div>
              <h4 className="font-semibold">{t('nodes.deployDialog.step3')}</h4>
              <p className="text-sm text-muted-foreground" dangerouslySetInnerHTML={{ __html: t('nodes.deployDialog.step3Description') }}></p>
              <CopyableCode command={deployCompose} t={t}  />
            </div>
          </div>
          <div className="flex gap-4 items-start">
            <div className="flex-shrink-0 flex h-8 w-8 items-center justify-center rounded-full bg-primary text-primary-foreground font-bold mt-1">4</div>
            <div>
              <h4 className="font-semibold">{t('nodes.deployDialog.step4')}</h4>
              <p className="text-sm text-muted-foreground">{t('nodes.deployDialog.step4Description')}</p>
              <CopyableCode command="docker compose up -d" t={t}  />
              <p className="mt-4 text-sm font-semibold text-green-500">{t('nodes.deployDialog.successMessage')}</p>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => setIsDeployDialogOpen(false)}>{t('nodes.deployDialog.close')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function EditNodeCard({ node, t }: { node: any; t: any }) {
  const [editName, setEditName] = useState(node.name || '');
  const [editPublicHost, setEditPublicHost] = useState(node.publicHost || '');
  const [editPublicPort, setEditPublicPort] = useState(String(node.publicPort || '8080'));
  const [editSftpPort, setEditSftpPort] = useState(String(node.sftpPort || '5657'));
  const [editUseDifferentHost, setEditUseDifferentHost] = useState(!!node.privateHost);
  const [editPrivateHost, setEditPrivateHost] = useState(node.privateHost || '');
  const [editPrivatePort, setEditPrivatePort] = useState(String(node.privatePort || '8080'));
  const [isDeployDialogOpen, setIsDeployDialogOpen] = useState(false);
  const [isDeleteConfirmOpen, setIsDeleteConfirmOpen] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);

  useEffect(() => {
    if (node) {
      setEditName(node.name || '');
      setEditPublicHost(node.publicHost || '');
      setEditPublicPort(String(node.publicPort || '8080'));
      setEditSftpPort(String(node.sftpPort || '5657'));
      setEditUseDifferentHost(!!node.privateHost);
      setEditPrivateHost(node.privateHost || '');
      setEditPrivatePort(String(node.privatePort || '8080'));
    }
  }, [node]);

  const handleUpdateNode = async () => {
    if (!editName.trim()) {
      sileo.error({ title: 'Error', description: 'El nombre es obligatorio.' });
      return;
    }
    setIsSaving(true);
    const pHost = editUseDifferentHost ? editPrivateHost.trim() : editPublicHost.trim();
    const pPort = editUseDifferentHost ? (Number(editPrivatePort) || 8080) : (Number(editPublicPort) || 8080);
    try {
      await api.put(`/api/nodes/${node.id}`, {
        name: editName.trim(),
        publicHost: editPublicHost.trim(),
        publicPort: Number(editPublicPort) || 8080,
        sftpPort: Number(editSftpPort) || 5657,
        privateHost: pHost,
        privatePort: pPort,
      });
      sileo.success({ title: t('nodes.editDialog.save'), description: `Nodo "${editName}" actualizado.` });
    } catch (e: any) {
      sileo.error({ title: 'Error al actualizar', description: e?.message || 'Error desconocido.' });
    } finally {
      setIsSaving(false);
    }
  };

  const handleDeleteNode = async () => {
    setIsDeleting(true);
    try {
      await api.delete(`/api/nodes/${node.id}`);
      sileo.success({ title: 'Nodo eliminado', description: `"${node.name}" fue eliminado correctamente.` });
      window.location.href = '/nodes/';
    } catch (e: any) {
      sileo.error({ title: 'Error al eliminar', description: e?.message || 'Error desconocido.' });
      setIsDeleting(false);
    }
  };

  return (
    <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
      <Card className="border-0">
        <CardHeader>
          <CardTitle>{t('nodes.detail.editNode.title')}</CardTitle>
        </CardHeader>
        <CardContent>
          {node?.isLocal ? (
            <Alert variant="default">
              <Info className="h-4 w-4" />
              <AlertTitle>{t('nodes.detail.editNode.localNodeAlertTitle')}</AlertTitle>
              <AlertDescription>
                {t('nodes.detail.editNode.localNodeAlertDescription')}
              </AlertDescription>
            </Alert>
          ) : (
            <div className="space-y-6">
              <div className="space-y-2">
                <Label htmlFor="edit-node-name">{t('nodes.detail.editNode.nameLabel')}</Label>
                <Input id="edit-node-name" value={editName} onChange={(e) => setEditName(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-public-host">{t('nodes.detail.editNode.publicHostLabel')}</Label>
                <Input id="edit-public-host" value={editPublicHost} onChange={(e) => setEditPublicHost(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-public-port">{t('nodes.detail.editNode.publicPortLabel')}</Label>
                <Input id="edit-public-port" type="number" value={editPublicPort} onChange={(e) => setEditPublicPort(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-sftp-port">{t('nodes.detail.editNode.sftpPortLabel')}</Label>
                <Input id="edit-sftp-port" type="number" value={editSftpPort} onChange={(e) => setEditSftpPort(e.target.value)} />
              </div>
              <div className="items-top flex space-x-2">
                <Checkbox
                  id="edit-use-different-host"
                  checked={editUseDifferentHost}
                  onCheckedChange={(checked) => setEditUseDifferentHost(Boolean(checked))}
                />
                <div className="grid gap-1.5 leading-none">
                  <label
                    htmlFor="edit-use-different-host"
                    className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
                  >
                    {t('nodes.addDialog.useDifferentHostLabel')}
                  </label>
                  <p className="text-sm text-muted-foreground">
                    {t('nodes.addDialog.useDifferentHostDescription')}
                  </p>
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
              <Button className="w-full" onClick={handleUpdateNode} disabled={isSaving}>
                {isSaving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {t('nodes.detail.editNode.updateButton')}
              </Button>
              <Separator />
              <div className="flex flex-col space-y-2">
                <Dialog open={isDeleteConfirmOpen} onOpenChange={setIsDeleteConfirmOpen}>
                  <DialogTrigger asChild>
                    <Button variant="destructive" className="w-full">
                      <Trash2 className="mr-2 h-4 w-4" />
                      {t('nodes.detail.editNode.deleteButton')}
                    </Button>
                  </DialogTrigger>
                  <DialogContent>
                    <DialogHeader>
                      <DialogTitle>Eliminar nodo</DialogTitle>
                      <DialogDescription>
                        ¿Estás seguro de que quieres eliminar este nodo? Esta acción no se puede deshacer.
                      </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                      <Button variant="outline" onClick={() => setIsDeleteConfirmOpen(false)} disabled={isDeleting}>Cancelar</Button>
                      <Button variant="destructive" onClick={handleDeleteNode} disabled={isDeleting}>
                        {isDeleting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        Eliminar nodo
                      </Button>
                    </DialogFooter>
                  </DialogContent>
                </Dialog>

                <Button variant="secondary" className="w-full" onClick={() => setIsDeployDialogOpen(true)}>
                  <Rocket className="mr-2" />
                  {t('nodes.detail.editNode.deployButton')}
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
      <DeployDialog isDeployDialogOpen={isDeployDialogOpen} setIsDeployDialogOpen={setIsDeployDialogOpen} node={node} t={t}  />
    </div>
  );
}


export default function NodeDetailPage({ params }: { params: { id: string } }) {
  const { nodes: realNodes, loading: nodesLoading } = useNodes();
  const { servers: allServers, loading: serversLoading } = useServers();
  const { t } = useTranslations();
  

  const [liveStats, setLiveStats] = useState<any>(null);
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const node = findNode(realNodes, params.id);
  const serversOnNode = allServers.filter((s: any) =>
    String(s.nodeId) === (node?.id ? String(node.id) : params.id)
  );

  useEffect(() => {
    if (node?.systemInfo && !liveStats) {
      setLiveStats(node.systemInfo);
    }
  }, [node]);

  useEffect(() => {
    if (!node?.id) return;
    const fetchLive = async () => {
      try {
        const data = await api.get(`/api/nodes/${node.id}/system`);
        setLiveStats(data);
      } catch (e) {
        // silently ignore; keep last known values
      }
    };
    fetchLive();
    pollingRef.current = setInterval(fetchLive, 3000);
    return () => {
      if (pollingRef.current) clearInterval(pollingRef.current);
    };
  }, [node?.id]);

  if (nodesLoading || serversLoading) {
    return (
      <div className="flex h-screen w-full items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-[3px] border-border border-t-primary border-l-accent" />
      </div>
    );
  }

  if (!node) {
    return (
      <div className="flex h-screen w-full items-center justify-center flex-col gap-4 text-center p-4">
        <p className="text-xl font-semibold text-muted-foreground">{t('common.notFound')}</p>
        <div className="bg-muted p-4 rounded-lg text-left max-w-md w-full space-y-2">
          <p className="text-sm font-mono text-muted-foreground">ID buscado: <span className="text-primary font-bold">{params.id}</span></p>
          <p className="text-xs text-muted-foreground">Nodos disponibles ({realNodes.length}):</p>
          <ul className="text-xs font-mono text-muted-foreground list-disc list-inside">
            {realNodes.map((n: any) => (
              <li key={n.id}>ID: <span className="font-bold">{n.id}</span> - {n.name} (isLocal: {String(n.isLocal)})</li>
            ))}
          </ul>
        </div>
        <Button onClick={() => window.location.href = '/nodes/'}>{t('common.back')}</Button>
      </div>
    );
  }

  const onlineServersCount = serversOnNode.filter((s: any) => s.status === 'online').length;
  const offlineServersCount = serversOnNode.length - onlineServersCount;

  return (
    <div className="flex flex-col gap-8">
      <PageHeader title={node.name} />

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
          <Card className="border-0 h-full">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t('nodes.detail.totalServers')}</CardTitle>
              <ServerIcon className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{serversOnNode.length}</div>
            </CardContent>
          </Card>
        </div>
        <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
          <Card className="border-0 h-full">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t('nodes.detail.serverStatus')}</CardTitle>
              <ServerIcon className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-xl font-bold">{t('nodes.detail.online', { count: onlineServersCount })}</div>
              <p className="text-xs text-muted-foreground">{t('nodes.detail.offline', { count: offlineServersCount })}</p>
            </CardContent>
          </Card>
        </div>
        <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
          <Card className="border-0 h-full">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium flex items-center gap-2">
                {t('nodes.detail.memoryUsage')}
                {liveStats && <span className="inline-block h-2 w-2 rounded-full bg-green-500 animate-pulse" title="En vivo" />}
              </CardTitle>
              <MemoryStick className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{liveStats ? `${Math.round((liveStats.memoryUsed || 0) / 1024 / 1024)} MB` : '—'}</div>
              <p className="text-xs text-muted-foreground">{t('nodes.detail.of')} {liveStats ? `${Math.round((liveStats.memoryTotal || 0) / 1024 / 1024 / 1024)} GB` : '∞'}</p>
              {liveStats && liveStats.memoryTotal > 0 && (
                <Progress className="mt-2 h-1.5" value={(liveStats.memoryUsed / liveStats.memoryTotal) * 100} />
              )}
            </CardContent>
          </Card>
        </div>
        <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
          <Card className="border-0 h-full">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium flex items-center gap-2">
                {t('nodes.detail.cpuUsage')}
                {liveStats && (
                  <span
                    className={`inline-block h-2 w-2 rounded-full animate-pulse ${(liveStats.cpuUsage || 0) > 80 ? 'bg-red-500' :
                      (liveStats.cpuUsage || 0) > 50 ? 'bg-yellow-500' : 'bg-green-500'
                      }`}
                    title="En vivo"
                  />
                )}
              </CardTitle>
              <Cpu className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold transition-all duration-500">
                {liveStats ? `${(liveStats.cpuUsage || 0).toFixed(1)}%` : '—'}
              </div>
              <p className="text-xs text-muted-foreground">
                {liveStats ? `${liveStats.cpuCores || 0} núcleos / ${liveStats.cpuThreads || 0} hilos` : '—'}
              </p>
              {liveStats && (
                <Progress
                  className={`mt-2 h-1.5 transition-all duration-500 ${(liveStats.cpuUsage || 0) > 80 ? '[&>div]:bg-red-500' :
                    (liveStats.cpuUsage || 0) > 50 ? '[&>div]:bg-yellow-500' : ''
                    }`}
                  value={liveStats.cpuUsage || 0}
                />
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      <Alert variant="default" className="border-green-500/50 bg-green-500/10 text-green-200">
        <CheckCircle className="h-4 w-4 !text-green-500" />
        <AlertTitle className="text-green-400">{t('nodes.detail.successAlertTitle')}</AlertTitle>
      </Alert>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2 space-y-6">
          <SystemInfoCard node={node} t={t} />
          <DisksCard disks={node.systemInfo?.disks || []} t={t} />
        </div>

        <div className="lg:col-span-1 space-y-6">
          <ServersCard serversOnNode={serversOnNode} t={t} />
          <EditNodeCard node={node} t={t}  />
        </div>
      </div>
    </div>
  );
}
