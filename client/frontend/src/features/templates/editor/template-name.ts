// The rules a local template's name has to follow, taken from what the panel does with it:
// PUT /api/templates/0/:name stores it as the primary key of a varchar(100) column (internal/
// models/template.go), and it travels as one segment of that URL. Gin matches routes on the
// decoded path, so a "/" (even sent as %2F) splits it and the request never reaches the handler,
// and the browser resolves "." and ".." segments away before sending. Anything else goes, once
// encoded. Whitespace at either end and control characters are refused too: the name is an id
// typed in links and searches, and such a name could not be told apart from its trimmed twin.

/** The most characters the templates table keeps for a name. */
export const TEMPLATE_NAME_MAX_LENGTH = 100;

// control characters: C0, DEL and C1
const CONTROL_CHARACTER = /[\u0000-\u001f\u007f-\u009f]/;

/** Why a name cannot be saved, as the code of its message under templates.editor.validation. */
export type TemplateNameProblem = 'nameRequired' | 'nameTooLong' | 'nameSlash' | 'nameDots' | 'nameWhitespace' | 'nameControl' | 'nameTaken';

// The locale is fixed: the browser's own would change the answer (Spanish keeps ñ apart from n,
// Swedish ä from a, Turkish I from i) while the database answers the same for everyone. "en"
// orders by the root collation; "und" is no help, Intl swaps it for the browser's locale.
const NAME_COLLATOR = new Intl.Collator('en', { sensitivity: 'base' });

/**
 * Whether two names reach the same stored template. The panel's MariaDB compares the name column
 * with its default collation, which ignores case and accents (ñ = n, ä = a, I = i) and trailing
 * spaces (PAD SPACE): PUT .../Paper finds the row "paper" and replaces it. So names are told
 * apart the way the database does, not character by character.
 */
export function sameTemplateName(a: string, b: string): boolean {
  return NAME_COLLATOR.compare(a.trimEnd(), b.trimEnd()) === 0;
}

/**
 * The name a save ends up stored under. The backend updates the row it finds and keeps that row's
 * name, so for the opened template a change of case or accents alone (paper → Paper) stays
 * "paper"; the editor saves and shows that name instead of pretending it was renamed.
 */
export function storedTemplateName(name: string, originalName: string | undefined): string {
  return originalName !== undefined && sameTemplateName(name, originalName) ? originalName : name;
}

/** Whether a local template in `names` would be reached by saving under `name`. */
export function nameIsTaken(name: string, names: readonly string[]): boolean {
  return names.some((taken) => sameTemplateName(taken, name));
}

/**
 * The first problem with `name`, or undefined when it can be saved. `takenNames` are the local
 * templates that already exist; saving under one of them would overwrite it, so a name is taken
 * unless it reaches the template the editor opened (`originalName`): the save updates that row.
 */
export function templateNameProblem(
  name: unknown,
  { takenNames = [], originalName }: { takenNames?: readonly string[]; originalName?: string } = {},
): TemplateNameProblem | undefined {
  if (typeof name !== 'string' || name === '') return 'nameRequired';
  if ([...name].length > TEMPLATE_NAME_MAX_LENGTH) return 'nameTooLong';
  if (name.includes('/')) return 'nameSlash';
  if (name === '.' || name === '..') return 'nameDots';
  if (name.trim() !== name) return 'nameWhitespace';
  if (CONTROL_CHARACTER.test(name)) return 'nameControl';
  if (originalName !== undefined && sameTemplateName(name, originalName)) return undefined;
  if (nameIsTaken(name, takenNames)) return 'nameTaken';
  return undefined;
}
