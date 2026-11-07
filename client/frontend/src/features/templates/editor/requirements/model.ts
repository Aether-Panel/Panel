// requirements.os and requirements.arch hold one value or several joined by "||"
// ("linux || windows"), as the node reads them (Requirements.Test: split on "||", trimmed). The
// form shows them as chips; a value it does not know stays a chip of its own.

/** Systems the node can run on, as Go's runtime.GOOS names them. */
export const KNOWN_OS = ['linux', 'windows', 'darwin'] as const;

/** Processor architectures, as Go's runtime.GOARCH names them. */
export const KNOWN_ARCH = ['amd64', 'arm64', '386', 'arm'] as const;

/** The values of a requirement row; empty for a missing or blank one. */
export function parseAlternatives(value: unknown): string[] {
  if (typeof value !== 'string') return [];
  return value
    .split('||')
    .map((part) => part.trim())
    .filter((part) => part !== '');
}

/** The row for `values`, as the community templates write it; undefined (no key) when empty. */
export function joinAlternatives(values: readonly string[]): string | undefined {
  return values.length === 0 ? undefined : values.join(' || ');
}

/** The row with `item` added (at the end) or, when it is there, removed. */
export function toggleAlternative(value: unknown, item: string): string | undefined {
  const values = parseAlternatives(value);
  return joinAlternatives(values.includes(item) ? values.filter((v) => v !== item) : [...values, item]);
}
