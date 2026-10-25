import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';

/** Small "Beta" tag for an experimental feature an admin has turned on in the panel settings. */
export function BetaBadge({ className }: Readonly<{ className?: string }>) {
    const { t } = useTranslations();
    return (
        <span
            className={cn(
                'inline-flex shrink-0 items-center rounded-full border border-primary/30 bg-primary/10 px-1.5 text-[10px] font-semibold uppercase leading-4 tracking-[0.06em] text-primary-bright',
                className,
            )}
        >
            {t('common.beta')}
        </span>
    );
}
