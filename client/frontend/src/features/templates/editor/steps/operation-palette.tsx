import { useMemo, useState } from 'react';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useTranslations } from '@/contexts/translations-context';
import { templateOperationsByCategory, type OperationDefinition } from '../../schema/operations';
import { operationIcon } from './operation-icons';
import { matchOperation } from './palette-search';

// "Añadir paso": every operation a template can use, grouped by category, each with its icon,
// name and what it does in one line. Typing filters by name, key, help or category; arrows move,
// Enter adds. A dialog rather than a popover, so it opens the same from the "Añadir paso" button
// and from a card's "Insertar después", and fills the width of a phone.

const GROUPS = templateOperationsByCategory();

/** One operation in the palette: icon, name, key, and what it does in a line. */
function OperationItem({ operation, text, category, onPick }: Readonly<{
  operation: OperationDefinition;
  text: { label: string; help: string } | undefined;
  /** Shown in search results, which are not grouped. */
  category?: string;
  onPick: (operation: OperationDefinition) => void;
}>) {
  const Icon = operationIcon(operation);
  return (
    <CommandItem value={operation.key} onSelect={() => onPick(operation)} className="items-start gap-3 rounded-md px-2.5 py-2">
      <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-border/60 bg-background/60 text-primary-bright" aria-hidden="true">
        <Icon className="h-4 w-4" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex items-baseline justify-between gap-3">
          <span className="truncate font-medium">{text?.label}</span>
          {category ? (
            <span className="hidden shrink-0 text-[11px] text-muted-foreground sm:inline">{category}</span>
          ) : (
            <span className="hidden shrink-0 font-mono text-[11px] text-muted-foreground sm:inline">{operation.key}</span>
          )}
        </span>
        <span className="line-clamp-2 text-xs text-muted-foreground sm:line-clamp-1">{text?.help}</span>
      </span>
    </CommandItem>
  );
}

export function OperationPalette({ open, onOpenChange, title, description, onPick, onCloseAutoFocus }: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  onPick: (operation: OperationDefinition) => void;
  /** Lets the caller keep the focus where it put it (the new step's first field). */
  onCloseAutoFocus?: (event: Event) => void;
}>) {
  const { t } = useTranslations();
  // the texts each operation is searched by, in the panel's language
  const texts = useMemo(
    () =>
      new Map(
        GROUPS.flatMap((group) =>
          group.operations.map((op) => [op.key, { key: op.key, label: t(op.labelKey), help: t(op.helpKey), category: t(group.labelKey) }] as const),
        ),
      ),
    [t],
  );

  const [search, setSearch] = useState('');
  // each opening starts from the whole list
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setSearch('');
  }
  // while searching: every match, best first (name before key before help), registry order on ties
  const results = useMemo(() => {
    if (search.trim() === '') return null;
    return GROUPS.flatMap((group) => group.operations.map((operation) => ({ operation, category: t(group.labelKey) })))
      .map((entry, order) => {
        const text = texts.get(entry.operation.key);
        return { ...entry, order, score: text ? matchOperation(text, search) : 0 };
      })
      .filter((entry) => entry.score > 0)
      .sort((a, b) => b.score - a.score || a.order - b.order);
  }, [search, texts, t]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[calc(100%-1rem)] max-w-xl gap-0 overflow-hidden p-0 sm:w-[calc(100%-2rem)]" onCloseAutoFocus={onCloseAutoFocus}>
        <DialogHeader className="space-y-1 border-b border-border/50 px-5 pb-3.5 pr-12 pt-5 text-left">
          <DialogTitle className="text-base">{title}</DialogTitle>
          <DialogDescription className="text-xs">{description}</DialogDescription>
        </DialogHeader>
        <Command
          // the ranking is done here: cmdk ranks within its groups, which put a help-text match
          // of an earlier category above a name match of a later one
          shouldFilter={false}
          className="rounded-none"
        >
          <CommandInput
            value={search}
            onValueChange={setSearch}
            placeholder={t('templates.editor.steps.palette.search')}
            aria-label={t('templates.editor.steps.palette.search')}
          />
          <CommandList className="max-h-[min(28rem,58svh)]">
            <CommandEmpty>{t('templates.editor.steps.palette.empty')}</CommandEmpty>
            {results === null ? (
              GROUPS.map((group) => (
                <CommandGroup key={group.category} heading={t(group.labelKey)}>
                  {group.operations.map((op) => (
                    <OperationItem key={op.key} operation={op} text={texts.get(op.key)} onPick={onPick} />
                  ))}
                </CommandGroup>
              ))
            ) : (
              <CommandGroup heading={t('templates.editor.steps.palette.results')}>
                {results.map(({ operation, category }) => (
                  <OperationItem key={operation.key} operation={operation} text={texts.get(operation.key)} category={category} onPick={onPick} />
                ))}
              </CommandGroup>
            )}
          </CommandList>
          <div className="hidden items-center gap-4 border-t border-border/50 px-4 py-2 text-[11px] text-muted-foreground sm:flex" aria-hidden="true">
            <span>
              <kbd className="font-sans">↑ ↓</kbd> {t('templates.editor.steps.palette.keyMove')}
            </span>
            <span>
              <kbd className="font-sans">↵</kbd> {t('templates.editor.steps.palette.keyPick')}
            </span>
            <span>
              <kbd className="font-sans">Esc</kbd> {t('templates.editor.steps.palette.keyClose')}
            </span>
          </div>
        </Command>
      </DialogContent>
    </Dialog>
  );
}
