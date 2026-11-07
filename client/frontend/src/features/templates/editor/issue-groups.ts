import { isPlainObject, type JsonObject, type Path } from './paths';
import { SECTION_KEYS, sectionOfPath, type EditorSectionId } from './section-keys';
import type { EditorIssue } from './validation';

// The validation summary's list: every issue under the section that edits it, the ones that
// block saving first, so the panel reads as "what to fix, then what to check".

export interface IssueGroup {
  /** The section that edits these issues' fields; null for keys no section edits (JSON only). */
  section: EditorSectionId | null;
  issues: EditorIssue[];
}

/** Blocking errors, then other errors, then warnings. */
function rank(issue: EditorIssue): number {
  if (issue.blocking) return 0;
  return issue.severity === 'error' ? 1 : 2;
}

const SECTION_ORDER = Object.keys(SECTION_KEYS) as EditorSectionId[];

/**
 * The issues by section: sections holding something that blocks saving come first, then the
 * rest in menu order (keys no section edits last). Inside a section, blocking errors, errors and
 * warnings, each in the order validation found them.
 */
export function groupIssues(issues: readonly EditorIssue[]): IssueGroup[] {
  const bySection = new Map<EditorSectionId | null, EditorIssue[]>();
  for (const issue of issues) {
    const section = sectionOfPath(issue.path);
    const list = bySection.get(section);
    if (list) list.push(issue);
    else bySection.set(section, [issue]);
  }
  const order = (section: EditorSectionId | null) => (section === null ? SECTION_ORDER.length : SECTION_ORDER.indexOf(section));
  return [...bySection.entries()]
    // sort is stable: equal ranks keep validation's order
    .map(([section, list]) => ({ section, issues: [...list].sort((a, b) => rank(a) - rank(b)) }))
    .sort((a, b) => {
      const blockingFirst = Number(b.issues.some((i) => i.blocking)) - Number(a.issues.some((i) => i.blocking));
      return blockingFirst || order(a.section) - order(b.section);
    });
}

/**
 * The path goTo() takes for an issue's dotted path, read against the template: a step into a list
 * is an index (install.2), a step into an object a key, even when it looks like a number (a
 * variable called "10"). Past what the template holds, digits count as an index.
 */
export function pathFromKey(template: JsonObject, key: string): Path {
  if (key === '') return [];
  const path: (string | number)[] = [];
  let current: unknown = template;
  for (const part of key.split('.')) {
    const index = /^\d+$/.test(part) ? Number(part) : null;
    if (Array.isArray(current) && index !== null) {
      path.push(index);
      current = current[index];
    } else if (isPlainObject(current)) {
      path.push(part);
      current = Object.hasOwn(current, part) ? current[part] : undefined;
    } else {
      path.push(index ?? part);
      current = undefined;
    }
  }
  return path;
}
