import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';

/**
 * RAM and disk are typed in MiB, as the containers take them (and as in Pterodactyl), while
 * people think in GiB: 2048 → "2 GiB", 512 → "0,5 GiB", 5000 → "4,88 GiB".
 */
export function formatGib(mib: number, locale: string): string {
  const gib = mib / 1024;
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: 2 }).format(gib)} GiB`;
}

/** The GiB a MiB field holds, under the field as it is typed; nothing for an empty or zero one. */
export function MibEquivalent({ mib, id, className }: Readonly<{ mib: number | ''; id?: string; className?: string }>) {
  const { language } = useTranslations();
  const value = typeof mib === 'number' && Number.isFinite(mib) && mib > 0 ? mib : null;
  return (
    <p id={id} className={cn('min-h-4 text-xs tabular-nums text-muted-foreground', className)}>
      {value !== null ? `= ${formatGib(value, language === 'es' ? 'es-ES' : 'en-US')}` : ' '}
    </p>
  );
}
