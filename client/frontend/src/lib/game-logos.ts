// The game logos in public/img/icons, named as a template's "icon" field names them
const LOGOS = new Set([
    '7days2die', 'ark', 'arma3', 'csgo', 'css', 'dontstarvetogether', 'eco', 'factorio', 'gmod',
    'minecraft-bedrock', 'minecraft-java', 'rust', 'starbound', 'terraria',
]);

/**
 * The logo of the game a template runs, or undefined when there is none (bots, databases, games
 * without a logo). The template's icon field decides; older templates without one are matched by
 * their name.
 */
export function gameLogo(template?: { name?: string; icon?: string | null }): string | undefined {
    const icon = template?.icon?.toLowerCase();
    if (icon) return LOGOS.has(icon) ? `/img/icons/${icon}.png` : undefined;

    const name = template?.name?.toLowerCase();
    if (!name) return undefined;
    if (name === 'pocketmine' || name.startsWith('minecraft-bedrock')) return '/img/icons/minecraft-bedrock.png';
    if (name === 'minecraft' || name.startsWith('minecraft-')) return '/img/icons/minecraft-java.png';
    const logo = [...LOGOS].find((game) => name === game || name.startsWith(`${game}-`));
    return logo ? `/img/icons/${logo}.png` : undefined;
}
