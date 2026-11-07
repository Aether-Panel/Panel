import { pathKey, type Path } from './paths';
import type { EditorIssue } from './validation';

// Text a field holds but cannot apply: a JSON block whose text does not parse keeps the last
// valid object in the template, so saving would store that object and drop what was typed. While
// it does, the field registers a blocking issue at its path, and saving waits until the text is
// fixed, emptied or replaced from elsewhere. Kept apart from React so the rules are unit-tested.

export interface HeldIssues {
  /** Adds the issue; the function returned removes it (only once). */
  register: (issue: EditorIssue) => () => void;
  readonly list: readonly EditorIssue[];
}

export function createHeldIssues(onChange: (list: readonly EditorIssue[]) => void = () => undefined): HeldIssues {
  let list: readonly EditorIssue[] = [];
  return {
    register(issue) {
      // a wrapper of its own, so the same issue registered twice needs two removals
      const entry = { ...issue };
      list = [...list, entry];
      onChange(list);
      return () => {
        if (!list.includes(entry)) return;
        list = list.filter((i) => i !== entry);
        onChange(list);
      };
    },
    get list() {
      return list;
    },
  };
}

/** The blocking issue of a JSON text at `path` that does not parse to an object. */
export function unappliedJsonIssue(path: Path, messageKey: string): EditorIssue {
  return { path: pathKey(path), messageKey, severity: 'error', blocking: true };
}
