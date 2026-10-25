'use client';
import { useState, type ReactNode } from 'react';
import { Package } from 'lucide-react';
import { cn } from '@/lib/utils';

export type ModpackSource = 'modrinth' | 'curseforge';

export type InstalledModpack = {
  source: ModpackSource;
  projectId: string;
  versionId: string;
  title: string;
  versionName: string;
  iconUrl: string;
  loader: string;
  gameVersion: string;
  installedAt: string;
};

export type ModpackStatus = {
  installed: InstalledModpack | null;
  installing: boolean;
  step: string;
  current: number;
  total: number;
  lastError: string;
  target?: InstalledModpack;
  curseforge: boolean;
  platform: { loader: string; gameVersion: string };
  memory: number;
};

export type ModpackHit = {
  id: string;
  slug: string;
  title: string;
  description: string;
  author: string;
  iconUrl: string;
  downloads: number;
  loaders: string[];
  gameVersions: string[];
  updatedAt: string;
};

export type ModpackSearchResult = {
  hits: ModpackHit[];
  total: number;
  offset: number;
  limit: number;
};

export type ModpackVersion = {
  id: string;
  name: string;
  versionNumber: string;
  gameVersions: string[];
  loaders: string[];
  publishedAt: string;
  downloads: number;
  type?: string;
  serverPack: boolean;
};

const KNOWN_LOADERS = new Set(['fabric', 'forge', 'neoforge', 'vanilla', 'paper', 'spigot']);

/** Readable loader name; unknown launchers are shown as they are. */
export function loaderLabel(t: (key: string) => string, loader: string): string {
  const key = (loader || 'vanilla').toLowerCase();
  return KNOWN_LOADERS.has(key) ? t(`servers.modpacks.loaders.${key}`) : loader;
}

export const SOURCE_NAMES: Record<ModpackSource, string> = { modrinth: 'Modrinth', curseforge: 'CurseForge' };

/** Where the pack lives on its site, for the "view on" link. */
export function projectUrl(source: ModpackSource, projectId: string, slug?: string) {
  if (source === 'modrinth') return `https://modrinth.com/modpack/${encodeURIComponent(slug || projectId)}`;
  return slug
    ? `https://www.curseforge.com/minecraft/modpacks/${encodeURIComponent(slug)}`
    : `https://www.curseforge.com/projects/${encodeURIComponent(projectId)}`;
}

/** Java the server will use for a game version; mirrors javaForMinecraft on the node. */
export function javaFor(gameVersion: string): string {
  const parts = gameVersion.split('.').map((n) => Number.parseInt(n, 10));
  if (parts.some(Number.isNaN) || parts.length < 2) return '21';
  const [major, minor, patch = 0] = parts;
  if (major >= 26) return '25';
  if (major > 1 || minor > 20 || (minor === 20 && patch >= 5)) return '21';
  if (minor >= 17) return '17';
  return '8';
}

/** "1.20.1" or "1.18.2 – 1.21.1" for a list sorted newest first. */
export function versionRange(versions: string[]): string {
  if (versions.length === 0) return '';
  if (versions.length === 1) return versions[0];
  return `${versions.at(-1)} – ${versions[0]}`;
}

const ICON_BOX = { sm: 'h-9 w-9 rounded-lg', md: 'h-12 w-12 rounded-xl', lg: 'h-16 w-16 rounded-2xl' } as const;
const ICON_GLYPH = { sm: 'h-4 w-4', md: 'h-5 w-5', lg: 'h-7 w-7' } as const;

/** The pack's icon, with a quiet fallback when it has none or it fails to load. */
export function PackIcon({ src, size = 'md', className }: Readonly<{ src?: string; size?: 'sm' | 'md' | 'lg'; className?: string }>) {
  const [failed, setFailed] = useState(false);
  const box = ICON_BOX[size];
  const icon = ICON_GLYPH[size];
  return (
    <span className={cn('relative grid shrink-0 place-items-center overflow-hidden border border-border/60 bg-secondary/60', box, className)}>
      {src && !failed ? (
        <img src={src} alt="" loading="lazy" decoding="async" onError={() => setFailed(true)} className="h-full w-full object-cover" />
      ) : (
        <Package className={cn('text-muted-foreground', icon)} aria-hidden="true" />
      )}
    </span>
  );
}

/** Small neutral tag for a loader or a game version. */
export function PackTag({ children, mono, className }: Readonly<{ children: ReactNode; mono?: boolean; className?: string }>) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-md border border-border/60 bg-secondary/60 px-1.5 py-px text-[11px] font-medium text-secondary-foreground',
        mono && 'font-mono tabular-nums',
        className,
      )}
    >
      {children}
    </span>
  );
}
