'use client';
import { useEffect, useState, useRef } from 'react';
import { useNodes } from '@/hooks/use-dashboard-data';
import { useServers } from '@/hooks/use-servers';
import { api } from '@/lib/api-client';
import { PageHeader } from '@/components/page-header';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { CheckCircle2, Clock, Cpu, HardDrive, Info, MemoryStick, Server as ServerIcon, SearchX, Trash2, Rocket, Copy, Loader2, ChevronLeft, ChevronRight } from '@/components/icons';
import type { AppIcon } from '@/components/icons';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { sileo } from "@/lib/toast";
import { cn } from '@/lib/utils';
import { useTranslations } from '@/contexts/translations-context';
import { EmptyState } from '@/components/empty-state';
import { keyedByContent } from '@/lib/keyed';
import { NodeDiskAlert } from '../node-disk-alert';

function findNode(realNodes: any[], idParam: string) {
  return realNodes.find((n: any) =>
    String(n.id) === idParam ||
    (idParam === 'node-local' && (n.isLocal || n.name === 'Nodo Local')) ||
    n.name.toLowerCase().replaceAll(/\s/g, '') === idParam.toLowerCase() ||
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

const GB = 1024 * 1024 * 1024;

const richText = '[&_code]:rounded [&_code]:bg-secondary [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-xs [&_code]:text-foreground';

/** Usage tone shared by every meter on the page. */
const usageTone = (percent: number) => {
  if (percent > 80) return 'bg-destructive';
  if (percent > 60) return 'bg-warning';
  return 'bg-primary';
};

function Meter({ value, label, className }: Readonly<{ value: number; label: string; className?: string }>) {
  const clamped = Math.min(100, Math.max(0, value || 0));
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(clamped)}
      className={cn('h-1.5 w-full overflow-hidden rounded-full bg-secondary', className)}
    >
      <div
        className={cn('h-full rounded-full transition-[width,background-color] duration-500 ease-panel', usageTone(clamped))}
        style={{ width: `${clamped}%` }}
      />
    </div>
  );
}

/** Title row shared by every section card on the page. */
function SectionHeader({ title, description, children, inline }: Readonly<{ title: string; description?: string; children?: React.ReactNode; inline?: boolean }>) {
  return (
    <div className={cn(
      'flex gap-4 border-b border-border/50 p-5 md:p-6',
      inline ? 'flex-row items-center justify-between' : 'flex-col sm:flex-row sm:items-center sm:justify-between',
    )}>
      <div className="min-w-0 space-y-1">
        <h2 className="text-lg font-semibold tracking-[-0.02em]">{title}</h2>
        {description && <p className="text-sm text-muted-foreground">{description}</p>}
      </div>
      {children && <div className="shrink-0 self-start sm:self-auto">{children}</div>}
    </div>
  );
}

const InfoItem = ({ label, value, mono, className }: { label: string; value: React.ReactNode; mono?: boolean; className?: string }) => (
  <div className={cn('flex items-baseline justify-between gap-4 py-1.5 text-sm', className)}>
    <dt className="shrink-0 text-muted-foreground">{label}</dt>
    <dd className={cn('min-w-0 break-words text-right font-medium', mono && 'font-mono text-[13px]')}>{value}</dd>
  </div>
);

function InfoSection({ title, children }: Readonly<{ title: string; children: React.ReactNode }>) {
  return (
    <section className="p-5 md:p-6">
      <h3 className="mb-2 text-sm font-semibold">{title}</h3>
      <dl className="grid gap-x-10 xl:grid-cols-2">{children}</dl>
    </section>
  );
}

function CopyableCode({ command }: Readonly<{ command: string }>) {
  const { t } = useTranslations();
  const handleCopy = () => {
    void navigator.clipboard.writeText(command);
    sileo.success({ title: t('common.copied') });
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

function StatCell({
  icon: Icon,
  label,
  value,
  detail,
  live,
  meter,
  className,
}: Readonly<{
  icon: AppIcon;
  label: string;
  value: React.ReactNode;
  detail?: React.ReactNode;
  live?: boolean;
  meter?: number;
  className?: string;
}>) {
  return (
    <div className={cn('flex min-w-0 flex-col gap-1 border-border/50 p-4 md:p-5', className)}>
      <dt className="flex items-center gap-2 text-sm text-muted-foreground">
        <Icon className="hidden h-4 w-4 shrink-0 sm:block" aria-hidden="true" />
        <span className="truncate">{label}</span>
      </dt>
      <dd className="truncate text-2xl font-semibold tabular-nums tracking-[-0.02em] text-foreground">{value}</dd>
      {detail && <dd className="text-pretty text-xs tabular-nums text-muted-foreground">{detail}</dd>}
      {meter !== undefined && (
        <dd className="pt-2">
          <Meter value={meter} label={label} />
        </dd>
      )}
    </div>
  );
}

function SystemInfoCard({ node }: Readonly<{ node: any }>) {
  const { t } = useTranslations();
  const info = node.systemInfo;
  return (
    <Card className="overflow-hidden">
      <SectionHeader title={t('nodes.detail.systemInfo.title')} />
      <div className="divide-y divide-border/50">
        <InfoSection title={t('nodes.detail.systemInfo.system')}>
          <InfoItem label={t('nodes.detail.systemInfo.os')} value={info?.os || '—'} />
          <InfoItem label={t('nodes.detail.systemInfo.arch')} value={info?.arch || '—'} mono />
          <InfoItem label={t('nodes.detail.systemInfo.version')} value={info?.platformVersion || '—'} mono />
          <InfoItem label={t('nodes.detail.systemInfo.publicAddress')} value={`${node.publicHost}:${node.publicPort}`} mono />
        </InfoSection>
        <InfoSection title={t('nodes.detail.systemInfo.general')}>
          <InfoItem label={t('nodes.detail.systemInfo.hostname')} value={info?.hostname || '—'} mono />
          <InfoItem label={t('nodes.detail.systemInfo.platform')} value={info?.platform || '—'} />
          <InfoItem label={t('nodes.detail.systemInfo.uptime')} value={formatUptime(info?.uptime)} />
        </InfoSection>
        <InfoSection title={t('nodes.detail.systemInfo.cpu')}>
          <InfoItem className="xl:col-span-2" label={t('nodes.detail.systemInfo.model')} value={info?.cpuModel || '—'} />
          <InfoItem label={t('nodes.detail.systemInfo.physicalCores')} value={info?.cpuCores != null ? String(info.cpuCores) : '—'} />
          <InfoItem label={t('nodes.detail.systemInfo.logicalCores')} value={info?.cpuThreads != null ? String(info.cpuThreads) : '—'} />
        </InfoSection>
        <InfoSection title={t('nodes.detail.systemInfo.memory')}>
          <InfoItem label={t('nodes.detail.systemInfo.total')} value={info?.memoryTotal ? `${(info.memoryTotal / GB).toFixed(1)} GB` : '—'} />
          <InfoItem label={t('nodes.detail.systemInfo.used')} value={info?.memoryUsed ? `${(info.memoryUsed / GB).toFixed(1)} GB` : '—'} />
          {/* what can still be given out, page cache included: "free" alone looked nearly full */}
          <InfoItem
            label={t('nodes.detail.systemInfo.available')}
            value={(info?.memoryAvailable ?? info?.memoryFree) ? `${((info.memoryAvailable ?? info.memoryFree) / GB).toFixed(1)} GB` : '—'}
          />
        </InfoSection>
      </div>
    </Card>
  );
}

function DisksCard({ disks }: Readonly<{ disks: any[] }>) {
  const { t } = useTranslations();
  return (
    <Card className="overflow-hidden">
      <SectionHeader title={t('nodes.detail.disks.title')} />
      <div className="p-5 md:p-6">
        {disks.length === 0 ? (
          <EmptyState icon={HardDrive} size="sm" as="p" title={t('nodes.detail.disks.empty')} />
        ) : (
          <ul className="space-y-5">
            {keyedByContent(disks, (disk: any) => String(disk.path)).map(({ key, item: disk }) => (
              <li key={key} className="space-y-2">
                <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-0.5 text-sm">
                  <span className="min-w-0 truncate font-mono text-[13px] text-foreground">{disk.path}</span>
                  <span className="tabular-nums text-muted-foreground">
                    {disk.used != null ? `${(disk.used / GB).toFixed(1)} GB` : '?'}
                    {' / '}
                    {disk.total != null ? `${(disk.total / GB).toFixed(1)} GB` : '?'}
                    {disk.usedPercent != null && (
                      <span className="ml-2 font-medium text-foreground">{disk.usedPercent.toFixed(1)}%</span>
                    )}
                  </span>
                </div>
                <Meter value={disk.usedPercent || 0} label={disk.path} />
              </li>
            ))}
          </ul>
        )}
      </div>
    </Card>
  );
}

function ServersCard({ serversOnNode }: Readonly<{ serversOnNode: any[] }>) {
  const { t } = useTranslations();
  const [currentPage, setCurrentPage] = useState(1);
  const serversPerPage = 5;
  const indexOfLastServer = currentPage * serversPerPage;
  const indexOfFirstServer = indexOfLastServer - serversPerPage;
  const currentServers = serversOnNode.slice(indexOfFirstServer, indexOfLastServer);
  const totalPages = Math.ceil(serversOnNode.length / serversPerPage);

  return (
    <Card className="overflow-hidden">
      <SectionHeader inline title={t('nodes.detail.serversOnNode.title')}>
        <Badge variant="secondary" className="tabular-nums">{serversOnNode.length}</Badge>
      </SectionHeader>
      <div className="p-5 md:p-6">
        {serversOnNode.length > 0 ? (
          <>
            <ul className="divide-y divide-border/50 overflow-hidden rounded-xl border border-border/60 bg-background/30">
              {currentServers.map((server: any) => {
                const online = server.status === 'online';
                return (
                  <li key={server.id}>
                    <a
                      href={`/servers/view/?id=${server.id}`}
                      className="group flex items-center gap-3 px-4 py-3 text-sm transition-colors duration-150 hover:bg-accent/30 focus-ring"
                    >
                      <span
                        className={cn('h-2 w-2 shrink-0 rounded-full', online ? 'bg-success' : 'bg-muted-foreground/40')}
                        title={t(`dashboard.status.${online ? 'online' : 'offline'}`)}
                        aria-hidden="true"
                      />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-medium text-foreground">{server.name}</span>
                        <span className="block truncate font-mono text-xs text-muted-foreground">{server.id}</span>
                      </span>
                      <span className="sr-only">{t(`dashboard.status.${online ? 'online' : 'offline'}`)}</span>
                      <span className="shrink-0 text-xs font-medium text-muted-foreground transition-colors duration-150 group-hover:text-foreground">
                        {t('nodes.detail.serversOnNode.view')}
                      </span>
                      <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                    </a>
                  </li>
                );
              })}
            </ul>
            {totalPages > 1 && (
              <div className="mt-4 flex items-center justify-between">
                <Button
                  variant="outline" size="icon"
                  onClick={() => setCurrentPage((prev) => Math.max(prev - 1, 1))}
                  disabled={currentPage === 1} className="h-9 w-9"
                  aria-label={t('nodes.detail.serversOnNode.previous')}
                >
                  <ChevronLeft aria-hidden="true" />
                </Button>
                <span className="text-xs tabular-nums text-muted-foreground">
                  {t('nodes.detail.serversOnNode.page', { current: currentPage, total: totalPages })}
                </span>
                <Button
                  variant="outline" size="icon"
                  onClick={() => setCurrentPage((prev) => Math.min(prev + 1, totalPages))}
                  disabled={currentPage === totalPages} className="h-9 w-9"
                  aria-label={t('nodes.detail.serversOnNode.next')}
                >
                  <ChevronRight aria-hidden="true" />
                </Button>
              </div>
            )}
          </>
        ) : (
          <EmptyState icon={ServerIcon} size="sm" as="p" title={t('nodes.detail.serversOnNode.empty')} />
        )}
      </div>
    </Card>
  );
}

/** The part of a node id the daemon's OAuth client is named after ("node-3" → "3"). */
function nodeClientSuffix(id: unknown) {
  const raw = String(id);
  return raw.includes('-') ? raw.split('-')[1] : id;
}

function DeployDialog({ isDeployDialogOpen, setIsDeployDialogOpen, node }: any) {
  const { t } = useTranslations();
  const origin = typeof window !== 'undefined' ? window.location.origin : 'https://192.168.0.12:8080';
  const clientId = node ? `.node_${nodeClientSuffix(node.id)}` : '';
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
      <DialogContent className="flex max-h-[min(90svh,56rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-3xl">
        <DialogHeader className="border-b border-border/60 p-6 pb-5 text-left">
          <DialogTitle className="text-xl font-semibold tracking-[-0.02em]">{t('nodes.deployDialog.title', { name: node?.name || '' })}</DialogTitle>
          <DialogDescription className={richText} dangerouslySetInnerHTML={{ __html: t('nodes.deployDialog.description') }} />
        </DialogHeader>
        <div className="custom-scrollbar min-h-0 flex-1 space-y-6 overflow-y-auto p-6">
          <ol className="space-y-6">
            <li className="space-y-1.5">
              <h3 className="text-sm font-semibold">{t('nodes.deployDialog.step1')}</h3>
              <p className="text-sm text-muted-foreground">{t('nodes.deployDialog.step1Description')}</p>
            </li>
            <li className="space-y-1.5">
              <h3 className="text-sm font-semibold">{t('nodes.deployDialog.step2')}</h3>
              <p className={cn('text-sm text-muted-foreground', richText)} dangerouslySetInnerHTML={{ __html: t('nodes.deployDialog.step2Description') }} />
              <div className="pt-1.5"><CopyableCode command="mkdir -p /opt/AetherPanel && cd /opt/AetherPanel" /></div>
            </li>
            <li className="space-y-1.5">
              <h3 className="text-sm font-semibold">{t('nodes.deployDialog.step3')}</h3>
              <p className={cn('text-sm text-muted-foreground', richText)} dangerouslySetInnerHTML={{ __html: t('nodes.deployDialog.step3Description') }} />
              <div className="pt-1.5"><CopyableCode command={deployCompose} /></div>
            </li>
            <li className="space-y-1.5">
              <h3 className="text-sm font-semibold">{t('nodes.deployDialog.step4')}</h3>
              <p className="text-sm text-muted-foreground">{t('nodes.deployDialog.step4Description')}</p>
              <div className="pt-1.5"><CopyableCode command="docker compose up -d" /></div>
            </li>
          </ol>
          <div className="flex items-start gap-2.5 rounded-xl border border-success/25 bg-success/10 px-4 py-3 text-sm text-success">
            <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <p>{t('nodes.deployDialog.successMessage')}</p>
          </div>
        </div>
        <DialogFooter className="border-t border-border/60 px-6 py-4">
          <Button variant="outline" onClick={() => setIsDeployDialogOpen(false)}>{t('nodes.deployDialog.close')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function EditNodeCard({ node }: Readonly<{ node: any }>) {
  const { t } = useTranslations();
  const [editName, setEditName] = useState(node.name || '');
  const [editPublicHost, setEditPublicHost] = useState(node.publicHost || '');
  const [editPublicPort, setEditPublicPort] = useState(String(node.publicPort || '8080'));
  const [editSftpPort, setEditSftpPort] = useState(String(node.sftpPort || '5657'));
  const [editUseDifferentHost, setEditUseDifferentHost] = useState(!!node.privateHost);
  const [editPrivateHost, setEditPrivateHost] = useState(node.privateHost || '');
  const [editPrivatePort, setEditPrivatePort] = useState(String(node.privatePort || '8080'));
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
      sileo.error({ title: t('common.error'), description: t('nodes.toast.nameRequired') });
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
      sileo.success({ title: t('common.success'), description: t('nodes.toast.updated', { name: editName }) });
    } catch (e: any) {
      sileo.error({ title: t('nodes.toast.updateFailed'), description: e?.message || t('nodes.toast.unknownError') });
    } finally {
      setIsSaving(false);
    }
  };

  const handleDeleteNode = async () => {
    setIsDeleting(true);
    try {
      await api.delete(`/api/nodes/${node.id}`);
      sileo.success({ title: t('common.success'), description: t('nodes.toast.deletedDescription', { name: node.name }) });
      window.location.href = '/nodes/';
    } catch (e: any) {
      sileo.error({ title: t('nodes.toast.deleteFailed'), description: e?.message || t('nodes.toast.unknownError') });
      setIsDeleting(false);
    }
  };

  return (
    <Card className="overflow-hidden">
      <SectionHeader
        title={t('nodes.detail.editNode.title')}
        description={node?.isLocal ? undefined : t('nodes.editDialog.description')}
      />
      {node?.isLocal ? (
        <div className="p-5 md:p-6">
          <div className="flex gap-3 rounded-xl border border-border/60 bg-background/30 p-4">
            <Info className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <div className="space-y-1">
              <p className="text-sm font-medium">{t('nodes.detail.editNode.localNodeAlertTitle')}</p>
              <p className="text-sm text-muted-foreground">{t('nodes.detail.editNode.localNodeAlertDescription')}</p>
            </div>
          </div>
        </div>
      ) : (
        <>
          <div className="space-y-5 p-5 md:p-6">
            <div className="space-y-2">
              <Label htmlFor="edit-node-name">{t('nodes.detail.editNode.nameLabel')}</Label>
              <Input id="edit-node-name" value={editName} onChange={(e) => setEditName(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="edit-public-host">{t('nodes.detail.editNode.publicHostLabel')}</Label>
              <Input id="edit-public-host" className="font-mono" value={editPublicHost} onChange={(e) => setEditPublicHost(e.target.value)} />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="edit-public-port">{t('nodes.detail.editNode.publicPortLabel')}</Label>
                <Input id="edit-public-port" type="number" className="font-mono" value={editPublicPort} onChange={(e) => setEditPublicPort(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-sftp-port">{t('nodes.detail.editNode.sftpPortLabel')}</Label>
                <Input id="edit-sftp-port" type="number" className="font-mono" value={editSftpPort} onChange={(e) => setEditSftpPort(e.target.value)} />
              </div>
            </div>
            <div className="rounded-xl border border-border/60 bg-background/30">
              <div className="flex items-start gap-3 p-4">
                <Checkbox
                  id="edit-use-different-host"
                  className="mt-0.5"
                  checked={editUseDifferentHost}
                  onCheckedChange={(checked) => setEditUseDifferentHost(Boolean(checked))}
                />
                <div className="grid gap-1">
                  <label htmlFor="edit-use-different-host" className="cursor-pointer text-sm font-medium leading-snug">
                    {t('nodes.addDialog.useDifferentHostLabel')}
                  </label>
                  <p className="text-sm text-muted-foreground">
                    {t('nodes.addDialog.useDifferentHostDescription')}
                  </p>
                </div>
              </div>
              {editUseDifferentHost && (
                <div className="grid grid-cols-1 gap-4 border-t border-border/50 p-4">
                  <div className="space-y-2">
                    <Label htmlFor="edit-private-host">{t('nodes.addDialog.privateHostLabel')}</Label>
                    <Input id="edit-private-host" className="font-mono" value={editPrivateHost} onChange={(e) => setEditPrivateHost(e.target.value)} />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="edit-private-port">{t('nodes.addDialog.privatePortLabel')}</Label>
                    <Input id="edit-private-port" type="number" className="font-mono" value={editPrivatePort} onChange={(e) => setEditPrivatePort(e.target.value)} />
                  </div>
                </div>
              )}
            </div>
            <Button className="w-full" onClick={handleUpdateNode} disabled={isSaving}>
              {isSaving && <Loader2 className="animate-spin" aria-hidden="true" />}
              {t('nodes.detail.editNode.updateButton')}
            </Button>
          </div>
          <div className="border-t border-border/50 p-5 md:p-6">
            <Dialog open={isDeleteConfirmOpen} onOpenChange={setIsDeleteConfirmOpen}>
              <DialogTrigger asChild>
                <Button
                  variant="outline"
                  className="w-full text-destructive-bright hover:border-destructive/50 hover:bg-destructive/10 hover:text-destructive-bright"
                >
                  <Trash2 aria-hidden="true" />
                  {t('nodes.detail.editNode.deleteButton')}
                </Button>
              </DialogTrigger>
              <DialogContent className="sm:max-w-md">
                <DialogHeader className="text-left">
                  <DialogTitle>{t('nodes.deleteDialog.title')}</DialogTitle>
                  <DialogDescription>
                    {t('nodes.deleteDialog.description', { name: node.name })}
                  </DialogDescription>
                </DialogHeader>
                <DialogFooter className="mt-2 gap-2 sm:gap-0">
                  <Button variant="outline" onClick={() => setIsDeleteConfirmOpen(false)} disabled={isDeleting}>{t('common.cancel')}</Button>
                  <Button variant="destructive" className="bg-destructive text-destructive-foreground hover:bg-destructive/90" onClick={handleDeleteNode} disabled={isDeleting}>
                    {isDeleting && <Loader2 className="animate-spin" aria-hidden="true" />}
                    {t('nodes.deleteDialog.confirm')}
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </div>
        </>
      )}
    </Card>
  );
}

function NodeDetailSkeleton() {
  return (
    <div className="flex flex-col gap-8" aria-busy="true">
      <div className="space-y-2">
        <Skeleton className="h-9 w-56" />
        <Skeleton className="h-4 w-80 max-w-full" />
      </div>
      <Skeleton className="h-[8.5rem] rounded-xl" />
      <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
        <div className="space-y-6 xl:col-span-2">
          <Skeleton className="h-[26rem] rounded-xl" />
          <Skeleton className="h-48 rounded-xl" />
        </div>
        <div className="space-y-6">
          <Skeleton className="h-72 rounded-xl" />
          <Skeleton className="h-56 rounded-xl" />
        </div>
      </div>
    </div>
  );
}


export default function NodeDetailPage({ params }: Readonly<{ params: { id: string } }>) {
  const { nodes: realNodes, loading: nodesLoading } = useNodes();
  const { servers: allServers, loading: serversLoading } = useServers();
  const { t } = useTranslations();
  const [isDeployDialogOpen, setIsDeployDialogOpen] = useState(false);

  const [liveStats, setLiveStats] = useState<any>(null);
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const node = findNode(realNodes, params.id);
  const serversOnNode = allServers.filter((s: any) =>
    String(s.nodeId) === (node?.id ? String(node.id) : params.id)
  );

  // Hand the node name to the shell breadcrumb, which otherwise shows the raw id.
  const nodeName = node?.name;
  useEffect(() => {
    if (!nodeName) return;
    window.dispatchEvent(new CustomEvent('aether:breadcrumb', { detail: { id: params.id, label: nodeName } }));
  }, [params.id, nodeName]);

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
      } catch {
        // the node did not answer: show no figures rather than old ones as if they were live
        setLiveStats(null);
      }
    };
    void fetchLive();
    pollingRef.current = setInterval(() => {
      // a hidden tab does not poll: each /system call keeps the node busy for a second
      if (document.visibilityState === 'hidden') return;
      void fetchLive();
    }, 3000);
    return () => {
      if (pollingRef.current) clearInterval(pollingRef.current);
    };
  }, [node?.id]);

  if (nodesLoading || serversLoading) {
    return <NodeDetailSkeleton />;
  }

  if (!node) {
    return (
      <Card className="flex flex-col items-center px-6 py-16 text-center">
        <SearchX className="mb-3 h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
        <h1 className="text-base font-semibold">{t('common.notFound')}</h1>
        <p className="mt-1 max-w-sm text-sm text-muted-foreground">
          {t('nodes.detail.notFoundDescription', { id: params.id })}
        </p>
        <Button variant="outline" className="mt-6" onClick={() => window.location.href = '/nodes/'}>
          <ChevronLeft aria-hidden="true" />
          {t('common.back')}
        </Button>
      </Card>
    );
  }

  const onlineServersCount = serversOnNode.filter((s: any) => s.status === 'online').length;
  const offlineServersCount = serversOnNode.length - onlineServersCount;
  const isOnline = !!node.systemInfo;
  const memoryPercent = liveStats && liveStats.memoryTotal > 0 ? (liveStats.memoryUsed / liveStats.memoryTotal) * 100 : undefined;
  const cpuPercent = liveStats ? (liveStats.cpuUsage || 0) : undefined;
  const memoryTotalLabel = liveStats ? `${Math.round((liveStats.memoryTotal || 0) / GB)} GB` : '∞';
  const memoryPercentLabel = memoryPercent !== undefined ? ` · ${Math.round(memoryPercent)}%` : '';
  const uptime = liveStats?.uptime ?? node.systemInfo?.uptime;

  return (
    <div className="flex flex-col gap-8">
      <PageHeader
        title={node.name}
        description={isOnline ? t('nodes.detail.successAlertTitle') : t('nodes.detail.offlineDescription')}
        badges={
          <span
            className={cn(
              'inline-flex items-center gap-1.5 whitespace-nowrap rounded-md px-2 py-0.5 text-xs font-medium',
              isOnline ? 'bg-success/10 text-success' : 'bg-destructive/10 text-destructive-bright',
            )}
          >
            <span className="h-1.5 w-1.5 rounded-full bg-current" aria-hidden="true" />
            <span className="first-letter:uppercase">{t(`dashboard.status.${isOnline ? 'online' : 'offline'}`)}</span>
          </span>
        }
      >
        {!node.isLocal && (
          <Button variant="outline" onClick={() => setIsDeployDialogOpen(true)}>
            <Rocket aria-hidden="true" />
            {t('nodes.detail.editNode.deployButton')}
          </Button>
        )}
      </PageHeader>

      <NodeDiskAlert nodes={[{ id: node.id, name: node.name, systemInfo: liveStats ?? node.systemInfo }]} linkToNode={false} />

      <Card className="overflow-hidden">
        <dl className="grid grid-cols-2 xl:grid-cols-4">
          <StatCell
            icon={ServerIcon}
            label={t('nodes.detail.totalServers')}
            value={serversOnNode.length}
            detail={`${t('nodes.detail.online', { count: onlineServersCount })} · ${t('nodes.detail.offline', { count: offlineServersCount })}`}
            className="border-b border-r xl:border-b-0"
          />
          <StatCell
            icon={Clock}
            label={t('nodes.detail.systemInfo.uptime')}
            value={formatUptime(uptime)}
            detail={[node.systemInfo?.platform, node.systemInfo?.platformVersion].filter(Boolean).join(' ') || undefined}
            className="border-b xl:border-b-0 xl:border-r"
          />
          <StatCell
            icon={MemoryStick}
            label={t('nodes.detail.memoryUsage')}
            live={!!liveStats}
            value={liveStats ? `${((liveStats.memoryUsed || 0) / GB).toFixed(1)} GB` : '—'}
            detail={`${t('nodes.detail.of')} ${memoryTotalLabel}${memoryPercentLabel}`}
            meter={memoryPercent}
            className="border-r"
          />
          <StatCell
            icon={Cpu}
            label={t('nodes.detail.cpuUsage')}
            live={!!liveStats}
            value={cpuPercent !== undefined ? `${cpuPercent.toFixed(1)}%` : '—'}
            detail={liveStats ? t('nodes.detail.coresThreads', { cores: liveStats.cpuCores || 0, threads: liveStats.cpuThreads || 0 }) : '—'}
            meter={cpuPercent}
          />
        </dl>
      </Card>

      <div className="grid grid-cols-1 items-start gap-6 xl:grid-cols-3">
        <div className="space-y-6 xl:col-span-2">
          <SystemInfoCard node={node} />
          <DisksCard disks={node.systemInfo?.disks || []} />
        </div>

        <div className="space-y-6 xl:col-span-1">
          <ServersCard serversOnNode={serversOnNode} />
          <EditNodeCard node={node} />
        </div>
      </div>

      <DeployDialog isDeployDialogOpen={isDeployDialogOpen} setIsDeployDialogOpen={setIsDeployDialogOpen} node={node} />
    </div>
  );
}
