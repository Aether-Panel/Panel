import { SettingsSection } from '@/features/settings/general-tab';
import { useTranslations } from '@/contexts/translations-context';
import { fieldId, useTemplateEditor } from '../editor-context';
import { isPlainObject, type JsonObject, type Path } from '../paths';
import { NotEditableHere, SelectControl, TextControl } from '../steps/field-controls';
import { writeOptional } from '../write';
import { GroupsEditor } from './groups-editor';

// Avanzado: variable groups for the server's settings page, and three things the node does while
// a server runs: JVM memory stats (stats), asking the game for its players (query) and sending a
// console command now and then (keepAlive). Each is a small object; clearing all of one removes it.

/** The keys each block edits; any other one is kept and named. */
const KNOWN_KEYS: Record<'stats' | 'query' | 'keepAlive', readonly string[]> = {
  stats: ['type', 'cmd'],
  query: ['type'],
  keepAlive: ['frequency', 'command'],
};

/** A block's object, the note on keys it does not show, or the fallback for a value of another shape. */
function useBlock(key: keyof typeof KNOWN_KEYS) {
  const { t } = useTranslations();
  const { template, issuesAt, change } = useTemplateEditor();
  const raw = template[key];
  const object: JsonObject = isPlainObject(raw) ? raw : {};
  const path = (field: string): Path => [key, field];
  const extra = Object.keys(object).filter((k) => !KNOWN_KEYS[key].includes(k));
  return {
    object,
    path,
    /** The value is neither an object nor nothing: left to the JSON view. */
    notEditable:
      raw !== undefined && raw !== null && !isPlainObject(raw) ? (
        <NotEditableHere id={fieldId([key])} label={t(`templates.editor.advanced.${key}.title`)} issues={issuesAt([key])} />
      ) : null,
    issuesAt: (field: string) => issuesAt(path(field)),
    write: (field: string, value: unknown) => change((tpl) => writeOptional(tpl, path(field), value, true)),
    extraNote:
      extra.length > 0 ? <p className="text-pretty text-xs text-muted-foreground">{t('templates.editor.advanced.extraKeys', { keys: extra.join(', ') })}</p> : null,
  };
}

function StatsBlock() {
  const { t } = useTranslations();
  const block = useBlock('stats');
  if (block.notEditable) return block.notEditable;
  const jcmd = block.object.type === 'jcmd';
  return (
    <div className="space-y-5">
      <SelectControl
        id={fieldId(block.path('type'))}
        label={t('templates.editor.advanced.stats.type.label')}
        hint={t('templates.editor.advanced.stats.type.hint')}
        value={block.object.type}
        issues={block.issuesAt('type')}
        options={[{ value: 'jcmd', labelKey: 'templates.editor.advanced.stats.type.jcmd' }]}
        optionLabel={(option) => t(option.labelKey ?? option.value)}
        onChange={(value) => block.write('type', value)}
      />
      {jcmd && (
        <TextControl
          id={fieldId(block.path('cmd'))}
          label={t('templates.editor.advanced.stats.cmd.label')}
          hint={t('templates.editor.advanced.stats.cmd.hint')}
          placeholder={t('templates.editor.steps.defaultValue', { value: 'jcmd' })}
          value={block.object.cmd}
          issues={block.issuesAt('cmd')}
          variables={false}
          mono
          onChange={(value) => block.write('cmd', value)}
        />
      )}
      {block.extraNote}
    </div>
  );
}

function QueryBlock() {
  const { t } = useTranslations();
  const block = useBlock('query');
  if (block.notEditable) return block.notEditable;
  const type = typeof block.object.type === 'string' ? block.object.type : '';
  let hint = 'unset';
  if (type === 'minecraft') hint = 'minecraft';
  else if (type === 'source' || type === 'steam') hint = 'source';
  else if (type !== '') hint = 'other';
  return (
    <div className="space-y-5">
      <SelectControl
        id={fieldId(block.path('type'))}
        label={t('templates.editor.advanced.query.type.label')}
        hint={t(`templates.editor.advanced.query.type.hint.${hint}`)}
        value={block.object.type}
        issues={block.issuesAt('type')}
        options={[
          { value: 'minecraft', labelKey: 'templates.editor.advanced.query.type.minecraft' },
          { value: 'source', labelKey: 'templates.editor.advanced.query.type.source' },
        ]}
        optionLabel={(option) => t(option.labelKey ?? option.value)}
        onChange={(value) => block.write('type', value)}
      />
      {block.extraNote}
    </div>
  );
}

function KeepAliveBlock() {
  const { t } = useTranslations();
  const block = useBlock('keepAlive');
  if (block.notEditable) return block.notEditable;
  return (
    <div className="space-y-5">
      <div className="grid gap-5 sm:grid-cols-2">
        <TextControl
          id={fieldId(block.path('frequency'))}
          label={t('templates.editor.advanced.keepAlive.frequency.label')}
          placeholder="5m"
          value={block.object.frequency}
          issues={block.issuesAt('frequency')}
          variables={false}
          mono
          onChange={(value) => block.write('frequency', value)}
        />
        <TextControl
          id={fieldId(block.path('command'))}
          label={t('templates.editor.advanced.keepAlive.command.label')}
          placeholder="save-all"
          value={block.object.command}
          issues={block.issuesAt('command')}
          variables={false}
          mono
          onChange={(value) => block.write('command', value)}
        />
      </div>
      <p className="text-pretty text-xs text-muted-foreground">{t('templates.editor.advanced.keepAlive.hint')}</p>
      {block.extraNote}
    </div>
  );
}

export function AdvancedSection() {
  const { t } = useTranslations();
  return (
    <div className="divide-y divide-border/50">
      <SettingsSection title={t('templates.editor.advanced.groups.title')} description={t('templates.editor.advanced.groups.description')}>
        <GroupsEditor />
      </SettingsSection>
      <SettingsSection title={t('templates.editor.advanced.stats.title')} description={t('templates.editor.advanced.stats.description')}>
        <StatsBlock />
      </SettingsSection>
      <SettingsSection title={t('templates.editor.advanced.query.title')} description={t('templates.editor.advanced.query.description')}>
        <QueryBlock />
      </SettingsSection>
      <SettingsSection title={t('templates.editor.advanced.keepAlive.title')} description={t('templates.editor.advanced.keepAlive.description')}>
        <KeepAliveBlock />
      </SettingsSection>
    </div>
  );
}
