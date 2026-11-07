// A template's boolean variable is read on the panel by Go's cast.ToBoolE: a number is true unless
// it is 0, and text is read with strconv.ParseBool's words. The template editor, its preview and
// "Crear servidor" all read it this way, so a value written "0" or "False" is off in every one.

/** strconv.ParseBool's words, which the node accepts for a boolean variable. */
const TRUE_WORDS = new Set(['1', 't', 'T', 'TRUE', 'true', 'True']);
const FALSE_WORDS = new Set(['0', 'f', 'F', 'FALSE', 'false', 'False']);

/** A boolean variable's value as the node reads it; undefined when it reads none. */
export function booleanValue(value: unknown): boolean | undefined {
  if (typeof value === 'boolean') return value;
  if (typeof value === 'number') return value !== 0;
  if (typeof value === 'string') {
    if (TRUE_WORDS.has(value)) return true;
    if (FALSE_WORDS.has(value)) return false;
  }
  return undefined;
}
