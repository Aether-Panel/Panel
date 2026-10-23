'use client';
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { ExternalLink, Maximize2, Minimize2, RefreshCw, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { MAP_NAMES, type MapKind } from './world-map-types';

/**
 * Same sandbox the panel sends as Content-Security-Policy: the map's scripts run, but in an
 * opaque origin with no access to the panel's session or storage.
 */
const SANDBOX = 'allow-scripts allow-popups allow-forms allow-pointer-lock allow-downloads';

type Props = {
  kind: MapKind;
  version?: string;
  viewUrl: string;
  onRemove: () => void;
  removing: boolean;
};

function ToolbarButton({ label, onClick, children, href, disabled }: { label: string; onClick?: () => void; children: ReactNode; href?: string; disabled?: boolean }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        {href ? (
          <Button asChild variant="ghost" size="icon" className="h-9 w-9 text-muted-foreground hover:text-foreground">
            <a href={href} target="_blank" rel="noopener noreferrer" aria-label={label}>
              {children}
            </a>
          </Button>
        ) : (
          <Button variant="ghost" size="icon" className="h-9 w-9 text-muted-foreground hover:text-foreground" onClick={onClick} aria-label={label} disabled={disabled}>
            {children}
          </Button>
        )}
      </TooltipTrigger>
      <TooltipContent side="bottom">{label}</TooltipContent>
    </Tooltip>
  );
}

/** The live map in a sandboxed frame, with a slim toolbar above it. */
export default function WorldMapFrame({ kind, version, viewUrl, onRemove, removing }: Props) {
  const { t } = useTranslations();
  const wrapper = useRef<HTMLDivElement>(null);
  // the address is kept until a reload, so the hourly change of the signed link does not
  // reload the map under the player
  const [src, setSrc] = useState(viewUrl);
  const [frameKey, setFrameKey] = useState(0);
  const [loaded, setLoaded] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  const canFullscreen = typeof document !== 'undefined' && !!document.fullscreenEnabled;

  useEffect(() => {
    const onChange = () => setFullscreen(document.fullscreenElement === wrapper.current);
    document.addEventListener('fullscreenchange', onChange);
    return () => document.removeEventListener('fullscreenchange', onChange);
  }, []);

  const reload = useCallback(() => {
    setLoaded(false);
    setSrc(viewUrl);
    setFrameKey((key) => key + 1);
  }, [viewUrl]);

  const toggleFullscreen = useCallback(() => {
    if (document.fullscreenElement) {
      void document.exitFullscreen();
    } else {
      void wrapper.current?.requestFullscreen().catch(() => undefined);
    }
  }, []);

  return (
    <TooltipProvider delayDuration={300}>
      <div
        ref={wrapper}
        className={cn(
          'flex min-w-0 flex-col overflow-hidden border border-border/60 bg-card',
          fullscreen ? 'h-full w-full rounded-none border-0' : 'rounded-xl',
        )}
      >
        <div className="flex min-w-0 items-center justify-between gap-2 border-b border-border/60 px-3 py-1.5">
          <div className="flex min-w-0 items-center gap-2.5">
            <span className="h-2 w-2 shrink-0 rounded-full bg-success shadow-[0_0_0_3px_hsl(var(--success)/0.15)]" aria-hidden="true" />
            <p className="min-w-0 truncate text-sm font-medium text-foreground">
              {MAP_NAMES[kind]}
              {version && <span className="ml-1.5 font-mono text-xs font-normal tabular-nums text-muted-foreground">{version}</span>}
            </p>
            <span className="hidden text-xs text-success sm:inline">{t('servers.map.live')}</span>
          </div>
          <div className="flex shrink-0 items-center">
            <ToolbarButton label={t('servers.map.openTab')} href={src}>
              <ExternalLink aria-hidden="true" />
            </ToolbarButton>
            {canFullscreen && (
              <ToolbarButton label={fullscreen ? t('servers.map.exitFullscreen') : t('servers.map.fullscreen')} onClick={toggleFullscreen}>
                {fullscreen ? <Minimize2 aria-hidden="true" /> : <Maximize2 aria-hidden="true" />}
              </ToolbarButton>
            )}
            <ToolbarButton label={t('servers.map.reload')} onClick={reload}>
              <RefreshCw aria-hidden="true" className={cn(!loaded && 'animate-spin motion-reduce:animate-none')} />
            </ToolbarButton>
            {!fullscreen && (
              <ToolbarButton label={t('servers.map.remove')} onClick={onRemove} disabled={removing}>
                <Trash2 aria-hidden="true" />
              </ToolbarButton>
            )}
          </div>
        </div>

        <div className={cn('relative min-w-0 bg-secondary/30', fullscreen ? 'flex-1' : 'h-[calc(100dvh-17rem)] min-h-[420px]')}>
          {!loaded && (
            <div role="status" aria-live="polite" className="absolute inset-0 flex flex-col gap-3 p-4">
              <span className="sr-only">{t('servers.map.loading')}</span>
              <Skeleton className="h-full w-full rounded-lg" />
            </div>
          )}
          <iframe
            key={frameKey}
            src={src}
            title={t('servers.map.frameTitle', { name: MAP_NAMES[kind] })}
            sandbox={SANDBOX}
            referrerPolicy="no-referrer"
            allow="fullscreen"
            onLoad={() => setLoaded(true)}
            className={cn('absolute inset-0 h-full w-full border-0 transition-opacity duration-300 ease-panel', loaded ? 'opacity-100' : 'opacity-0')}
          />
        </div>
      </div>
    </TooltipProvider>
  );
}
