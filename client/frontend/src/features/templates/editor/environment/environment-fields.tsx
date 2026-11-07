import { useTranslations } from '@/contexts/translations-context';
import { ENVIRONMENT_TYPES } from '../../schema/template';
import { AdvancedFold } from '../advanced-fold';
import { fieldId, useTemplateEditor } from '../editor-context';
import { isPlainObject, pathKey, type JsonObject, type Path } from '../paths';
import { NotEditableHere, SelectControl, SuggestControl, TextControl } from '../steps/field-controls';
import { KeyValueControl } from '../steps/key-value-control';
import { isEmptyValue } from '../steps/model';
import { StringListControl } from '../steps/string-list-control';
import { isUnderPath } from '../validation';
import { JsonBlock } from '../json-block';
import { writeOptional } from '../write';
import { DEFAULT_DOCKER_IMAGE } from './images';

// One environment (the default one, or one of the supported ones): its type and, for Docker, the
// container's settings as internal/servers/docker.Docker reads them. Keys the chosen type does
// not use (Docker settings on a host environment) are kept and named, never dropped.

/** The keys Docker reads, in the order the form shows them. */
const DOCKER_KEYS = ['image', 'portBindings', 'networkName', 'containerRoot', 'bindings', 'labels', 'hostConfig', 'config'] as const;
const DOCKER_ADVANCED = ['networkName', 'containerRoot', 'bindings', 'labels', 'hostConfig', 'config'] as const;

/** tty and standard are older names of host: shown, but not offered for new templates. */
const OFFERED_TYPES = ['host', 'docker'] as const;

/** Which explanation the type select shows: Docker's, the host's (and its older names), or none chosen. */
function hintFor(type: string): 'docker' | 'host' | 'unset' {
  if (type === 'docker') return 'docker';
  return type === '' ? 'unset' : 'host';
}

export function EnvironmentFields({ path, value, required = false, prune = false }: Readonly<{
  /** ['environment'] or ['supportedEnvironments', i]. */
  path: Path;
  value: unknown;
  /** A supported environment needs a type; the default one may go without (a warning). */
  required?: boolean;
  /** Remove the whole object once its last key is cleared (the default environment). */
  prune?: boolean;
}>) {
  const { t } = useTranslations();
  const { issues, issuesAt, change } = useTemplateEditor();
  const id = fieldId(path);
  if (value !== undefined && value !== null && !isPlainObject(value)) {
    return <NotEditableHere id={id} label={t('templates.editor.environment.type.label')} issues={issuesAt(path)} />;
  }
  const environment: JsonObject = isPlainObject(value) ? value : {};
  const type = typeof environment.type === 'string' ? environment.type : '';
  const at = (key: string): Path => [...path, key];
  const write = (key: string, next: unknown) => change((tpl) => writeOptional(tpl, at(key), next, prune));
  const below = (key: string) => issues.filter((i) => i.path !== pathKey(at(key)) && isUnderPath(i.path, pathKey(at(key))));
  const docker = type === 'docker';
  const typeOptions = (ENVIRONMENT_TYPES as readonly string[])
    .filter((option) => (OFFERED_TYPES as readonly string[]).includes(option) || option === type)
    .map((option) => ({ value: option, labelKey: `templates.editor.environment.types.${option}` }));

  // what this type does not read, kept as it is and named
  const shown = new Set<string>(['type', ...(docker ? DOCKER_KEYS : [])]);
  const extra = Object.keys(environment).filter((key) => !shown.has(key));
  const advancedInUse = DOCKER_ADVANCED.some((key) => !isEmptyValue(environment[key]) || issues.some((i) => isUnderPath(i.path, pathKey(at(key)))));

  return (
    <div className="space-y-5">
      <SelectControl
        id={fieldId(at('type'))}
        label={t('templates.editor.environment.type.label')}
        required={required}
        hint={t(`templates.editor.environment.type.hint.${hintFor(type)}`)}
        value={environment.type}
        issues={issuesAt(at('type'))}
        options={typeOptions}
        optionLabel={(option) => t(option.labelKey ?? option.value)}
        onChange={(next) => write('type', next)}
      />
      {docker && (
        <>
          <TextControl
            id={fieldId(at('image'))}
            label={t('templates.editor.environment.image.label')}
            hint={t('templates.editor.environment.image.hint')}
            placeholder={t('templates.editor.steps.defaultValue', { value: DEFAULT_DOCKER_IMAGE })}
            value={environment.image}
            issues={issuesAt(at('image'))}
            variables
            mono
            onChange={(next) => write('image', next)}
          />
          <StringListControl
            id={fieldId(at('portBindings'))}
            path={at('portBindings')}
            label={t('templates.editor.environment.portBindings.label')}
            hint={t('templates.editor.environment.portBindings.hint')}
            placeholder="0.0.0.0:${port}:${port}/tcp"
            value={environment.portBindings}
            issues={issuesAt(at('portBindings'))}
            itemIssues={below('portBindings')}
            variables
            onChange={(next) => write('portBindings', next)}
          />
          <AdvancedFold
            id={`${fieldId(path)}-advanced`}
            label={t('templates.editor.environment.advanced')}
            summary={DOCKER_ADVANCED.join(', ')}
            inUse={advancedInUse}
            holds={(target) => DOCKER_ADVANCED.some((key) => isUnderPath(pathKey(target), pathKey(at(key))))}
          >
            <div className="grid gap-5 sm:grid-cols-2">
              <SuggestControl
                id={fieldId(at('networkName'))}
                label={t('templates.editor.environment.networkName.label')}
                hint={t('templates.editor.environment.networkName.hint')}
                placeholder={t('templates.editor.steps.defaultValue', { value: 'bridge' })}
                value={environment.networkName}
                issues={issuesAt(at('networkName'))}
                variables
                options={[{ value: 'bridge' }, { value: 'host' }]}
                optionLabel={(option) => option.value}
                onChange={(next) => write('networkName', next)}
              />
              <TextControl
                id={fieldId(at('containerRoot'))}
                label={t('templates.editor.environment.containerRoot.label')}
                hint={t('templates.editor.environment.containerRoot.hint')}
                placeholder={t('templates.editor.steps.defaultValue', { value: '/SkyPanel' })}
                value={environment.containerRoot}
                issues={issuesAt(at('containerRoot'))}
                variables={false}
                mono
                onChange={(next) => write('containerRoot', next)}
              />
              <div className="min-w-0 sm:col-span-2">
                <KeyValueControl
                  id={fieldId(at('bindings'))}
                  path={at('bindings')}
                  label={t('templates.editor.environment.bindings.label')}
                  hint={t('templates.editor.environment.bindings.hint')}
                  keyPlaceholder="/srv/shared"
                  valuePlaceholder="/data"
                  newKey={t('templates.editor.environment.bindings.newKey')}
                  value={environment.bindings}
                  issues={issuesAt(at('bindings'))}
                  itemIssues={below('bindings')}
                  variables={false}
                  onChange={(next) => write('bindings', next)}
                />
              </div>
              <div className="min-w-0 sm:col-span-2">
                <KeyValueControl
                  id={fieldId(at('labels'))}
                  path={at('labels')}
                  label={t('templates.editor.environment.labels.label')}
                  hint={t('templates.editor.environment.labels.hint')}
                  newKey="label"
                  value={environment.labels}
                  issues={issuesAt(at('labels'))}
                  itemIssues={below('labels')}
                  variables
                  onChange={(next) => write('labels', next)}
                />
              </div>
              {(['hostConfig', 'config'] as const).map((key) => (
                <div key={key} className="min-w-0 sm:col-span-2">
                  <JsonBlock
                    id={fieldId(at(key))}
                    label={t(`templates.editor.environment.${key}.label`)}
                    hint={t(`templates.editor.environment.${key}.hint`)}
                    value={environment[key]}
                    issues={issues.filter((i) => isUnderPath(i.path, pathKey(at(key))))}
                    onApply={(next) => write(key, next)}
                  />
                </div>
              ))}
            </div>
          </AdvancedFold>
        </>
      )}
      {extra.length > 0 && (
        <p className="text-pretty text-xs text-muted-foreground">
          {t(docker ? 'templates.editor.environment.extraKeys' : 'templates.editor.environment.extraKeysHost', { keys: extra.join(', ') })}
        </p>
      )}
    </div>
  );
}
