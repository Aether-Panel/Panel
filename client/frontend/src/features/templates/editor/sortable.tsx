import { createContext, useCallback, useContext, useId, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import {
  closestCenter, DndContext, KeyboardSensor, PointerSensor, useSensor, useSensors,
  type Announcements, type DragEndEvent, type Modifier, type UniqueIdentifier,
} from '@dnd-kit/core';
import { SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { ArrowDown, ArrowUp, GripVertical } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';

// Reordering for the editor's lists (variables, options, steps): drag by a handle, with the
// mouse, touch or the keyboard (space to lift, arrows to move, space to drop, escape to cancel),
// and ↑ ↓ buttons next to it for whoever prefers them or cannot drag. dnd-kit announces drags to
// screen readers in the panel's language; the buttons announce through the list's own region.

/** Items only move up and down, so a drag does not wander sideways out of the list. */
const verticalOnly: Modifier = ({ transform }) => ({ ...transform, x: 0 });

const AnnounceContext = createContext<(message: string) => void>(() => undefined);

/**
 * A vertical list whose items (each with useSortableItem) can be dragged into place. `ids` are
 * the items' ids in order; `onMove(from, to)` gets their positions; `itemLabel` names an item
 * for the announcements.
 */
export function SortableList({ ids, onMove, itemLabel, children }: Readonly<{
  ids: readonly string[];
  onMove: (from: number, to: number) => void;
  itemLabel: (id: string) => string;
  children: ReactNode;
}>) {
  const { t } = useTranslations();
  const [message, setMessage] = useState('');
  const sensors = useSensors(
    // a few pixels before a drag starts, so a click on the handle stays a click
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const announcements = useMemo<Announcements>(() => {
    const params = (id: UniqueIdentifier, at: UniqueIdentifier) => ({
      item: itemLabel(String(id)),
      position: ids.indexOf(String(at)) + 1,
      count: ids.length,
    });
    return {
      onDragStart: ({ active }) => t('templates.editor.sortable.picked', params(active.id, active.id)),
      onDragOver: ({ active, over }) =>
        over ? t('templates.editor.sortable.over', params(active.id, over.id)) : t('templates.editor.sortable.outside', { item: itemLabel(String(active.id)) }),
      onDragEnd: ({ active, over }) =>
        over ? t('templates.editor.sortable.dropped', params(active.id, over.id)) : t('templates.editor.sortable.cancelled', params(active.id, active.id)),
      onDragCancel: ({ active }) => t('templates.editor.sortable.cancelled', params(active.id, active.id)),
    };
  }, [ids, itemLabel, t]);

  const onDragEnd = ({ active, over }: DragEndEvent) => {
    if (!over || active.id === over.id) return;
    const from = ids.indexOf(String(active.id));
    const to = ids.indexOf(String(over.id));
    if (from >= 0 && to >= 0) onMove(from, to);
  };

  // a new message replaces the old one, so moving the same way twice is announced twice
  const announce = useCallback((text: string) => setMessage((previous) => (previous === text ? `${text} ` : text)), []);

  return (
    <AnnounceContext.Provider value={announce}>
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        modifiers={[verticalOnly]}
        onDragEnd={onDragEnd}
        accessibility={{ announcements, screenReaderInstructions: { draggable: t('templates.editor.sortable.instructions') } }}
      >
        <SortableContext items={ids as string[]} strategy={verticalListSortingStrategy}>
          {children}
        </SortableContext>
      </DndContext>
      <p className="sr-only" aria-live="polite" aria-atomic="true">{message}</p>
    </AnnounceContext.Provider>
  );
}

/**
 * Ids for the items of a list whose items have none (a variable's options, steps): React and
 * drag and drop need an item to keep its id while it moves. Call move/insert/remove together
 * with the change itself so the ids follow; a list changed elsewhere (the JSON view) gets ids
 * added or dropped at its end.
 */
export function useListIds(count: number) {
  const state = useRef({ ids: [] as string[], next: 0 });
  const current = state.current;
  if (current.ids.length > count) current.ids = current.ids.slice(0, count);
  else if (current.ids.length < count) {
    current.ids = [...current.ids, ...Array.from({ length: count - current.ids.length }, () => `item-${current.next++}`)];
  }
  const actions = useMemo(
    () => ({
      move(from: number, to: number) {
        const ids = state.current.ids.slice();
        const [id] = ids.splice(from, 1);
        ids.splice(to, 0, id);
        state.current.ids = ids;
      },
      /** Gives the item inserted at `at` a new id, and returns it (the `ids` of this render predate it). */
      insert(at: number): string {
        const ids = state.current.ids.slice();
        const id = `item-${state.current.next++}`;
        ids.splice(at, 0, id);
        state.current.ids = ids;
        return id;
      },
      remove(at: number) {
        state.current.ids = state.current.ids.filter((_, i) => i !== at);
      },
    }),
    [],
  );
  return { ids: current.ids, ...actions };
}

/** What a sortable item spreads: `ref` and `style` on its outer element, `handleProps` on its handle. */
export function useSortableItem(id: string, disabled = false) {
  const { t } = useTranslations();
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({
    id,
    // an item that cannot move (its handle says why) is not picked up
    disabled,
    attributes: { roleDescription: t('templates.editor.sortable.roleDescription') },
  });
  const style: CSSProperties = {
    // translate only: items of different heights must not be stretched into each other
    transform: CSS.Translate.toString(transform),
    transition,
    position: 'relative',
    zIndex: isDragging ? 10 : undefined,
  };
  return { ref: setNodeRef, style, isDragging, handleProps: { ...attributes, ...listeners, ref: setActivatorNodeRef } };
}

type HandleProps = ReturnType<typeof useSortableItem>['handleProps'];

/** The grip an item is dragged by; `label` says what it moves ("Mover «port»"). */
export function DragHandle({ handleProps, label, className }: Readonly<{ handleProps: HandleProps; label: string; className?: string }>) {
  return (
    <button
      type="button"
      {...handleProps}
      aria-label={label}
      className={cn(
        // touch-none: on a phone the finger drags the item instead of scrolling the page
        'grid h-8 w-6 shrink-0 cursor-grab touch-none place-items-center rounded-md text-muted-foreground/70 transition-colors duration-150 hover:bg-accent/70 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70 active:cursor-grabbing aria-disabled:cursor-not-allowed aria-disabled:opacity-35',
        className,
      )}
    >
      <GripVertical className="h-4 w-4" aria-hidden="true" />
    </button>
  );
}

/**
 * ↑ and ↓ for an item at `index` of `count`. At an end the button stays focusable (aria-disabled),
 * so focus is not lost when an item reaches the top; it follows the item after the move.
 */
export function MoveButtons({ index, count, label, onMove, min = 0, lockedReason, className }: Readonly<{
  index: number;
  count: number;
  label: string;
  onMove: (from: number, to: number) => void;
  /** The first position the item can take (items above it cannot be passed). */
  min?: number;
  /** Why the item cannot move at all; both buttons are then off and describe it. */
  lockedReason?: string;
  className?: string;
}>) {
  const { t } = useTranslations();
  const announce = useContext(AnnounceContext);
  const up = useRef<HTMLButtonElement>(null);
  const down = useRef<HTMLButtonElement>(null);
  const reasonId = useId();
  const locked = lockedReason !== undefined;

  const move = (to: number, button: HTMLButtonElement | null) => {
    if (locked || to < min || to >= count) return;
    onMove(index, to);
    announce(t('templates.editor.sortable.movedTo', { item: label, position: to + 1, count }));
    // React may re-insert the item's element to reorder the list, which drops its focus
    requestAnimationFrame(() => button?.focus());
  };

  const buttonClass =
    'grid h-8 w-8 place-items-center rounded-md text-muted-foreground transition-colors duration-150 hover:bg-accent/70 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70 aria-disabled:pointer-events-none aria-disabled:opacity-35';
  const described = locked ? { 'aria-describedby': reasonId, title: lockedReason } : {};
  return (
    <span className={cn('flex shrink-0 items-center', className)}>
      {locked && (
        <span id={reasonId} className="sr-only">
          {lockedReason}
        </span>
      )}
      <button
        ref={up}
        type="button"
        className={buttonClass}
        aria-label={t('templates.editor.sortable.moveUp', { item: label })}
        aria-disabled={locked || index <= min}
        {...described}
        onClick={() => move(index - 1, up.current)}
      >
        <ArrowUp className="h-4 w-4" aria-hidden="true" />
      </button>
      <button
        ref={down}
        type="button"
        className={buttonClass}
        aria-label={t('templates.editor.sortable.moveDown', { item: label })}
        aria-disabled={locked || index === count - 1}
        {...described}
        onClick={() => move(index + 1, down.current)}
      >
        <ArrowDown className="h-4 w-4" aria-hidden="true" />
      </button>
    </span>
  );
}
