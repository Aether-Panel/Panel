import { SettingsSection } from '@/features/settings/general-tab';
import { useTranslations } from '@/contexts/translations-context';
import { fieldId, useTemplateEditor } from '../editor-context';
import { isPlainObject, pathKey, type JsonObject, type Path } from '../paths';
import { BooleanControl, NotEditableHere, NumberControl, TextControl } from '../steps/field-controls';
import { KeyValueControl } from '../steps/key-value-control';
import { StdinControl } from '../steps/stdin-control';
import { StepListFold } from '../steps/step-list-fold';
import { isUnderPath } from '../validation';
import { writeOptional } from '../write';
import { CommandEditor } from './command-editor';
import { StopEditor } from './stop-editor';

// Ejecución: how the server starts (its command, folder and environment variables), how it
// stops, how the panel talks to its console, when it starts or restarts on its own, and the
// steps run before it starts and after it stops.

const RUN_FLAGS = ['autostart', 'autorecover', 'autorestart'] as const;

export function RunSection() {
  const { t } = useTranslations();
  const { template, issues, issuesAt, update, change } = useTemplateEditor();
  const raw = template.run;
  // anything but an object (or nothing) is left to the JSON view, rather than replaced by a write
  if (raw !== undefined && raw !== null && !isPlainObject(raw)) {
    return (
      <div className="p-5 md:p-6">
        <NotEditableHere id={fieldId(['run'])} label={t('templates.editor.sections.run.label')} issues={issuesAt(['run'])} />
      </div>
    );
  }
  const run: JsonObject = raw ?? {};
  const path = (key: string): Path => ['run', key];
  const write = (key: string, value: unknown) => change((tpl) => writeOptional(tpl, path(key), value));
  const under = (key: string) => issues.filter((i) => isUnderPath(i.path, pathKey(path(key))));
  const below = (key: string) => under(key).filter((i) => i.path !== pathKey(path(key)));

  return (
    <div className="divide-y divide-border/50">
      <SettingsSection title={t('templates.editor.run.start.title')} description={t('templates.editor.run.start.description')}>
        <div className="space-y-5">
          <CommandEditor />
          <TextControl
            id={fieldId(path('workingDirectory'))}
            label={t('templates.editor.run.workingDirectory.label')}
            hint={t('templates.editor.run.workingDirectory.hint')}
            value={run.workingDirectory}
            issues={issuesAt(path('workingDirectory'))}
            variables={false}
            mono
            onChange={(value) => write('workingDirectory', value)}
          />
        </div>
      </SettingsSection>

      <SettingsSection title={t('templates.editor.run.stop.title')} description={t('templates.editor.run.stop.description')}>
        <StopEditor />
      </SettingsSection>

      <SettingsSection title={t('templates.editor.run.environmentVars.title')} description={t('templates.editor.run.environmentVars.description')}>
        <KeyValueControl
          id={fieldId(path('environmentVars'))}
          path={path('environmentVars')}
          label={t('templates.editor.run.environmentVars.label')}
          hint={t('templates.editor.run.environmentVars.hint')}
          keyPlaceholder={t('templates.editor.run.environmentVars.keyPlaceholder')}
          valuePlaceholder={t('templates.editor.run.environmentVars.valuePlaceholder')}
          newKey="VARIABLE"
          value={run.environmentVars}
          issues={issuesAt(path('environmentVars'))}
          itemIssues={below('environmentVars')}
          variables
          // the last one removed takes the key with it
          onChange={(value) => write('environmentVars', value)}
        />
      </SettingsSection>

      <SettingsSection title={t('templates.editor.run.console.title')} description={t('templates.editor.run.console.description')}>
        <StdinControl
          id={fieldId(path('stdin'))}
          path={path('stdin')}
          label={t('templates.editor.run.console.label')}
          hint={t('templates.editor.run.console.hint')}
          value={run.stdin}
          issues={under('stdin')}
          onChange={(value) => write('stdin', value)}
        />
      </SettingsSection>

      <SettingsSection title={t('templates.editor.run.behaviour.title')} description={t('templates.editor.run.behaviour.description')}>
        <div className="space-y-3">
          {RUN_FLAGS.map((flag) => (
            <BooleanControl
              key={flag}
              id={fieldId(path(flag))}
              label={t(`templates.editor.run.behaviour.${flag}.label`)}
              hint={t(`templates.editor.run.behaviour.${flag}.hint`)}
              value={run[flag]}
              issues={issuesAt(path(flag))}
              // the node writes all three, false included; templates keep them
              onChange={(value) => update(path(flag), value)}
            />
          ))}
          <div className="pt-2 sm:max-w-xs">
            <NumberControl
              id={fieldId(path('expectedExitCode'))}
              label={t('templates.editor.run.behaviour.expectedExitCode.label')}
              hint={t('templates.editor.run.behaviour.expectedExitCode.hint')}
              placeholder={t('templates.editor.steps.defaultValue', { value: '0' })}
              value={run.expectedExitCode}
              issues={issuesAt(path('expectedExitCode'))}
              variables={false}
              onChange={(value) => write('expectedExitCode', value)}
            />
          </div>
        </div>
      </SettingsSection>

      <StepListFold
        path={path('pre')}
        title={t('templates.editor.run.pre.title')}
        note={t('templates.editor.run.optional')}
        description={t('templates.editor.run.pre.description')}
        emptyTitle={t('templates.editor.run.pre.empty.title')}
        emptyDescription={t('templates.editor.run.pre.empty.description')}
      />
      <StepListFold
        path={path('post')}
        title={t('templates.editor.run.post.title')}
        note={t('templates.editor.run.optional')}
        description={t('templates.editor.run.post.description')}
        emptyTitle={t('templates.editor.run.post.empty.title')}
        emptyDescription={t('templates.editor.run.post.empty.description')}
      />
    </div>
  );
}
