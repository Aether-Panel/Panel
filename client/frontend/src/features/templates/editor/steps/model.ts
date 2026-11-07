import type { OperationDefinition } from '../../schema/operations';
import type { OperationField } from '../../schema/fields';
import { deleteIn, getIn, isPlainObject, setIn, type JsonObject, type Path } from '../paths';

// The pure side of the step lists (install, uninstall, run.pre, run.post): reading a step's
// arguments the way the node does and writing them back without touching anything else. The
// forms only ever change the keys they edit, so unknown arguments survive every edit.

/** The steps under `path`; a missing or null list (Go writes nil lists as null) is empty. */
export function stepsAt(template: JsonObject, path: Path): readonly unknown[] {
  const list = getIn(template, path);
  return Array.isArray(list) ? list : [];
}

/** A step's type, when it is an object with a string type. */
export function stepType(step: unknown): string | undefined {
  return isPlainObject(step) && typeof step.type === 'string' ? step.type : undefined;
}

/**
 * A new step of an operation: only its type. Required arguments have no value the node would
 * take as a default, so nothing is made up; the form shows them as required until filled in.
 */
export function newStep(operation: OperationDefinition): JsonObject {
  return { type: operation.key };
}

/** Whether a step holds anything besides its type, so deleting it loses something. */
export function stepHasContent(step: unknown): boolean {
  if (!isPlainObject(step)) return step !== undefined;
  return Object.keys(step).some((key) => key !== 'type');
}

/** The key that holds the field's value now: its own name, else an older spelling in use. */
export function fieldKey(step: JsonObject, field: OperationField): string {
  if (Object.hasOwn(step, field.name)) return field.name;
  return field.aliases?.find((alias) => Object.hasOwn(step, alias)) ?? field.name;
}

/** The value the form shows: the field's own key, else an older spelling (steamgamedl's appID). */
export function readField(step: JsonObject, field: OperationField): unknown {
  return step[fieldKey(step, field)];
}

/**
 * What clearing a field writes: nothing (the key goes away), except for an argument whose
 * factory reads "" as not given (emptyIsUnset: curseforge's fileId), where "" keeps its place.
 * Any other "" would be a value the node reads, or rejects.
 */
export function clearedValue(field: OperationField): '' | undefined {
  return field.emptyIsUnset ? '' : undefined;
}

/** An empty text, list or map: what a cleared control hands back. */
export function isEmptyValue(value: unknown): boolean {
  if (value === undefined || value === null || value === '') return true;
  if (Array.isArray(value)) return value.length === 0;
  return isPlainObject(value) && Object.keys(value).length === 0;
}

/**
 * The step with `value` for `field`, written under the field's own name:
 * - an empty value clears the field (see clearedValue);
 * - a switch turned back to the node's default removes the key, as if never set;
 * - an older spelling the value was under is replaced in place by the field's name (and any
 *   other spelling removed), so editing `appID` leaves one `appId` where it was.
 * Every other key of the step stays as it was.
 */
export function writeField(step: JsonObject, field: OperationField, value: unknown): JsonObject {
  let next: unknown = value;
  if (isEmptyValue(value)) next = clearedValue(field);
  else if (field.widget.kind === 'boolean' && !field.required && value === field.defaultValue) next = undefined;

  const aliases = (field.aliases ?? []).filter((alias) => Object.hasOwn(step, alias));
  if (aliases.length === 0) {
    if (next === undefined) return Object.hasOwn(step, field.name) ? deleteIn(step, [field.name]) : step;
    return setIn(step, [field.name], next);
  }

  // the field's name takes the place of the first spelling in use, unless it is there already
  const renameAt = Object.hasOwn(step, field.name) ? field.name : aliases[0];
  const entries: [string, unknown][] = [];
  for (const [key, current] of Object.entries(step)) {
    if (key === renameAt) {
      if (next !== undefined) entries.push([field.name, next]);
    } else if (!aliases.includes(key)) {
      entries.push([key, current]);
    }
  }
  return Object.fromEntries(entries);
}

/** writeField on the step at `stepPath` of `template`; anything but an object there is left alone. */
export function writeFieldIn(template: JsonObject, stepPath: Path, field: OperationField, value: unknown): JsonObject {
  const step = getIn(template, stepPath);
  if (!isPlainObject(step)) return template;
  const next = writeField(step, field, value);
  return next === step ? template : setIn(template, stepPath, next);
}

/**
 * The value a number field's text stands for. Digits become a number, unless the value was a
 * string before ("25565" stays a string: the node casts either); anything else stays the text
 * typed, so `${port}` works where variables do and a typo shows its error instead of vanishing.
 * Empty text is undefined: the field is cleared.
 */
export function numberFromText(text: string, previous: unknown): number | string | undefined {
  if (text === '') return undefined;
  if (/^-?\d+$/.test(text) && typeof previous !== 'string') {
    const n = Number(text);
    if (Number.isSafeInteger(n)) return n;
  }
  return text;
}

/** The list at `listPath` with `step` inserted at `index` (the list is created when missing). */
export function insertStep(template: JsonObject, listPath: Path, index: number, step: unknown): JsonObject {
  const list = stepsAt(template, listPath).slice();
  list.splice(Math.max(0, Math.min(index, list.length)), 0, step);
  return setIn(template, listPath, list);
}

/**
 * The template without the step at `index`. Emptying the list removes its key, as clearing a
 * field does: the node reads a missing list and an empty one alike.
 */
export function removeStep(template: JsonObject, listPath: Path, index: number): JsonObject {
  const list = stepsAt(template, listPath);
  if (index < 0 || index >= list.length) return template;
  if (list.length === 1) return deleteIn(template, listPath);
  return deleteIn(template, [...listPath, index]);
}

/** A deep copy of a step (it is plain JSON), so editing the copy leaves the original alone. */
export function copyStep(step: unknown): unknown {
  return step === undefined ? undefined : JSON.parse(JSON.stringify(step));
}

/** `base`, else `base_2`, `base_3`… whichever is not taken: the name of a new map entry. */
export function uniqueEntryKey(base: string, taken: Iterable<string>): string {
  const used = new Set(taken);
  if (!used.has(base)) return base;
  let n = 2;
  while (used.has(`${base}_${n}`)) n++;
  return `${base}_${n}`;
}

/** The map with `from` renamed to `to` in place; the same map when `to` is taken or `from` missing. */
export function renameEntry(object: JsonObject, from: string, to: string): JsonObject {
  if (from === to || !Object.hasOwn(object, from) || Object.hasOwn(object, to)) return object;
  return Object.fromEntries(Object.entries(object).map(([key, value]) => [key === from ? to : key, value]));
}

/** A console configuration with `key` set to `value`, or without it when the value is empty. */
export function withStdinKey(config: JsonObject, key: string, value: unknown): JsonObject {
  const next = { ...config };
  if (value === undefined || value === '') delete next[key];
  else next[key] = value;
  return next;
}
