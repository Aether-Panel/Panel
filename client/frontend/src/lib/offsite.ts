/**
 * Off-site (S3) backup values shared by the panel settings, a server's Backups tab and the
 * task editor, so all three offer exactly what the node accepts.
 */

/**
 * Copies to keep, on the server by a task (MaxAutoBackupKeep) and in S3 (OffsiteKeep): the node
 * accepts the same 1-50 for both.
 */
export const MIN_KEEP = 1;
export const MAX_KEEP = 50;

/** The intervals the node accepts for the periodic copy, in hours (0 = off). */
export const PERIODIC_HOURS = [0, 12, 24, 48, 72, 168, 336, 720] as const;

/** How often a task's backup step also uploads its copy to S3. */
export const OFFSITE_FREQUENCIES = ['never', 'always', 'weekend', 'weekly', 'biweekly', 'monthly'] as const;
export type OffsiteFrequency = (typeof OFFSITE_FREQUENCIES)[number];

type Translate = (key: string, options?: Record<string, string | number>) => string;

/** An interval as people say it: off, hours up to three days, then days. */
export function periodicHoursLabel(hours: number, t: Translate): string {
  if (hours === 0) return t('servers.backups.offsite.intervalOff');
  if (hours <= 72) return t('servers.backups.offsite.intervalHours', { count: hours });
  return t('servers.backups.offsite.intervalDays', { count: hours / 24 });
}

/** A copies-to-keep value typed by hand, held to what the node accepts. */
export function clampKeep(raw: string): number {
  return Math.min(MAX_KEEP, Math.max(MIN_KEEP, Number.parseInt(raw, 10) || MIN_KEEP));
}
