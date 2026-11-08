/**
 * The logos in public/img/icons, keyed as a template's "icon" field names them. Games use their
 * square app icon (PNG); languages, databases and services use their brand mark (SVG). `plate`
 * marks a logo drawn on a transparent background that needs a light tile behind it to stay
 * visible on the dark theme (a black or dark-blue mark would disappear otherwise).
 */
const LOGOS: Record<string, { file: string; plate?: boolean }> = {
    // languages and runtimes
    nodejs: { file: 'nodejs.svg', plate: true },
    python: { file: 'python.svg', plate: true },
    java: { file: 'java.svg', plate: true },
    bun: { file: 'bun.svg', plate: true },
    go: { file: 'go.svg', plate: true },
    php: { file: 'php.svg', plate: true },
    laravel: { file: 'laravel.svg', plate: true },
    dotnet: { file: 'dotnet.svg' },
    // databases and services
    mariadb: { file: 'mariadb.svg', plate: true },
    mongodb: { file: 'mongodb.svg', plate: true },
    postgresql: { file: 'postgresql.svg', plate: true },
    redis: { file: 'redis.svg', plate: true },
    lavalink: { file: 'lavalink.svg' },
    n8n: { file: 'n8n.svg' },
    teamspeak3: { file: 'teamspeak3.svg', plate: true },
    discord: { file: 'discord.svg', plate: true },
    // games
    '7days2die': { file: '7days2die.png' },
    ark: { file: 'ark.png' },
    arma3: { file: 'arma3.png' },
    armareforger: { file: 'armareforger.png' },
    barotrauma: { file: 'barotrauma.png' },
    corekeeper: { file: 'corekeeper.png' },
    csgo: { file: 'csgo.png' },
    css: { file: 'css.png' },
    cstrike: { file: 'cstrike.svg', plate: true },
    dontstarvetogether: { file: 'dontstarvetogether.png' },
    eco: { file: 'eco.png' },
    factorio: { file: 'factorio.png' },
    fivem: { file: 'fivem.svg', plate: true },
    gmod: { file: 'gmod.png' },
    hytale: { file: 'hytale.png' },
    insurgencysandstorm: { file: 'insurgencysandstorm.png' },
    killingfloor2: { file: 'killingfloor2.png' },
    left4dead2: { file: 'left4dead2.png' },
    'minecraft-java': { file: 'minecraft-java.png' },
    'minecraft-bedrock': { file: 'minecraft-bedrock.png' },
    mordhau: { file: 'mordhau.png' },
    necesse: { file: 'necesse.png' },
    palworld: { file: 'palworld.png' },
    rust: { file: 'rust.png' },
    satisfactory: { file: 'satisfactory.png' },
    squad: { file: 'squad.png' },
    starbound: { file: 'starbound.png' },
    stn: { file: 'stn.png' },
    terraria: { file: 'terraria.png' },
    tf2: { file: 'tf2.png' },
    unturned: { file: 'unturned.png' },
    valheim: { file: 'valheim.png' },
    vintagestory: { file: 'vintagestory.png' },
    zomboid: { file: 'zomboid.png' },
};

/** Names that do not start with their logo's key (older templates without an icon field). */
const ALIASES: Record<string, string> = {
    minecraft: 'minecraft-java',
    pocketmine: 'minecraft-bedrock',
    'vintage-story': 'vintagestory',
    eco9: 'eco',
    teamspeak: 'teamspeak3',
    ts3: 'teamspeak3',
    node: 'nodejs',
    projectzomboid: 'zomboid',
    garrysmod: 'gmod',
};

/** Every value a template's "icon" field can take to show one of these logos, for the icon picker. */
export const TEMPLATE_LOGO_KEYS: readonly string[] = Object.keys(LOGOS);

export type TemplateLogo = { src: string; plate: boolean };

function logoFor(key: string): TemplateLogo {
    const { file, plate } = LOGOS[key];
    return { src: `/img/icons/${file}`, plate: !!plate };
}

/** The logo key a name points to: itself, an alias, or a key it starts with (terraria-tshock). */
function keyFromName(raw: string | null | undefined): string | undefined {
    const name = raw?.trim().toLowerCase();
    if (!name) return undefined;
    if (Object.hasOwn(LOGOS, name)) return name;
    if (Object.hasOwn(ALIASES, name)) return ALIASES[name];
    if (name.startsWith('minecraft-bedrock')) return 'minecraft-bedrock';
    if (name.startsWith('minecraft-')) return 'minecraft-java';
    // terraria-tshock, discord-js, discord-py…; an alias works the same way (teamspeak-…)
    const key = Object.keys(LOGOS).find((k) => name.startsWith(`${k}-`));
    if (key) return key;
    const alias = Object.keys(ALIASES).find((a) => name.startsWith(`${a}-`));
    return alias ? ALIASES[alias] : undefined;
}

/**
 * The logo of what a template (or a server made from one) runs, or undefined when there is none.
 * The icon field decides; templates without one, or with an icon we have no logo for, are
 * matched by their name.
 */
export function templateLogo(template?: { name?: string | null; icon?: string | null }): TemplateLogo | undefined {
    const key = keyFromName(template?.icon) ?? keyFromName(template?.name);
    return key ? logoFor(key) : undefined;
}
