import { isPlainObject, type JsonObject } from '../paths';

// run.command, as the node reads it (Server.resolveCommand): a single command line, or a list of
// entries { if, command } where the first entry whose condition holds is used and, when none
// does, the first entry without a condition. The form edits either shape and converts between
// them only when nothing would be lost.

export type CommandMode = 'single' | 'conditional';

/**
 * The shape of the command: one line (also when it is missing or null, which the form fills
 * as one line), a list of entries, or something the form cannot show (a number, a list holding
 * anything but objects), which is left to the JSON.
 */
export function commandMode(value: unknown): CommandMode | 'other' {
  if (value === undefined || value === null || typeof value === 'string') return 'single';
  if (Array.isArray(value)) return value.every(isPlainObject) ? 'conditional' : 'other';
  return 'other';
}

/** The entries of a list command; anything else has none. */
export function commandEntries(value: unknown): readonly JsonObject[] {
  return Array.isArray(value) ? value.filter(isPlainObject) : [];
}

/** Whether an entry has a condition the node evaluates: a missing, null or blank `if` has none. */
export function hasCondition(entry: JsonObject): boolean {
  return typeof entry.if === 'string' ? entry.if.trim() !== '' : entry.if !== undefined && entry.if !== null;
}

/**
 * The single command as a list: one entry without a condition, which the node uses when no other
 * entry's condition holds, so the server starts exactly as before.
 */
export function toConditional(value: unknown): JsonObject[] {
  if (Array.isArray(value)) return commandEntries(value).slice();
  return [{ command: typeof value === 'string' ? value : '' }];
}

/**
 * Whether the list can become one line without losing anything: at most one entry, holding only
 * its command (no condition, no console settings of its own, nothing unknown).
 */
export function canMakeSingle(value: unknown): boolean {
  const entries = commandEntries(value);
  if (!Array.isArray(value) || entries.length !== value.length || entries.length > 1) return false;
  if (entries.length === 0) return true;
  const entry = entries[0];
  return (
    (entry.command === undefined || typeof entry.command === 'string') &&
    !hasCondition(entry) &&
    Object.keys(entry).every((key) => key === 'command' || key === 'if')
  );
}

/** The one line a list that canMakeSingle() becomes. */
export function toSingle(value: unknown): string {
  const command = commandEntries(value)[0]?.command;
  return typeof command === 'string' ? command : '';
}

/** Where the node finds its fallback: the first entry without a condition; -1 when none has. */
export function defaultEntryIndex(entries: readonly JsonObject[]): number {
  return entries.findIndex((entry) => !hasCondition(entry));
}
