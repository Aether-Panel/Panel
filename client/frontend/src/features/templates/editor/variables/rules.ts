import type { WarningRule } from '../warnings';
import { analyzeVariableUsage, PANEL_VARIABLES, referenceCount, templateVariables } from './usage';

// The warning rules about variables (see warnings.ts). They share one usage analysis per
// template object (analyzeVariableUsage caches it), so adding them costs one walk.

/**
 * `${name}` where no variable, step output or built-in has that name: the node leaves the text
 * as it is. It is a warning, not an error, because a shell variable (`${HOME}` in a command)
 * looks the same and is fine.
 */
export const undefinedVariables: WarningRule = (template) =>
  analyzeVariableUsage(template).undefinedTokens.map(({ name, path }) => ({
    path,
    code: 'undefinedVariable',
    params: { token: `\${${name}}` },
  }));

/** A condition reading a name that is no variable: the node fails the step on it. */
export const undefinedConditionVariables: WarningRule = (template) =>
  analyzeVariableUsage(template).undefinedConditionNames.map(({ name, path }) => ({
    path,
    code: 'undefinedConditionVariable',
    params: { name },
  }));

/**
 * A variable no command, step or condition uses. The ones the panel reads by name (resources,
 * address, MOTD, mod launcher) are used without a token, so they are left out. Internal ones
 * count too: hidden from users, they only matter where the template uses them.
 */
export const unusedVariables: WarningRule = (template) => {
  const usage = analyzeVariableUsage(template);
  return Object.keys(templateVariables(template))
    .filter((key) => !PANEL_VARIABLES.has(key) && referenceCount(usage, key) === 0)
    .map((key) => ({ path: ['data', key], code: 'unusedVariable' }));
};

/**
 * A key with a hyphen (or a leading digit) inside a condition: CEL reads `server-port` as
 * server minus port, so the condition never sees the variable. `${server-port}` works there.
 */
export const conditionVariableKeys: WarningRule = (template) =>
  analyzeVariableUsage(template).unsafeConditionKeys.map(({ name, path }) => ({
    path,
    code: 'conditionVariableKey',
    params: { name, token: `\${${name}}` },
  }));

export const VARIABLE_WARNING_RULES: readonly WarningRule[] = [undefinedVariables, undefinedConditionVariables, unusedVariables, conditionVariableKeys];
