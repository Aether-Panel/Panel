import { useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { AlertTriangle, Plus, X } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { fieldId } from '../editor-context';
import { FieldIssues, issuesId, textValue } from '../field';
import { isPlainObject, pathKey, type JsonObject, type Path } from '../paths';
import type { EditorIssue } from '../validation';

// The variables a group shows on the server's settings page, as chips in the group's order. A
// chip whose variable no longer exists says so (and is what the warning points at); new ones
// are picked from the template's variables.

export function GroupVariables({ path, keys, variables, otherGroups, issues, onChange }: Readonly<{
  /** ['groups', i, 'variables']. */
  path: Path;
  keys: readonly string[];
  /** The template's variables. */
  variables: Readonly<Record<string, unknown>>;
  /** Which other group already lists a key, to say so in the picker. */
  otherGroups: ReadonlyMap<string, string>;
  /** Every issue under the list. */
  issues: readonly EditorIssue[];
  onChange: (keys: string[]) => void;
}>) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const picked = useRef(false);
  const id = fieldId(path);
  const labelId = `${id}-label`;
  const available = Object.keys(variables).filter((key) => !keys.includes(key));
  const latest = useRef(keys);
  latest.current = keys;

  const remove = (index: number) => {
    const current = latest.current;
    onChange(current.filter((_, i) => i !== index));
    // the focus goes to the chip taking its place, or to "Añadir"
    const next = current.length > 1 ? Math.min(index, current.length - 2) : -1;
    requestAnimationFrame(() => document.getElementById(next >= 0 ? fieldId([...path, next]) : `${id}-add`)?.focus());
  };

  return (
    <div className="space-y-2">
      <p id={labelId} className="text-sm font-medium leading-none">{t('templates.editor.advanced.groups.variables.label')}</p>
      <p className="text-pretty text-xs text-muted-foreground">{t('templates.editor.advanced.groups.variables.hint')}</p>
      <ul id={id} tabIndex={-1} aria-labelledby={labelId} aria-describedby={issuesId(id)} className="flex flex-wrap gap-1.5 outline-none">
        {keys.map((key, index) => {
          const known = Object.hasOwn(variables, key);
          const variable = variables[key];
          const display = known && isPlainObject(variable) ? textValue((variable as JsonObject).display) : '';
          const chipIssues = issues.filter((i) => i.path === pathKey([...path, index]));
          return (
            <li
              key={`${index}-${key}`}
              className={cn(
                'inline-flex h-8 max-w-full items-center gap-1 rounded-lg border pl-2.5 pr-1 text-sm',
                known ? 'border-border/70 bg-secondary/60' : 'border-warning/40 bg-warning/10 text-warning',
              )}
            >
              {!known && <AlertTriangle className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />}
              <span className="min-w-0 truncate font-mono text-[13px]" title={display || undefined}>{key}</span>
              {!known && <span className="sr-only">{t('templates.editor.advanced.groups.variables.missing')}</span>}
              <button
                type="button"
                id={fieldId([...path, index])}
                onClick={() => remove(index)}
                aria-label={t('templates.editor.advanced.groups.variables.remove', { key })}
                aria-describedby={chipIssues.length > 0 ? issuesId(id) : undefined}
                className="grid h-6 w-6 shrink-0 place-items-center rounded-md text-muted-foreground transition-colors duration-150 hover:bg-destructive/10 hover:text-destructive-bright focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
              >
                <X className="h-3.5 w-3.5" aria-hidden="true" />
              </button>
            </li>
          );
        })}
        <li>
          <Popover
            open={open}
            onOpenChange={(next) => {
              if (next) picked.current = false;
              setOpen(next);
            }}
          >
            <PopoverTrigger asChild>
              <Button id={`${id}-add`} type="button" variant="outline" size="sm" className="h-8 border-dashed bg-transparent text-muted-foreground hover:text-foreground">
                <Plus aria-hidden="true" />
                {t('templates.editor.advanced.groups.variables.add')}
              </Button>
            </PopoverTrigger>
            <PopoverContent
              align="start"
              className="w-[min(22rem,calc(100vw-2rem))] p-0"
              onCloseAutoFocus={(e) => {
                // after a pick the focus stays on "Añadir", ready for the next one
                if (picked.current) {
                  e.preventDefault();
                  document.getElementById(`${id}-add`)?.focus();
                }
              }}
            >
              <Command filter={(value, search) => (value.toLowerCase().includes(search.trim().toLowerCase()) ? 1 : 0)}>
                <CommandInput placeholder={t('templates.editor.insertVariable.search')} aria-label={t('templates.editor.insertVariable.search')} />
                <CommandList>
                  <CommandEmpty>
                    {Object.keys(variables).length === 0 ? t('templates.editor.advanced.groups.variables.none') : t('templates.editor.insertVariable.empty')}
                  </CommandEmpty>
                  {available.length > 0 && (
                    <CommandGroup heading={t('templates.editor.insertVariable.templateGroup')}>
                      {available.map((key) => {
                        const variable = variables[key];
                        const display = isPlainObject(variable) ? textValue(variable.display) : '';
                        const elsewhere = otherGroups.get(key);
                        return (
                          <CommandItem
                            key={key}
                            value={`${key} ${display}`}
                            onSelect={() => {
                              picked.current = true;
                              setOpen(false);
                              onChange([...latest.current, key]);
                            }}
                          >
                            <span className="min-w-0 flex-1 truncate font-mono text-[13px]">{key}</span>
                            <span className="max-w-[55%] truncate text-xs text-muted-foreground">
                              {elsewhere !== undefined ? t('templates.editor.advanced.groups.variables.inOther', { group: elsewhere }) : display}
                            </span>
                          </CommandItem>
                        );
                      })}
                    </CommandGroup>
                  )}
                </CommandList>
              </Command>
            </PopoverContent>
          </Popover>
        </li>
      </ul>
      <FieldIssues id={issuesId(id)} issues={issues} />
    </div>
  );
}
