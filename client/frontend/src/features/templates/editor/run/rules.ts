import { isPlainObject } from '../paths';
import type { WarningRule } from '../warnings';
import { commandEntries, commandMode, hasCondition } from './command';
import { hasStopCode, hasStopCommand } from './stop';

// Warnings about how the server starts and stops (see warnings.ts): what the node accepts but
// does not do what the template seems to say.

/**
 * A list of commands where every entry has a condition: when none holds, the node has nothing
 * to start the server with. Entries after the first one without a condition are never used.
 */
export const commandFallback: WarningRule = (template) => {
  const run = template.run;
  if (!isPlainObject(run) || commandMode(run.command) !== 'conditional') return [];
  const entries = commandEntries(run.command);
  const defaults = entries.flatMap((entry, index) => (hasCondition(entry) ? [] : [index]));
  if (defaults.length === 0) return [{ path: ['run', 'command'], code: 'noDefaultCommand' }];
  return defaults.slice(1).map((index) => ({ path: ['run', 'command', index], code: 'extraDefaultCommand' }));
};

/**
 * How the server stops: with a stop code the node sends the signal and never the stop command,
 * so a template with both keeps a command that does nothing; with neither, stopping types an
 * empty line into the console, which most games ignore.
 */
export const stopMethod: WarningRule = (template) => {
  const run = template.run;
  if (!isPlainObject(run)) return [];
  const code = hasStopCode(run);
  const command = hasStopCommand(run);
  if (code && command) return [{ path: ['run', 'stop'], code: 'stopCommandIgnored' }];
  if (!code && !command) return [{ path: ['run', 'stop'], code: 'noStopMethod' }];
  return [];
};

export const RUN_WARNING_RULES: readonly WarningRule[] = [commandFallback, stopMethod];
