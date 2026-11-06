// Immutable edits of the raw template object. The editor never edits the schema's parse output
// (zod rebuilds objects in schema key order and an invalid template has none), so these helpers
// work on plain JSON values and keep everything they do not touch as it was: untouched branches
// keep their identity and an existing key keeps its place, so the JSON view does not reshuffle.

/** One step into the object: a key of an object or an index of a list. */
export type PathSegment = string | number;
export type Path = readonly PathSegment[];

export type JsonObject = Record<string, unknown>;

export function isPlainObject(value: unknown): value is JsonObject {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** The dotted form issues are keyed by ("run.pre.2.target"), the same as issues.ts issuePath. */
export function pathKey(path: Path): string {
  return path.map(String).join('.');
}

/** The value at `path`, or undefined when any step of it is missing. */
export function getIn(root: unknown, path: Path): unknown {
  let current = root;
  for (const segment of path) {
    if (Array.isArray(current) && typeof segment === 'number') current = current[segment];
    else if (isPlainObject(current)) current = Object.hasOwn(current, segment) ? current[segment] : undefined;
    else return undefined;
  }
  return current;
}

/**
 * A copy of `root` with `value` at `path`. Missing parents are created (a list for a numeric
 * step, an object otherwise); a parent that is null or of another type is replaced, which is
 * what the form means when it writes into `"run": null`. An existing key keeps its position,
 * a new key goes last. Writing the value already there returns `root` itself.
 */
export function setIn<T>(root: T, path: Path, value: unknown): T {
  if (path.length === 0) return value as T;
  const [head, ...rest] = path;

  if (typeof head === 'number') {
    const list = Array.isArray(root) ? root : [];
    const next = setIn(list[head], rest, value);
    if (Array.isArray(root) && head < list.length && Object.is(next, list[head])) return root;
    const copy = list.slice();
    copy[head] = next;
    return copy as T;
  }

  const object: JsonObject = isPlainObject(root) ? root : {};
  // a missing child is built by the recursion: a list for a numeric step, an object otherwise
  const next = setIn(Object.hasOwn(object, head) ? object[head] : undefined, rest, value);
  if (isPlainObject(root) && Object.hasOwn(root, head) && Object.is(next, root[head])) return root;
  // spreading keeps the key order and assigning an existing key does not move it
  return { ...object, [head]: next } as T;
}

/**
 * A copy of `root` without what is at `path`: an object loses the key, a list loses the item
 * (the ones after it move up). A path that leads nowhere returns `root` itself.
 */
export function deleteIn<T>(root: T, path: Path): T {
  if (path.length === 0) return root;
  const [head, ...rest] = path;

  if (Array.isArray(root) && typeof head === 'number') {
    if (head < 0 || head >= root.length) return root;
    if (rest.length === 0) return root.filter((_, i) => i !== head) as T;
    const next = deleteIn(root[head], rest);
    if (Object.is(next, root[head])) return root;
    const copy = root.slice();
    copy[head] = next;
    return copy as T;
  }

  if (!isPlainObject(root) || !Object.hasOwn(root, head)) return root;
  if (rest.length === 0) {
    const copy = { ...root };
    delete copy[head];
    return copy as T;
  }
  const next = deleteIn(root[head], rest);
  if (Object.is(next, root[head])) return root;
  return { ...root, [head]: next } as T;
}

/**
 * Moves an entry of the list or object at `path` from position `from` to position `to`: the
 * reorder behind drag and drop and the ↑ ↓ buttons. For an object (the variables map) the key
 * order is what moves. Out-of-range positions leave `root` as it is.
 */
export function moveIn<T>(root: T, path: Path, from: number, to: number): T {
  const container = getIn(root, path);
  if (Array.isArray(container)) {
    if (from === to || from < 0 || to < 0 || from >= container.length || to >= container.length) return root;
    const copy = container.slice();
    const [item] = copy.splice(from, 1);
    copy.splice(to, 0, item);
    return setIn(root, path, copy);
  }
  if (isPlainObject(container)) {
    const entries = Object.entries(container);
    if (from === to || from < 0 || to < 0 || from >= entries.length || to >= entries.length) return root;
    const [entry] = entries.splice(from, 1);
    entries.splice(to, 0, entry);
    return setIn(root, path, Object.fromEntries(entries));
  }
  return root;
}

/**
 * Renames a key of the object at `path` in place, keeping its position and value: changing a
 * variable's key must not send it to the end of the list. Returns `root` itself when the key is
 * missing or the new key is already taken by another entry.
 */
export function renameKeyIn<T>(root: T, path: Path, from: string, to: string): T {
  const container = getIn(root, path);
  if (!isPlainObject(container) || !Object.hasOwn(container, from) || from === to || Object.hasOwn(container, to)) return root;
  const renamed = Object.fromEntries(Object.entries(container).map(([key, value]) => [key === from ? to : key, value]));
  return setIn(root, path, renamed);
}
