import { templateSchema } from '../schema/template';
import { issueMessageKey, issuePath } from '../schema/issues';
import { msg } from '../schema/common';
import { pathKey, type JsonObject } from './paths';
import { templateNameProblem } from './template-name';
import { templateWarnings, type WarningRule } from './warnings';

// Live validation of the template being edited: the schema's errors, the name rules and the
// warning rules, as one list the form, the section nav and the JSON view all read.

export type IssueSeverity = 'error' | 'warning';

export interface EditorIssue {
  /** Dotted path of what the issue is about ("run.pre.2.target"); "" is the whole template. */
  path: string;
  /** i18n key of the message. */
  messageKey: string;
  params?: Record<string, string | number>;
  severity: IssueSeverity;
  /** Saving is disabled while any blocking issue is left. */
  blocking: boolean;
}

export interface ValidationContext {
  /**
   * Local templates that already exist, so a new name does not overwrite one of them; null when
   * they could not be read, which blocks saving (any name might overwrite one).
   */
  takenNames?: readonly string[] | null;
  /** The name the editor opened the template under; keeping it is not taking a name. */
  originalName?: string;
  warningRules?: readonly WarningRule[];
}

const UNKNOWN_OPERATION = msg('unknownOperation');
const STEP_NOT_OBJECT = msg('stepNotObject');

/**
 * How bad a schema issue is. Most errors are left to the node, which validates again on save
 * and whose answer is shown, so a check here that is stricter than the node never locks anyone
 * out. Blocking is kept for what the panel cannot store in any useful way: a step that is not
 * even an object. An unknown step type is only a warning: a newer node may know it.
 */
function schemaSeverity(messageKey: string): Pick<EditorIssue, 'severity' | 'blocking'> {
  if (messageKey === UNKNOWN_OPERATION) return { severity: 'warning', blocking: false };
  return { severity: 'error', blocking: messageKey === STEP_NOT_OBJECT };
}

export function validateTemplate(template: JsonObject, context: ValidationContext = {}): EditorIssue[] {
  const issues: EditorIssue[] = [];

  const { takenNames, originalName } = context;
  const nameProblem = templateNameProblem(template.name, { takenNames: takenNames ?? [], originalName });
  if (nameProblem) issues.push({ path: 'name', messageKey: msg(nameProblem), severity: 'error', blocking: true });
  else if (takenNames === null) issues.push({ path: 'name', messageKey: msg('namesUnchecked'), severity: 'error', blocking: true });

  const result = templateSchema.safeParse(template);
  if (!result.success) {
    for (const issue of result.error.issues) {
      const messageKey = issueMessageKey(issue);
      issues.push({ path: issuePath(issue), messageKey, ...schemaSeverity(messageKey) });
    }
  }

  for (const warning of templateWarnings(template, context.warningRules)) {
    issues.push({
      path: pathKey(warning.path),
      messageKey: `templates.editor.warnings.${warning.code}`,
      params: warning.params,
      severity: 'warning',
      blocking: false,
    });
  }

  return issues;
}

export interface IssueCounts {
  errors: number;
  warnings: number;
}

export function countIssues(issues: readonly EditorIssue[]): IssueCounts {
  let errors = 0;
  for (const issue of issues) if (issue.severity === 'error') errors++;
  return { errors, warnings: issues.length - errors };
}

/** Whether `path` is `prefix` or lies under it ("run.pre.0" is under "run", "runner" is not). */
export function isUnderPath(path: string, prefix: string): boolean {
  return prefix === '' || path === prefix || path.startsWith(`${prefix}.`);
}

/** The issues grouped by their exact path, for fields that show their own messages. */
export function issuesByPath(issues: readonly EditorIssue[]): ReadonlyMap<string, EditorIssue[]> {
  const map = new Map<string, EditorIssue[]>();
  for (const issue of issues) {
    const list = map.get(issue.path);
    if (list) list.push(issue);
    else map.set(issue.path, [issue]);
  }
  return map;
}
