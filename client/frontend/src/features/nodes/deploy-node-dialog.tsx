import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { CheckCircle2, Copy, Loader2, Terminal } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';

const richText = '[&_code]:rounded [&_code]:bg-secondary [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-xs [&_code]:text-foreground';

/** A node as the node pages have it: its numeric id, name and SFTP port. */
export type DeployableNode = { id: number | string; name: string; sftpPort?: number; isLocal?: boolean };

type JoinCode = { code: string; expiresAt: string; panelUrl: string; command: string };
type Deployment = { clientId: string; clientSecret: string };

export function CopyableCode({ command }: Readonly<{ command: string }>) {
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

/** The docker-compose.yml of a node set up by hand, with its real credentials. */
function manualCompose(node: DeployableNode, deployment: Deployment, origin: string): string {
  const sftp = node.sftpPort || 5657;
  return [
    'services:',
    '  skypanel:',
    '    image: ghcr.io/aether-panel/panel',
    '    container_name: skypanel',
    '    restart: unless-stopped',
    '    ports:',
    '      - "8080:8080"',
    `      - "${sftp}:${sftp}"`,
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
    `      - SKYPANEL_DAEMON_AUTH_CLIENTID=${deployment.clientId}`,
    `      - SKYPANEL_DAEMON_AUTH_CLIENTSECRET=${deployment.clientSecret}`,
    `      - SKYPANEL_DAEMON_SFTP_HOST=0.0.0.0:${sftp}`,
    '    user: "0:0"',
  ].join('\n');
}

/**
 * How to set up a node on another machine. The usual way is one command: the panel makes a
 * join code (once, for an hour) and the installer on the new machine trades it for the node's
 * credentials, installs Docker and starts the daemon. The manual way is still there, with the
 * node's real credentials.
 */
export function DeployNodeDialog({ node, onClose }: Readonly<{ node: DeployableNode | null; onClose: () => void }>) {
  const { t, language } = useTranslations();
  const [join, setJoin] = useState<JoinCode | null>(null);
  const [joinError, setJoinError] = useState('');
  const [generating, setGenerating] = useState(false);
  const [deployment, setDeployment] = useState<Deployment | null>(null);
  const [showManual, setShowManual] = useState(false);
  const [connected, setConnected] = useState(false);

  // a code belongs to the node it was made for: another node starts again
  useEffect(() => {
    setJoin(null);
    setJoinError('');
    setDeployment(null);
    setShowManual(false);
    setConnected(false);
  }, [node?.id]);

  // once there is a command (or the manual steps are open), watch for the node to answer, so
  // the dialog says when it is connected instead of claiming it up front
  const waiting = !!node && (join !== null || showManual) && !connected;
  useEffect(() => {
    if (!node || !waiting) return;
    let alive = true;
    const check = () => {
      api.get(`/api/nodes/${node.id}/system`)
        .then(() => alive && setConnected(true))
        .catch(() => undefined);
    };
    check();
    const timer = setInterval(check, 5000);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [node, waiting]);

  // the manual steps need the node's secret, which is only fetched when they are opened
  useEffect(() => {
    if (!node || !showManual || deployment) return;
    let alive = true;
    api.get<Deployment>(`/api/nodes/${node.id}/deployment`)
      .then((data) => alive && setDeployment(data))
      .catch((e: any) => alive && sileo.error({ title: t('nodes.deployDialog.manualFailed'), description: e?.message }));
    return () => {
      alive = false;
    };
  }, [node, showManual, deployment, t]);

  const generate = async () => {
    if (!node) return;
    setGenerating(true);
    setJoinError('');
    try {
      setJoin(await api.post<JoinCode>(`/api/nodes/${node.id}/join`, {}));
    } catch (e: any) {
      setJoinError(e?.message || t('nodes.toast.unknownError'));
    } finally {
      setGenerating(false);
    }
  };

  const expires = join
    ? new Date(join.expiresAt).toLocaleTimeString(language, { hour: '2-digit', minute: '2-digit' })
    : '';
  const origin = globalThis.location?.origin ?? '';

  let connectionStatus: ReactNode = null;
  if (connected) {
    connectionStatus = (
      <output className="flex items-start gap-2.5 rounded-xl border border-success/25 bg-success/10 px-4 py-3 text-sm text-success">
        <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        <span>{t('nodes.deployDialog.successMessage')}</span>
      </output>
    );
  } else if (waiting) {
    connectionStatus = (
      <output className="flex items-start gap-2.5 rounded-xl border border-border/60 bg-background/40 px-4 py-3 text-sm text-muted-foreground">
        <Loader2 className="mt-0.5 h-4 w-4 shrink-0 animate-spin" aria-hidden="true" />
        <span>{t('nodes.deployDialog.waitingMessage')}</span>
      </output>
    );
  }

  return (
    <Dialog open={!!node} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="flex max-h-[min(90svh,56rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-3xl">
        <DialogHeader className="border-b border-border/60 p-6 pb-5 text-left">
          <DialogTitle className="text-xl font-semibold tracking-[-0.02em]">{t('nodes.deployDialog.title', { name: node?.name || '' })}</DialogTitle>
          <DialogDescription className={richText} dangerouslySetInnerHTML={{ __html: t('nodes.deployDialog.description') }} />
        </DialogHeader>
        <div className="custom-scrollbar min-h-0 flex-1 space-y-6 overflow-y-auto p-6">
          <section className="space-y-3" aria-labelledby="deploy-one-command">
            <div className="space-y-1.5">
              <h3 id="deploy-one-command" className="flex items-center gap-2 text-sm font-semibold">
                <Terminal className="h-4 w-4 text-primary" aria-hidden="true" />
                {t('nodes.deployDialog.oneCommandTitle')}
              </h3>
              <p className={cn('text-pretty text-sm text-muted-foreground', richText)} dangerouslySetInnerHTML={{ __html: t('nodes.deployDialog.oneCommandDescription') }} />
            </div>

            {join ? (
              <div className="space-y-2">
                <CopyableCode command={join.command} />
                <p className="text-pretty text-xs text-muted-foreground">{t('nodes.deployDialog.codeExpires', { time: expires })}</p>
              </div>
            ) : null}

            {joinError ? (
              <p role="alert" className="text-pretty rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive-bright">{joinError}</p>
            ) : null}

            <Button onClick={() => void generate()} disabled={generating} variant={join ? 'outline' : 'default'}>
              {generating && <Loader2 className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />}
              {join ? t('nodes.deployDialog.regenerate') : t('nodes.deployDialog.generate')}
            </Button>

            <p className="text-pretty text-xs text-muted-foreground">{t('nodes.deployDialog.requirements')}</p>
          </section>

          <section className="space-y-4 border-t border-border/60 pt-5">
            <Button variant="ghost" className="-ml-3 h-auto px-3 py-1.5 text-sm font-semibold" onClick={() => setShowManual((open) => !open)} aria-expanded={showManual}>
              {showManual ? t('nodes.deployDialog.hideManual') : t('nodes.deployDialog.showManual')}
            </Button>
            {showManual && (
              <ol className="space-y-6">
                <li className="space-y-1.5">
                  <h4 className="text-sm font-semibold">{t('nodes.deployDialog.step1')}</h4>
                  <p className="text-sm text-muted-foreground">{t('nodes.deployDialog.step1Description')}</p>
                </li>
                <li className="space-y-1.5">
                  <h4 className="text-sm font-semibold">{t('nodes.deployDialog.step2')}</h4>
                  <p className={cn('text-sm text-muted-foreground', richText)} dangerouslySetInnerHTML={{ __html: t('nodes.deployDialog.step2Description') }} />
                  <div className="pt-1.5"><CopyableCode command="mkdir -p /opt/AetherPanel && cd /opt/AetherPanel" /></div>
                </li>
                <li className="space-y-1.5">
                  <h4 className="text-sm font-semibold">{t('nodes.deployDialog.step3')}</h4>
                  <p className={cn('text-sm text-muted-foreground', richText)} dangerouslySetInnerHTML={{ __html: t('nodes.deployDialog.step3Description') }} />
                  <div className="pt-1.5">
                    {node && deployment ? (
                      <CopyableCode command={manualCompose(node, deployment, origin)} />
                    ) : (
                      <p className="flex items-center gap-2 text-sm text-muted-foreground">
                        <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                        {t('nodes.deployDialog.loadingSecret')}
                      </p>
                    )}
                  </div>
                </li>
                <li className="space-y-1.5">
                  <h4 className="text-sm font-semibold">{t('nodes.deployDialog.step4')}</h4>
                  <p className="text-sm text-muted-foreground">{t('nodes.deployDialog.step4Description')}</p>
                  <div className="pt-1.5"><CopyableCode command="docker compose up -d" /></div>
                </li>
              </ol>
            )}
          </section>

          {connectionStatus}
        </div>
        <DialogFooter className="border-t border-border/60 px-6 py-4">
          <Button variant="outline" onClick={onClose}>{t('nodes.deployDialog.close')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
