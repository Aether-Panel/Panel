import { CONDITION_SAFE_VARIABLE_KEY } from '../../schema/common';

// A step's condition (`if`): a CEL expression the node evaluates before running the step
// (conditions/executor.go). Most templates only tell host and Docker installs apart, so those
// two get presets; they write the exact expression the community templates use.

export const CONDITION_PRESETS = ['none', 'host', 'docker', 'custom'] as const;
export type ConditionPreset = (typeof CONDITION_PRESETS)[number];

/** What each preset writes; "none" removes the key. */
export const PRESET_EXPRESSIONS = { host: "env == 'host'", docker: "env == 'docker'" } as const;

// env == 'host', with either quote and any spacing, or the other way round ('host' == env)
const ENV_EQUALS = /^env\s*==\s*(['"])(host|docker)\1$|^(['"])(host|docker)\3\s*==\s*env$/;

/**
 * The preset a condition matches. No condition (missing, null or blank, which the node skips)
 * is "none"; `env == 'host'` or `env == 'docker'` written any equivalent way is that preset;
 * anything else, a non-string included, is "custom".
 */
export function conditionPreset(value: unknown): ConditionPreset {
  if (value === undefined || value === null) return 'none';
  if (typeof value !== 'string') return 'custom';
  const expression = value.trim();
  if (expression === '') return 'none';
  const match = ENV_EQUALS.exec(expression);
  if (!match) return 'custom';
  return (match[2] ?? match[4]) as 'host' | 'docker';
}

/**
 * The text that reads variable `key` inside a condition: its bare name, which CEL reads; a key
 * CEL cannot name (a hyphen, a leading digit) as a quoted `'${key}'` instead, which the node
 * replaces with the value before evaluating, so it compares as text.
 */
export function conditionReference(key: string): string {
  return CONDITION_SAFE_VARIABLE_KEY.test(key) ? key : `'\${${key}}'`;
}
