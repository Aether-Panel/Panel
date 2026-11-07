import { z } from 'zod';
import type * as Icons from '@/components/icons';
import { conditionSchema, durationSchema, msg, numberOrVariable } from './common';
import { widgetSchema, type FieldWidget, type OperationField, type SelectOption } from './fields';

// Every step type the node can run (internal/servers/operation_process.go registers them), with
// what the editor needs to offer it: category, texts, icon and one entry per argument. The
// arguments come from each operation's Go factory (internal/operations/<key>/factory.go):
// names, JSON types, defaults and what the factory refuses. operations.json lists the same keys
// for internal/servers/operations_manifest_test.go, which fails when the node gains an
// operation this file does not describe.

/** The palette's groups, in the order it shows them. */
export const OPERATION_CATEGORIES = ['files', 'downloads', 'minecraft', 'steam', 'runtimes', 'system', 'server'] as const;
export type OperationCategory = (typeof OPERATION_CATEGORIES)[number];

/** An icon exported by @/components/icons, resolved by name where the palette renders it. */
export type OperationIconName = keyof typeof Icons;

export interface OperationDefinition {
  key: string;
  category: OperationCategory;
  labelKey: string;
  helpKey: string;
  icon: OperationIconName;
  /**
   * Offered when building a template. The server operations (start, stop, restart, backup) are
   * registered for scheduled tasks: during an install the node refuses to start or back up the
   * server, and in run.pre/post they would start or stop the server from inside its own start.
   */
  templates: boolean;
  fields: readonly OperationField[];
  /** A whole step of this type: type, optional if, the arguments; other keys are kept. */
  stepSchema: z.ZodType;
}

type FieldSpec = {
  name: string;
  widget: FieldWidget;
  required?: boolean;
  /**
   * Defaults to true for controls that hold free text: the node replaces the ${} tokens of every
   * string argument (and list of strings) before the factory reads it.
   */
  variables?: boolean;
  /** The field has a placeholder text in the locales. */
  placeholder?: boolean;
  defaultValue?: string | number | boolean;
  aliases?: readonly string[];
  advanced?: boolean;
  /** The factory reads "" as not given; see OperationField.emptyIsUnset. */
  emptyIsUnset?: boolean;
};

type OperationSpec = {
  key: string;
  category: OperationCategory;
  icon: OperationIconName;
  templates?: boolean;
  fields: readonly FieldSpec[];
  /** Checks spanning several arguments, reported on one of them. */
  refine?: (args: Record<string, unknown>, ctx: z.RefinementCtx) => void;
};

const text = { kind: 'text' } as const;
const multiline = { kind: 'multiline' } as const;
const path = { kind: 'path' } as const;
const stringList = { kind: 'stringList' } as const;
const boolean = { kind: 'boolean' } as const;
const stdin = { kind: 'stdin' } as const;
const number = (min?: number, max?: number): FieldWidget => ({ kind: 'number', min, max });
const select = (options: readonly (string | SelectOption)[], allowCustom: boolean): FieldWidget => ({
  kind: 'select',
  options: options.map((o) => (typeof o === 'string' ? { value: o } : o)),
  allowCustom,
});

/** Whether a value counts as given: the node treats a missing argument and "" alike. */
const isSet = (v: unknown) => v !== undefined && v !== null && v !== '';

/** forgedl and neoforgedl refuse a step with neither a version nor a Minecraft version. */
const versionOrMinecraftVersion = (args: Record<string, unknown>, ctx: z.RefinementCtx) => {
  if (!isSet(args.version) && !isSet(args.minecraftVersion)) {
    ctx.addIssue({ code: 'custom', path: ['minecraftVersion'], message: msg('versionOrMinecraftVersion') });
  }
};

const JAVA_VERSIONS = ['8', '11', '17', '21', '25'];
const NODE_VERSIONS = ['18', '20', '22', '24'];
const MINECRAFT_CHANNELS = ['latest', 'release', 'snapshot'];

const SPECS: readonly OperationSpec[] = [
  // files
  {
    key: 'alterfile',
    category: 'files',
    icon: 'PencilLine',
    fields: [
      { name: 'file', widget: path, required: true, placeholder: true },
      { name: 'search', widget: text, required: true, placeholder: true },
      { name: 'replace', widget: text, placeholder: true },
      { name: 'regex', widget: boolean, defaultValue: false },
    ],
  },
  {
    key: 'writefile',
    category: 'files',
    icon: 'FilePlus',
    fields: [
      { name: 'target', widget: path, required: true, placeholder: true },
      { name: 'text', widget: multiline },
    ],
  },
  {
    key: 'mkdir',
    category: 'files',
    icon: 'FolderPlus',
    fields: [{ name: 'target', widget: path, required: true, placeholder: true }],
  },
  {
    key: 'move',
    category: 'files',
    icon: 'ArrowRightLeft',
    fields: [
      { name: 'source', widget: path, required: true, placeholder: true },
      { name: 'target', widget: path, required: true, placeholder: true },
    ],
  },
  {
    key: 'extract',
    category: 'files',
    icon: 'Package',
    fields: [
      { name: 'source', widget: path, required: true, placeholder: true },
      { name: 'destination', widget: path, placeholder: true },
    ],
  },
  {
    key: 'archive',
    category: 'files',
    icon: 'FileArchive',
    fields: [
      { name: 'source', widget: stringList, required: true, placeholder: true },
      { name: 'destination', widget: path, required: true, placeholder: true },
    ],
  },
  // downloads
  {
    key: 'download',
    category: 'downloads',
    icon: 'Download',
    fields: [{ name: 'files', widget: stringList, required: true, placeholder: true }],
  },
  {
    key: 'githubdl',
    category: 'downloads',
    icon: 'Github',
    fields: [
      { name: 'repository', widget: text, required: true, placeholder: true },
      { name: 'assetMatch', widget: text, placeholder: true },
      { name: 'outputVariable', widget: text, placeholder: true, variables: false, advanced: true },
    ],
  },
  {
    key: 'scraperdl',
    category: 'downloads',
    icon: 'SearchCheck',
    fields: [
      { name: 'url', widget: text, required: true, placeholder: true },
      { name: 'regex', widget: text, required: true, placeholder: true },
      { name: 'downloadUrl', widget: text, required: true, placeholder: true },
      { name: 'outputVariable', widget: text, placeholder: true, variables: false, advanced: true },
    ],
  },
  // minecraft
  {
    key: 'mojangdl',
    category: 'minecraft',
    icon: 'Blocks',
    fields: [
      { name: 'version', widget: select(MINECRAFT_CHANNELS, true), required: true, placeholder: true },
      { name: 'target', widget: path, required: true, placeholder: true },
    ],
  },
  {
    key: 'paperdl',
    category: 'minecraft',
    icon: 'Blocks',
    fields: [
      { name: 'project', widget: select(['paper', 'folia', 'velocity', 'waterfall'], true), defaultValue: 'paper' },
      { name: 'minecraftVersion', widget: text, required: true, placeholder: true },
      { name: 'build', widget: text, required: true, placeholder: true },
      { name: 'target', widget: path, required: true, placeholder: true },
    ],
  },
  {
    key: 'fabricdl',
    category: 'minecraft',
    icon: 'Puzzle',
    fields: [],
  },
  {
    key: 'forgedl',
    category: 'minecraft',
    icon: 'Wrench',
    fields: [
      { name: 'minecraftVersion', widget: text, placeholder: true },
      { name: 'version', widget: text, placeholder: true },
      { name: 'target', widget: path, required: true, placeholder: true },
      { name: 'outputVariable', widget: text, variables: false, defaultValue: 'opForgeVersion', advanced: true },
    ],
    refine: versionOrMinecraftVersion,
  },
  {
    key: 'neoforgedl',
    category: 'minecraft',
    icon: 'Wrench',
    fields: [
      { name: 'minecraftVersion', widget: text, placeholder: true },
      { name: 'version', widget: text, placeholder: true },
      { name: 'target', widget: path, required: true, placeholder: true },
      { name: 'outputVariable', widget: text, variables: false, defaultValue: 'opNeoForgeVersion', advanced: true },
    ],
    refine: versionOrMinecraftVersion,
  },
  {
    key: 'resolveforgeversion',
    category: 'minecraft',
    icon: 'SearchCheck',
    fields: [
      { name: 'minecraftVersion', widget: text, placeholder: true },
      { name: 'version', widget: text, placeholder: true },
      { name: 'outputVariable', widget: text, variables: false, defaultValue: 'opForgeVersion' },
    ],
  },
  {
    key: 'resolveneoforgeversion',
    category: 'minecraft',
    icon: 'SearchCheck',
    fields: [
      { name: 'minecraftVersion', widget: text, placeholder: true },
      { name: 'version', widget: text, placeholder: true },
      { name: 'outputVariable', widget: text, variables: false, defaultValue: 'opNeoForgeVersion' },
    ],
  },
  {
    key: 'spongedl',
    category: 'minecraft',
    icon: 'Puzzle',
    fields: [
      {
        name: 'spongeType',
        widget: select(
          [
            { value: 'spongevanilla', labelKey: 'templates.editor.operations.spongedl.fields.spongeType.options.spongevanilla' },
            { value: 'spongeforge', labelKey: 'templates.editor.operations.spongedl.fields.spongeType.options.spongeforge' },
          ],
          false,
        ),
        required: true,
      },
      { name: 'minecraftVersion', widget: text, placeholder: true },
      { name: 'spongeVersion', widget: text, placeholder: true },
      { name: 'recommended', widget: boolean, defaultValue: false },
    ],
  },
  {
    key: 'curseforge',
    category: 'minecraft',
    icon: 'DownloadCloud',
    fields: [
      { name: 'projectId', widget: number(0), required: true, placeholder: true },
      // factory.go: an empty fileId means the newest file
      { name: 'fileId', widget: number(0), aliases: ['fileID'], placeholder: true, emptyIsUnset: true },
      { name: 'java', widget: text, defaultValue: 'java', placeholder: true, advanced: true },
    ],
  },
  // steam
  {
    key: 'steamgamedl',
    category: 'steam',
    icon: 'Gamepad2',
    fields: [
      { name: 'appId', widget: text, required: true, aliases: ['appID'], placeholder: true },
      { name: 'username', widget: text, placeholder: true },
      { name: 'password', widget: text, advanced: true },
      { name: 'extraArgs', widget: stringList, placeholder: true, advanced: true },
    ],
  },
  // runtimes
  {
    key: 'javadl',
    category: 'runtimes',
    icon: 'Coffee',
    fields: [{ name: 'version', widget: select(JAVA_VERSIONS, true), required: true, placeholder: true }],
  },
  {
    key: 'nodejsdl',
    category: 'runtimes',
    icon: 'FileCode',
    fields: [{ name: 'version', widget: select(NODE_VERSIONS, true), required: true, placeholder: true }],
  },
  // system
  {
    key: 'command',
    category: 'system',
    icon: 'Terminal',
    fields: [
      { name: 'commands', widget: stringList, required: true, placeholder: true },
      { name: 'stdin', widget: stdin, advanced: true },
    ],
  },
  {
    key: 'console',
    category: 'system',
    icon: 'Info',
    fields: [{ name: 'message', widget: multiline, required: true, placeholder: true }],
  },
  {
    key: 'stdin',
    category: 'system',
    icon: 'CornerDownLeft',
    fields: [{ name: 'command', widget: text, required: true, placeholder: true }],
  },
  {
    key: 'sleep',
    category: 'system',
    icon: 'Hourglass',
    fields: [{ name: 'duration', widget: text, required: true, placeholder: true }],
  },
  {
    key: 'dockerpull',
    category: 'system',
    icon: 'Layers',
    fields: [{ name: 'imageName', widget: text, required: true, placeholder: true }],
  },
  // server: scheduled tasks only (internal/servers/task_operations.go)
  { key: 'start', category: 'server', icon: 'Play', templates: false, fields: [] },
  { key: 'stop', category: 'server', icon: 'Square', templates: false, fields: [] },
  { key: 'restart', category: 'server', icon: 'RotateCw', templates: false, fields: [] },
  {
    key: 'backup',
    category: 'server',
    icon: 'Archive',
    templates: false,
    fields: [
      // AutoBackupKeep: 1-50, 5 when missing
      { name: 'keep', widget: number(1, 50), defaultValue: 5 },
      {
        name: 'offsite',
        widget: select(
          ['never', 'always', 'weekend', 'weekly', 'biweekly', 'monthly'].map((value) => ({
            value,
            labelKey: `templates.editor.operations.backup.fields.offsite.options.${value}`,
          })),
          false,
        ),
        // a step without it uploads every copy, as tasks did before the option existed; "" too
        // (ParseOffsiteFrequency)
        defaultValue: 'always',
        emptyIsUnset: true,
      },
      // OffsiteKeep reads a JSON number only, so no ${} tokens
      { name: 'offsiteKeep', widget: number(1, 50), variables: false, defaultValue: 4 },
    ],
  },
];

/** Arguments whose value the node reads through a parser of its own rather than the widget's. */
const SCHEMA_OVERRIDES: Record<string, Record<string, z.ZodType>> = {
  sleep: { duration: durationSchema },
  // offered as a list of versions, but cast to a number: 21 and "21" both work
  javadl: { version: numberOrVariable({ min: 1, required: true }) },
  nodejsdl: { version: numberOrVariable({ min: 1, required: true }) },
};

function buildField(spec: OperationSpec, f: FieldSpec): OperationField {
  const base = `templates.editor.operations.${spec.key}.fields.${f.name}`;
  const required = f.required ?? false;
  const variables = f.variables ?? (f.widget.kind === 'select' ? f.widget.allowCustom : !['boolean', 'keyValue', 'stdin'].includes(f.widget.kind));
  return {
    name: f.name,
    widget: f.widget,
    labelKey: `${base}.label`,
    helpKey: `${base}.help`,
    placeholderKey: f.placeholder ? `${base}.placeholder` : undefined,
    required,
    variables,
    defaultValue: f.defaultValue,
    emptyIsUnset: f.emptyIsUnset ?? false,
    aliases: f.aliases,
    advanced: f.advanced ?? false,
    schema: SCHEMA_OVERRIDES[spec.key]?.[f.name] ?? widgetSchema(f.widget, required, variables, f.emptyIsUnset),
  };
}

function buildStepSchema(spec: OperationSpec, fields: readonly OperationField[]): z.ZodType {
  // null is how Go reads a missing condition
  const shape: Record<string, z.ZodType> = { type: z.literal(spec.key), if: conditionSchema.nullable().optional() };
  for (const f of fields) {
    // a required argument with another spelling is checked below, as either spelling will do
    shape[f.name] = f.required && !f.aliases?.length ? f.schema : f.schema.optional();
  }
  // Older spellings are checked here rather than in the shape, so the JSON Schema (and Monaco's
  // completion) leaves them out while templates that still use them load and validate.
  return z.looseObject(shape).superRefine((args, ctx) => {
    for (const f of fields) {
      for (const alias of f.aliases ?? []) {
        if (args[alias] === undefined) continue;
        const result = f.schema.safeParse(args[alias]);
        for (const issue of result.error?.issues ?? []) {
          ctx.addIssue({ code: 'custom', path: [alias, ...issue.path], message: issue.message });
        }
      }
      if (f.required && f.aliases?.length && ![f.name, ...f.aliases].some((n) => isSet(args[n]))) {
        ctx.addIssue({ code: 'custom', path: [f.name], message: msg('required') });
      }
    }
    spec.refine?.(args, ctx);
  });
}

/** Every operation, in palette order within each category. */
export const OPERATIONS: readonly OperationDefinition[] = SPECS.map((spec) => {
  const fields = spec.fields.map((f) => buildField(spec, f));
  return {
    key: spec.key,
    category: spec.category,
    labelKey: `templates.editor.operations.${spec.key}.label`,
    helpKey: `templates.editor.operations.${spec.key}.help`,
    icon: spec.icon,
    templates: spec.templates ?? true,
    fields,
    stepSchema: buildStepSchema(spec, fields),
  };
});

const BY_KEY = new Map(OPERATIONS.map((o) => [o.key, o]));

/** The operation a step's type names, or undefined for a type the node does not know. */
export function getOperation(key: string): OperationDefinition | undefined {
  return BY_KEY.get(key);
}

/** The operations offered when building a template, grouped by category in palette order. */
export function templateOperationsByCategory(): { category: OperationCategory; labelKey: string; operations: OperationDefinition[] }[] {
  return OPERATION_CATEGORIES.map((category) => ({
    category,
    labelKey: `templates.editor.operationCategories.${category}`,
    operations: OPERATIONS.filter((o) => o.templates && o.category === category),
  })).filter((group) => group.operations.length > 0);
}

/** The line operations.json holds for each operation, which the Go test compares with the node. */
export function manifestEntries(): { key: string; templates: boolean }[] {
  return OPERATIONS.map((o) => ({ key: o.key, templates: o.templates })).sort((a, b) => (a.key < b.key ? -1 : 1));
}
