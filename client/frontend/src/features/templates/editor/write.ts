import { deleteIn, getIn, isPlainObject, setIn, type JsonObject, type Path } from './paths';
import { isEmptyValue } from './steps/model';

// The write rule of the run, environment, requirements and advanced forms: an optional value
// that is cleared loses its key, as if it had never been written (the node reads a missing key
// and an empty one alike), and every other key stays where it was.

/**
 * `template` with `value` at `path`; an empty value ("", [], {}, undefined) removes the key.
 * With `prune`, an object left without keys by that removal goes too (`requirements: {}` after
 * its last value is cleared), so clearing a form leaves the template as it was before it.
 */
export function writeOptional(template: JsonObject, path: Path, value: unknown, prune = false): JsonObject {
  if (!isEmptyValue(value)) return setIn(template, path, value);
  const next = deleteIn(template, path);
  const parentPath = path.slice(0, -1);
  // nothing removed: an object that was already empty stays as written
  if (!prune || parentPath.length === 0 || next === template) return next;
  const parent = getIn(next, parentPath);
  return isPlainObject(parent) && Object.keys(parent).length === 0 ? deleteIn(next, parentPath) : next;
}
