'use client';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Check, Copy, Download, ExternalLink, GitBranch, Github, Loader2, Lock, Search, Unlink } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { useTranslations } from '@/contexts/translations-context';
import { api, ApiError } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';

/** GET /api/self/github */
type GitHubStatus = { configured: boolean; connected: boolean; login?: string; avatarUrl?: string };
type Repo = { fullName: string; name: string; owner: string; private: boolean; defaultBranch: string; description: string; updatedAt: string; sizeKb: number };
type DeviceCode = { userCode: string; verificationUri: string; expiresIn: number; interval: number };
/** GET /api/servers/:id/github/install */
type InstallStatus = { installing: boolean; step?: string; repository?: string; branch?: string; downloaded: number; lastError?: string };

type T = (key: string, options?: Record<string, string | number>) => string;

const errorText = (err: unknown, fallback: string) => (err instanceof ApiError && err.message ? err.message : fallback);

function formatSize(bytes: number) {
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${Math.max(0, bytes / 1024).toFixed(0)} KB`;
}

/**
 * Installs one of the user's GitHub repositories into the server: connect the account with a
 * device code, pick a repository and a branch, install. The node downloads it from GitHub, so
 * big projects never go through the browser or SFTP.
 */
export default function GitHubInstallCard({ serverId }: Readonly<{ serverId: string }>) {
  const { t } = useTranslations();
  const [status, setStatus] = useState<GitHubStatus | null>(null);
  const [failed, setFailed] = useState(false);

  const load = useCallback(async () => {
    try {
      setStatus(await api.get<GitHubStatus>('/api/self/github'));
      setFailed(false);
    } catch {
      setFailed(true);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  let body;
  if (failed) body = <p className="text-sm text-muted-foreground">{t('servers.github.loadError')}</p>;
  else if (!status) body = <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" aria-label={t('common.loading')} />;
  else if (!status.configured) body = <p className="text-pretty text-sm text-muted-foreground">{t('servers.github.notConfigured')}</p>;
  else if (!status.connected) body = <ConnectButton t={t} onConnected={load} />;
  else body = <ConnectedView t={t} serverId={serverId} status={status} onUnlinked={load} />;

  return (
    <section className="space-y-4 rounded-xl border border-border/60 bg-background/30 p-4 sm:p-5" aria-labelledby="github-install-title">
      <div className="flex items-start gap-3">
        <span aria-hidden="true" className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-border/60 bg-card text-foreground">
          <Github className="h-4 w-4" />
        </span>
        <div className="min-w-0 space-y-1">
          <h3 id="github-install-title" className="text-sm font-semibold">{t('servers.github.title')}</h3>
          <p className="text-pretty text-sm text-muted-foreground">{t('servers.github.description')}</p>
        </div>
      </div>
      {body}
    </section>
  );
}

/** Starts the device flow and waits until the user entered the code on GitHub. */
function ConnectButton({ t, onConnected }: Readonly<{ t: T; onConnected: () => Promise<void> }>) {
  const [code, setCode] = useState<DeviceCode | null>(null);
  const [starting, setStarting] = useState(false);
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // each flow gets a number; a poll answering for an older flow (the dialog was closed, the page
  // left or the flow restarted) does nothing
  const flow = useRef(0);

  const stop = useCallback(() => {
    flow.current += 1;
    if (timer.current) clearTimeout(timer.current);
    timer.current = null;
  }, []);
  useEffect(() => stop, [stop]);

  const poll = useCallback((interval: number, id: number) => {
    timer.current = setTimeout(async () => {
      try {
        const res = await api.post<{ status: string; login?: string }>('/api/self/github/device/poll', {});
        if (id !== flow.current) return;
        if (res.status === 'connected') {
          setCode(null);
          sileo.success({ title: t('servers.github.connectedToast', { login: res.login ?? '' }) });
          await onConnected();
          return;
        }
        if (res.status === 'expired' || res.status === 'denied') {
          setCode(null);
          sileo.error({ title: t('common.error'), description: t(`servers.github.device.${res.status}`) });
          return;
        }
        poll(interval, id);
      } catch (err) {
        if (id !== flow.current) return;
        setCode(null);
        sileo.error({ title: t('common.error'), description: errorText(err, t('servers.github.device.failed')) });
      }
    }, interval * 1000);
  }, [onConnected, t]);

  const start = async () => {
    setStarting(true);
    try {
      const res = await api.post<DeviceCode>('/api/self/github/device', {});
      stop();
      setCode(res);
      setCopied(false);
      poll(Math.max(res.interval, 5), flow.current);
    } catch (err) {
      sileo.error({ title: t('common.error'), description: errorText(err, t('servers.github.device.failed')) });
    } finally {
      setStarting(false);
    }
  };

  const copy = async () => {
    if (!code) return;
    try {
      await navigator.clipboard.writeText(code.userCode);
      setCopied(true);
    } catch {
      // the code is on screen to type by hand
    }
  };

  return (
    <>
      <Button onClick={() => void start()} disabled={starting}>
        {starting ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Github aria-hidden="true" />}
        {t('servers.github.connect')}
      </Button>
      <Dialog open={code !== null} onOpenChange={(open) => { if (!open) { stop(); setCode(null); } }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader className="text-left">
            <DialogTitle>{t('servers.github.device.title')}</DialogTitle>
            <DialogDescription>{t('servers.github.device.description')}</DialogDescription>
          </DialogHeader>
          {code && (
            <div className="space-y-4">
              <div className="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-background px-4 py-3">
                <span className="select-all font-mono text-2xl font-semibold tracking-[0.2em] text-foreground">{code.userCode}</span>
                <Button variant="ghost" size="icon" onClick={() => void copy()} aria-label={t('servers.github.device.copy')}>
                  {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
                </Button>
              </div>
              <Button asChild className="w-full">
                <a href={code.verificationUri} target="_blank" rel="noopener noreferrer">
                  <ExternalLink aria-hidden="true" />
                  {t('servers.github.device.open')}
                </a>
              </Button>
              <output className="flex items-center gap-2 text-xs text-muted-foreground">
                <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
                {t('servers.github.device.waiting')}
              </output>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}

/** The connected account, its repositories and the install. */
function ConnectedView({ t, serverId, status, onUnlinked }: Readonly<{ t: T; serverId: string; status: GitHubStatus; onUnlinked: () => Promise<void> }>) {
  const [repos, setRepos] = useState<Repo[] | null>(null);
  const [page, setPage] = useState(1);
  const [more, setMore] = useState(false);
  const [loadingRepos, setLoadingRepos] = useState(false);
  const [reposError, setReposError] = useState('');
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<Repo | null>(null);
  const [unlinking, setUnlinking] = useState(false);

  const loadRepos = useCallback(async (next: number) => {
    setLoadingRepos(true);
    setReposError('');
    try {
      const res = await api.get<{ repos: Repo[]; more: boolean }>(`/api/self/github/repos?page=${next}`);
      setRepos((prev) => (next === 1 ? res.repos : [...(prev ?? []), ...res.repos]));
      setMore(res.more);
      setPage(next);
    } catch (err) {
      setReposError(errorText(err, t('servers.github.reposError')));
      // the panel drops a connection GitHub no longer accepts: the status then shows the connect
      // button; any other failure stays here with a retry
      if (err instanceof ApiError && err.status === 422) await onUnlinked();
    } finally {
      setLoadingRepos(false);
    }
  }, [onUnlinked, t]);

  useEffect(() => {
    void loadRepos(1);
  }, [loadRepos]);

  const unlink = async () => {
    setUnlinking(true);
    try {
      await api.delete('/api/self/github');
      sileo.success({ title: t('servers.github.unlinkedToast') });
      await onUnlinked();
    } catch (err) {
      sileo.error({ title: t('common.error'), description: errorText(err, t('servers.github.unlinkError')) });
    } finally {
      setUnlinking(false);
    }
  };

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!repos) return [];
    return q ? repos.filter((r) => r.fullName.toLowerCase().includes(q) || r.description?.toLowerCase().includes(q)) : repos;
  }, [repos, query]);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2.5">
          {status.avatarUrl && <img src={status.avatarUrl} alt="" className="h-7 w-7 shrink-0 rounded-full border border-border/60" />}
          <p className="min-w-0 truncate text-sm">
            <span className="text-muted-foreground">{t('servers.github.connectedAs')} </span>
            <span className="font-medium text-foreground">{status.login}</span>
          </p>
        </div>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="outline" size="sm" disabled={unlinking}>
              {unlinking ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Unlink aria-hidden="true" />}
              {t('servers.github.unlink')}
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t('servers.github.unlinkDialog.title')}</AlertDialogTitle>
              <AlertDialogDescription>{t('servers.github.unlinkDialog.description')}</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
              <AlertDialogAction onClick={() => void unlink()}>{t('servers.github.unlink')}</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>

      <InstallProgress t={t} serverId={serverId} />

      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t('servers.github.search')}
          aria-label={t('servers.github.search')}
          className="pl-9"
        />
      </div>

      {reposError && repos === null ? (
        <div role="alert" className="flex flex-col gap-3 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2.5 text-sm text-destructive-bright sm:flex-row sm:items-center sm:justify-between">
          <span className="min-w-0 [overflow-wrap:anywhere]">{reposError}</span>
          <Button variant="outline" size="sm" onClick={() => void loadRepos(1)} disabled={loadingRepos} className="self-start sm:self-auto">
            {loadingRepos && <Loader2 className="animate-spin" aria-hidden="true" />}
            {t('common.retry')}
          </Button>
        </div>
      ) : (
        <RepoList t={t} repos={repos} shown={shown} selected={selected} onSelect={setSelected} />
      )}
      {reposError && repos !== null && <p role="alert" className="text-xs text-destructive-bright [overflow-wrap:anywhere]">{reposError}</p>}

      {more && (
        <Button variant="outline" size="sm" onClick={() => void loadRepos(page + 1)} disabled={loadingRepos}>
          {loadingRepos && <Loader2 className="animate-spin" aria-hidden="true" />}
          {t('servers.github.loadMore')}
        </Button>
      )}

      {selected && <InstallForm key={selected.fullName} t={t} serverId={serverId} repo={selected} />}
    </div>
  );
}

function RepoList({ t, repos, shown, selected, onSelect }: Readonly<{ t: T; repos: Repo[] | null; shown: Repo[]; selected: Repo | null; onSelect: (repo: Repo) => void }>) {
  if (repos === null) {
    return <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" aria-label={t('common.loading')} />;
  }
  if (shown.length === 0) {
    return <p className="text-sm text-muted-foreground">{repos.length === 0 ? t('servers.github.noRepos') : t('servers.github.noMatch')}</p>;
  }
  return (
    <ul className="custom-scrollbar max-h-80 divide-y divide-border/50 overflow-y-auto rounded-lg border border-border/60" aria-label={t('servers.github.reposLabel')}>
      {shown.map((repo) => {
        const active = selected?.fullName === repo.fullName;
        return (
          <li key={repo.fullName}>
            <button
              type="button"
              onClick={() => onSelect(repo)}
              aria-pressed={active}
              className={cn(
                'flex w-full min-w-0 items-start gap-3 px-3 py-2.5 text-left transition-colors hover:bg-muted/40 focus-visible:bg-muted/40 focus-visible:outline-none',
                active && 'bg-primary/10 hover:bg-primary/10',
              )}
            >
              <div className="min-w-0 flex-1 space-y-0.5">
                <p className="flex min-w-0 items-center gap-1.5 text-sm font-medium text-foreground">
                  <span className="truncate">{repo.fullName}</span>
                  {repo.private && (
                    <span className="inline-flex shrink-0 items-center gap-1 rounded-full border border-border/60 px-1.5 py-px text-[10px] font-medium text-muted-foreground">
                      <Lock className="h-2.5 w-2.5" aria-hidden="true" />
                      {t('servers.github.private')}
                    </span>
                  )}
                </p>
                {repo.description && <p className="line-clamp-1 text-xs text-muted-foreground [overflow-wrap:anywhere]">{repo.description}</p>}
              </div>
              <span className="mt-0.5 inline-flex shrink-0 items-center gap-1 font-mono text-[11px] text-muted-foreground">
                <GitBranch className="h-3 w-3" aria-hidden="true" />
                <span className="max-w-[8rem] truncate">{repo.defaultBranch}</span>
              </span>
            </button>
          </li>
        );
      })}
    </ul>
  );
}

/** Branch picker and install button for the chosen repository. */
function InstallForm({ t, serverId, repo }: Readonly<{ t: T; serverId: string; repo: Repo }>) {
  const [branches, setBranches] = useState<string[]>([repo.defaultBranch]);
  const [branch, setBranch] = useState(repo.defaultBranch);
  const [installing, setInstalling] = useState(false);

  useEffect(() => {
    let alive = true;
    api.get<{ defaultBranch: string; branches: string[] }>(`/api/self/github/repos/${encodeURIComponent(repo.owner)}/${encodeURIComponent(repo.name)}/branches`)
      .then((res) => {
        if (!alive) return;
        const list = res.branches.includes(res.defaultBranch) ? res.branches : [res.defaultBranch, ...res.branches];
        setBranches(list);
      })
      // the default branch alone can still be installed
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [repo.owner, repo.name]);

  const install = async () => {
    setInstalling(true);
    try {
      // the default branch is sent as empty, so the panel uses whatever GitHub says it is now
      await api.post(`/api/servers/${serverId}/github/install`, { repository: repo.fullName, branch: branch === repo.defaultBranch ? '' : branch });
      sileo.success({ title: t('servers.github.installStarted', { repo: repo.fullName }) });
      globalThis.dispatchEvent(new CustomEvent(INSTALL_EVENT, { detail: serverId }));
    } catch (err) {
      sileo.error({ title: t('common.error'), description: errorText(err, t('servers.github.installError')) });
    } finally {
      setInstalling(false);
    }
  };

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border/60 bg-card/40 p-3 sm:flex-row sm:items-end">
      <div className="min-w-0 flex-1 space-y-1.5">
        <p className="truncate text-sm font-medium">{repo.fullName}</p>
        <Select value={branch} onValueChange={setBranch}>
          <SelectTrigger aria-label={t('servers.github.branch')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {branches.map((name) => (
              <SelectItem key={name} value={name}>
                <span className="font-mono text-xs">{name}</span>
                {name === repo.defaultBranch && <span className="ml-2 text-xs text-muted-foreground">{t('servers.github.defaultBranch')}</span>}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <AlertDialog>
        <AlertDialogTrigger asChild>
          <Button disabled={installing} className="sm:shrink-0">
            {installing ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Download aria-hidden="true" />}
            {t('servers.github.install')}
          </Button>
        </AlertDialogTrigger>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('servers.github.installDialog.title', { repo: repo.fullName, branch })}</AlertDialogTitle>
            <AlertDialogDescription>{t('servers.github.installDialog.description')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction className="bg-destructive text-destructive-foreground hover:bg-destructive/90" onClick={() => void install()}>
              {t('servers.github.installDialog.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

/** Sent when an install starts, so the progress line begins polling at once. */
const INSTALL_EVENT = 'aether:github-install';

/** The install that is running on the node, and how the last one ended. */
function InstallProgress({ t, serverId }: Readonly<{ t: T; serverId: string }>) {
  const [job, setJob] = useState<InstallStatus | null>(null);
  const wasInstalling = useRef(false);

  useEffect(() => {
    let alive = true;
    let timer: ReturnType<typeof setTimeout> | null = null;
    const check = async () => {
      try {
        const res = await api.get<InstallStatus>(`/api/servers/${serverId}/github/install`);
        if (!alive) return;
        setJob(res);
        if (wasInstalling.current && !res.installing) {
          if (res.step === 'done') sileo.success({ title: t('servers.github.installDone', { repo: res.repository ?? '' }) });
          else if (res.lastError) sileo.error({ title: t('servers.github.installFailed'), description: res.lastError });
        }
        wasInstalling.current = res.installing;
        if (res.installing) timer = setTimeout(() => void check(), 1500);
      } catch {
        // no permission or an older node: no progress line
      }
    };
    void check();
    const onStart = (event: Event) => {
      if ((event as CustomEvent<string>).detail !== serverId) return;
      wasInstalling.current = true;
      if (timer) clearTimeout(timer);
      void check();
    };
    globalThis.addEventListener(INSTALL_EVENT, onStart);
    return () => {
      alive = false;
      if (timer) clearTimeout(timer);
      globalThis.removeEventListener(INSTALL_EVENT, onStart);
    };
  }, [serverId, t]);

  if (!job?.step || (!job.installing && job.step !== 'failed')) return null;
  const failed = !job.installing && job.step === 'failed';
  const content = (
    <>
      {!failed && <Loader2 className="mt-0.5 h-4 w-4 shrink-0 animate-spin" aria-hidden="true" />}
      <div className="min-w-0 space-y-0.5">
        <p className="font-medium [overflow-wrap:anywhere]">
          {failed ? t('servers.github.installFailed') : t(`servers.github.steps.${job.step}`, { size: formatSize(job.downloaded) })}
        </p>
        <p className="text-xs text-muted-foreground [overflow-wrap:anywhere]">
          {failed ? job.lastError : `${job.repository} · ${job.branch}`}
        </p>
      </div>
    </>
  );
  const frame = 'flex items-start gap-2 rounded-lg border px-3 py-2.5 text-sm';
  if (failed) {
    return <div role="alert" className={cn(frame, 'border-destructive/40 bg-destructive/10 text-destructive-bright')}>{content}</div>;
  }
  return <output className={cn(frame, 'border-border/60 bg-card/40 text-foreground')}>{content}</output>;
}
