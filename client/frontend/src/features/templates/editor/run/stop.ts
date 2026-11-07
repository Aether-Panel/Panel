import type { JsonObject } from '../paths';

// How the node stops a server (Server.Stop): with a stopCode other than 0 it sends that signal
// to the process; otherwise it types the stop command into the console. A template normally has
// one of the two; when it has both, the code wins and the command is never sent.

export type StopMode = 'command' | 'signal';

/** Signals templates use to stop a server, most common first; the value is what stopCode holds. */
export const STOP_SIGNALS = [
  { code: 15, name: 'SIGTERM' },
  { code: 2, name: 'SIGINT' },
  { code: 9, name: 'SIGKILL' },
  { code: 1, name: 'SIGHUP' },
  { code: 3, name: 'SIGQUIT' },
] as const;

/** Whether `run` has a stop code the node acts on: any value but missing, null and 0. */
export function hasStopCode(run: JsonObject): boolean {
  return run.stopCode !== undefined && run.stopCode !== null && run.stopCode !== 0;
}

/** Whether `run` has a stop command: any value but missing, null and "". */
export function hasStopCommand(run: JsonObject): boolean {
  return run.stop !== undefined && run.stop !== null && run.stop !== '';
}

/**
 * The way the template stops the server, as the node decides it: the signal when there is a
 * code, the command when there is one; null when there is neither (the form then keeps showing
 * the way the user picked last).
 */
export function stopMode(run: JsonObject): StopMode | null {
  if (hasStopCode(run)) return 'signal';
  if (hasStopCommand(run)) return 'command';
  return null;
}

/** What the other way held when the user switched away from it, to bring back on switching again. */
export interface StopMemory {
  stop?: unknown;
  stopCode?: unknown;
}

/**
 * The object with the key `from` replaced by `to` (holding `value`) at the same position, or
 * `to` added at the end when `from` is not there; `value` undefined only removes `from`.
 */
function replaceKey(object: JsonObject, from: string, to: string, value: unknown): JsonObject {
  const entries: [string, unknown][] = [];
  let placed = value === undefined;
  for (const [key, current] of Object.entries(object)) {
    if (key === from) {
      if (!placed) entries.push([to, value]);
      placed = true;
    } else if (key !== to || value === undefined) {
      entries.push([key, current]);
    }
  }
  if (!placed) entries.push([to, value]);
  return Object.fromEntries(entries);
}

/**
 * `run` switched to stopping the other way, by the user's choice: the key of the way left is
 * removed (so the node does what the form shows) and remembered, and the key of the way chosen
 * gets back what it held before, in the place of the removed one. Nothing is made up: switching
 * to a way never used leaves its field empty.
 */
export function switchStopMode(run: JsonObject, to: StopMode, memory: StopMemory): { run: JsonObject; memory: StopMemory } {
  if (to === 'signal') {
    const next = { ...memory, stop: Object.hasOwn(run, 'stop') ? run.stop : memory.stop };
    const restored = Object.hasOwn(run, 'stopCode') ? run.stopCode : memory.stopCode;
    return { run: replaceKey(run, 'stop', 'stopCode', restored), memory: next };
  }
  const next = { ...memory, stopCode: Object.hasOwn(run, 'stopCode') ? run.stopCode : memory.stopCode };
  const restored = Object.hasOwn(run, 'stop') ? run.stop : memory.stop;
  return { run: replaceKey(run, 'stopCode', 'stop', restored), memory: next };
}
