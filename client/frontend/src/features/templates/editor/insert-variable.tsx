import { useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Braces } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';
import { useTemplateEditor, useTemplateEditorActions } from './editor-context';
import { textValue } from './field';
import { isPlainObject } from './paths';
import { templateVariables } from './variables/usage';

// "Insertar variable": a searchable list of the template's variables that types `${key}` into a
// text field. Next to one field, give it that field's insert (useInsertableField(path).insert);
// anywhere else it types into the field focused last, or at the JSON editor's cursor.

/** Names the node fills in by itself, offered after the template's own variables. */
const BUILTIN_VARIABLES = ['rootDir'] as const;

/** The searchable list in the open popover; mounted only while open, so only then it reads the template. */
function VariableCommand({ onPick }: Readonly<{ onPick: (key: string) => void }>) {
  const { t } = useTranslations();
  const { template } = useTemplateEditor();
  const variables = Object.entries(templateVariables(template));
  return (
    <Command
      // cmdk matches the value, which holds the key and the visible name
      filter={(value, search) => (value.toLowerCase().includes(search.trim().toLowerCase()) ? 1 : 0)}
    >
      <CommandInput placeholder={t('templates.editor.insertVariable.search')} aria-label={t('templates.editor.insertVariable.search')} />
      <CommandList>
        <CommandEmpty>{t('templates.editor.insertVariable.empty')}</CommandEmpty>
        {variables.length > 0 && (
          <CommandGroup heading={t('templates.editor.insertVariable.templateGroup')}>
            {variables.map(([key, variable]) => {
              const display = isPlainObject(variable) ? textValue(variable.display) : '';
              return (
                <CommandItem key={key} value={`${key} ${display}`} onSelect={() => onPick(key)}>
                  <span className="min-w-0 flex-1 truncate font-mono text-[13px]">{`\${${key}}`}</span>
                  {display && <span className="max-w-[45%] truncate text-xs text-muted-foreground">{display}</span>}
                </CommandItem>
              );
            })}
          </CommandGroup>
        )}
        <CommandGroup heading={t('templates.editor.insertVariable.builtinGroup')}>
          {BUILTIN_VARIABLES.map((key) => (
            <CommandItem key={key} value={`${key} ${t(`templates.editor.insertVariable.builtin.${key}`)}`} onSelect={() => onPick(key)}>
              <span className="min-w-0 flex-1 truncate font-mono text-[13px]">{`\${${key}}`}</span>
              <span className="max-w-[45%] truncate text-xs text-muted-foreground">{t(`templates.editor.insertVariable.builtin.${key}`)}</span>
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </Command>
  );
}

export function InsertVariableButton({ onInsert, compact = false, className }: Readonly<{
  /** Inserts into one field; without it the text goes to the field focused last. */
  onInsert?: (text: string) => void;
  /** Only the icon (the label stays for screen readers), for tight rows. */
  compact?: boolean;
  className?: string;
}>) {
  const { t } = useTranslations();
  // the closed button reads no template state, so a form full of them is not re-rendered by every edit
  const { insertIntoField } = useTemplateEditorActions();
  const [open, setOpen] = useState(false);
  // the field takes focus back once the text is in, so the popover must not return it to the button
  const inserted = useRef(false);
  const label = t('templates.editor.insertVariable.button');

  const insert = (key: string) => {
    const text = `\${${key}}`;
    inserted.current = true;
    setOpen(false);
    if (onInsert) onInsert(text);
    else if (!insertIntoField(text)) {
      inserted.current = false;
      sileo.info({ title: t('templates.editor.insertVariable.noField') });
    }
  };

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        if (next) inserted.current = false;
        setOpen(next);
      }}
    >
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className={cn('font-normal', compact && 'w-9 px-0', className)}
          aria-label={compact ? label : undefined}
          title={compact ? label : undefined}
        >
          <Braces aria-hidden="true" />
          {!compact && label}
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-[min(20rem,calc(100vw-2rem))] p-0"
        onCloseAutoFocus={(e) => {
          if (inserted.current) e.preventDefault();
        }}
      >
        <VariableCommand onPick={insert} />
      </PopoverContent>
    </Popover>
  );
}
