import { z } from 'zod';
import { templateSchema } from './template';

// The template schema as JSON Schema, for Monaco's JSON mode: it underlines what the form would
// flag and completes keys, operation types and their arguments. Draft-07 is the dialect Monaco's
// JSON service reads fully. Checks written as refinements (a variable's value against its type,
// forgedl's version-or-Minecraft-version) have no JSON Schema form and stay with the form's
// live validation.

/** The id Monaco registers the schema under; any stable URI works. */
export const TEMPLATE_JSON_SCHEMA_URI = 'inmemory://aether-panel/template.schema.json';

let cached: z.core.JSONSchema.BaseSchema | undefined;

/** Built once, on first use: the JSON view is loaded lazily and so is this. */
export function templateJsonSchema(): z.core.JSONSchema.BaseSchema {
  cached ??= z.toJSONSchema(templateSchema, { target: 'draft-07' });
  return cached;
}
