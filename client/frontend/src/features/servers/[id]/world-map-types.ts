export type MapKind = 'bluemap' | 'dynmap';

export type WorldMapStatus = {
  supported: boolean;
  /** Why the map cannot run here: "vanilla" or "unknown". */
  reason: '' | 'vanilla' | 'unknown';
  platform?: 'paper' | 'fabric' | 'forge' | 'neoforge';
  gameVersion?: string;
  installed: '' | MapKind;
  version?: string;
  installing: boolean;
  lastError?: string;
  running: boolean;
  /** The map's own web server answers. */
  ready: boolean;
  /** Signed link to the viewer, only while a map is installed. */
  viewUrl?: string;
};

export const MAP_NAMES: Record<MapKind, string> = {
  bluemap: 'BlueMap',
  dynmap: 'Dynmap',
};

export const PLATFORM_NAMES: Record<string, string> = {
  paper: 'Paper',
  fabric: 'Fabric',
  forge: 'Forge',
  neoforge: 'NeoForge',
};
