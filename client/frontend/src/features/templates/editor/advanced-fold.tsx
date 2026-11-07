import { useLayoutEffect, type ReactNode } from 'react';
import { ChevronRight } from '@/components/icons';
import { cn } from '@/lib/utils';
import { useFocusRequest } from './editor-context';
import type { Path } from './paths';
import { useLatchedOpen } from './steps/use-latched-open';

/**
 * "Opciones avanzadas": settings few templates need, folded under a button that names them. It
 * opens by itself when one of them holds a value or an issue and then stays open until closed
 * (see useLatchedOpen), and goTo() on one of its fields opens it even after the user closed it.
 */
export function AdvancedFold({ id, label, summary, inUse, holds, children }: Readonly<{
  /** The body's element id. */
  id: string;
  label: string;
  /** The names of what is inside, shown next to the label on wider screens. */
  summary?: string;
  /** One of its settings has a value or an issue. */
  inUse: boolean;
  /** Whether the field at a path goTo() asks for is inside. */
  holds: (path: Path) => boolean;
  children: ReactNode;
}>) {
  const { isOpen, toggle, open } = useLatchedOpen(inUse);
  // the request is read alone, so a memoized parent (a step card) does not re-render on every edit
  const focusRequest = useFocusRequest();
  useLayoutEffect(() => {
    if (focusRequest && holds(focusRequest.path)) open();
    // only a new request opens it; editing afterwards does not
  }, [focusRequest]);

  return (
    <div className="rounded-xl border border-border/60">
      <button
        type="button"
        aria-expanded={isOpen}
        aria-controls={isOpen ? id : undefined}
        onClick={toggle}
        className="flex w-full items-center gap-2 rounded-xl px-4 py-3 text-left text-sm font-medium transition-colors duration-150 hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
      >
        <ChevronRight
          className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200 ease-panel motion-reduce:transition-none', isOpen && 'rotate-90')}
          aria-hidden="true"
        />
        <span className="min-w-0 flex-1 sm:flex-none sm:shrink-0">{label}</span>
        {/* the names give way first: the label never wraps for them */}
        {summary && <span className="hidden min-w-0 flex-1 truncate text-right text-xs font-normal text-muted-foreground sm:block">{summary}</span>}
      </button>
      {isOpen && (
        <div id={id} className="border-t border-border/50 p-4">
          {children}
        </div>
      )}
    </div>
  );
}
