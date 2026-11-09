import { Loader2, Upload, X } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { useTranslations } from '@/contexts/translations-context';
import type { UploadProgress } from './file-upload';
import { localeFor } from './file-history-sheet';

/** 1536 → "1,5 KB", in the panel's language */
export function formatBytes(bytes: number, locale: string): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = Math.max(0, bytes);
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  const digits = unit === 0 || value >= 100 ? 0 : 1;
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: digits, minimumFractionDigits: digits }).format(value)} ${units[unit]}`;
}

/** 75 → "1 min 15 s"; the units read the same in Spanish and English */
export function formatDuration(seconds: number): string {
  const s = Math.max(1, Math.round(seconds));
  if (s < 60) return `${s} s`;
  const m = Math.floor(s / 60);
  if (m < 60) return s % 60 ? `${m} min ${s % 60} s` : `${m} min`;
  const h = Math.floor(m / 60);
  return m % 60 ? `${h} h ${m % 60} min` : `${h} h`;
}

type Props = Readonly<{
  progress: UploadProgress;
  /** bytes per second, null until it can be told */
  rate: number | null;
  cancelling: boolean;
  onCancel: () => void;
}>;

/**
 * An upload in progress: how far it is by bytes (a big file and a small one are not the same
 * step), the item going up, the speed and the time left, and a way to stop it.
 */
export function UploadProgressPanel({ progress, rate, cancelling, onCancel }: Props) {
  const { t, language } = useTranslations();
  const locale = localeFor(language);
  const { done, total, sentBytes, totalBytes, current } = progress;

  // only empty folders: no bytes to count, so the items are the measure
  const fraction = totalBytes > 0 ? sentBytes / totalBytes : done / Math.max(total, 1);
  const percent = Math.min(100, Math.floor(fraction * 100));
  const step = Math.min(done + 1, total);
  const left = rate && rate > 0 ? (totalBytes - sentBytes) / rate : null;

  const details = [
    totalBytes > 0 ? t('servers.fileManager.uploadPanel.bytes', { sent: formatBytes(sentBytes, locale), total: formatBytes(totalBytes, locale) }) : null,
    rate ? t('servers.fileManager.uploadPanel.speed', { speed: formatBytes(rate, locale) }) : null,
    left !== null && left >= 1 ? t('servers.fileManager.uploadPanel.left', { time: formatDuration(left) }) : null,
  ].filter(Boolean);

  return (
    <section
      aria-label={t('servers.fileManager.uploadPanel.label')}
      className="space-y-2.5 rounded-xl border border-primary/25 bg-primary/[0.04] p-3 animate-in fade-in-0 slide-in-from-top-1 duration-200 ease-panel sm:p-4"
    >
      <div className="flex items-start gap-3">
        <span className="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-primary/25 bg-background/60">
          {cancelling
            ? <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" aria-hidden="true" />
            : <Upload className="h-4 w-4 text-primary-bright" aria-hidden="true" />}
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline justify-between gap-3">
            <p className="truncate text-sm font-medium">
              {cancelling
                ? t('servers.fileManager.uploadPanel.cancelling')
                : t('servers.fileManager.uploadPanel.title', { step, total })}
            </p>
            <span className="shrink-0 text-sm font-semibold tabular-nums">{percent}%</span>
          </div>
          <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground" title={current?.path}>
            {current?.path ?? ' '}
          </p>
        </div>
        <Button
          variant="ghost"
          size="sm"
          className="-mr-1 h-8 shrink-0 px-2 text-xs text-muted-foreground hover:text-foreground"
          onClick={onCancel}
          disabled={cancelling}
        >
          <X aria-hidden="true" />
          <span className="hidden sm:inline">{t('common.cancel')}</span>
          <span className="sr-only sm:hidden">{t('common.cancel')}</span>
        </Button>
      </div>

      <div
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={percent}
        aria-label={t('servers.fileManager.uploadPanel.label')}
        className="h-1.5 overflow-hidden rounded-full bg-secondary"
      >
        <div
          className="h-full rounded-full bg-primary transition-[width] duration-300 ease-panel"
          style={{ width: `${percent}%` }}
        />
      </div>

      {details.length > 0 && (
        <p className="flex flex-wrap gap-x-2 text-xs tabular-nums text-muted-foreground">
          {details.map((text, i) => (
            <span key={i} className="whitespace-nowrap">
              {i > 0 && <span aria-hidden="true" className="mr-2">·</span>}
              {text}
            </span>
          ))}
        </p>
      )}
    </section>
  );
}
