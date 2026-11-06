import { z } from 'zod';
import { conditionSchema, integerSchema, msg, stdinSchema, typeError, VARIABLE_KEY } from './common';
import { OPERATIONS } from './operations';

// A template as the node reads it (pkg/skypanel: Server, Execution, Variable, MetadataType…).
// Every object is loose: keys this schema does not name are kept as they are, so parsing a
// template never drops anything and the editor can save it back unchanged. The checks mirror
// what makes the node reject a template (a wrong JSON type fails its decoding) or makes a step
// fail when it runs (an unknown operation, a missing required argument). null is accepted
// wherever Go decodes it as "not given" (an object, a list, a map, a condition).

const str = () => z.string({ error: typeError() });
const bool = () => z.boolean({ error: typeError() });
const int = () => integerSchema();
const stringList = () => z.array(str(), { error: typeError() });
const stringMap = () => z.record(z.string(), str(), { error: typeError() });

/** One install, uninstall, pre or post step: its type picks the operation its arguments follow. */
export const stepSchema = z.discriminatedUnion(
  'type',
  OPERATIONS.map((o) => o.stepSchema) as unknown as [z.ZodObject, ...z.ZodObject[]],
  // a step that is not an object at all (zod types this map as union-only, but a non-object
  // input arrives as invalid_type), or one whose type names no operation
  { error: (issue) => msg((issue.code as string) === 'invalid_type' ? 'stepNotObject' : 'unknownOperation') },
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
    // Variable.UnmarshalJSON casts a boolean variable's value with cast.ToBoolE, and the node
    // rejects the template when that fails; a missing value is read as "", which fails too. An
    // integer that does not cast is not an error there: it silently becomes 0 (a warning for
    // the editor to raise, not something this schema can express).
    if (v.type !== 'boolean') return;
    const value = v.value;
    const ok = typeof value === 'boolean' || typeof value === 'number' || (typeof value === 'string' && GO_BOOL_STRINGS.includes(value));
    if (!ok) ctx.addIssue({ code: 'custom', path: ['value'], message: msg('invalidBoolean') });
  });

export const groupSchema = z.looseObject({
  if: conditionSchema.nullable().optional(),
  display: str().optional(),
  description: str().optional(),
  variables: stringList().nullable().optional(),
  order: int().optional(),
});

/** One entry of a conditional run command: the first whose condition holds is used. */
export const commandEntrySchema = z.looseObject({
  if: conditionSchema.nullable().optional(),
  command: str(),
  stdin: stdinSchema.nullable().optional(),
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
  stdin: stdinSchema.nullable().optional(),
  autostart: bool().optional(),
  autorecover: bool().optional(),
  autorestart: bool().optional(),
  expectedExitCode: int().optional(),
});

/** Environment types the node can create servers in (internal/servers/env_loader.go). */
export const ENVIRONMENT_TYPES = ['host', 'docker', 'tty', 'standard'] as const;

/**
 * Where the server runs; the docker keys follow internal/servers/docker.Docker. The type may be
 * "": MetadataType.MarshalJSON always writes it, so a template saved without an environment
 * comes back as { "type": "" }. Choosing one is a warning for the editor, not an error here.
 */
export const environmentSchema = z.looseObject({
  type: z.enum(['', ...ENVIRONMENT_TYPES] as const, { error: typeError('invalidEnvironment') }).optional(),
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
  run: runSchema.nullable().optional(),
  environment: environmentSchema.nullable().optional(),
  supportedEnvironments: z.array(environmentSchema, { error: typeError() }).nullable().optional(),
  requirements: requirementsSchema.nullable().optional(),
  stats: typedSchema.nullable().optional(),
  query: typedSchema.nullable().optional(),
  keepAlive: keepAliveSchema.nullable().optional(),
});

export type Template = z.infer<typeof templateSchema>;
export type TemplateStep = z.infer<typeof stepSchema>;
export type TemplateVariable = z.infer<typeof variableSchema>;
export type TemplateEnvironment = z.infer<typeof environmentSchema>;
