import { z } from 'zod';
import { msg, numberOrVariable, stdinSchema, typeError } from './common';

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
 *   or one holding `${variable}`, and a string already there stays a string
 * - boolean: true or false
 * - select: one of the options, or any string when allowCustom (a version, `${variable}`); a
 *   field the node casts to a number (javadl's version) also takes a JSON number, per its schema
 * - stdin: a console configuration object (type, ip, port, password, secure), the same editor
 *   as the run section's
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
  /** Older spellings the node also reads (steamgamedl's appID); the form shows whichever is set. */
  aliases?: readonly string[];
  /** Seldom needed; a form may tuck it under "more options". */
  advanced: boolean;
  /** Checks the value; optional unless required (aliases are checked by the step schema). */
  schema: z.ZodType;
}

/** The schema of a value edited with this control, before it is made optional. */
export function widgetSchema(widget: FieldWidget, required: boolean, variables: boolean): z.ZodType {
  switch (widget.kind) {
    case 'text':
    case 'multiline':
    case 'path': {
      const s = z.string({ error: typeError() });
      return required ? s.min(1, { error: msg('required') }) : s;
    }
    case 'stringList': {
      const list = z.array(z.string({ error: typeError() }), { error: typeError() });
      return required ? list.min(1, { error: msg('required') }) : list;
    }
    case 'keyValue':
      return z.record(z.string(), z.string({ error: typeError() }), { error: typeError() });
    case 'number': {
      if (variables) return numberOrVariable(widget);
      let num = z.number({ error: typeError('invalidNumber') }).int({ error: msg('invalidNumber') });
      if (widget.min !== undefined) num = num.min(widget.min, { error: msg('outOfRange') });
      if (widget.max !== undefined) num = num.max(widget.max, { error: msg('outOfRange') });
      return num;
    }
    case 'boolean':
      return z.boolean({ error: typeError() });
    case 'select': {
      if (!widget.allowCustom) {
        const values = widget.options.map((o) => o.value) as [string, ...string[]];
        return z.enum(values, { error: typeError('invalidOption') });
      }
      const s = z.string({ error: typeError() });
      return required ? s.min(1, { error: msg('required') }) : s;
    }
    case 'stdin':
      return stdinSchema;
  }
}
