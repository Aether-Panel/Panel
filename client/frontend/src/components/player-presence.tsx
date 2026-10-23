import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

/** A steady hue per name, so the same player keeps the same colour everywhere. */
function nameHue(name: string) {
  let hash = 0;
  for (const char of name) hash = (hash * 31 + char.charCodeAt(0)) >>> 0;
  return hash % 360;
}

function initials(name: string) {
  const clean = name.replace(/[^\p{L}\p{N}]+/gu, ' ').trim();
  if (!clean) return '?';
  const parts = clean.split(/\s+/);
  return (parts.length > 1 ? parts[0][0] + parts[1][0] : clean.slice(0, 2)).toUpperCase();
}

/** A small round badge with a player's initials on their own hue. */
export function PlayerAvatar({ name, className }: { name: string; className?: string }) {
  const hue = nameHue(name);
  return (
    <span
      aria-hidden="true"
      className={cn('inline-flex shrink-0 items-center justify-center rounded-full font-semibold text-white ring-2 ring-card', className)}
      style={{ background: `linear-gradient(135deg, hsl(${hue} 70% 55%), hsl(${(hue + 40) % 360} 70% 42%))` }}
    >
      {initials(name)}
    </span>
  );
}

/**
 * The faces of who is online, overlapping, with everyone's name on hover. Shows up to
 * `limit` and a "+N" for the rest.
 */
export function PlayerStack({ names, total, limit = 4, label }: { names: string[]; total: number; limit?: number; label: string }) {
  if (names.length === 0) return null;
  const shown = names.slice(0, limit);
  const more = Math.max(total, names.length) - shown.length;

  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            aria-label={`${label}: ${names.join(', ')}`}
            className="focus-ring flex items-center rounded-full"
          >
            <span className="flex -space-x-1.5">
              {shown.map((name, index) => (
                // a short stagger, so the faces arrive one after another
                <span
                  key={`${index}-${name}`}
                  className="inline-flex animate-in fade-in-0 zoom-in-50 fill-mode-both duration-300 ease-panel"
                  style={{ animationDelay: `${index * 60}ms` }}
                >
                  <PlayerAvatar name={name} className="h-5 w-5 text-[9px]" />
                </span>
              ))}
            </span>
            {more > 0 && (
              <span className="ml-1 text-[11px] font-medium tabular-nums text-muted-foreground">+{more}</span>
            )}
          </button>
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-[16rem]">
          <p className="mb-1 text-xs font-medium text-muted-foreground">{label}</p>
          <ul className="space-y-1">
            {names.map((name, index) => (
              <li key={`${index}-${name}`} className="flex items-center gap-2 text-xs">
                <PlayerAvatar name={name} className="h-4 w-4 text-[8px] ring-0" />
                <span className="truncate">{name}</span>
              </li>
            ))}
          </ul>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

/** A dot that breathes while people are playing and rests when nobody is. */
export function LiveDot({ active, className }: { active: boolean; className?: string }) {
  return (
    <span aria-hidden="true" className={cn('relative flex h-2 w-2 shrink-0', className)}>
      {active && <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-success opacity-60" />}
      <span className={cn('relative inline-flex h-2 w-2 rounded-full', active ? 'bg-success shadow-[0_0_8px_hsl(var(--success)/0.8)]' : 'bg-muted-foreground/40')} />
    </span>
  );
}
