import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import en from '@/lib/locales/en.json';
import es from '@/lib/locales/es.json';
import { ENVIRONMENT_TYPES } from '../schema/template';
import { KNOWN_ARCH, KNOWN_OS } from './requirements/model';
import { STOP_SIGNALS } from './run/stop';

// Every text the editor asks for exists in both languages: the keys written out in its sources,
// the ones its sections build from a name, and the message of every warning code.

type Tree = { [key: string]: string | Tree };

const lookup = (tree: Tree, key: string): unknown => key.split('.').reduce<unknown>((node, part) => (node as Tree | undefined)?.[part], tree);

const EDITOR = fileURLToPath(new URL('.', import.meta.url));

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.tsx?$/.test(name) && !name.endsWith('.test.ts') ? [path] : [];
  });
}

const paths = sources(EDITOR);
const files = paths.map((path) => readFileSync(path, 'utf8'));
/** Where the warning rules live (see warnings.ts). */
const ruleFiles = paths.filter((path) => /(rules|warnings|advanced[\\/]model)\.ts$/.test(path)).map((path) => readFileSync(path, 'utf8'));

/** Keys written out whole in a string literal ('templates.editor.run.stop.title'). */
const literalKeys = [...new Set(files.flatMap((text) => [...text.matchAll(/'(templates\.editor\.[A-Za-z0-9_.]+[A-Za-z0-9_])'/g)].map((m) => m[1])))];

/** Warning codes the rules return ({ code: 'thirdPartyImage' }). */
const warningCodes = [...new Set(ruleFiles.flatMap((text) => [...text.matchAll(/\bcode: '([A-Za-z]+)'/g)].map((m) => m[1])))];

/** Keys the sections build from a name. */
const builtKeys = [
  ...['autostart', 'autorecover', 'autorestart'].flatMap((flag) => ['label', 'hint'].map((k) => `templates.editor.run.behaviour.${flag}.${k}`)),
  ...STOP_SIGNALS.map((s) => `templates.editor.run.stop.signals.${s.name}`),
  ...['docker', 'host', 'unset'].map((k) => `templates.editor.environment.type.hint.${k}`),
  ...['unset', ...ENVIRONMENT_TYPES].map((k) => `templates.editor.environment.types.${k}`),
  ...['hostConfig', 'config'].flatMap((key) => ['label', 'hint'].map((k) => `templates.editor.environment.${key}.${k}`)),
  ...KNOWN_OS.map((v) => `templates.editor.requirements.os.values.${v}`),
  ...KNOWN_ARCH.map((v) => `templates.editor.requirements.arch.values.${v}`),
  ...['os', 'arch'].flatMap((key) => ['label', 'hint'].map((k) => `templates.editor.requirements.${key}.${k}`)),
  ...['stats', 'query', 'keepAlive'].map((key) => `templates.editor.advanced.${key}.title`),
  ...['unset', 'minecraft', 'source', 'other'].map((k) => `templates.editor.advanced.query.type.hint.${k}`),
];

describe('editor texts', () => {
  it('finds the keys it checks', () => {
    expect(literalKeys.length).toBeGreaterThan(100);
    expect(warningCodes).toContain('thirdPartyImage');
  });

  it.each([
    ['es', es as unknown as Tree],
    ['en', en as unknown as Tree],
  ])('exist in %s', (_, tree) => {
    const keys = [...literalKeys, ...builtKeys, ...warningCodes.map((code) => `templates.editor.warnings.${code}`)];
    const missing = keys.filter((key) => typeof lookup(tree, key) !== 'string');
    expect(missing).toEqual([]);
  });
});
