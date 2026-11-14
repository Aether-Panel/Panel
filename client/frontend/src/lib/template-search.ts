/** Folds case and accents, so "Pokémon" and "pokemon" match. */
function fold(text: string): string {
  return text.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
}

/**
 * Other words people type for a template, by its type, icon or name: someone looking for C# or
 * Golang finds the template even though neither word is in it.
 */
const ALIASES: Record<string, string[]> = {
  dotnet: ['.net', 'net', 'c#', 'csharp', 'asp.net', 'aspnet', 'f#'],
  go: ['golang'],
  nodejs: ['node', 'js', 'javascript', 'typescript', 'ts', 'npm'],
  bun: ['javascript', 'typescript', 'js', 'ts'],
  python: ['py', 'django', 'flask', 'fastapi'],
  php: ['symfony', 'codeigniter', 'slim', 'wordpress', 'composer'],
  laravel: ['php', 'artisan'],
  java: ['jar', 'spring', 'springboot', 'maven', 'gradle', 'jvm'],
  discord: ['bot'],
  postgresql: ['postgres', 'sql', 'database', 'base de datos'],
  mariadb: ['mysql', 'sql', 'database', 'base de datos'],
  mongodb: ['mongo', 'nosql', 'database', 'base de datos'],
  redis: ['cache', 'database', 'base de datos'],
  n8n: ['automation', 'automatizacion', 'workflow', 'workflows', 'flujos', 'zapier', 'make', 'webhook'],
  'minecraft-java': ['mc', 'paper', 'spigot', 'forge', 'fabric', 'neoforge'],
  'minecraft-bedrock': ['mc', 'bedrock', 'pe'],
  csgo: ['cs2', 'counter-strike', 'counter strike'],
  zomboid: ['project zomboid'],
  teamspeak3: ['ts3', 'voice', 'voz'],
};

type Searchable = { name?: string; display?: string; type?: string; icon?: string | null };

/** The words a template can be found by: its name, display name, type, icon and their aliases. */
function haystack(template: Searchable): string {
  const keys = [template.name, template.type, template.icon ?? undefined].filter(Boolean) as string[];
  const words = [...keys, template.display ?? ''];
  for (const key of keys) {
    const aliases = ALIASES[key.toLowerCase()];
    if (aliases) words.push(...aliases);
  }
  return fold(words.join(' '));
}

/** Whether a template matches every word of the query; an empty query matches everything. */
export function matchesTemplate(template: Searchable, query: string): boolean {
  const words = fold(query).split(/\s+/).filter(Boolean);
  if (words.length === 0) return true;
  const text = haystack(template);
  return words.every((word) => text.includes(word));
}
