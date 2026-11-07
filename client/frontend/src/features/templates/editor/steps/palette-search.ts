// How "Añadir paso" filters the operations: every word typed must appear in the operation's
// name, key, help or category, ignoring case and accents ("direccion" finds "dirección"). A
// substring match, not a fuzzy one: with one-line help texts a fuzzy match finds nearly anything.

/** The texts an operation is searched by, already translated. */
export interface OperationSearchText {
  key: string;
  label: string;
  help: string;
  category: string;
}

/** Lowercase without accents, so "Dirección" and "direccion" compare equal. */
export function foldText(text: string): string {
  return text.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase();
}

/**
 * How well an operation matches the search, for cmdk's filter: 0 hides it, more ranks it higher.
 * Every word must be found somewhere; words found in the name count most, then the key, then the
 * category and the help. An empty search matches everything equally.
 */
export function matchOperation(op: OperationSearchText, search: string): number {
  const words = foldText(search).split(/\s+/).filter(Boolean);
  if (words.length === 0) return 1;
  const label = foldText(op.label);
  const key = op.key.toLowerCase();
  const rest = foldText(`${op.category} ${op.help}`);
  let score = 0;
  for (const word of words) {
    if (label.startsWith(word) || label.includes(` ${word}`)) score += 4;
    else if (label.includes(word)) score += 3;
    else if (key.includes(word)) score += 2;
    else if (rest.includes(word)) score += 1;
    else return 0;
  }
  // cmdk ranks by this number; keep it within (0, 1]
  return score / (words.length * 4);
}
