import { parseTemplateJson, type JsonProblem } from './document';
import { isPlainObject, type JsonObject } from './paths';
import { TEMPLATE_NAME_MAX_LENGTH } from './template-name';

// Where a template in the editor comes from: a blank one, a saved one, a copy or a .json file.

/** What "Nueva plantilla → En blanco" starts from: the keys every template fills in. */
export function blankTemplate(): JsonObject {
  return {
    name: '',
    display: '',
    type: '',
    data: {},
    install: [],
    run: { command: '', stop: '' },
    environment: { type: 'host' },
  };
}

/** `template` with its name first, so the JSON view opens with it; the other keys keep their order. */
function withName(template: JsonObject, name: string): JsonObject {
  // the first object fixes the key's place, the last one its value
  return Object.assign({ name }, template, { name });
}

/**
 * A template as the API returned it, ready to edit. GET /api/templates/:repo/:name wraps the
 * template with its name (last) and, for repositories, its README: the name goes first, and the
 * README goes, because PUT reads only the template and would drop it on save anyway.
 */
export function prepareLoadedTemplate(raw: JsonObject, name: string): JsonObject {
  const template = { ...raw };
  delete template.readme;
  return withName(template, name);
}

/**
 * A free name for a copy of `name`: the name itself when no local template has it (a community
 * template copied to local keeps its name), else name-copy, name-copy-2… cut to fit the column.
 */
export function copyName(name: string, takenNames: readonly string[]): string {
  const taken = new Set(takenNames);
  if (name !== '' && !taken.has(name)) return name;
  const base = name || 'template';
  for (let n = 1; ; n++) {
    const suffix = n === 1 ? '-copy' : `-copy-${n}`;
    const candidate = [...base].slice(0, TEMPLATE_NAME_MAX_LENGTH - suffix.length).join('') + suffix;
    if (!taken.has(candidate)) return candidate;
  }
}

/**
 * A copy of `source` to save as a new local template. When the name had to change, the display
 * name says it is a copy too (`copyLabel` is the translated word), so both can be told apart.
 */
export function duplicateTemplate(source: JsonObject, sourceName: string, takenNames: readonly string[], copyLabel: string): JsonObject {
  const name = copyName(sourceName, takenNames);
  const copy = prepareLoadedTemplate(source, name);
  if (name !== sourceName && typeof copy.display === 'string' && copy.display !== '') {
    return { ...copy, display: `${copy.display} (${copyLabel})` };
  }
  return copy;
}

/** A template read from a .json file; without a name it takes the file's (minecraft.json → minecraft). */
export function importTemplate(text: string, fileName: string): { template: JsonObject } | { problem: JsonProblem } {
  const parsed = parseTemplateJson(text);
  if ('problem' in parsed) return parsed;
  const name = typeof parsed.template.name === 'string' && parsed.template.name !== '' ? parsed.template.name : fileName.replace(/\.json$/i, '');
  return { template: withName(parsed.template, name) };
}

// The list page reads the file and the editor (another page) opens it: the template crosses over
// in this tab's sessionStorage. It stays there, so reloading the editor opens it again, until
// the next import replaces it.
const IMPORT_KEY = 'aether:template-import';

/** Keeps an imported template for the editor to pick up; false when the browser has no storage. */
export function stashImportedTemplate(template: JsonObject): boolean {
  try {
    sessionStorage.setItem(IMPORT_KEY, JSON.stringify(template));
    return true;
  } catch {
    return false;
  }
}

/** The template stashed by the list page; null when there is none (or no storage). */
export function readImportedTemplate(): JsonObject | null {
  try {
    const text = sessionStorage.getItem(IMPORT_KEY);
    const value: unknown = text ? JSON.parse(text) : null;
    return isPlainObject(value) ? value : null;
  } catch {
    return null;
  }
}

/** What the editor opens, from its address: /templates/editor/?name=…, ?from=<repo>:<name>, ?import. */
export type EditorSource =
  | { kind: 'blank' }
  | { kind: 'local'; name: string }
  | { kind: 'copy'; repoId: number; name: string }
  | { kind: 'import' };

export function editorSource(search: string): EditorSource {
  const params = new URLSearchParams(search);
  const name = params.get('name');
  if (name) return { kind: 'local', name };
  const from = params.get('from');
  if (from) {
    // the repository id never holds a ':', the template name may
    const split = from.indexOf(':');
    const repoId = Number(from.slice(0, split));
    const sourceName = from.slice(split + 1);
    if (split > 0 && Number.isInteger(repoId) && repoId >= 0 && sourceName) return { kind: 'copy', repoId, name: sourceName };
  }
  if (params.has('import')) return { kind: 'import' };
  return { kind: 'blank' };
}
