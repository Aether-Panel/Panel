import { CONDITION_SAFE_VARIABLE_KEY, VARIABLE_KEY } from '../../schema/common';
import { getOperation } from '../../schema/operations';
import { deleteIn, isPlainObject, renameKeyIn, type JsonObject, type Path } from '../paths';

// Where a template uses its variables, the way the node reads them:
// - `${key}` tokens are replaced by plain text search (internal/utils/arguments.go) in the run
//   command, the environment variables, the console settings, every text argument of the
//   install, uninstall, pre and post steps, the Docker settings and the required binaries;
// - a condition (`if`) reads a variable by its bare name, as a CEL identifier, and can also hold
//   `${key}` tokens (conditions/variables.go replaces them first).
// The analysis is the base of the variable warnings, the "used N times" counts and renaming a
// variable together with its uses.

/** Names the node fills in by itself, usable as `${name}` without defining them. */
const BUILTIN_TOKENS = new Set(['rootDir']);
/** run.post steps also get whether the server stopped cleanly and its exit code. */
const POST_TOKENS = new Set(['success', 'exitCode']);
/**
 * Names a condition can read without a variable (conditions/consts.go and RunCondition), plus
 * CEL's literals and its `in` operator.
 */
const CONDITION_NAMES = new Set(['env', 'os', 'arch', 'success', 'serverId', 'exitCode', 'true', 'false', 'null', 'in']);
/** Tokens a step replaces by itself before the node sees them: scraperdl's `${match}`. */
const STEP_TOKENS: Record<string, Record<string, readonly string[]>> = { scraperdl: { downloadUrl: ['match'] } };

/**
 * Variables the panel reads by name, outside any command: the resource limits (cpu, memory,
 * disk), the address (ip, port), the MOTD the server settings show, the mod launcher the world
 * map and version detection look at, and the Java and game versions modpack installs use
 * (internal/servers/modpack_loader.go javaSetup, modpack.go's platform). A template defines them
 * without using them.
 */
export const PANEL_VARIABLES: ReadonlySet<string> = new Set(['cpu', 'memory', 'disk', 'ip', 'port', 'motd', 'modlauncher', 'javaversion', 'version']);

const TOKEN = /\$\{([^}]*)\}/g;
// a CEL string literal: '…' or "…" with backslash escapes (triple quotes read as an empty literal
// followed by another one, which is still outside the code)
const LITERAL = /'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*"/g;
// an identifier that is not part of a longer name, a number or a member access (a.b)
const IDENTIFIER = /(?<![A-Za-z0-9_.$])[A-Za-z_]\w*(?!\w)/g;

/** A place in the template that names a variable. */
export interface VariableReference {
  /** The string that holds it: a step argument, the run command, a condition… */
  path: Path;
  /** `${key}` in a text, or the bare name in a condition. */
  kind: 'token' | 'condition';
}

export interface VariableUsage {
  /** Every reference to a name, by name, in template order. */
  references: ReadonlyMap<string, readonly VariableReference[]>;
  /** `${name}` tokens that name no variable, built-in or step output, with their place. */
  undefinedTokens: readonly { name: string; path: Path }[];
  /** Names a condition reads that are no variable and nothing the node provides. */
  undefinedConditionNames: readonly { name: string; path: Path }[];
  /** Variables whose key a condition cannot read (a hyphen, a leading digit) used in one. */
  unsafeConditionKeys: readonly { name: string; path: Path }[];
}

/** The variables of a template: its `data` object, or nothing when it is missing or broken. */
export function templateVariables(template: JsonObject): JsonObject {
  return isPlainObject(template.data) ? template.data : {};
}

const escapeRegExp = (text: string) => text.replace(/[.*+?^${}()|[\]\\]/g, String.raw`\$&`);

/**
 * A key as a condition reads it. A key that is not a CEL identifier (server-port) is matched
 * with its hyphens, so `server-port` is found whole and `port` inside it is not.
 */
function conditionNamePattern(name: string): RegExp {
  return new RegExp(String.raw`(?<![A-Za-z0-9_.$-])${escapeRegExp(name)}(?![A-Za-z0-9_-])(?!\s*\()`, 'g');
}

/** Applies `fn` to the parts of a condition outside its string literals. */
function mapConditionCode(condition: string, fn: (code: string) => string): string {
  let out = '';
  let last = 0;
  for (const match of condition.matchAll(LITERAL)) {
    out += fn(condition.slice(last, match.index)) + match[0];
    last = match.index + match[0].length;
  }
  return out + fn(condition.slice(last));
}

interface Visitor {
  /** A text whose `${}` tokens the node replaces; `local` names are filled in by its step. */
  text: (value: string, path: Path, local: ReadonlySet<string>) => string;
  condition: (value: string, path: Path) => string;
  /** A step that sets a variable while it runs. */
  produces?: (name: string) => void;
}

const NONE: ReadonlySet<string> = new Set();

/** Visits every string leaf under `value`, giving back the value with the visitor's changes. */
function mapTexts(value: unknown, path: Path, local: ReadonlySet<string>, visitor: Visitor): unknown {
  if (typeof value === 'string') return visitor.text(value, path, local);
  if (Array.isArray(value)) return mapList(value, (item, i) => mapTexts(item, [...path, i], local, visitor));
  if (isPlainObject(value)) return mapObject(value, (item, key) => mapTexts(item, [...path, key], local, visitor));
  return value;
}

/** A list with `fn` applied to each item; the same list when nothing changed. */
function mapList(list: readonly unknown[], fn: (item: unknown, index: number) => unknown): unknown[] {
  let changed = false;
  const next = list.map((item, i) => {
    const mapped = fn(item, i);
    if (!Object.is(mapped, item)) changed = true;
    return mapped;
  });
  return changed ? next : (list as unknown[]);
}

/** An object with `fn` applied to each value, keys in place; the same object when nothing changed. */
function mapObject(object: JsonObject, fn: (value: unknown, key: string) => unknown): JsonObject {
  let changed = false;
  const entries = Object.entries(object).map(([key, value]) => {
    const mapped = fn(value, key);
    if (!Object.is(mapped, value)) changed = true;
    return [key, mapped] as const;
  });
  // fromEntries defines every key as data, "__proto__" included
  return changed ? Object.fromEntries(entries) : object;
}

/** A step: its condition, then each argument but its type. */
function mapStep(step: unknown, path: Path, post: boolean, visitor: Visitor): unknown {
  if (!isPlainObject(step)) return step;
  const type = typeof step.type === 'string' ? step.type : '';
  const operation = getOperation(type);
  if (operation && visitor.produces) {
    // steps that resolve a version store it under outputVariable, or a default name
    const output = operation.fields.find((f) => f.name === 'outputVariable');
    const name = typeof step.outputVariable === 'string' && step.outputVariable !== '' ? step.outputVariable : output?.defaultValue;
    if (output && typeof name === 'string') visitor.produces(name);
  }
  return mapObject(step, (value, key) => {
    if (key === 'type') return value;
    if (key === 'if') return typeof value === 'string' ? visitor.condition(value, [...path, key]) : value;
    const stepTokens = STEP_TOKENS[type]?.[key] ?? [];
    const local = post || stepTokens.length ? new Set([...stepTokens, ...(post ? POST_TOKENS : [])]) : NONE;
    return mapTexts(value, [...path, key], local, visitor);
  });
}

function mapSteps(steps: unknown, path: Path, post: boolean, visitor: Visitor): unknown {
  return Array.isArray(steps) ? mapList(steps, (step, i) => mapStep(step, [...path, i], post, visitor)) : steps;
}

/** The run object: its command (one or a list with conditions), pre/post steps and the rest. */
function mapRun(run: unknown, visitor: Visitor): unknown {
  if (!isPlainObject(run)) return run;
  return mapObject(run, (value, key) => {
    const path = ['run', key];
    if (key === 'pre' || key === 'post') return mapSteps(value, path, key === 'post', visitor);
    if (key === 'command' && Array.isArray(value)) return mapList(value, (entry, i) => mapCommandEntry(entry, [...path, i], visitor));
    return mapTexts(value, path, NONE, visitor);
  });
}

/** One entry of a conditional command list: its `if` is a condition, the rest texts. */
function mapCommandEntry(entry: unknown, path: Path, visitor: Visitor): unknown {
  if (!isPlainObject(entry)) return entry;
  return mapObject(entry, (v, k) => {
    if (k !== 'if') return mapTexts(v, [...path, k], NONE, visitor);
    return typeof v === 'string' ? visitor.condition(v, [...path, k]) : v;
  });
}

/** An environment: every setting the node reads with tokens replaced, not its type. */
function mapEnvironment(environment: unknown, path: Path, visitor: Visitor): unknown {
  if (!isPlainObject(environment)) return environment;
  return mapObject(environment, (value, key) => (key === 'type' ? value : mapTexts(value, [...path, key], NONE, visitor)));
}

/**
 * Walks every place of the template where the node reads variables, in template order. What the
 * visitor returns replaces the string, so the same walk analyses and renames.
 */
function mapTemplate(template: JsonObject, visitor: Visitor): JsonObject {
  return mapObject(template, (value, key) => {
    switch (key) {
      case 'install':
      case 'uninstall':
        return mapSteps(value, [key], false, visitor);
      case 'run':
        return mapRun(value, visitor);
      case 'environment':
        return mapEnvironment(value, [key], visitor);
      case 'supportedEnvironments':
        return Array.isArray(value) ? mapList(value, (env, i) => mapEnvironment(env, [key, i], visitor)) : value;
      case 'requirements':
        return isPlainObject(value) ? mapObject(value, (v, k) => (k === 'binaries' ? mapTexts(v, [key, k], NONE, visitor) : v)) : value;
      case 'groups':
        // a group's condition decides whether the create form shows it
        return Array.isArray(value)
          ? mapList(value, (group, i) =>
              isPlainObject(group) && typeof group.if === 'string' ? { ...group, if: visitor.condition(group.if, [key, i, 'if']) } : group,
            )
          : value;
      default:
        return value;
    }
  });
}

/** The `${name}` tokens of a text whose name could be a variable key. */
function tokenNames(text: string): string[] {
  const names: string[] = [];
  for (const match of text.matchAll(TOKEN)) if (VARIABLE_KEY.test(match[1])) names.push(match[1]);
  return names;
}

/**
 * The names a condition reads: `${name}` tokens, the variables whose keys are not identifiers
 * (matched whole), and the identifiers outside string literals that are not member accesses
 * (a.b) or function calls (file_exists(…)).
 */
function conditionNames(condition: string, unsafeKeys: readonly string[]): { tokens: string[]; unsafe: string[]; names: string[] } {
  const tokens = tokenNames(condition);
  const unsafe: string[] = [];
  const names: string[] = [];
  mapConditionCode(condition, (code) => {
    let rest = code.replace(TOKEN, ' ');
    for (const key of unsafeKeys) {
      const pattern = conditionNamePattern(key);
      if (pattern.test(rest)) {
        unsafe.push(key);
        rest = rest.replace(pattern, ' ');
      }
    }
    for (const match of rest.matchAll(IDENTIFIER)) {
      if (/^\s*\(/.test(rest.slice(match.index + match[0].length))) continue;
      names.push(match[0]);
    }
    return code;
  });
  return { tokens, unsafe, names };
}

const cache = new WeakMap<JsonObject, VariableUsage>();

/**
 * Where `template` uses which names. Cached per template object: the warning rules and the
 * Variables section ask about the same object, and an edit makes a new one.
 */
export function analyzeVariableUsage(template: JsonObject): VariableUsage {
  const cached = cache.get(template);
  if (cached) return cached;

  const variables = templateVariables(template);
  const defined = new Set(Object.keys(variables));
  const unsafeKeys = [...defined].filter((key) => !CONDITION_SAFE_VARIABLE_KEY.test(key));
  const produced = new Set<string>();
  const references = new Map<string, VariableReference[]>();
  const tokens: { name: string; path: Path; local: ReadonlySet<string> }[] = [];
  const conditionReads: { name: string; path: Path }[] = [];
  const unsafeConditionKeys: { name: string; path: Path }[] = [];

  const refer = (name: string, reference: VariableReference) => {
    const list = references.get(name);
    if (list) list.push(reference);
    else references.set(name, [reference]);
  };

  mapTemplate(template, {
    text(value, path, local) {
      for (const name of tokenNames(value)) {
        refer(name, { path, kind: 'token' });
        tokens.push({ name, path, local });
      }
      return value;
    },
    condition(value, path) {
      const found = conditionNames(value, unsafeKeys);
      for (const name of found.tokens) {
        refer(name, { path, kind: 'token' });
        // a condition's tokens also see what the node gives conditions (env, os, arch…)
        tokens.push({ name, path, local: CONDITION_NAMES });
      }
      for (const name of found.unsafe) {
        refer(name, { path, kind: 'condition' });
        unsafeConditionKeys.push({ name, path });
      }
      for (const name of found.names) {
        refer(name, { path, kind: 'condition' });
        conditionReads.push({ name, path });
      }
      return value;
    },
    produces(name) {
      produced.add(name);
    },
  });

  const usage: VariableUsage = {
    references,
    undefinedTokens: tokens
      .filter((t) => !defined.has(t.name) && !produced.has(t.name) && !BUILTIN_TOKENS.has(t.name) && !t.local.has(t.name))
      .map(({ name, path }) => ({ name, path })),
    // a step output becomes one of the server's variables (operation_process.go), so later
    // conditions read it as well
    undefinedConditionNames: conditionReads.filter((r) => !defined.has(r.name) && !produced.has(r.name) && !CONDITION_NAMES.has(r.name)),
    unsafeConditionKeys,
  };
  cache.set(template, usage);
  return usage;
}

/** How many places use the variable `key`. */
export function referenceCount(usage: VariableUsage, key: string): number {
  return usage.references.get(key)?.length ?? 0;
}

/**
 * Renames the variable `from` to `to` where it is used: its `${from}` tokens and, in conditions,
 * its bare name. A condition cannot read a name with a hyphen (or a leading digit), so when `to`
 * is one of those the bare names stay as they are, and the condition warnings point at them.
 * Gives back the template (itself when nothing changed) and how many places changed. The
 * variable's own key and the groups are not touched here (see renameVariable).
 */
export function renameVariableUsages(template: JsonObject, from: string, to: string): { template: JsonObject; count: number } {
  if (from === to) return { template, count: 0 };
  let count = 0;
  const token = `\${${from}}`;
  const replaceTokens = (text: string, local: ReadonlySet<string>) => {
    if (local.has(from) || !text.includes(token)) return text;
    count++;
    return text.split(token).join(`\${${to}}`);
  };
  const name = conditionNamePattern(from);
  const renameNames = CONDITION_SAFE_VARIABLE_KEY.test(to);
  const next = mapTemplate(template, {
    text: (value, _path, local) => replaceTokens(value, local),
    condition(value) {
      const withTokens = value.includes(token) ? value.split(token).join(`\${${to}}`) : value;
      const renamed = mapConditionCode(withTokens, (code) =>
        // keep tokens of other names intact while renaming bare names
        code.replace(/\$\{[^}]*\}|[^$]+|\$/g, (part) => (part.startsWith('${') || !renameNames ? part : part.replace(name, to))),
      );
      if (renamed !== value) count++;
      return renamed;
    },
  });
  return { template: next, count };
}

/**
 * Renames a variable: its key in `data` (keeping its place), the groups that list it and, when
 * `withUsages`, every place that uses it. Returns the template itself when the key cannot be
 * renamed (missing, or `to` is taken).
 */
export function renameVariable(template: JsonObject, from: string, to: string, withUsages: boolean): JsonObject {
  const renamed = renameKeyIn(template, ['data'], from, to);
  if (renamed === template) return template;
  const next = replaceInGroups(renamed, from, to);
  return withUsages ? renameVariableUsages(next, from, to).template : next;
}

/** Removes a variable and takes it out of the groups that list it. */
export function deleteVariable(template: JsonObject, key: string): JsonObject {
  const next = deleteIn(template, ['data', key]);
  return next === template ? template : replaceInGroups(next, key, null);
}

/** Groups list their variables by key: a renamed one is renamed there, a deleted one removed. */
function replaceInGroups(template: JsonObject, from: string, to: string | null): JsonObject {
  if (!Array.isArray(template.groups)) return template;
  const groups = mapList(template.groups, (group) => {
    if (!isPlainObject(group) || !Array.isArray(group.variables) || !group.variables.includes(from)) return group;
    const variables = to === null ? group.variables.filter((v) => v !== from) : group.variables.map((v) => (v === from ? to : v));
    return { ...group, variables };
  });
  return groups === template.groups ? template : { ...template, groups };
}
