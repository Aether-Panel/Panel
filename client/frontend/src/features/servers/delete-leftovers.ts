// A server delete that went through but left MySQL databases on a host that could not drop
// them answers 200 with this list (204 when nothing was left). The page navigates away right
// after the delete, so the list is handed to the servers page in sessionStorage.

export type LeftoverDatabase = {
  hostId: number;
  hostName: string;
  database: string;
  username: string;
};

const STORAGE_KEY = 'aether.serverDeleteLeftovers';

function isLeftover(value: unknown): value is LeftoverDatabase {
  if (!value || typeof value !== 'object') return false;
  const v = value as Record<string, unknown>;
  return typeof v.database === 'string' && typeof v.username === 'string' && typeof v.hostName === 'string';
}

/** The leftovers in the delete's answer: none for a 204 (empty text) or anything unexpected. */
export function leftoversFrom(answer: unknown): LeftoverDatabase[] {
  if (!answer || typeof answer !== 'object') return [];
  const list = (answer as { leftoverDatabases?: unknown }).leftoverDatabases;
  return Array.isArray(list) ? list.filter(isLeftover) : [];
}

/** "database (user) on host", one per leftover, for the warning. */
export function describeLeftovers(list: LeftoverDatabase[]): string {
  return list
    .map((l) => (l.hostName ? `${l.database} (${l.username}) @ ${l.hostName}` : `${l.database} (${l.username})`))
    .join(', ');
}

export function rememberLeftovers(list: LeftoverDatabase[]): void {
  if (list.length === 0) return;
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(list));
  } catch {
    // without storage the warning is lost; the panel's log still lists them
  }
}

/** The leftovers of the last delete, once: reading them clears them. */
export function takeLeftovers(): LeftoverDatabase[] {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (raw === null) return [];
    sessionStorage.removeItem(STORAGE_KEY);
    return leftoversFrom({ leftoverDatabases: JSON.parse(raw) });
  } catch {
    return [];
  }
}
