import { AlertTriangle } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import type { OperationDefinition } from '../../schema/operations';
import { fieldId, useTemplateEditorActions } from '../editor-context';
import { isPlainObject, pathKey, type JsonObject, type Path } from '../paths';
import type { EditorIssue } from '../validation';
import { ConditionField } from './condition-field';
import { OperationForm } from './operation-form';
import { StepJsonEditor } from './step-json';

interface StepCardBodyProps {
  /** The element id the card's toggle controls. */
  id: string;
  step: unknown;
  stepPath: Path;
  /** The step's type as written, when it has one. */
  type: string | undefined;
  /** The operation that type names, when the node knows it. */
  operation: OperationDefinition | undefined;
  /** The step shows as JSON instead of its form. */
  json: boolean;
  /** The issues under the step. */
  issues: readonly EditorIssue[];
}

/** An open step card: its form and condition, or its JSON. */
export function StepCardBody({ id, step, stepPath, type, operation, json, issues }: Readonly<StepCardBodyProps>) {
  const { t } = useTranslations();
  const isObject = isPlainObject(step);
  return (
    <div id={id} className="space-y-5 border-t border-border/50 px-3 py-4 sm:px-5 sm:py-5">
      {operation && !operation.templates && (
        <p className="flex items-start gap-2 rounded-lg border border-warning/30 bg-warning/10 px-3 py-2.5 text-xs text-warning">
          <AlertTriangle className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          <span className="text-pretty">{t('templates.editor.steps.taskOnly')}</span>
        </p>
      )}
      {json ? (
        <StepJson step={step} stepPath={stepPath} type={type} hasForm={isObject && operation !== undefined} issues={issues} />
      ) : (
        isObject && operation && <StepForm operation={operation} step={step} stepPath={stepPath} issues={issues} />
      )}
    </div>
  );
}

/** The step as JSON, with a word on why when it has no form. */
function StepJson({
  step,
  stepPath,
  type,
  hasForm,
  issues,
}: Readonly<{ step: unknown; stepPath: Path; type: string | undefined; hasForm: boolean; issues: readonly EditorIssue[] }>) {
  const { t } = useTranslations();
  const { update } = useTemplateEditorActions();
  return (
    <>
      {!hasForm && (
        <p className="text-pretty text-xs text-muted-foreground">
          {t(isPlainObject(step) ? 'templates.editor.steps.json.unknownHint' : 'templates.editor.steps.json.invalidHint', { type: type ?? '' })}
        </p>
      )}
      <StepJsonEditor id={fieldId(stepPath)} step={step} stepPath={stepPath} issues={issues} onApply={(next) => update(stepPath, next)} />
    </>
  );
}

/** The operation's form, then the step's condition (`if`), each with its own issues. */
function StepForm({
  operation,
  step,
  stepPath,
  issues,
}: Readonly<{ operation: OperationDefinition; step: JsonObject; stepPath: Path; issues: readonly EditorIssue[] }>) {
  const { update } = useTemplateEditorActions();
  const ifPath = [...stepPath, 'if'];
  const ifIssues = issues.filter((i) => i.path === pathKey(ifPath));
  const formIssues = issues.filter((i) => !ifIssues.includes(i));
  return (
    <>
      <OperationForm operation={operation} step={step} stepPath={stepPath} issues={formIssues} />
      <div className="border-t border-border/50 pt-5">
        <ConditionField id={fieldId(ifPath)} value={step.if} issues={ifIssues} onChange={(next) => update(ifPath, next)} />
      </div>
    </>
  );
}
