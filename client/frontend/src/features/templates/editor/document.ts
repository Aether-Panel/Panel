import { deleteIn, isPlainObject, setIn, type JsonObject, type Path } from './paths';

// The editor's state: one template object and the JSON text showing it. The form and the JSON
// view are two views of the same object. A form edit patches the object and rewrites the text;
// a JSON edit keeps the text as typed and replaces the object only when the text parses to one,
// so half-typed JSON never reaches the form, which keeps showing the last valid object.

/** Why the JSON text is not the template: it does not parse, or it is not an object. */
export type JsonProblem = { code: 'syntax'; detail: string } | { code: 'notObject' };

export interface EditorDocument {
  /** The last valid template: what the form shows, what is validated and what is saved. */
  template: JsonObject;
  /** The JSON view's text, which may be mid-edit and not parse. */
  jsonText: string;
  jsonProblem: JsonProblem | null;
  /** The template as loaded or last saved, compact, to tell whether there are unsaved changes. */
  baseline: string;
}

/** The text the JSON view shows and the download holds. */
export function serializeTemplate(template: JsonObject): string {
  return JSON.stringify(template, null, 2);
}

/**
 * A document for `template`. `saved` says whether it already exists as it is: a copy or an
 * imported file does not, so it counts as unsaved from the start and leaving asks first.
 */
export function createDocument(template: JsonObject, saved = true): EditorDocument {
  return { template, jsonText: serializeTemplate(template), jsonProblem: null, baseline: saved ? JSON.stringify(template) : '' };
}

/**
 * Replaces the template through `change` (any of the paths.ts helpers). The JSON text is rewritten
 * from the new object, so a form edit made while the JSON does not parse drops that broken text.
 * A change that leaves the object as it was keeps the document as it is.
 */
export function changeTemplate(doc: EditorDocument, change: (template: JsonObject) => JsonObject): EditorDocument {
  const template = change(doc.template);
  if (template === doc.template) return doc;
  return { ...doc, template, jsonText: serializeTemplate(template), jsonProblem: null };
}

/** Sets the value at `path`; undefined removes the key, as an emptied optional field should. */
export function updateAt(doc: EditorDocument, path: Path, value: unknown): EditorDocument {
  return changeTemplate(doc, (t) => (value === undefined ? deleteIn(t, path) : setIn(t, path, value)));
}

export function removeAt(doc: EditorDocument, path: Path): EditorDocument {
  return changeTemplate(doc, (t) => deleteIn(t, path));
}

/** The template a JSON text holds, or why it holds none. */
export function parseTemplateJson(text: string): { template: JsonObject } | { problem: JsonProblem } {
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch (e) {
    return { problem: { code: 'syntax', detail: e instanceof Error ? e.message : String(e) } };
  }
  return isPlainObject(value) ? { template: value } : { problem: { code: 'notObject' } };
}

/** A JSON view edit: the text stays as typed; the template follows it while it is valid. */
export function setJsonText(doc: EditorDocument, jsonText: string): EditorDocument {
  const parsed = parseTemplateJson(jsonText);
  if ('problem' in parsed) return { ...doc, jsonText, jsonProblem: parsed.problem };
  return { ...doc, jsonText, jsonProblem: null, template: parsed.template };
}

/**
 * After a save: what was saved is the new baseline. The saved template is passed in because
 * edits made while the request was on its way are not saved yet.
 */
export function markSaved(doc: EditorDocument, saved: JsonObject): EditorDocument {
  return { ...doc, baseline: JSON.stringify(saved) };
}

/** Unsaved changes: the template differs from the baseline, or the JSON text is broken. */
export function isDirty(doc: EditorDocument): boolean {
  return doc.jsonProblem !== null || JSON.stringify(doc.template) !== doc.baseline;
}
