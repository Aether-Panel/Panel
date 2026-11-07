import { isPlainObject, type JsonObject } from '../paths';
import { templateVariables } from '../variables/usage';
import type { WarningRule } from '../warnings';

// The Advanced section's pure side: variable groups, and the keep-alive's checks.

/** The groups of a template; a missing or null list (or anything else) has none. */
export function templateGroups(template: JsonObject): readonly unknown[] {
  return Array.isArray(template.groups) ? template.groups : [];
}

/** The variable keys a group lists, as text; anything else in the list is left out. */
export function groupVariables(group: unknown): string[] {
  if (!isPlainObject(group) || !Array.isArray(group.variables)) return [];
  return group.variables.filter((key): key is string => typeof key === 'string');
}

/**
 * A new, empty group, with the keys the node's Group always writes. Its order follows the others'
 * when they have one (the panel lists groups as they come, so order is only kept consistent).
 */
export function newGroup(groups: readonly unknown[]): JsonObject {
  const orders = groups.flatMap((g) => (isPlainObject(g) && typeof g.order === 'number' ? [g.order] : []));
  const group: JsonObject = { display: '', description: '', variables: [] };
  if (orders.length > 0) group.order = Math.max(...orders) + 1;
  return group;
}

/** A number as time.ParseDuration reads it: 5, 1.5, 1. or .5. */
const DURATION_NUMBER = /(?:\d+(?:\.\d*)?|\.\d+)/;
/** time.ParseDuration's units, with both spellings of the micro sign. */
const DURATION_UNIT = /(?:ns|us|µs|μs|ms|s|m|h)/;

/**
 * What time.ParseDuration reads (the keep-alive's frequency is parsed with it): numbers, each
 * with a unit, such as 30s, 5m or 1h30m. The node turns down zero and negative ones as well.
 */
export const GO_DURATION = new RegExp(`^(?:${DURATION_NUMBER.source}${DURATION_UNIT.source})+$`);

export function isPositiveDuration(value: unknown): boolean {
  if (typeof value !== 'string') return false;
  const text = value.trim();
  if (text !== value || !GO_DURATION.test(text)) return false;
  // some digit other than 0, so it is longer than nothing
  return /[1-9]/.test(text.replace(/[a-zµμ]+/g, ' '));
}

/**
 * The keep-alive runs only with both a frequency and a command, and a frequency the node can
 * read (it logs "invalid frequency" on the console and sends nothing otherwise).
 */
export const keepAliveRule: WarningRule = (template) => {
  const keepAlive = template.keepAlive;
  if (!isPlainObject(keepAlive)) return [];
  const frequency = keepAlive.frequency;
  const command = keepAlive.command;
  const hasFrequency = typeof frequency === 'string' && frequency !== '';
  const hasCommand = typeof command === 'string' && command !== '';
  if (hasFrequency && !isPositiveDuration(frequency)) return [{ path: ['keepAlive', 'frequency'], code: 'keepAliveFrequency' }];
  if (hasFrequency && !hasCommand) return [{ path: ['keepAlive', 'command'], code: 'keepAliveIncomplete' }];
  if (hasCommand && !hasFrequency) return [{ path: ['keepAlive', 'frequency'], code: 'keepAliveIncomplete' }];
  return [];
};

/** A group listing a key no variable has: the settings page has nothing to show for it. */
export const groupVariablesRule: WarningRule = (template) => {
  const variables = templateVariables(template);
  return templateGroups(template).flatMap((group, index) => {
    if (!isPlainObject(group) || !Array.isArray(group.variables)) return [];
    return group.variables.flatMap((key, at) =>
      typeof key === 'string' && !Object.hasOwn(variables, key)
        ? [{ path: ['groups', index, 'variables', at], code: 'groupUnknownVariable', params: { key } }]
        : [],
    );
  });
};

export const ADVANCED_WARNING_RULES: readonly WarningRule[] = [keepAliveRule, groupVariablesRule];
