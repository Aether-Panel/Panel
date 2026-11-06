import { z } from 'zod';

// Pieces shared by the template schema and the operation registry. Every message a schema here
// produces is an i18n key (see issues.ts), so forms and the JSON view show it translated.

/** The i18n key of a validation message. */
export const msg = (code: string) => `templates.editor.validation.${code}`;

/**
 * The message of a type check: "required" when the value is missing altogether (zod reports a
 * missing key as the wrong type), the given code otherwise.
 */
export const typeError =
  (code = 'invalidType') =>
  (issue: { input?: unknown }) =>
    issue.input === undefined ? msg('required') : msg(code);

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
export const CONDITION_SAFE_VARIABLE_KEY = /^[A-Za-z_][A-Za-z0-9_]*$/;

/** A condition in CEL (`env == 'host'`); the node evaluates it, so only its type is checked. */
export const conditionSchema = z.string({ error: typeError() });

/**
 * A whole number written as a number or, since the node casts it after replacing tokens, as a
 * string of digits or one holding a `${variable}` (javadl's "${javaversion}").
 */
export function numberOrVariable(bounds: { min?: number; max?: number } = {}) {
  let num = z.number({ error: typeError('invalidNumber') }).int({ error: msg('invalidNumber') });
  if (bounds.min !== undefined) num = num.min(bounds.min, { error: msg('outOfRange') });
  if (bounds.max !== undefined) num = num.max(bounds.max, { error: msg('outOfRange') });
  return z.union([num, z.string().regex(/^\d+$|\$\{[^}]+\}/, { error: msg('invalidNumber') })], {
    error: typeError('invalidNumber'),
  });
}

/**
 * What Go's cast.ToDurationE reads: a duration such as "30s" or "1h30m", a bare number of
 * nanoseconds, or a `${variable}` replaced before the step runs.
 */
export const durationSchema = z.union(
  [
    z.number().int().nonnegative(),
    z.string().regex(/^(\d+(\.\d+)?(ns|us|µs|ms|s|m|h))+$|^\d+$|\$\{[^}]+\}/, { error: msg('invalidDuration') }),
  ],
  { error: typeError('invalidDuration') },
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
