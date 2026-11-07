import { booleanValue } from '@/lib/template-boolean';
import { VARIABLE_KEY } from '../../schema/common';
import { VARIABLE_TYPES } from '../../schema/template';
import { isPlainObject, setIn, type JsonObject } from '../paths';
import { templateVariables } from './usage';

// Editing one variable: creating, copying and inserting it, changing its type and reading or
// writing its default value and options in the JSON types the node expects. Pure, so the
// section's components stay thin and these rules are unit-tested.

export type VariableType = (typeof VARIABLE_TYPES)[number];

/** What a new variable starts as: the fields every community template writes. */
export function newVariable(): JsonObject {
  return { type: 'string', value: '', display: '', desc: '', required: false, userEdit: true };
}

/** The variable's type as the node reads it: missing or "" is text. */
export function variableType(variable: JsonObject): string {
  return typeof variable.type === 'string' && variable.type !== '' ? variable.type : 'string';
}

export function isKnownType(type: string): type is VariableType {
  return (VARIABLE_TYPES as readonly string[]).includes(type);
}

/**
 * The first of `base`, `base2`, `base3`… that is not taken. `base` is cleaned up to the
 * characters a key may hold, so a copy of any key is a valid key.
 */
export function uniqueVariableKey(base: string, taken: Iterable<string>): string {
  const used = new Set(taken);
  const clean = base.replace(/[^A-Za-z0-9_-]/g, '') || 'variable';
  if (!used.has(clean)) return clean;
  for (let n = 2; ; n++) if (!used.has(`${clean}${n}`)) return `${clean}${n}`;
}

/**
 * A copy of `template` with `variable` added under `key`: right after `after` when given (a
 * duplicate goes next to its original), else last. A missing or broken `data` becomes an object.
 */
export function insertVariable(template: JsonObject, key: string, variable: JsonObject, after?: string): JsonObject {
  const variables = templateVariables(template);
  if (Object.hasOwn(variables, key)) return template;
  const entries = Object.entries(variables);
  const at = after !== undefined && Object.hasOwn(variables, after) ? entries.findIndex(([k]) => k === after) + 1 : entries.length;
  entries.splice(at, 0, [key, variable]);
  return setIn(template, ['data'], Object.fromEntries(entries));
}

/**
 * What an integer field's text is written as: a number when it is one written plainly ("25565",
 * "-3"), else the text itself, so "007" and a half-typed "-" stay as typed (the integer warning
 * tells about text the node cannot read).
 */
export function integerFromText(text: string): number | string {
  if (!/^-?\d+$/.test(text)) return text;
  const number = Number(text);
  return Number.isSafeInteger(number) && String(number) === text ? number : text;
}

/**
 * The variable with another type. Its value follows when it means the same in the new type
 * (a number to text, "25" to 25, "true" to true); a boolean needs a value the node reads, so it
 * gets false otherwise. Options and every other field stay, so switching back loses nothing.
 */
export function withVariableType(variable: JsonObject, type: string): JsonObject {
  return { ...variable, type, value: convertValue(variable.value, type) ?? '' };
}

function convertValue(value: unknown, type: string): unknown {
  switch (type) {
    case 'boolean':
      return booleanValue(value) ?? false;
    case 'integer':
      if (typeof value === 'string') return integerFromText(value);
      if (typeof value === 'boolean') return value ? 1 : 0;
      return value;
    case 'string':
      return typeof value === 'number' || typeof value === 'boolean' ? String(value) : value;
    default:
      return value;
  }
}

/** One choice of an option variable, as it is in the JSON (other keys are kept). */
export type VariableOption = JsonObject;

/** The options of a variable; anything that is not a list reads as none. */
export function variableOptions(variable: JsonObject): VariableOption[] {
  return Array.isArray(variable.options) ? variable.options.map((o) => (isPlainObject(o) ? o : { value: o })) : [];
}

/**
 * Whether the create form shows the variable as a list to pick from: an option variable, or any
 * variable with options (create-server-stepper's TemplateVariableInput checks the options first;
 * community templates have text variables with options).
 */
export function usesOptions(variable: JsonObject): boolean {
  return variableType(variable) === 'option' || variableOptions(variable).length > 0;
}

/** Whether two option values are the same choice: equal, or the same text ("1" and 1). */
export function sameOptionValue(a: unknown, b: unknown): boolean {
  return Object.is(a, b) || (a !== null && b !== null && a !== undefined && b !== undefined && String(a) === String(b));
}

/**
 * What an option's value field writes: a value that was a number stays a number while the text
 * is one, so editing "8" into "16" does not turn the JSON number into a string.
 */
export function optionValueFromText(text: string, previous: unknown): string | number {
  if (typeof previous !== 'number') return text;
  const number = integerFromText(text);
  return typeof number === 'number' ? number : text;
}

/** A new option for a list: the next "optionN" value not in it yet, and no text. */
export function newOption(options: readonly VariableOption[]): VariableOption {
  const taken = new Set(options.map((o) => String(o.value)));
  let n = options.length + 1;
  while (taken.has(`option${n}`)) n++;
  return { value: `option${n}`, display: '' };
}

/**
 * Whether JavaScript keeps `key` first in any object: a key that is an array index ("0", "25")
 * is always listed before the others, in numeric order, so such a variable cannot be moved.
 */
export function isPinnedKey(key: string): boolean {
  return /^(0|[1-9]\d*)$/.test(key) && Number(key) < 2 ** 32 - 1;
}

/** Whether `key` may name a variable (the node's token rules; see VARIABLE_KEY). */
export function isValidVariableKey(key: string): boolean {
  return VARIABLE_KEY.test(key);
}
