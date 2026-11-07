// Edits that a field holds back until it is left (a variable's key is written on blur, since a
// half-typed key could clash or be invalid). Whatever would lose the field or read the template
// first (saving, downloading, another view or section, folding a card) writes them through
// flush(). Kept apart from React so the rules are unit-tested.

export interface PendingCommits {
  /** Adds a held-back edit's writer; the function returned removes it. */
  register: (commit: () => void) => () => void;
  /** Writes every held-back edit, in the order they were registered. */
  flush: () => void;
  readonly size: number;
}

export function createPendingCommits(onSizeChange: (size: number) => void = () => undefined): PendingCommits {
  const commits = new Set<() => void>();
  return {
    register(commit) {
      // a wrapper of its own, so registering the same function twice needs two removals
      const entry = () => commit();
      commits.add(entry);
      onSizeChange(commits.size);
      return () => {
        if (commits.delete(entry)) onSizeChange(commits.size);
      };
    },
    flush() {
      // over a snapshot: a commit usually unregisters itself as it writes, one registered meanwhile
      // waits for the next flush, and one removed meanwhile still writes what it held
      const snapshot = [...commits];
      for (const commit of snapshot) commit();
    },
    get size() {
      return commits.size;
    },
  };
}
