'use client';
import { useId, useState } from 'react';
import { Box, Check, Download, Info, Loader2, Map as MapIcon, RotateCcw, XCircle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { MAP_NAMES, PLATFORM_NAMES, type MapKind, type WorldMapStatus } from './world-map-types';

const CHOICES: { kind: MapKind; icon: typeof Box }[] = [
  { kind: 'bluemap', icon: Box },
  { kind: 'dynmap', icon: MapIcon },
];

type Props = {
  status: WorldMapStatus;
  busy: boolean;
  onInstall: (kind: MapKind, acceptDownload: boolean) => void;
};

/** Pick BlueMap or Dynmap and install it; BlueMap needs the owner's consent to Mojang's download. */
export default function WorldMapChoice({ status, busy, onInstall }: Props) {
  const { t } = useTranslations();
  const ids = useId();
  const [kind, setKind] = useState<MapKind>('bluemap');
  const [accepted, setAccepted] = useState(false);
  const needsConsent = kind === 'bluemap';
  const canInstall = !busy && (!needsConsent || accepted);
  const platform = status.platform ? PLATFORM_NAMES[status.platform] ?? status.platform : '';

  return (
    <Card className="overflow-hidden">
      <section className="space-y-6 p-5 md:p-6">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex min-w-0 items-start gap-3">
            <span aria-hidden="true" className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-border/60 bg-card text-primary-bright">
              <MapIcon className="h-5 w-5" />
            </span>
            <div className="min-w-0 space-y-1">
              <h2 className="text-sm font-semibold text-foreground">{t('servers.map.title')}</h2>
              <p className="text-pretty text-sm text-muted-foreground">{t('servers.map.description')}</p>
            </div>
          </div>
          {platform && (
            <span className="inline-flex shrink-0 items-center gap-1.5 self-start rounded-md border border-border/60 bg-secondary/60 px-2 py-0.5 text-xs font-medium text-secondary-foreground">
              {platform}
              {status.gameVersion && <span className="font-mono tabular-nums text-muted-foreground">{status.gameVersion}</span>}
            </span>
          )}
        </div>

        <div
          role="radiogroup"
          aria-label={t('servers.map.pick')}
          className="grid gap-3 md:grid-cols-2"
          onKeyDown={(event) => {
            // arrow keys move between the two choices, as in any radio group
            if (!['ArrowRight', 'ArrowDown', 'ArrowLeft', 'ArrowUp'].includes(event.key)) return;
            event.preventDefault();
            const next: MapKind = kind === 'bluemap' ? 'dynmap' : 'bluemap';
            setKind(next);
            document.getElementById(`${ids}-${next}`)?.focus();
          }}
        >
          {CHOICES.map(({ kind: choice, icon: Icon }) => {
            const selected = kind === choice;
            return (
              <button
                key={choice}
                id={`${ids}-${choice}`}
                type="button"
                role="radio"
                aria-checked={selected}
                tabIndex={selected ? 0 : -1}
                disabled={busy}
                onClick={() => setKind(choice)}
                className={cn(
                  'group relative flex min-w-0 flex-col gap-3 rounded-xl border p-4 text-left transition-colors duration-200 ease-panel focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70 focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-60',
                  selected ? 'border-primary/60 bg-primary/[0.06]' : 'border-border/60 bg-card hover:border-primary/30 hover:bg-secondary/40',
                )}
              >
                <span className="flex items-start justify-between gap-3">
                  <span className="flex min-w-0 items-center gap-3">
                    <span
                      aria-hidden="true"
                      className={cn(
                        'flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border transition-colors duration-200',
                        selected ? 'border-primary/40 bg-primary/10 text-primary-bright' : 'border-border/60 bg-secondary/60 text-muted-foreground',
                      )}
                    >
                      <Icon className="h-4 w-4" />
                    </span>
                    <span className="min-w-0">
                      <span className="block truncate text-sm font-semibold text-foreground">{MAP_NAMES[choice]}</span>
                      <span className="block truncate text-xs text-muted-foreground">{t(`servers.map.kinds.${choice}.tagline`)}</span>
                    </span>
                  </span>
                  <span
                    aria-hidden="true"
                    className={cn(
                      'flex h-5 w-5 shrink-0 items-center justify-center rounded-full border transition-colors duration-200',
                      selected ? 'border-primary bg-primary text-primary-foreground' : 'border-border',
                    )}
                  >
                    {selected && <Check className="h-3 w-3 animate-in zoom-in-50 duration-150" />}
                  </span>
                </span>
                <span className="text-pretty text-sm text-muted-foreground">{t(`servers.map.kinds.${choice}.description`)}</span>
                <span className="flex flex-wrap gap-1.5">
                  {(['a', 'b', 'c'] as const).map((point) => (
                    <span key={point} className="rounded-md bg-secondary/60 px-1.5 py-0.5 text-[11px] text-secondary-foreground">
                      {t(`servers.map.kinds.${choice}.points.${point}`)}
                    </span>
                  ))}
                </span>
              </button>
            );
          })}
        </div>

        {needsConsent && (
          <div className="flex items-start gap-3 rounded-xl border border-border/60 bg-secondary/30 p-4 animate-in fade-in-0 slide-in-from-top-1 duration-200 ease-panel">
            <Checkbox id={`${ids}-eula`} checked={accepted} onCheckedChange={(value) => setAccepted(value === true)} disabled={busy} className="mt-0.5" aria-describedby={`${ids}-eula-hint`} />
            <div className="min-w-0 space-y-1">
              <Label htmlFor={`${ids}-eula`} className="cursor-pointer text-sm font-medium leading-snug">
                {t('servers.map.eula')}
              </Label>
              <p id={`${ids}-eula-hint`} className="text-xs text-muted-foreground">
                {t('servers.map.eulaHint')}{' '}
                <a href="https://www.minecraft.net/eula" target="_blank" rel="noopener noreferrer" className="text-primary-bright underline-offset-4 hover:underline focus-visible:underline focus-visible:outline-none">
                  {t('servers.map.eulaLink')}
                </a>
              </p>
            </div>
          </div>
        )}

        {status.lastError && (
          <p role="status" className="flex items-start gap-2 text-sm text-destructive-bright animate-in fade-in-0 duration-300">
            <XCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <span className="min-w-0 break-words">{t('servers.map.failed', { reason: status.lastError })}</span>
          </p>
        )}

        <div className="flex flex-col-reverse gap-3 border-t border-border/60 pt-5 sm:flex-row sm:items-center sm:justify-between">
          <div className="space-y-1.5 text-xs text-muted-foreground">
            <p className="flex items-start gap-2">
              <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              {status.running ? t('servers.map.restartNote') : t('servers.map.offNote')}
            </p>
            <p className="flex items-start gap-2">
              <RotateCcw className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              {t('servers.map.restoreNote')}
            </p>
          </div>
          <Button onClick={() => onInstall(kind, needsConsent && accepted)} disabled={!canInstall} className="w-full shrink-0 sm:w-auto">
            {busy ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Download aria-hidden="true" />}
            {t('servers.map.install', { name: MAP_NAMES[kind] })}
          </Button>
        </div>
      </section>
    </Card>
  );
}
