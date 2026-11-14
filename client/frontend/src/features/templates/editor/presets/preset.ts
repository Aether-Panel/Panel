import type { JsonObject } from '../paths';

// What a base template ("Nueva plantilla → Plantilla base") is made of. Each one is copied from
// the closest community template, cut down to what the case needs, and written in the user's
// language: the variables' names and descriptions are what server creators read.

/** The panel's t(): a text in the user's language. */
export type PresetText = (key: string) => string;

export interface TemplatePreset {
  /** In the address (/templates/editor/?preset=<id>) and in its texts: templates.presets.<id>.… */
  id: string;
  /** The logo key (lib/template-logos.ts) the template uses as its icon, when there is one. */
  logo?: string;
  /** The template, with an empty name: the user names it before saving. */
  build: (t: PresetText) => JsonObject;
}

interface VariableSpec {
  type: 'string' | 'integer' | 'boolean' | 'option';
  value: string | number | boolean;
  required: boolean;
  userEdit: boolean;
  options?: { value: string; display: string }[];
}

/** A variable laid out as the community templates write them, its texts from templates.presets.<preset>.vars.<key>. */
export function presetVariable(t: PresetText, preset: string, key: string, spec: VariableSpec): JsonObject {
  const { type, value, required, userEdit, options } = spec;
  const text = `templates.presets.${preset}.vars.${key}`;
  return {
    type,
    value,
    display: t(`${text}.display`),
    desc: t(`${text}.desc`),
    required,
    userEdit,
    ...(options ? { options } : {}),
  };
}

/** The keys every base template starts with: no name yet, its display name, category and icon. */
export function presetHead(t: PresetText, preset: TemplatePreset['id'], type: string, logo?: string): JsonObject {
  return { name: '', display: t(`templates.presets.${preset}.display`), type, ...(logo ? { icon: logo } : {}) };
}

/** The run keys every base template ends with, as the community templates write them: console on stdin, nothing automatic. */
export function runDefaults(): JsonObject {
  return { stdin: { type: 'stdin' }, autostart: false, autorecover: false, autorestart: false };
}
