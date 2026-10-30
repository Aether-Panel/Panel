'use client';
import { useEffect, useMemo, useState } from 'react';
import { Users } from '@/components/icons';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { cn } from '@/lib/utils';

type Point = { t: number; players?: number; hasPlayers?: boolean };

/** Days in the order the week is shown, Monday first; values are Date.getDay(). */
const WEEK = [1, 2, 3, 4, 5, 6, 0];
const HOURS = Array.from({ length: 24 }, (_, hour) => hour);

/**
 * When people play: the last week's average players for every weekday and hour, in the
 * viewer's own time, so owners know when to hold events and when a restart hurts least.
 * Hidden for servers that never reported players.
 */
export default function PeakHours({ serverId }: Readonly<{ serverId: string }>) {
  const { t, language } = useTranslations();
  const locale = language === 'es' ? 'es-ES' : 'en-US';
  const [points, setPoints] = useState<Point[] | null>(null);

  useEffect(() => {
    let cancelled = false;
    api.get<{ points: Point[] }>(`/api/servers/${serverId}/metrics?range=7d`)
      .then((res) => !cancelled && setPoints(res.points ?? []))
      .catch(() => !cancelled && setPoints([]));
    return () => {
      cancelled = true;
    };
  }, [serverId]);

  const grid = useMemo(() => {
    const sums = Array.from({ length: 7 }, () => new Array<number>(24).fill(0));
    const counts = Array.from({ length: 7 }, () => new Array<number>(24).fill(0));
    for (const p of points ?? []) {
      if (!p.hasPlayers) continue;
      const d = new Date(p.t * 1000);
      sums[d.getDay()][d.getHours()] += p.players ?? 0;
      counts[d.getDay()][d.getHours()] += 1;
    }
    const values = sums.map((row, day) => row.map((sum, hour) => (counts[day][hour] ? sum / counts[day][hour] : null)));
    let best: { day: number; hour: number; value: number } | null = null;
    let max = 0;
    values.forEach((row, day) => row.forEach((value, hour) => {
      if (value !== null && value > max) {
        max = value;
        best = { day, hour, value };
      }
    }));
    return { values, max, best: best as { day: number; hour: number; value: number } | null };
  }, [points]);

  if (!points || grid.best === null) return null;

  const dayName = (day: number, style: 'short' | 'long') =>
    new Date(2024, 0, 7 + day).toLocaleDateString(locale, { weekday: style });
  const hourLabel = (hour: number) => new Date(2024, 0, 1, hour).toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' });
  const best = grid.best;

  return (
    <Card>
      <CardHeader className="flex flex-col gap-3 space-y-0 border-b border-border/50 p-5 sm:flex-row sm:items-start sm:justify-between md:p-6">
        <div className="min-w-0 space-y-1">
          <CardTitle className="text-lg font-semibold tracking-[-0.02em]">{t('servers.peakHours.title')}</CardTitle>
          <CardDescription>{t('servers.peakHours.description')}</CardDescription>
        </div>
        <p className="flex shrink-0 items-center gap-2 rounded-lg border border-success/30 bg-success/[0.08] px-3 py-1.5 text-xs text-success">
          <Users className="h-3.5 w-3.5" aria-hidden="true" />
          <span>
            {t('servers.peakHours.best', { day: dayName(best.day, 'long'), time: hourLabel(best.hour), count: best.value.toFixed(1) })}
          </span>
        </p>
      </CardHeader>
      <CardContent className="p-5 md:p-6">
        <p className="sr-only">
          {t('servers.peakHours.best', { day: dayName(best.day, 'long'), time: hourLabel(best.hour), count: best.value.toFixed(1) })}
        </p>
        <div aria-hidden="true" className="grid grid-cols-[2.25rem_minmax(0,1fr)] gap-x-2 gap-y-[3px] sm:grid-cols-[2.75rem_minmax(0,1fr)]">
          {WEEK.map((day, row) => (
            <div key={day} className="contents">
              <span className="self-center text-[11px] capitalize text-muted-foreground">{dayName(day, 'short').replace('.', '')}</span>
              <div className="grid gap-[3px]" style={{ gridTemplateColumns: 'repeat(24, minmax(0, 1fr))' }}>
                {HOURS.map((hour) => {
                  const value = grid.values[day][hour];
                  const level = value === null || grid.max === 0 ? 0 : value / grid.max;
                  const isBest = best.day === day && best.hour === hour;
                  return (
                    <span
                      key={hour}
                      title={value === null
                        ? `${dayName(day, 'long')} ${hourLabel(hour)} · ${t('servers.peakHours.noData')}`
                        : `${dayName(day, 'long')} ${hourLabel(hour)} · ${t('servers.peakHours.average', { count: value.toFixed(1) })}`}
                      className={cn(
                        // a fixed height keeps the week compact on wide screens; the cells only stretch sideways
                        'h-3 rounded-[3px] animate-in fade-in-0 zoom-in-50 fill-mode-both duration-500 ease-panel sm:h-[18px]',
                        // no data, nobody online and some people online must not look alike
                        value === null && 'border border-dashed border-border/70',
                        value === 0 && 'bg-secondary',
                        value !== null && value > 0 && 'bg-success',
                        isBest && 'ring-2 ring-success/70 ring-offset-1 ring-offset-card',
                      )}
                      style={{
                        opacity: value === null || value === 0 ? 1 : 0.3 + level * 0.7,
                        // the week fills in left to right, row by row
                        animationDelay: `${hour * 12 + row * 20}ms`,
                      }}
                    />
                  );
                })}
              </div>
            </div>
          ))}
          <span />
          <div className="mt-1.5 flex justify-between text-[10px] tabular-nums text-muted-foreground">
            {[0, 6, 12, 18, 23].map((hour) => (
              <span key={hour}>{hourLabel(hour)}</span>
            ))}
          </div>
        </div>
        <div className="mt-4 flex items-center justify-end gap-2 text-[11px] text-muted-foreground">
          <span>{t('servers.peakHours.fewer')}</span>
          <span aria-hidden="true" className="h-2.5 w-2.5 rounded-[3px] bg-secondary" />
          {[0.3, 0.55, 0.8, 1].map((o) => (
            <span key={o} aria-hidden="true" className="h-2.5 w-2.5 rounded-[3px] bg-success" style={{ opacity: o }} />
          ))}
          <span>{t('servers.peakHours.more')}</span>
        </div>
      </CardContent>
    </Card>
  );
}
