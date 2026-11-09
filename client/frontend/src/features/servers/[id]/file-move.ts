/**
 * Moving files and folders inside the server: where each one lands, and the moves that cannot
 * happen (onto itself, or a folder into its own inside).
 */

/** A file or folder to move, by its name in the folder it is in. */
export type MoveEntry = { name: string; isFile: boolean };

/** One move: from and to are paths from the server root ("" is the root). */
export type MoveStep = { name: string; from: string; to: string };

export type MoveProblem = 'same' | 'inside';

export const joinPath = (dir: string, name: string) => (dir ? `${dir}/${name}` : name);

/** The folder a path is in ("" for the root). */
export function parentOf(path: string): string {
  const at = path.lastIndexOf('/');
  return at > 0 ? path.slice(0, at) : '';
}

/**
 * A folder typed by hand, as the daemon reads it: "~/plugins/", "/plugins" and "plugins" are
 * all "plugins", and "", "/", "~" and "." the root. null when it would leave the server ("..").
 */
export function normalizeFolder(input: string): string | null {
  const parts = input
    .trim()
    .replaceAll('\\', '/')
    .replace(/^~(?=\/|$)/, '')
    .split('/')
    .filter((part) => part !== '' && part !== '.');
  if (parts.includes('..')) return null;
  return parts.join('/');
}

/** Why a move cannot happen, or null when it can. */
export function moveProblem(from: string, to: string, isFile: boolean): MoveProblem | null {
  if (from === to) return 'same';
  if (!isFile && to.startsWith(`${from}/`)) return 'inside';
  return null;
}

/**
 * The moves for entries of fromDir going into toDir, each keeping its name, and the first
 * problem found (then nothing should move: half a selection moved is harder to undo).
 */
export function planMove(fromDir: string, entries: MoveEntry[], toDir: string): { steps: MoveStep[]; problem: MoveProblem | null } {
  const steps: MoveStep[] = [];
  for (const entry of entries) {
    const from = joinPath(fromDir, entry.name);
    const to = joinPath(toDir, entry.name);
    const problem = moveProblem(from, to, entry.isFile);
    if (problem) return { steps: [], problem };
    steps.push({ name: entry.name, from, to });
  }
  return { steps, problem: null };
}

/**
 * Runs the steps one after another through move and stops at the first failure, saying how
 * many went (a failure in the middle leaves the rest where they were).
 */
export async function runMoves(steps: MoveStep[], move: (step: MoveStep) => Promise<void>): Promise<{ done: number; error?: unknown }> {
  let done = 0;
  for (const step of steps) {
    try {
      await move(step);
    } catch (error) {
      return { done, error };
    }
    done++;
  }
  return { done };
}

/** The MIME type a dragged row carries, so a drop can tell it from files off the computer. */
export const ROW_DRAG_TYPE = 'application/x-aether-files';
