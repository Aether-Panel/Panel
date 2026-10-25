/** What a game told the panel about who is playing, whatever protocol it speaks. */
export type PlayerInfo = {
  online: number;
  max: number;
  /** Names the game shares: Minecraft sends a sample, some Steam games send none. */
  names: string[];
  /** Steam servers also report the map and the game's own name. */
  map?: string;
  game?: string;
  version?: string;
};

type MinecraftQuery = { numPlayers: number; maxPlayers: number; version?: string; players?: string[] | null };
type SourceQuery = { numPlayers: number; maxPlayers: number; bots?: number; map?: string; game?: string; players?: string[] | null };

/** Reads `/api/servers/:id/query`, which answers `{ minecraft }` or `{ source }`. */
export function parsePlayerQuery(data: unknown): PlayerInfo | null {
  if (!data || typeof data !== 'object') return null;
  const { minecraft, source } = data as { minecraft?: MinecraftQuery; source?: SourceQuery };
  if (minecraft) {
    return {
      online: minecraft.numPlayers ?? 0,
      max: minecraft.maxPlayers ?? 0,
      names: minecraft.players ?? [],
      version: minecraft.version || undefined,
    };
  }
  if (source) {
    return {
      // bots fill seats but are not people
      online: Math.max(0, (source.numPlayers ?? 0) - (source.bots ?? 0)),
      max: source.maxPlayers ?? 0,
      names: source.players ?? [],
      map: source.map || undefined,
      game: source.game || undefined,
    };
  }
  return null;
}
