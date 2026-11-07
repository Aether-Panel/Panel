import { z } from 'zod';

// Pieces shared by the template schema and the operation registry. Every message a schema here
// produces is an i18n key (see issues.ts), so forms and the JSON view show it translated.

/** The i18n key of a validation message. */
export const msg = (code: string) => `templates.editor.validation.${code}`;

/**
 * The message of a type check: "required" when the value is missing (zod reports a missing key
 * as the wrong type) or, for a required field, empty ("" fails a number or a fixed choice); the
 * given code otherwise, so an optional field given "" is told its value is wrong.
 */
export const typeError =
  (code = 'invalidType', required = false) =>
  (issue: { input?: unknown }) =>
    issue.input === undefined || (required && issue.input === '') ? msg('required') : msg(code);

/** A `${name}` token, which the node replaces with the variable's value before a step runs. */
export const VARIABLE_TOKEN = /\$\{[^}]+\}/;

/**
 * Variable keys the node accepts: tokens are replaced by plain text search, so anything without
 * spaces or braces works, and community templates use hyphens (server-port, bot-js-file).
 */
export const VARIABLE_KEY = /^[A-Za-z0-9_-]+$/;

/**
 * Keys a condition (`if`) can read: CEL identifiers, so no hyphens and no leading digit. A key
 * outside this is still valid, only unusable in conditions, so it deserves a warning, not an error.
 */
export const CONDITION_SAFE_VARIABLE_KEY = /^[A-Za-z_]\w*$/;

/** A condition in CEL (`env == 'host'`); the node evaluates it, so only its type is checked. */
export const conditionSchema = z.string({ error: typeError() });

/** A whole number with no variables (stopCode, a group's order): the node decodes it as an int. */
export function integerSchema(bounds: { min?: number; max?: number; required?: boolean } = {}) {
  let num = z.number({ error: typeError('invalidInteger', bounds.required) }).int({ error: msg('invalidInteger') });
  if (bounds.min !== undefined) num = num.min(bounds.min, { error: msg('outOfRange') });
  if (bounds.max !== undefined) num = num.max(bounds.max, { error: msg('outOfRange') });
  return num;
}

/**
 * A whole number written as a number or, since the node casts it after replacing tokens, as a
 * string of digits or one holding a `${variable}` (javadl's "${javaversion}"). emptyIsUnset
 * takes "" too, only for an argument whose factory reads "" as not given (curseforge's fileId);
 * elsewhere cast turns "" into 0, which the step then refuses.
 */
export function numberOrVariable(bounds: { min?: number; max?: number; required?: boolean; emptyIsUnset?: boolean } = {}) {
  const pattern = bounds.emptyIsUnset ? /^$|^\d+$|\$\{[^}]+\}/ : /^\d+$|\$\{[^}]+\}/;
  let num = z.number({ error: typeError('invalidNumber', bounds.required) }).int({ error: msg('invalidNumber') });
  if (bounds.min !== undefined) num = num.min(bounds.min, { error: msg('outOfRange') });
  if (bounds.max !== undefined) num = num.max(bounds.max, { error: msg('outOfRange') });
  // a string fails inside its own branch, which zod reports instead of the union's message, so
  // a required field left "" is named there, before the pattern
  let text = z.string();
  if (bounds.required) text = text.min(1, { error: msg('required'), abort: true });
  return z.union([num, text.regex(pattern, { error: msg('invalidNumber') })], {
    error: typeError('invalidNumber', bounds.required),
  });
}

/** A Go duration with its units: "30s", "1h30m", "1.5h". */
const GO_DURATION = /^(\d+(\.\d+)?(ns|us|µs|ms|s|m|h))+$/;
/** A bare number, which cast reads as nanoseconds. */
const NANOSECONDS = /^\d+$/;

/**
 * What Go's cast.ToDurationE reads: a duration such as "30s" or "1h30m", a bare number of
 * nanoseconds, or (anywhere in the text) a `${variable}` replaced before the step runs. One
 * pattern made of the three, so the JSON view's schema checks it too.
 */
export const NODE_DURATION = new RegExp([GO_DURATION, NANOSECONDS, VARIABLE_TOKEN].map((part) => part.source).join('|'));

/** A sleep's duration (NODE_DURATION). Only sleep's required duration uses it, so "" is reported as missing. */
export const durationSchema = z.union(
  [
    z.number().int().nonnegative(),
    z
      .string()
      .min(1, { error: msg('required'), abort: true })
      .regex(NODE_DURATION, { error: msg('invalidDuration') }),
  ],
  { error: typeError('invalidDuration', true) },
);

/** Console types the node knows (pkg/skypanel/environment.go); "" means the standard input. */
export const STDIN_TYPES = ['stdin', 'rcon', 'rconws', 'telnet'] as const;

/** How the panel sends console commands to the game (skypanel.StdinConsoleConfiguration). */
export const stdinSchema = z.looseObject({
  type: z.enum(['', ...STDIN_TYPES] as const, { error: msg('invalidOption') }).optional(),
  ip: z.string({ error: typeError() }).optional(),
  // a string in Go, so a JSON number would make the node reject the template
  port: z.string({ error: typeError() }).optional(),
  password: z.string({ error: typeError() }).optional(),
  // rconws only: wss:// unless false
  secure: z.boolean({ error: typeError() }).optional(),
});
