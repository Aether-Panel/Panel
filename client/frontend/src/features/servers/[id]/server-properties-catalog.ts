// What the visual editor knows about server.properties: each option's control, limits and
// Minecraft's own default (used when the key is not in the file yet). Labels and help text
// live in the locales under servers.properties.fields.<id>, where id is the key with dots
// replaced by underscores.

export type PropertySection = 'game' | 'players' | 'world' | 'display' | 'network' | 'advanced';

export type PropertyField = {
  key: string;
  section: PropertySection;
  type: 'bool' | 'enum' | 'number' | 'text' | 'secret';
  /** Minecraft's default, shown when the key is missing from the file */
  def: string;
  options?: string[];
  min?: number;
  max?: number;
  /** shown even when the file does not have it (otherwise only when present) */
  core?: boolean;
  /** the panel manages it (the address and port); shown read only */
  locked?: boolean;
  /** only changes newly generated worlds */
  newWorldsOnly?: boolean;
  /** a value that deserves a warning under the field */
  warnWhen?: string;
};

export const PROPERTY_SECTIONS: PropertySection[] = ['game', 'players', 'world', 'display', 'network', 'advanced'];

const bool = (key: string, section: PropertySection, def: 'true' | 'false', extra: Partial<PropertyField> = {}): PropertyField =>
  ({ key, section, type: 'bool', def, ...extra });

const num = (key: string, section: PropertySection, def: string, min: number, max: number, extra: Partial<PropertyField> = {}): PropertyField =>
  ({ key, section, type: 'number', def, min, max, ...extra });

export const PROPERTY_FIELDS: PropertyField[] = [
  // the match itself
  { key: 'gamemode', section: 'game', type: 'enum', def: 'survival', options: ['survival', 'creative', 'adventure', 'spectator'], core: true },
  { key: 'difficulty', section: 'game', type: 'enum', def: 'easy', options: ['peaceful', 'easy', 'normal', 'hard'], core: true },
  bool('hardcore', 'game', 'false', { core: true, warnWhen: 'true' }),
  bool('pvp', 'game', 'true', { core: true }),
  bool('force-gamemode', 'game', 'false', { core: true }),
  bool('allow-flight', 'game', 'false', { core: true }),
  bool('allow-nether', 'game', 'true', { core: true }),
  bool('enable-command-block', 'game', 'false', { core: true }),
  bool('spawn-monsters', 'game', 'true'),
  bool('spawn-animals', 'game', 'true'),
  bool('spawn-npcs', 'game', 'true'),

  // who plays
  num('max-players', 'players', '20', 1, 10000, { core: true }),
  bool('white-list', 'players', 'false', { core: true }),
  bool('enforce-whitelist', 'players', 'false', { core: true }),
  bool('online-mode', 'players', 'true', { core: true, warnWhen: 'false' }),
  num('spawn-protection', 'players', '16', 0, 10000, { core: true }),
  num('player-idle-timeout', 'players', '0', 0, 100000, { core: true }),
  { key: 'op-permission-level', section: 'players', type: 'enum', def: '4', options: ['1', '2', '3', '4'] },
  bool('enforce-secure-profile', 'players', 'true'),
  bool('hide-online-players', 'players', 'false', { core: true }),

  // the world
  { key: 'level-name', section: 'world', type: 'text', def: 'world', core: true },
  { key: 'level-seed', section: 'world', type: 'text', def: '', core: true, newWorldsOnly: true },
  {
    key: 'level-type', section: 'world', type: 'enum', def: 'minecraft:normal', core: true, newWorldsOnly: true,
    options: ['minecraft:normal', 'minecraft:flat', 'minecraft:large_biomes', 'minecraft:amplified', 'minecraft:single_biome_surface'],
  },
  bool('generate-structures', 'world', 'true', { core: true, newWorldsOnly: true }),
  num('view-distance', 'world', '10', 3, 32, { core: true }),
  num('simulation-distance', 'world', '10', 3, 32, { core: true }),
  num('max-world-size', 'world', '29999984', 1, 29999984),
  { key: 'generator-settings', section: 'world', type: 'text', def: '{}', newWorldsOnly: true },

  // how the server presents itself (the MOTD has its own card)
  bool('enable-status', 'display', 'true', { core: true }),
  { key: 'resource-pack', section: 'display', type: 'text', def: '', core: true },
  { key: 'resource-pack-sha1', section: 'display', type: 'text', def: '' },
  bool('require-resource-pack', 'display', 'false', { core: true }),
  { key: 'resource-pack-prompt', section: 'display', type: 'text', def: '', core: true },

  // network and performance
  { key: 'server-port', section: 'network', type: 'number', def: '25565', core: true, locked: true },
  { key: 'server-ip', section: 'network', type: 'text', def: '', core: true, locked: true },
  num('network-compression-threshold', 'network', '256', -1, 65535),
  num('rate-limit', 'network', '0', 0, 100000),
  num('max-tick-time', 'network', '60000', -1, 600000),
  num('entity-broadcast-range-percentage', 'network', '100', 10, 1000),
  num('pause-when-empty-seconds', 'network', '60', 0, 86400),
  { key: 'region-file-compression', section: 'network', type: 'enum', def: 'deflate', options: ['deflate', 'lz4', 'none'] },
  bool('sync-chunk-writes', 'network', 'true'),
  bool('use-native-transport', 'network', 'true'),
  bool('prevent-proxy-connections', 'network', 'false'),
  bool('accepts-transfers', 'network', 'false'),

  // remote access and the rest
  bool('enable-rcon', 'advanced', 'false'),
  num('rcon.port', 'advanced', '25575', 1, 65535),
  { key: 'rcon.password', section: 'advanced', type: 'secret', def: '' },
  bool('enable-query', 'advanced', 'false'),
  num('query.port', 'advanced', '25565', 1, 65535),
  bool('broadcast-console-to-ops', 'advanced', 'true'),
  bool('broadcast-rcon-to-ops', 'advanced', 'true'),
  { key: 'function-permission-level', section: 'advanced', type: 'enum', def: '2', options: ['1', '2', '3', '4'] },
  bool('log-ips', 'advanced', 'true'),
  bool('enable-jmx-monitoring', 'advanced', 'false'),
  { key: 'bug-report-link', section: 'advanced', type: 'text', def: '' },
  { key: 'initial-enabled-packs', section: 'advanced', type: 'text', def: 'vanilla' },
  { key: 'initial-disabled-packs', section: 'advanced', type: 'text', def: '' },
  { key: 'text-filtering-config', section: 'advanced', type: 'text', def: '' },
];

/** Keys handled outside the field list (the MOTD card) */
export const SPECIAL_KEYS = new Set(['motd']);

export const KNOWN_KEYS = new Set([...PROPERTY_FIELDS.map((f) => f.key), ...SPECIAL_KEYS]);

/** The locale id of a key: dots would split the translation path */
export const fieldId = (key: string) => key.replace(/\./g, '_');

/** Checks a value against the field's rules; returns a locale key for the problem, or null */
export function validateField(field: PropertyField, value: string): string | null {
  if (field.type !== 'number') return null;
  if (!/^-?\d+$/.test(value.trim())) return 'servers.properties.errors.number';
  const n = Number(value);
  if (field.min !== undefined && n < field.min) return 'servers.properties.errors.min';
  if (field.max !== undefined && n > field.max) return 'servers.properties.errors.max';
  return null;
}
