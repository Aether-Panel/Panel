import { z } from 'zod';
import { integerSchema, msg, numberOrVariable, stdinSchema, typeError } from './common';

// What a form needs to draw one argument of an operation: the control, its texts (i18n keys),
// whether it is required, and the zod schema its value is checked with. The registry
// (operations.ts) describes every argument with this, so a step's form is rendered from data.

/** A choice of a select; without labelKey the value itself is shown (a Java version, say). */
export interface SelectOption {
  value: string;
  labelKey?: string;
}

/**
 * The control an argument is edited with. Values are written in the JSON type noted here.
 * - text / multiline / path: a string (path is relative to the server folder)
 * - stringList: an array of strings, one per row
 * - keyValue: an object of string values
 * - number: a whole number; when the field takes variables it may also be a string of digits
 *   or one holding `${variable}` (the form writes digits as a number, anything else as text)
 * - boolean: true or false
 * - select: one of the options, or any string when allowCustom (a version, `${variable}`); a
 *   field the node casts to a number (javadl's version) also takes a JSON number, per its schema
 * - stdin: a console configuration object (type, ip, port, password, secure), the same editor
 *   as the run section's; null is read as none
 */
export type FieldWidget =
  | { kind: 'text' }
  | { kind: 'multiline' }
  | { kind: 'path' }
  | { kind: 'stringList' }
  | { kind: 'keyValue' }
  | { kind: 'number'; min?: number; max?: number }
  | { kind: 'boolean' }
  | { kind: 'select'; options: readonly SelectOption[]; allowCustom: boolean }
  | { kind: 'stdin' };

export type FieldWidgetKind = FieldWidget['kind'];

export interface OperationField {
  /** The argument's key in the step's JSON. */
  name: string;
  widget: FieldWidget;
  labelKey: string;
  helpKey: string;
  placeholderKey?: string;
  /** The node refuses the step (or does nothing useful) without it. */
  required: boolean;
  /** `${variable}` tokens in it are replaced before the step runs, so a variable picker helps. */
  variables: boolean;
  /** What the node uses when the argument is missing or empty: a hint, never written. */
  defaultValue?: string | number | boolean;
  /**
   * The factory reads "" as not given (curseforge's fileId, backup's offsite), so clearing the
   * field may write "". Elsewhere "" in a number or a fixed choice is an error the node raises.
   */
  emptyIsUnset: boolean;
  /** Older spellings the node also reads (steamgamedl's appID); the form shows whichever is set. */
  aliases?: readonly string[];
  /** Seldom needed; a form may tuck it under "more options". */
  advanced: boolean;
  /** Checks the value; optional unless required (aliases are checked by the step schema). */
  schema: z.ZodType;
}

/** Text, which "" leaves missing when it is required. */
function textSchema(required: boolean): z.ZodType {
  const s = z.string({ error: typeError() });
  return required ? s.min(1, { error: msg('required') }) : s;
}

/** A select's value: one of its options (or "" when that means unset), or any text when allowCustom. */
function selectSchema(widget: Extract<FieldWidget, { kind: 'select' }>, required: boolean, emptyIsUnset: boolean): z.ZodType {
  if (widget.allowCustom) return textSchema(required);
  const values = widget.options.map((o) => o.value);
  return z.enum((emptyIsUnset ? [...values, ''] : values) as [string, ...string[]], { error: typeError('invalidOption', required) });
}

/** The schema of a value edited with this control, before it is made optional. */
export function widgetSchema(widget: FieldWidget, required: boolean, variables: boolean, emptyIsUnset = false): z.ZodType {
  switch (widget.kind) {
    case 'text':
    case 'multiline':
    case 'path':
      return textSchema(required);
    case 'stringList': {
      const list = z.array(z.string({ error: typeError() }), { error: typeError() });
      return required ? list.min(1, { error: msg('required') }) : list;
    }
    case 'keyValue': {
      const map = z.record(z.string(), z.string({ error: typeError() }), { error: typeError() });
      return required ? map.refine((m) => Object.keys(m).length > 0, { error: msg('required') }) : map;
    }
    case 'number': {
      return variables ? numberOrVariable({ ...widget, required, emptyIsUnset }) : integerSchema({ ...widget, required });
    }
    case 'boolean':
      return z.boolean({ error: typeError() });
    case 'select':
      return selectSchema(widget, required, emptyIsUnset);
    case 'stdin':
      return stdinSchema.nullable();
  }
}
