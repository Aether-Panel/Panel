import { getOperation } from '../../schema/operations';
import { isPlainObject, type JsonObject } from '../paths';
import { readField } from './model';

// The one line a folded step card shows under its name: the arguments that tell two steps of
// the same kind apart (what is downloaded, which command runs, where a file goes). It is built
// from the raw values, `${variables}` included, and is the same in every language.

/** A value as one short line: text as is (first line only), numbers as digits, lists joined. */
function line(value: unknown): string {
  if (typeof value === 'string') return value.split('\n').find((l) => l.trim() !== '')?.trim() ?? '';
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (Array.isArray(value)) return value.map(line).filter(Boolean).join(', ');
  return '';
}

/** The name a download is saved under: the last part of the URL's path. */
export function fileNameOf(url: string): string {
  const path = url.split(/[?#]/)[0].replace(/^[a-z][a-z0-9+.-]*:\/\/[^/]*/i, '');
  const last = path.split('/').findLast(Boolean);
  return last ?? url;
}

/** The host of a URL, when the text is one ("https://host/…"). */
function hostOf(url: string): string | undefined {
  return /^[a-z][a-z0-9+.-]*:\/\/([^/?#]+)/i.exec(url)?.[1];
}

/** "first +2" for a list of several values. */
function firstAndMore(values: readonly string[]): string {
  if (values.length <= 1) return values[0] ?? '';
  return `${values[0]} +${values.length - 1}`;
}

const arrow = (from: string, to: string) => (from && to ? `${from} → ${to}` : from || (to && `→ ${to}`));
/** The texts of a value: a list's lines, or the value's own line when it has one. */
function strings(value: unknown): string[] {
  if (Array.isArray(value)) return value.map(line).filter(Boolean);
  const text = line(value);
  return text ? [text] : [];
}

type Summarize = (arg: (name: string) => string, raw: (name: string) => unknown) => string;

const SUMMARIES: Record<string, Summarize> = {
  download: (_, raw) => {
    const files = strings(raw('files'));
    if (files.length === 1) {
      const host = hostOf(files[0]);
      return host ? arrow(host, fileNameOf(files[0])) : files[0];
    }
    return firstAndMore(files.map(fileNameOf));
  },
  command: (_, raw) => firstAndMore(strings(raw('commands'))),
  archive: (arg, raw) => arrow(strings(raw('source')).join(', '), arg('destination')),
  move: (arg) => arrow(arg('source'), arg('target')),
  extract: (arg) => arrow(arg('source'), arg('destination')),
  alterfile: (arg) => {
    const change = arrow(arg('search'), arg('replace'));
    return [arg('file'), change].filter(Boolean).join(': ');
  },
  githubdl: (arg) => [arg('repository'), arg('assetMatch')].filter(Boolean).join(' · '),
  mojangdl: (arg) => arrow(arg('version'), arg('target')),
  paperdl: (arg) => arrow([arg('project') || 'paper', arg('minecraftVersion'), arg('build')].filter(Boolean).join(' '), arg('target')),
  forgedl: (arg) => arrow([arg('minecraftVersion'), arg('version')].filter(Boolean).join(' '), arg('target')),
  neoforgedl: (arg) => arrow([arg('minecraftVersion'), arg('version')].filter(Boolean).join(' '), arg('target')),
  resolveforgeversion: (arg) => arrow([arg('minecraftVersion'), arg('version')].filter(Boolean).join(' '), `\${${arg('outputVariable') || 'opForgeVersion'}}`),
  resolveneoforgeversion: (arg) => arrow([arg('minecraftVersion'), arg('version')].filter(Boolean).join(' '), `\${${arg('outputVariable') || 'opNeoForgeVersion'}}`),
  spongedl: (arg) => [arg('spongeType'), arg('minecraftVersion'), arg('spongeVersion')].filter(Boolean).join(' '),
  curseforge: (arg) => [arg('projectId'), arg('fileId')].filter(Boolean).join(' / '),
  steamgamedl: (arg) => [arg('appId') && `App ${arg('appId')}`, arg('username')].filter(Boolean).join(' · '),
  javadl: (arg) => (arg('version') ? `Java ${arg('version')}` : ''),
  nodejsdl: (arg) => (arg('version') ? `Node.js ${arg('version')}` : ''),
};

/**
 * The arguments of `step` in one line, e.g. "piston-data.mojang.com → server.jar" for a download
 * or "java -jar installer.jar +2" for three commands; "" when there is nothing to show (no
 * arguments yet, an operation without any, a step that is not an object).
 */
export function stepSummary(step: unknown): string {
  if (!isPlainObject(step) || typeof step.type !== 'string') return '';
  const operation = getOperation(step.type);
  if (!operation) return '';
  const fields = new Map(operation.fields.map((f) => [f.name, f]));
  // read through the field so an older spelling (appID) shows too
  const raw = (name: string) => {
    const field = fields.get(name);
    return field ? readField(step as JsonObject, field) : step[name];
  };
  const arg = (name: string) => line(raw(name));
  const summarize = SUMMARIES[operation.key];
  if (summarize) return summarize(arg, raw);
  // the others: their required arguments, else whatever is set, in the registry's order
  const shown = operation.fields.filter((f) => !f.advanced);
  const required = shown.filter((f) => f.required).map((f) => arg(f.name)).filter(Boolean);
  return (required.length ? required : shown.map((f) => arg(f.name)).filter(Boolean)).join(' · ');
}
