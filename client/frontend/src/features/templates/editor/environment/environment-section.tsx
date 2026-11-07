import { useCallback, useRef } from 'react';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { EmptyState } from '@/components/empty-state';
import { Box, ChevronDown, Plus, Server, X } from '@/components/icons';
import { SettingsSection } from '@/features/settings/general-tab';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { ENVIRONMENT_TYPES } from '../../schema/template';
import { fieldId, useTemplateEditor } from '../editor-context';
import { FieldIssues, issuesId, textValue } from '../field';
import { deleteIn, isPlainObject, moveIn, pathKey, setIn } from '../paths';
import { IssueCountBadges } from '../section-nav';
import { DragHandle, MoveButtons, SortableList, useListIds, useSortableItem } from '../sortable';
import { countIssues, isUnderPath, type EditorIssue } from '../validation';
import { EnvironmentFields } from './environment-fields';

// Entorno: the template's default environment, and its supported ones. The panel's "Crear
// servidor" form always creates servers in Docker, with the first Docker entry of the supported
// environments (its image, ports and settings); the default environment is kept in the template
// but that form does not use it.

const LIST_PATH = ['supportedEnvironments'];
const NEW_TYPES = ['host', 'docker'] as const;

const NAMED_TYPES = new Set(['', ...ENVIRONMENT_TYPES]);

/** The name of an environment's type; a type the panel does not know keeps its own. */
function environmentLabel(t: (key: string) => string, environment: unknown): string {
  const type = isPlainObject(environment) && typeof environment.type === 'string' ? environment.type : '';
  if (!NAMED_TYPES.has(type)) return type;
  return type === '' ? t('templates.editor.environment.types.unset') : t(`templates.editor.environment.types.${type}`);
}

function SupportedCard({ sortId, environment, index, count, issues, onMove, onRemove }: Readonly<{
  sortId: string;
  environment: unknown;
  index: number;
  count: number;
  issues: readonly EditorIssue[];
  onMove: (from: number, to: number) => void;
  onRemove: (index: number) => void;
}>) {
  const { t } = useTranslations();
  const { ref, style, isDragging, handleProps } = useSortableItem(sortId);
  const path = [...LIST_PATH, index];
  const type = isPlainObject(environment) ? environment.type : undefined;
  const typeName = environmentLabel(t, environment);
  const image = isPlainObject(environment) && type === 'docker' ? textValue(environment.image) : '';
  const name = t('templates.editor.environment.supported.item', { n: index + 1, type: typeName });
  const headingId = `${fieldId(path)}-heading`;
  const Icon = type === 'docker' ? Box : Server;

  return (
    <li
      ref={ref}
      style={style}
      aria-labelledby={headingId}
      className={cn('rounded-xl border border-border/60 bg-background/30', isDragging && 'border-primary/40 bg-card shadow-elevation-3')}
    >
      <div className="flex items-center gap-1 border-b border-border/50 py-1.5 pl-1.5 pr-1 sm:gap-1.5 sm:pl-2">
        <DragHandle handleProps={handleProps} label={t('templates.editor.sortable.handle', { item: name })} />
        <span className="hidden h-7 w-7 shrink-0 place-items-center rounded-md bg-secondary text-muted-foreground sm:grid" aria-hidden="true">
          <Icon className="h-4 w-4" />
        </span>
        <span className="min-w-0 flex-1 px-1.5">
          <span id={headingId} className="block truncate text-sm font-medium">{name}</span>
          {image && <span className="block truncate font-mono text-xs text-muted-foreground">{image}</span>}
        </span>
        <IssueCountBadges counts={countIssues(issues)} />
        <MoveButtons index={index} count={count} label={name} onMove={onMove} />
        <button
          type="button"
          id={`${fieldId(path)}-remove`}
          onClick={() => onRemove(index)}
          aria-label={t('templates.editor.environment.supported.remove', { item: name })}
          className="grid h-8 w-8 shrink-0 place-items-center rounded-md text-muted-foreground transition-colors duration-150 hover:bg-destructive/10 hover:text-destructive-bright focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>
      <div className="p-4">
        <EnvironmentFields path={path} value={environment} required />
      </div>
    </li>
  );
}

function SupportedEnvironments() {
  const { t } = useTranslations();
  const { template, issues, issuesAt, change } = useTemplateEditor();
  const raw = template.supportedEnvironments;
  const list = Array.isArray(raw) ? raw : [];
  const ids = useListIds(list.length);
  const latest = useRef(list);
  latest.current = list;
  const addId = `${fieldId(LIST_PATH)}-add`;

  const onMove = (from: number, to: number) => {
    if (from === to || to < 0 || to >= latest.current.length) return;
    ids.move(from, to);
    change((tpl) => moveIn(tpl, LIST_PATH, from, to));
  };
  const onRemove = (index: number) => {
    const count = latest.current.length;
    const neighbour = index + 1 < count ? index : index - 1;
    ids.remove(index);
    // the last one removed takes the key with it, as an emptied list of steps does
    change((tpl) => (count === 1 ? deleteIn(tpl, LIST_PATH) : deleteIn(tpl, [...LIST_PATH, index])));
    requestAnimationFrame(() => document.getElementById(count > 1 ? `${fieldId([...LIST_PATH, neighbour])}-remove` : addId)?.focus());
  };
  const add = (type: (typeof NEW_TYPES)[number]) => {
    const at = latest.current.length;
    ids.insert(at);
    change((tpl) => setIn(tpl, [...LIST_PATH, at], { type }));
    // into the new environment's first setting: Docker's image, the host's type
    setTimeout(() => document.getElementById(fieldId([...LIST_PATH, at, type === 'docker' ? 'image' : 'type']))?.focus(), 50);
  };
  const itemLabel = useCallback(
    (sortId: string) => {
      const index = ids.ids.indexOf(sortId);
      return t('templates.editor.environment.supported.item', { n: index + 1, type: environmentLabel(t, latest.current[index]) });
    },
    [ids.ids, t],
  );

  const addMenu = (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button id={addId} type="button" variant="outline" className={cn(list.length > 0 && 'w-full border-dashed bg-transparent text-muted-foreground hover:text-foreground')} size={list.length > 0 ? 'default' : 'sm'}>
          <Plus aria-hidden="true" />
          {t('templates.editor.environment.supported.add')}
          <ChevronDown aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align={list.length > 0 ? 'center' : 'start'}>
        {NEW_TYPES.map((type) => (
          <DropdownMenuItem key={type} onSelect={() => add(type)}>
            {type === 'docker' ? <Box aria-hidden="true" /> : <Server aria-hidden="true" />}
            {t(`templates.editor.environment.types.${type}`)}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );

  if (raw !== undefined && raw !== null && !Array.isArray(raw)) {
    return <p className="text-pretty text-sm text-muted-foreground">{t('templates.editor.steps.notEditableHere')}</p>;
  }

  return (
    // goTo() on the list as a whole (no Docker entry) lands here
    <div id={fieldId(LIST_PATH)} tabIndex={-1} aria-describedby={issuesId(fieldId(LIST_PATH))} className="space-y-3 rounded-lg outline-none">
      <FieldIssues id={issuesId(fieldId(LIST_PATH))} issues={issuesAt(LIST_PATH)} />
      {list.length === 0 ? (
        <EmptyState
          icon={Box}
          size="sm"
          title={t('templates.editor.environment.supported.empty.title')}
          description={t('templates.editor.environment.supported.empty.description')}
          action={addMenu}
        />
      ) : (
        <>
          <SortableList ids={ids.ids} onMove={onMove} itemLabel={itemLabel}>
            <ol className="space-y-2" aria-label={t('templates.editor.environment.supported.title')}>
              {list.map((environment, index) => (
                <SupportedCard
                  key={ids.ids[index]}
                  sortId={ids.ids[index]}
                  environment={environment}
                  index={index}
                  count={list.length}
                  issues={issues.filter((i) => isUnderPath(i.path, pathKey([...LIST_PATH, index])))}
                  onMove={onMove}
                  onRemove={onRemove}
                />
              ))}
            </ol>
          </SortableList>
          {addMenu}
        </>
      )}
    </div>
  );
}

export function EnvironmentSection() {
  const { t } = useTranslations();
  const { template } = useTemplateEditor();
  return (
    <div className="divide-y divide-border/50">
      <SettingsSection title={t('templates.editor.environment.default.title')} description={t('templates.editor.environment.default.description')}>
        <EnvironmentFields path={['environment']} value={template.environment} prune />
      </SettingsSection>
      <SettingsSection title={t('templates.editor.environment.supported.title')} description={t('templates.editor.environment.supported.description')}>
        <SupportedEnvironments />
      </SettingsSection>
    </div>
  );
}
