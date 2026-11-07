import { Check } from '@/components/icons';
import { SettingsSection } from '@/features/settings/general-tab';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { fieldId, useTemplateEditor } from '../editor-context';
import { FieldIssues, issuesId } from '../field';
import { getIn, isPlainObject, pathKey, type JsonObject, type Path } from '../paths';
import { NotEditableHere } from '../steps/field-controls';
import { StringListControl } from '../steps/string-list-control';
import { isUnderPath } from '../validation';
import { writeOptional } from '../write';
import { KNOWN_ARCH, KNOWN_OS, parseAlternatives, toggleAlternative } from './model';

// Requisitos: what a node needs to install the template (Requirements.Test). The node refuses to
// create the server elsewhere: on another system or architecture, or (on host) without one of
// the programs listed.

/** One requirement row as toggle chips: any of the values chosen will do; none chosen, any node. */
function AlternativesField({ path, labelKey, known }: Readonly<{ path: Path; labelKey: string; known: readonly string[] }>) {
  const { t } = useTranslations();
  const { template, issuesAt, change } = useTemplateEditor();
  const id = fieldId(path);
  const value = (isPlainObject(template.requirements) ? template.requirements : {})[String(path[1])];
  const issues = issuesAt(path);
  if (value !== undefined && typeof value !== 'string') return <NotEditableHere id={id} label={t(`${labelKey}.label`)} issues={issues} />;
  const chosen = parseAlternatives(value);
  // a value the panel does not list (freebsd, ppc64le) stays, as a chip of its own
  const options = [...known, ...chosen.filter((v) => !known.includes(v))];
  const labelId = `${id}-label`;
  const hintId = `${id}-hint`;

  return (
    <div className="space-y-2">
      <p id={labelId} className="text-sm font-medium leading-none">{t(`${labelKey}.label`)}</p>
      <div role="group" aria-labelledby={labelId} aria-describedby={`${hintId} ${issuesId(id)}`} className="flex flex-wrap gap-2">
        {options.map((option, index) => {
          const on = chosen.includes(option);
          return (
            <button
              key={option}
              // goTo() lands on the first chip
              id={index === 0 ? id : undefined}
              type="button"
              aria-pressed={on}
              // the row as it is when the change applies, not as this render saw it
              onClick={() => change((tpl) => writeOptional(tpl, path, toggleAlternative(getIn(tpl, path), option), true))}
              className={cn(
                'inline-flex h-8 items-center gap-1.5 rounded-lg border px-3 text-sm transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70',
                on ? 'border-primary/50 bg-primary/10 text-foreground' : 'border-border/70 text-muted-foreground hover:bg-accent/50 hover:text-foreground',
              )}
            >
              {on && <Check className="h-3.5 w-3.5 text-primary-bright" aria-hidden="true" />}
              <span>{known.includes(option) ? t(`${labelKey}.values.${option}`) : option}</span>
            </button>
          );
        })}
      </div>
      <p id={hintId} className="text-pretty text-xs text-muted-foreground">
        {chosen.length === 0 ? t('templates.editor.requirements.any') : t(`${labelKey}.hint`, { value: chosen.join(' || ') })}
      </p>
      <FieldIssues id={issuesId(id)} issues={issues} />
    </div>
  );
}

export function RequirementsSection() {
  const { t } = useTranslations();
  const { template, issues, issuesAt, change } = useTemplateEditor();
  const raw = template.requirements;
  if (raw !== undefined && raw !== null && !isPlainObject(raw)) {
    return (
      <div className="p-5 md:p-6">
        <NotEditableHere id={fieldId(['requirements'])} label={t('templates.editor.sections.requirements.label')} issues={issuesAt(['requirements'])} />
      </div>
    );
  }
  const requirements: JsonObject = raw ?? {};
  const binaries: Path = ['requirements', 'binaries'];
  const known = new Set(['os', 'arch', 'binaries']);
  const extra = Object.keys(requirements).filter((key) => !known.has(key));

  return (
    <div className="divide-y divide-border/50">
      <SettingsSection title={t('templates.editor.requirements.platform.title')} description={t('templates.editor.requirements.platform.description')}>
        <div className="space-y-6">
          <AlternativesField path={['requirements', 'os']} labelKey="templates.editor.requirements.os" known={KNOWN_OS} />
          <AlternativesField path={['requirements', 'arch']} labelKey="templates.editor.requirements.arch" known={KNOWN_ARCH} />
        </div>
      </SettingsSection>
      <SettingsSection title={t('templates.editor.requirements.binaries.title')} description={t('templates.editor.requirements.binaries.description')}>
        <div className="space-y-3">
          <StringListControl
            id={fieldId(binaries)}
            path={binaries}
            label={t('templates.editor.requirements.binaries.label')}
            hint={t('templates.editor.requirements.binaries.hint')}
            placeholder="java || java21"
            value={requirements.binaries}
            issues={issuesAt(binaries)}
            itemIssues={issues.filter((i) => i.path !== pathKey(binaries) && isUnderPath(i.path, pathKey(binaries)))}
            variables
            onChange={(next) => change((tpl) => writeOptional(tpl, binaries, next, true))}
          />
          {extra.length > 0 && (
            <p className="text-pretty text-xs text-muted-foreground">{t('templates.editor.requirements.extraKeys', { keys: extra.join(', ') })}</p>
          )}
        </div>
      </SettingsSection>
    </div>
  );
}
