/**
 * Schedules for scheduled tasks, kept as simple choices in the UI and stored as cron.
 *
 * The node reads cron with an optional seconds field and an optional CRON_TZ= prefix. The
 * panel always writes five fields behind the creator's time zone, so "05:00" means 05:00
 * where the person who set it lives, whatever zone the node runs in.
 */

export type Schedule =
  | { kind: 'daily'; time: string }
  | { kind: 'weekly'; time: string; days: number[] }
  | { kind: 'interval'; hours: number }
  | { kind: 'custom'; expression: string }
  | { kind: 'manual' };

export type ScheduleKind = Schedule['kind'];

/** Hour steps that divide the day evenly, so "every N hours" keeps the same times daily. */
export const INTERVAL_HOURS = [1, 2, 3, 4, 6, 8, 12];

/** Days in the order the week is shown (Monday first); values are cron's (0 = Sunday). */
export const WEEK_DAYS = [1, 2, 3, 4, 5, 6, 0];

export function localTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  } catch {
    return 'UTC';
  }
}

function splitTime(time: string): [number, number] {
  const [h, m] = time.split(':').map((part) => Number.parseInt(part, 10));
  return [Number.isFinite(h) ? h : 0, Number.isFinite(m) ? m : 0];
}

export function toCron(schedule: Schedule, zone: string): string {
  const prefix = `CRON_TZ=${zone} `;
  switch (schedule.kind) {
    case 'daily': {
      const [h, m] = splitTime(schedule.time);
      return `${prefix}${m} ${h} * * *`;
    }
    case 'weekly': {
      const [h, m] = splitTime(schedule.time);
      const days = [...schedule.days].sort((a, b) => a - b).join(',');
      return `${prefix}${m} ${h} * * ${days}`;
    }
    case 'interval':
      return schedule.hours === 1 ? `${prefix}0 * * * *` : `${prefix}0 */${schedule.hours} * * *`;
    case 'custom':
      return `${prefix}${schedule.expression.trim().replace(/\s+/g, ' ')}`;
    case 'manual':
      return '';
  }
}

const pad = (n: number) => String(n).padStart(2, '0');
const isNumber = (value: string, max: number) => /^\d{1,2}$/.test(value) && Number(value) <= max;

/** Daily or weekly at a fixed time, from cron's minute, hour and day-of-week fields. */
function timedSchedule(minute: string, hour: string, dow: string): Schedule | null {
  if (!isNumber(minute, 59) || !isNumber(hour, 23)) return null;
  const time = `${pad(Number(hour))}:${pad(Number(minute))}`;
  if (dow === '*') return { kind: 'daily', time };
  if (/^[0-6](,[0-6])*$/.test(dow)) {
    const days = [...new Set(dow.split(',').map(Number))];
    return { kind: 'weekly', time, days };
  }
  return null;
}

/** Every N hours on the hour, from cron's minute, hour and day-of-week fields. */
function intervalSchedule(minute: string, hour: string, dow: string): Schedule | null {
  if (minute !== '0' || dow !== '*') return null;
  if (hour === '*') return { kind: 'interval', hours: 1 };
  const step = /^\*\/(\d+)$/.exec(hour);
  if (step && INTERVAL_HOURS.includes(Number(step[1]))) return { kind: 'interval', hours: Number(step[1]) };
  return null;
}

/** One of the simple choices for five cron fields, or null when it takes custom cron. */
function simpleSchedule(fields: string[]): Schedule | null {
  if (fields.length !== 5) return null;
  const [minute, hour, dom, month, dow] = fields;
  if (dom !== '*' || month !== '*') return null;
  return timedSchedule(minute, hour, dow) ?? intervalSchedule(minute, hour, dow);
}

/** Reads a stored cron back into a choice, falling back to "custom" for anything else. */
export function fromCron(spec: string | undefined): { schedule: Schedule; zone: string | null } {
  if (!spec?.trim()) return { schedule: { kind: 'manual' }, zone: null };

  let fields = spec.trim().split(/\s+/);
  let zone: string | null = null;
  const zoneMatch = /^(?:CRON_TZ|TZ)=(.+)$/.exec(fields[0]);
  if (zoneMatch) {
    zone = zoneMatch[1];
    fields = fields.slice(1);
  }

  const schedule: Schedule = simpleSchedule(fields) ?? { kind: 'custom', expression: fields.join(' ') };
  return { schedule, zone };
}

type Translate = (key: string, params?: Record<string, string | number>) => string;

function weekdayName(day: number, locale: string) {
  // 7 January 2024 was a Sunday, so day d of that week has cron number d
  return new Date(2024, 0, 7 + day).toLocaleDateString(locale, { weekday: 'short' });
}

export function weekdayLabels(locale: string) {
  return WEEK_DAYS.map((day) => ({ day, label: weekdayName(day, locale) }));
}

export function describeSchedule(schedule: Schedule, t: Translate, locale: string): string {
  switch (schedule.kind) {
    case 'daily':
      return t('servers.tasks.schedule.describe.daily', { time: schedule.time });
    case 'weekly': {
      const days = WEEK_DAYS.filter((d) => schedule.days.includes(d)).map((d) => weekdayName(d, locale));
      return t('servers.tasks.schedule.describe.weekly', { days: days.join(', '), time: schedule.time });
    }
    case 'interval':
      return schedule.hours === 1
        ? t('servers.tasks.schedule.describe.intervalOne')
        : t('servers.tasks.schedule.describe.interval', { count: schedule.hours });
    case 'custom':
      return t('servers.tasks.schedule.describe.custom', { expression: schedule.expression });
    case 'manual':
      return t('servers.tasks.schedule.describe.manual');
  }
}

/**
 * The next run of a simple schedule, in the viewer's own time. Only worked out when the task
 * is in the viewer's zone: without CRON_TZ it runs on the node's clock, which the browser
 * does not know. Custom cron is left to the node.
 */
export function nextRun(schedule: Schedule, zone: string | null, now = new Date()): Date | null {
  if (zone !== localTimeZone()) return null;

  if (schedule.kind === 'interval') {
    const next = new Date(now);
    next.setMinutes(0, 0, 0);
    do {
      next.setHours(next.getHours() + 1);
    } while (next.getHours() % schedule.hours !== 0);
    return next;
  }
  if (schedule.kind !== 'daily' && schedule.kind !== 'weekly') return null;

  const [h, m] = splitTime(schedule.time);
  for (let offset = 0; offset <= 7; offset++) {
    const candidate = new Date(now.getFullYear(), now.getMonth(), now.getDate() + offset, h, m, 0, 0);
    if (candidate <= now) continue;
    if (schedule.kind === 'weekly' && !schedule.days.includes(candidate.getDay())) continue;
    return candidate;
  }
  return null;
}

export function describeNextRun(date: Date, t: Translate, locale: string, now = new Date()): string {
  const time = date.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' });
  const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const days = Math.round((startOfDay(date) - startOfDay(now)) / 86_400_000);
  if (days === 0) return t('servers.tasks.schedule.today', { time });
  if (days === 1) return t('servers.tasks.schedule.tomorrow', { time });
  return `${date.toLocaleDateString(locale, { weekday: 'short', day: 'numeric', month: 'short' })} ${time}`;
}
