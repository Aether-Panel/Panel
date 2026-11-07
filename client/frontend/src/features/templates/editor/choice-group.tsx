import { useRef, type KeyboardEvent } from 'react';
import { cn } from '@/lib/utils';

// A small set of mutually exclusive ways to fill a value ("Un comando | Según condición"), drawn
// as a segmented control. It is a radiogroup with manual activation: arrows move the focus, and
// Space, Enter or a click choose. Choosing on every arrow would rewrite the template on the way
// through, converting it back and forth.

export interface Choice<T extends string> {
  value: T;
  label: string;
  /** Why it cannot be chosen now; the option stays focusable and says so. */
  disabledReason?: string;
}

export function ChoiceGroup<T extends string>({ labelledBy, describedBy, choices, value, onChoose, selectedId, className }: Readonly<{
  /** The id of the text naming the group. */
  labelledBy: string;
  describedBy?: string;
  choices: readonly Choice<T>[];
  value: T;
  onChoose: (value: T) => void;
  /** An id for the chosen option, so goTo() on the value lands on it. */
  selectedId?: string;
  className?: string;
}>) {
  const group = useRef<HTMLDivElement>(null);

  const onKey = (e: KeyboardEvent<HTMLButtonElement>, index: number) => {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
    if (!step) return;
    e.preventDefault();
    const next = choices[(index + step + choices.length) % choices.length];
    group.current?.querySelector<HTMLButtonElement>(`[data-choice="${next.value}"]`)?.focus();
  };

  return (
    <div
      ref={group}
      role="radiogroup"
      aria-labelledby={labelledBy}
      aria-describedby={describedBy}
      className={cn('inline-grid w-full auto-cols-fr grid-flow-col gap-1 rounded-lg bg-muted p-1 text-muted-foreground sm:w-auto', className)}
    >
      {choices.map((choice, index) => {
        const checked = choice.value === value;
        const reasonId = choice.disabledReason ? `${labelledBy}-${choice.value}-reason` : undefined;
        return (
          <button
            key={choice.value}
            type="button"
            role="radio"
            data-choice={choice.value}
            id={checked ? selectedId : undefined}
            aria-checked={checked}
            aria-disabled={choice.disabledReason ? true : undefined}
            aria-describedby={reasonId}
            title={choice.disabledReason}
            tabIndex={checked ? 0 : -1}
            onClick={() => !checked && !choice.disabledReason && onChoose(choice.value)}
            onKeyDown={(e) => onKey(e, index)}
            className="inline-flex h-8 min-w-0 items-center justify-center whitespace-nowrap rounded-md px-3 text-sm font-medium transition-colors duration-150 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70 aria-checked:bg-background aria-checked:text-foreground aria-checked:shadow-sm aria-disabled:cursor-not-allowed aria-disabled:opacity-50 aria-disabled:hover:text-muted-foreground"
          >
            <span className="truncate">{choice.label}</span>
          </button>
        );
      })}
      {/* outside the buttons, so the reason describes an option without becoming its name */}
      {choices.map((choice) =>
        choice.disabledReason ? (
          <span key={choice.value} id={`${labelledBy}-${choice.value}-reason`} className="sr-only">
            {choice.disabledReason}
          </span>
        ) : null,
      )}
    </div>
  );
}
