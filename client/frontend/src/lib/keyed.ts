/**
 * Pairs each entry of a list that has no ids of its own (lines of text, parsed fragments)
 * with a key made from its content, numbered when the same content repeats, so React keys
 * stay unique and stable without falling back to the position in the list.
 */
export function keyedByContent<T>(items: readonly T[], text: (item: T) => string = String): { key: string; item: T }[] {
    const seen = new Map<string, number>();
    return items.map((item) => {
        const base = text(item);
        const count = seen.get(base) ?? 0;
        seen.set(base, count + 1);
        return { key: `${base}␟${count}`, item };
    });
}
