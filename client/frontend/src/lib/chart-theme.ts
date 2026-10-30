/**
 * One colour per kind of data across every chart of the panel, so CPU, memory, download and
 * upload read the same on the server page, its history and the dashboard.
 */
export const CHART_COLORS = {
  cpu: 'hsl(var(--chart-1))',
  memory: 'hsl(var(--chart-2))',
  download: 'hsl(var(--chart-3))',
  upload: 'hsl(var(--chart-5))',
  players: 'hsl(var(--success))',
  playersPeak: 'hsl(var(--chart-4))',
} as const;

/** The locale charts write times and dates in: the panel's language, not the browser's. */
export function chartLocale(language: string): string {
  return language === 'es' ? 'es-ES' : 'en-US';
}

/** A live sample's time, to the second, in the panel's language (24 h in Spanish). */
export function formatClock(ts: number, language: string): string {
  return new Date(ts).toLocaleTimeString(chartLocale(language), { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}
