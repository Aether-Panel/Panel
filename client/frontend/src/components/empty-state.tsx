import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { cn } from '@/lib/utils';

type EmptyStateProps = {
  icon: LucideIcon;
  title: ReactNode;
  description?: ReactNode;
  /** Buttons or links under the text. */
  action?: ReactNode;
  /** "error" tints the icon for failed loads; the box itself stays calm. */
  tone?: 'default' | 'error';
  /** "sm" for boxes inside cards and dialogs, "md" for a page or a tab. */
  size?: 'sm' | 'md';
  /** Heading level for the title, so it fits the page outline. */
  as?: 'h1' | 'h2' | 'h3' | 'h4' | 'p';
  className?: string;
};

/**
 * The panel's one empty / no-results / failed-to-load box: a dashed frame, an icon on a
 * soft halo, a title, a line of help and an optional action. It fades in once, so a list
 * that turns out to be empty does not snap into place.
 */
export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  tone = 'default',
  size = 'md',
  as: Title = 'h3',
  className,
}: Readonly<EmptyStateProps>) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center rounded-xl border border-dashed border-border/60 text-center animate-in fade-in-0 zoom-in-[0.98] duration-300 ease-panel',
        size === 'md' ? 'px-6 py-12' : 'px-4 py-8',
        className,
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          'relative mb-4 flex items-center justify-center rounded-xl border bg-card/80',
          size === 'md' ? 'h-12 w-12' : 'h-10 w-10',
          tone === 'error'
            ? 'border-destructive/30 text-destructive-bright shadow-[0_0_24px_-6px_hsl(var(--destructive)/0.45)]'
            : 'border-border/60 text-muted-foreground shadow-[0_0_24px_-8px_hsl(var(--glow)/0.35)]',
        )}
      >
        <Icon className={size === 'md' ? 'h-5 w-5' : 'h-4 w-4'} />
      </span>
      <Title className={cn('font-semibold text-foreground', size === 'md' ? 'text-base' : 'text-sm')}>{title}</Title>
      {description && (
        <p className={cn('mt-1 max-w-[46ch] text-muted-foreground', size === 'md' ? 'text-sm' : 'text-xs')}>{description}</p>
      )}
      {action && <div className="mt-5 flex flex-wrap items-center justify-center gap-2">{action}</div>}
    </div>
  );
}
