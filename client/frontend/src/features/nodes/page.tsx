'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useState } from 'react';
import type { Dispatch, ReactNode, SetStateAction } from 'react';
import { PageHeader } from '@/components/page-header';
import { Button } from '@/components/ui/button';
import { AlertTriangle, CheckCircle2, Copy, MoreHorizontal, Network, Plus, Loader2, Trash2 } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Skeleton } from '@/components/ui/skeleton';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger, DropdownMenuSeparator } from '@/components/ui/dropdown-menu';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import { sileo } from "@/lib/toast";
import { api } from '@/lib/api-client';
import { cn } from '@/lib/utils';

import { useTranslations } from '@/contexts/translations-context';
import { useNodes } from '@/hooks/use-dashboard-data';
import { useServers } from '@/hooks/use-servers';
import { EmptyState } from '@/components/empty-state';

type Field<T> = [T, Dispatch<SetStateAction<T>>];

/** The connection fields shared by the create and edit dialogs. */
function NodeFields({
  idPrefix,
  name,
  publicHost,
  publicPort,
  sftpPort,
  useDifferentHost,
  privateHost,
  privatePort,
  markRequired,
  onSubmit,
}: Readonly<{
  idPrefix: string;
  name: Field<string>;
  publicHost: Field<string>;
  publicPort: Field<string>;
  sftpPort: Field<string>;
  useDifferentHost: Field<boolean>;
  privateHost: Field<string>;
  privatePort: Field<string>;
  markRequired?: boolean;
  onSubmit?: () => void;
}>) {
  const { t } = useTranslations();
  const required = markRequired ? <span className="text-destructive-bright" aria-hidden="true">*</span> : null;

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor={`${idPrefix}-node-name`}>{t('nodes.addDialog.nameLabel')} {required}</Label>
          <Input
            id={`${idPrefix}-node-name`}
            placeholder={markRequired ? t('nodes.addDialog.namePlaceholder') : undefined}
            value={name[0]}
            onChange={(e) => name[1](e.target.value)}
            onKeyDown={onSubmit ? (e) => e.key === 'Enter' && onSubmit() : undefined}
            aria-required={markRequired || undefined}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor={`${idPrefix}-public-host`}>{t('nodes.addDialog.publicHostLabel')} {required}</Label>
          <Input
            id={`${idPrefix}-public-host`}
            placeholder={markRequired ? t('nodes.addDialog.publicHostPlaceholder') : undefined}
            value={publicHost[0]}
            onChange={(e) => publicHost[1](e.target.value)}
            className="font-mono"
            aria-required={markRequired || undefined}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor={`${idPrefix}-public-port`}>{t('nodes.addDialog.publicPortLabel')}</Label>
          <Input
            id={`${idPrefix}-public-port`}
            type="number"
            value={publicPort[0]}
            onChange={(e) => publicPort[1](e.target.value)}
            className="font-mono"
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor={`${idPrefix}-sftp-port`}>{t('nodes.addDialog.sftpPortLabel')}</Label>
          <Input
            id={`${idPrefix}-sftp-port`}
            type="number"
            value={sftpPort[0]}
            onChange={(e) => sftpPort[1](e.target.value)}
            className="font-mono"
          />
        </div>
      </div>

      <div className="rounded-xl border border-border/60 bg-background/30">
        <div className="flex items-start gap-3 p-4">
          <Checkbox
            id={`${idPrefix}-use-different-host`}
            className="mt-0.5"
            checked={useDifferentHost[0]}
            onCheckedChange={(checked) => useDifferentHost[1](Boolean(checked))}
          />
          <div className="grid gap-1">
            <label htmlFor={`${idPrefix}-use-different-host`} className="cursor-pointer text-sm font-medium leading-snug">
              {t('nodes.addDialog.useDifferentHostLabel')}
            </label>
            <p className="text-sm text-muted-foreground">{t('nodes.addDialog.useDifferentHostDescription')}</p>
          </div>
        </div>
        {useDifferentHost[0] && (
          <div className="grid grid-cols-1 gap-4 border-t border-border/50 p-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor={`${idPrefix}-private-host`}>{t('nodes.addDialog.privateHostLabel')}</Label>
              <Input
                id={`${idPrefix}-private-host`}
                placeholder={t('nodes.addDialog.privateHostPlaceholder')}
                value={privateHost[0]}
                onChange={(e) => privateHost[1](e.target.value)}
                className="font-mono"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor={`${idPrefix}-private-port`}>{t('nodes.addDialog.privatePortLabel')}</Label>
              <Input
                id={`${idPrefix}-private-port`}
                type="number"
                value={privatePort[0]}
                onChange={(e) => privatePort[1](e.target.value)}
                className="font-mono"
              />
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

const STATUS_TONE: Record<string, string> = {
  online: 'bg-success/10 text-success',
  offline: 'bg-destructive/10 text-destructive-bright',
  deploying: 'bg-warning/10 text-warning',
};

function NodeStatusBadge({ status }: Readonly<{ status: string }>) {
  const { t } = useTranslations();
  return (
    <span
      className={cn(
        'inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-md px-2 py-0.5 text-xs font-medium',
        STATUS_TONE[status] || 'bg-secondary text-muted-foreground',
      )}
    >
      <span className="h-1.5 w-1.5 rounded-full bg-current" aria-hidden="true" />
      <span className="first-letter:uppercase">{t(`dashboard.status.${status}`)}</span>
    </span>
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

const richText = '[&_code]:rounded [&_code]:bg-secondary [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-xs [&_code]:text-foreground';

/** The part of a node id the daemon's OAuth client is named after ("node-3" → "3"). */
function nodeClientSuffix(id: unknown) {
  const raw = String(id);
  return raw.includes('-') ? raw.split('-')[1] : id;
}

function NodesSkeleton() {
  return (
    <div className="flex flex-col gap-8" aria-busy="true">
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div className="space-y-2">
          <Skeleton className="h-9 w-64" />
          <Skeleton className="h-4 w-80 max-w-full" />
        </div>
        <Skeleton className="h-10 w-36 rounded-lg" />
      </div>
      <Card className="overflow-hidden">
        <div className="space-y-2 border-b border-border/50 p-5 md:p-6">
          <Skeleton className="h-5 w-40" />
          <Skeleton className="h-4 w-56 max-w-full" />
        </div>
        <div className="space-y-2 p-5 md:p-6">
          {Array.from({ length: 3 }, (_, i) => <Skeleton key={i} className="h-14 rounded-xl" />)}
        </div>
      </Card>
    </div>
  );
}

export default function NodesPage() {
  const { role, hasScope } = useAuth();
  const { t } = useTranslations();
  const { nodes: realNodes, loading: nodesLoading, error, refresh } = useNodes();
  const { servers: allServers, loading: serversLoading } = useServers();
  const [isMounted, setIsMounted] = useState(false);


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
  const [editingNode, setEditingNode] = useState<any>(null);
  const [editName, setEditName] = useState('');
  const [editPublicHost, setEditPublicHost] = useState('');
  const [editPublicPort, setEditPublicPort] = useState('8080');
  const [editSftpPort, setEditSftpPort] = useState('5657');
  const [editUseDifferentHost, setEditUseDifferentHost] = useState(false);
  const [editPrivateHost, setEditPrivateHost] = useState('');
  const [editPrivatePort, setEditPrivatePort] = useState('8080');
  const [isSaving, setIsSaving] = useState(false);

  // ── Deploy state ──────────────────────────────────────────
  const [deployingNode, setDeployingNode] = useState<any>(null);

  // ── Delete state ──────────────────────────────────────────
  const [deletingNode, setDeletingNode] = useState<any>(null);
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

  // Ensure Radix Dialog/DropdownMenu cleanup finished so the UI is not left blocked
  useEffect(() => {
    const anyOpen = isCreateOpen || !!editingNode || !!deletingNode || !!deployingNode;
    if (!anyOpen) {
      const timer = setTimeout(() => {
        document.body.style.pointerEvents = 'auto';
        document.body.style.overflow = 'auto';
      }, 100);
      return () => clearTimeout(timer);
    }
  }, [isCreateOpen, editingNode, deletingNode, deployingNode]);

  // ── Handlers ──────────────────────────────────────────────

  const handleCreateNode = async () => {
    if (!createName.trim()) {
      sileo.error({ title: t('common.error'), description: t('nodes.toast.nameRequired') });
      return;
    }
    if (!createPublicHost.trim()) {
      sileo.error({ title: t('common.error'), description: t('nodes.toast.publicHostRequired') });
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
      sileo.success({ title: t('common.success'), description: t('nodes.toast.created', { name: createName }) });
      // reset form
      setCreateName(''); setCreatePublicHost(''); setCreatePublicPort('8080');
      setCreateSftpPort('5657'); setCreateUseDifferentHost(false);
      setCreatePrivateHost(''); setCreatePrivatePort('8080');
      setIsCreateOpen(false);
      void refresh();
    } catch (e: any) {
      sileo.error({ title: t('nodes.toast.createFailed'), description: e?.message || t('nodes.toast.unknownError') });
    } finally {
      setIsCreating(false);
    }
  };

  const handleUpdateNode = async () => {
    if (!editingNode) return;
    if (!editName.trim()) {
      sileo.error({ title: t('common.error'), description: t('nodes.toast.nameRequired') });
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
      sileo.success({ title: t('common.success'), description: t('nodes.toast.updated', { name: editName }) });
      setEditingNode(null);
      void refresh();
    } catch (e: any) {
      sileo.error({ title: t('nodes.toast.updateFailed'), description: e?.message || t('nodes.toast.unknownError') });
    } finally {
      setIsSaving(false);
    }
  };

  const handleDeleteNode = async () => {
    if (!deletingNode) return;
    setIsDeleting(true);
    try {
      await api.delete(`/api/nodes/${deletingNode.id}`);
      sileo.success({ title: t('common.success'), description: t('nodes.toast.deletedDescription', { name: deletingNode.name }) });
      setDeletingNode(null);
      void refresh();
    } catch (e: any) {
      sileo.error({ title: t('nodes.toast.deleteFailed'), description: e?.message || t('nodes.toast.unknownError') });
    } finally {
      setIsDeleting(false);
    }
  };

  const origin = typeof window !== 'undefined' ? window.location.origin : 'https://192.168.0.12:8080';
  const clientId = deployingNode ? `.node_${nodeClientSuffix(deployingNode.id)}` : '';
  const deployCompose = deployingNode ? [
    'services:',
    '  skypanel:',
    '    image: ghcr.io/aether-panel/panel',
    '    container_name: skypanel',
    '    restart: unless-stopped',
    '    ports:',
    '      - "8080:8080"',
    `      - "${deployingNode.sftpPort}:${deployingNode.sftpPort}"`,
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
    `      - SKYPANEL_DAEMON_SFTP_HOST=0.0.0.0:${deployingNode.sftpPort}`,
    '    user: "0:0"'
  ].join('\n') : '';

  if (!isMounted || !hasScope('nodes.view') || nodesLoading || serversLoading) {
    return <NodesSkeleton />;
  }

  const nodes = realNodes.map((n: any) => ({
    ...n,
    status: n.systemInfo ? 'online' : 'offline',
    location: n.isLocal ? 'Local' : (n.publicHost || t('nodes.unknownLocation'))
  }));

  const serverCount = (node: any) => allServers.filter((s: any) => String(s.nodeId) === String(node.id)).length;

  const renderActions = (node: any) => (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="h-9 w-9 text-muted-foreground hover:text-foreground"
          aria-label={t('nodes.actions.openMenu', { name: node.name })}
        >
          <MoreHorizontal aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>{t('servers.actions.menuLabel')}</DropdownMenuLabel>
        <DropdownMenuItem onClick={() => window.location.href = `/nodes/view/?id=${node.id}`}>
          {t('nodes.actions.viewDetails')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTimeout(() => setEditingNode(node))}>{t('nodes.actions.edit')}</DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTimeout(() => setDeployingNode(node), 50)}>{t('nodes.actions.deploy')}</DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="text-destructive-bright focus:bg-destructive/10 focus:text-destructive-bright"
          onClick={() => setTimeout(() => setDeletingNode(node), 50)}
        >
          <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
          {t('nodes.actions.delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );

  let nodeList: ReactNode;
  if (error && nodes.length === 0) {
    nodeList = (
      <EmptyState
        icon={AlertTriangle}
        tone="error"
        title={t('nodes.error.title')}
        description={t('nodes.error.description')}
        action={
          <Button variant="outline" onClick={() => void refresh()}>
            {t('common.retry')}
          </Button>
        }
      />
    );
  } else if (nodes.length === 0) {
    nodeList = (
      <EmptyState
        icon={Network}
        title={t('nodes.empty.title')}
        description={t('nodes.empty.description')}
        action={
          <Button onClick={() => setIsCreateOpen(true)}>
            <Plus aria-hidden="true" />
            {t('nodes.addNode')}
          </Button>
        }
      />
    );
  } else {
    nodeList = (
      <>
        {/* Phones and tablets (the sidebar leaves too little room for the table): one stacked row per node. */}
        <ul className="divide-y divide-border/50 overflow-hidden rounded-xl border border-border/60 bg-background/30 lg:hidden">
          {nodes.map((node) => (
            <li key={node.id} className="flex items-center gap-2 py-2 pl-4 pr-2">
              <div className="min-w-0 flex-1 py-1">
                <div className="flex min-w-0 items-center justify-between gap-2">
                  <a
                    href={`/nodes/view/?id=${node.id}`}
                    className="min-w-0 truncate rounded-sm text-sm font-semibold text-foreground hover:underline focus-ring"
                  >
                    {node.name}
                  </a>
                  <NodeStatusBadge status={node.status} />
                </div>
                <p className="mt-1 flex flex-wrap items-baseline gap-x-1.5 text-xs text-muted-foreground">
                  <span className={cn('min-w-0 max-w-full truncate', !node.isLocal && 'font-mono')}>{node.location}</span>
                  <span aria-hidden="true">·</span>
                  <span className="whitespace-nowrap tabular-nums">{t('nodes.serversCount', { count: serverCount(node) })}</span>
                </p>
              </div>
              {renderActions(node)}
            </li>
          ))}
        </ul>

        {/* Desktop: the full table. */}
        <div className="hidden overflow-hidden rounded-xl border border-border/60 bg-background/30 lg:block">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead>{t('nodes.table.name')}</TableHead>
                <TableHead>{t('nodes.table.location')}</TableHead>
                <TableHead>{t('dashboard.table.status')}</TableHead>
                <TableHead className="text-right">{t('nodes.table.servers')}</TableHead>
                <TableHead className="w-14"><span className="sr-only">{t('dashboard.table.actions')}</span></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {nodes.map((node) => (
                <TableRow key={node.id} className="hover:bg-accent/30">
                  <TableCell className="max-w-[16rem] font-medium">
                    <a href={`/nodes/view/?id=${node.id}`} className="block truncate rounded-sm hover:underline focus-ring">
                      {node.name}
                    </a>
                  </TableCell>
                  <TableCell className={cn('max-w-[16rem] truncate text-muted-foreground', !node.isLocal && 'font-mono text-xs')}>
                    {node.location}
                  </TableCell>
                  <TableCell>
                    <NodeStatusBadge status={node.status} />
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {serverCount(node)}
                  </TableCell>
                  <TableCell className="py-2 text-right">
                    {renderActions(node)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </>
    );
  }

  return (
    <div className="stagger flex flex-col gap-8">
      <PageHeader title={t('nodes.title')} description={t('nodes.description')}>
        {/* ── Create Node Dialog ── */}
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus aria-hidden="true" />
              {t('nodes.addNode')}
            </Button>
          </DialogTrigger>
          <DialogContent className="flex max-h-[min(90svh,52rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
            <DialogHeader className="border-b border-border/60 p-6 pb-5 text-left">
              <DialogTitle className="text-xl font-semibold tracking-[-0.02em]">{t('nodes.addDialog.title')}</DialogTitle>
              <DialogDescription>{t('nodes.addDialog.description')}</DialogDescription>
            </DialogHeader>
            <div className="custom-scrollbar min-h-0 flex-1 overflow-y-auto p-6">
              <NodeFields
                idPrefix="create"
                markRequired
                onSubmit={handleCreateNode}
                name={[createName, setCreateName]}
                publicHost={[createPublicHost, setCreatePublicHost]}
                publicPort={[createPublicPort, setCreatePublicPort]}
                sftpPort={[createSftpPort, setCreateSftpPort]}
                useDifferentHost={[createUseDifferentHost, setCreateUseDifferentHost]}
                privateHost={[createPrivateHost, setCreatePrivateHost]}
                privatePort={[createPrivatePort, setCreatePrivatePort]}
              />
            </div>
            <DialogFooter className="gap-2 border-t border-border/60 px-6 py-4 sm:gap-0">
              <Button variant="outline" onClick={() => setIsCreateOpen(false)} disabled={isCreating}>
                {t('nodes.editDialog.cancel')}
              </Button>
              <Button onClick={handleCreateNode} disabled={isCreating}>
                {isCreating && <Loader2 className="animate-spin" aria-hidden="true" />}
                {t('nodes.addDialog.createButton')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </PageHeader>

      <Card className="overflow-hidden">
        <div className="flex flex-col gap-1 border-b border-border/50 p-5 md:p-6">
          <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('nodes.allNodes')}</h2>
          <p className="text-sm text-muted-foreground">{t('nodes.listDescription', { count: nodes.length })}</p>
        </div>

        <div className="p-5 md:p-6">
          {nodeList}
        </div>
      </Card>

      {/* ── Edit Node Dialog ── */}
      <Dialog open={!!editingNode} onOpenChange={(isOpen) => !isOpen && setEditingNode(null)}>
        <DialogContent className="flex max-h-[min(90svh,52rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
          <DialogHeader className="border-b border-border/60 p-6 pb-5 text-left">
            <DialogTitle className="text-xl font-semibold tracking-[-0.02em]">{t('nodes.editDialog.title')}</DialogTitle>
            <DialogDescription>{t('nodes.editDialog.description')}</DialogDescription>
          </DialogHeader>
          <div className="custom-scrollbar min-h-0 flex-1 overflow-y-auto p-6">
            <NodeFields
              idPrefix="edit"
              name={[editName, setEditName]}
              publicHost={[editPublicHost, setEditPublicHost]}
              publicPort={[editPublicPort, setEditPublicPort]}
              sftpPort={[editSftpPort, setEditSftpPort]}
              useDifferentHost={[editUseDifferentHost, setEditUseDifferentHost]}
              privateHost={[editPrivateHost, setEditPrivateHost]}
              privatePort={[editPrivatePort, setEditPrivatePort]}
            />
          </div>
          <DialogFooter className="gap-2 border-t border-border/60 px-6 py-4 sm:gap-0">
            <Button variant="outline" onClick={() => setEditingNode(null)} disabled={isSaving}>{t('nodes.editDialog.cancel')}</Button>
            <Button type="submit" onClick={handleUpdateNode} disabled={isSaving}>
              {isSaving && <Loader2 className="animate-spin" aria-hidden="true" />}
              {t('nodes.editDialog.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── Delete Confirmation Dialog ── */}
      <Dialog open={!!deletingNode} onOpenChange={(isOpen) => !isOpen && setDeletingNode(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader className="text-left">
            <DialogTitle>{t('nodes.deleteDialog.title')}</DialogTitle>
            <DialogDescription>
              {t('nodes.deleteDialog.description', { name: deletingNode?.name || '' })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="mt-2 gap-2 sm:gap-0">
            <Button variant="outline" onClick={() => setDeletingNode(null)} disabled={isDeleting}>{t('common.cancel')}</Button>
            <Button className="bg-destructive text-destructive-foreground hover:bg-destructive/90" variant="destructive" onClick={handleDeleteNode} disabled={isDeleting}>
              {isDeleting && <Loader2 className="animate-spin" aria-hidden="true" />}
              {t('nodes.deleteDialog.confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── Deploy Node Dialog ── */}
      <Dialog open={!!deployingNode} onOpenChange={(isOpen) => !isOpen && setDeployingNode(null)}>
        <DialogContent className="flex max-h-[min(90svh,56rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-3xl">
          <DialogHeader className="border-b border-border/60 p-6 pb-5 text-left">
            <DialogTitle className="text-xl font-semibold tracking-[-0.02em]">{t('nodes.deployDialog.title', { name: deployingNode?.name || '' })}</DialogTitle>
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
            <Button variant="outline" onClick={() => setDeployingNode(null)}>{t('nodes.deployDialog.close')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
