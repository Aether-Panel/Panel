import { z } from 'zod';
import { conditionSchema, msg, stdinSchema, typeError, VARIABLE_KEY } from './common';
import { OPERATIONS } from './operations';

// A template as the node reads it (pkg/skypanel: Server, Execution, Variable, MetadataType…).
// Every object is loose: keys this schema does not name are kept as they are, so parsing a
// template never drops anything and the editor can save it back unchanged. The checks mirror
// what makes the node reject a template (a wrong JSON type fails its decoding) or makes a step
// fail when it runs (an unknown operation, a missing required argument).

const str = () => z.string({ error: typeError() });
const bool = () => z.boolean({ error: typeError() });
const int = () => z.number({ error: typeError('invalidNumber') }).int({ error: msg('invalidNumber') });
const stringList = () => z.array(str(), { error: typeError() });
const stringMap = () => z.record(z.string(), str(), { error: typeError() });

/** One install, uninstall, pre or post step: its type picks the operation its arguments follow. */
export const stepSchema = z.discriminatedUnion(
  'type',
  OPERATIONS.map((o) => o.stepSchema) as unknown as [z.ZodObject, ...z.ZodObject[]],
  { error: msg('unknownOperation') },
);

// null is what Go writes for an empty list, and reads back as one
const stepListSchema = z.array(stepSchema, { error: typeError() }).nullable();

/** The variable types the create-server form knows; the node reads any other as text. */
export const VARIABLE_TYPES = ['string', 'integer', 'boolean', 'option'] as const;

/** strconv.ParseBool, which cast.ToBoolE uses for strings. */
const GO_BOOL_STRINGS = ['1', 't', 'T', 'TRUE', 'true', 'True', '0', 'f', 'F', 'FALSE', 'false', 'False'];

export const variableOptionSchema = z.looseObject({
  value: z.union([z.string(), z.number(), z.boolean()], { error: typeError() }),
  display: str().optional(),
});

export const variableSchema = z
  .looseObject({
    // an enum first so the JSON view suggests the known types; any other string still loads
    type: z.union([z.enum(VARIABLE_TYPES), z.string()], { error: typeError() }).optional(),
    value: z.unknown().optional(),
    display: str().optional(),
    desc: str().optional(),
    required: bool().optional(),
    internal: bool().optional(),
    userEdit: bool().optional(),
    options: z.array(variableOptionSchema, { error: typeError() }).nullable().optional(),
  })
  .superRefine((v, ctx) => {
    // Variable.UnmarshalJSON casts the value to the type, and the node rejects the template
    // when that fails
    const value = v.value;
    if (value === undefined || value === null || value === '') return;
    if (v.type === 'integer') {
      const ok = (typeof value === 'number' && Number.isInteger(value)) || (typeof value === 'string' && /^-?\d+$/.test(value.trim()));
      if (!ok) ctx.addIssue({ code: 'custom', path: ['value'], message: msg('invalidNumber') });
    } else if (v.type === 'boolean') {
      const ok = typeof value === 'boolean' || typeof value === 'number' || (typeof value === 'string' && GO_BOOL_STRINGS.includes(value));
      if (!ok) ctx.addIssue({ code: 'custom', path: ['value'], message: msg('invalidBoolean') });
    }
  });

export const groupSchema = z.looseObject({
  if: conditionSchema.optional(),
  display: str().optional(),
  description: str().optional(),
  variables: stringList().nullable().optional(),
  order: int().optional(),
});

/** One entry of a conditional run command: the first whose condition holds is used. */
export const commandEntrySchema = z.looseObject({
  if: conditionSchema.optional(),
  command: str(),
  stdin: stdinSchema.optional(),
});

export const runSchema = z.looseObject({
  // null is what the node writes for a template without one
  command: z.union([str(), z.array(commandEntrySchema)], { error: typeError('invalidCommand') }).nullable().optional(),
  stop: str().optional(),
  stopCode: int().optional(),
  pre: stepListSchema.optional(),
  post: stepListSchema.optional(),
  environmentVars: stringMap().nullable().optional(),
  workingDirectory: str().optional(),
  stdin: stdinSchema.optional(),
  autostart: bool().optional(),
  autorecover: bool().optional(),
  autorestart: bool().optional(),
  expectedExitCode: int().optional(),
});

/** Environment types the node can create servers in (internal/servers/env_loader.go). */
export const ENVIRONMENT_TYPES = ['host', 'docker', 'tty', 'standard'] as const;

/** Where the server runs; the docker keys follow internal/servers/docker.Docker. */
export const environmentSchema = z.looseObject({
  type: z.enum(ENVIRONMENT_TYPES, { error: typeError('invalidEnvironment') }),
  image: str().optional(),
  portBindings: stringList().nullable().optional(),
  bindings: stringMap().nullable().optional(),
  networkName: str().optional(),
  containerRoot: str().optional(),
  labels: stringMap().nullable().optional(),
  hostConfig: z.record(z.string(), z.unknown()).nullable().optional(),
  config: z.record(z.string(), z.unknown()).nullable().optional(),
});

export const requirementsSchema = z.looseObject({
  // "linux || windows", as Go's runtime names them
  os: str().optional(),
  arch: str().optional(),
  binaries: stringList().nullable().optional(),
});

/** stats ("jcmd") and query ("minecraft", "source"): a type and whatever it reads. */
const typedSchema = z.looseObject({ type: str().optional() });

export const keepAliveSchema = z.looseObject({
  frequency: str().optional(),
  command: str().optional(),
});

export const templateSchema = z.looseObject({
  type: str().optional(),
  display: str().optional(),
  icon: str().optional(),
  data: z
    .record(z.string().regex(VARIABLE_KEY), variableSchema, {
      error: (issue) => (issue.code === 'invalid_key' ? msg('invalidVariableKey') : typeError()(issue)),
    })
    .nullable()
    .optional(),
  groups: z.array(groupSchema, { error: typeError() }).nullable().optional(),
  install: stepListSchema.optional(),
  uninstall: stepListSchema.optional(),
  run: runSchema.optional(),
  environment: environmentSchema.optional(),
  supportedEnvironments: z.array(environmentSchema, { error: typeError() }).nullable().optional(),
  requirements: requirementsSchema.optional(),
  stats: typedSchema.optional(),
  query: typedSchema.optional(),
  keepAlive: keepAliveSchema.optional(),
});

export type Template = z.infer<typeof templateSchema>;
export type TemplateStep = z.infer<typeof stepSchema>;
export type TemplateVariable = z.infer<typeof variableSchema>;
export type TemplateEnvironment = z.infer<typeof environmentSchema>;
