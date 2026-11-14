import { describe, expect, it } from 'vitest';
import en from '@/lib/locales/en.json';
import es from '@/lib/locales/es.json';
import manifest from './operations.json';
import { manifestEntries, OPERATION_CATEGORIES, OPERATIONS, templateOperationsByCategory } from './operations';

type Tree = { [key: string]: string | Tree };

function lookup(tree: Tree, key: string): unknown {
  return key.split('.').reduce<unknown>((node, part) => (node as Tree | undefined)?.[part], tree);
}

/** Every dotted key ending in a string, to compare the two languages' shape. */
function leaves(tree: Tree, prefix = ''): string[] {
  return Object.entries(tree).flatMap(([k, v]) => (typeof v === 'string' ? [`${prefix}${k}`] : leaves(v, `${prefix}${k}.`)));
}

/** Every i18n key the registry hands to the editor. */
function registryKeys(): string[] {
  const keys = OPERATION_CATEGORIES.map((c) => `templates.editor.operationCategories.${c}`);
  for (const op of OPERATIONS) {
    keys.push(op.labelKey, op.helpKey);
    for (const f of op.fields) {
      keys.push(f.labelKey, f.helpKey);
      if (f.placeholderKey) keys.push(f.placeholderKey);
      if (f.widget.kind === 'select') {
        for (const o of f.widget.options) if (o.labelKey) keys.push(o.labelKey);
      }
    }
  }
  return keys;
}

describe('operation registry', () => {
  it('has one entry per key', () => {
    const keys = OPERATIONS.map((o) => o.key);
    expect(new Set(keys).size).toBe(keys.length);
  });

  it('matches operations.json, which the Go test compares with the node', () => {
    // when this fails, write the expected value into operations.json
    expect(manifest.operations).toEqual(manifestEntries());
  });

  it('offers every template operation in a palette group', () => {
    const offered = templateOperationsByCategory().flatMap((g) => g.operations.map((o) => o.key));
    expect(offered.sort()).toEqual(OPERATIONS.filter((o) => o.templates).map((o) => o.key).sort());
    expect(templateOperationsByCategory().map((g) => g.category)).not.toContain('server');
  });

  it('names every argument once', () => {
    for (const op of OPERATIONS) {
      const names = op.fields.flatMap((f) => [f.name, ...(f.aliases ?? [])]);
      expect(new Set(names).size, op.key).toBe(names.length);
    }
  });

  it('lets "" stand for unset only where the factory reads it so', () => {
    const fields = OPERATIONS.flatMap((o) => o.fields.filter((f) => f.emptyIsUnset).map((f) => `${o.key}.${f.name}`));
    expect(fields.sort()).toEqual(['backup.offsite', 'curseforge.fileId']);
  });

  it('refuses a step without its required arguments', () => {
    for (const op of OPERATIONS) {
      const result = op.stepSchema.safeParse({ type: op.key });
      const required = op.fields.filter((f) => f.required).map((f) => f.name);
      const flagged = result.success ? [] : result.error.issues.map((i) => String(i.path[0]));
      expect(flagged, op.key).toEqual(expect.arrayContaining(required));
    }
  });
});

describe('operation texts', () => {
  it('exist in Spanish and English', () => {
    const missing = registryKeys().flatMap((key) =>
      [['es', es], ['en', en]].filter(([, tree]) => typeof lookup(tree as Tree, key) !== 'string').map(([lang]) => `${lang}: ${key}`),
    );
    expect(missing).toEqual([]);
  });

  it('have the same keys in both languages', () => {
    const editorEs = leaves((es.templates as unknown as Tree).editor as Tree);
    const editorEn = leaves((en.templates as unknown as Tree).editor as Tree);
    expect(editorEs.sort()).toEqual(editorEn.sort());
  });

  it('describe nothing the registry lacks', () => {
    const used = new Set(registryKeys());
    const operations = leaves(((en.templates as unknown as Tree).editor as Tree).operations as Tree, 'templates.editor.operations.');
    expect(operations.filter((key) => !used.has(key))).toEqual([]);
  });
});
