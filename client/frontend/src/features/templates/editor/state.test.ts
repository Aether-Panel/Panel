import { readdirSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { createDocument, isDirty, markSaved, removeAt, setJsonText, updateAt } from './document';
import { copyName, duplicateTemplate, editorSource, importTemplate, prepareLoadedTemplate } from './initial';
import { deleteIn, getIn, moveIn, renameKeyIn, setIn, type JsonObject } from './paths';
import { sameTemplateName, storedTemplateName, templateNameProblem } from './template-name';
import { countIssues, isUnderPath, validateTemplate, type EditorIssue } from './validation';
import { defaultEnvironment, integerVariableValue, templateWarnings } from './warnings';

const FIXTURES = new URL('../schema/__fixtures__/community/', import.meta.url);

/** "path: key [severity, blocking]" for readable assertions. */
function describeIssues(issues: EditorIssue[]): string[] {
  return issues.map((i) => `${i.path}: ${i.messageKey} [${i.severity}${i.blocking ? ', blocking' : ''}]`);
}

describe('setIn', () => {
  const template = { name: 'a', run: { command: 'x', stop: 'y' }, install: [{ type: 'mkdir', target: 'a' }], data: {} };

  it('keeps the order of existing keys', () => {
    const next = setIn(template, ['run', 'command'], 'z');
    expect(Object.keys(next.run)).toEqual(['command', 'stop']);
    expect(Object.keys(next)).toEqual(['name', 'run', 'install', 'data']);
    expect(JSON.stringify(next)).toBe(JSON.stringify({ ...template, run: { command: 'z', stop: 'y' } }));
  });

  it('adds a new key last', () => {
    expect(Object.keys(setIn(template, ['run', 'workingDirectory'], 'srv').run)).toEqual(['command', 'stop', 'workingDirectory']);
  });

  it('copies only the branch it changes and never mutates', () => {
    const before = JSON.stringify(template);
    const next = setIn(template, ['install', 0, 'target'], 'b');
    expect(JSON.stringify(template)).toBe(before);
    expect(next).not.toBe(template);
    expect(next.install).not.toBe(template.install);
    expect(next.run).toBe(template.run);
    expect(next.data).toBe(template.data);
    expect(next.install[0]).toEqual({ type: 'mkdir', target: 'b' });
  });

  it('returns the same object when the value is already there', () => {
    expect(setIn(template, ['run', 'command'], 'x')).toBe(template);
    expect(setIn(template, ['install', 0, 'type'], 'mkdir')).toBe(template);
  });

  it('builds missing parents and replaces a null one', () => {
    expect(setIn({}, ['run', 'pre', 0, 'type'], 'mkdir')).toEqual({ run: { pre: [{ type: 'mkdir' }] } });
    expect(setIn({ run: null, stats: { type: '' } }, ['run', 'stop'], 'stop')).toEqual({ run: { stop: 'stop' }, stats: { type: '' } });
  });

  it('appends to a list at its length', () => {
    expect(setIn(template, ['install', 1], { type: 'fabricdl' }).install).toHaveLength(2);
  });

  it('refuses list indexes that are negative, fractional or past the end', () => {
    expect(setIn(template, ['install', 2], { type: 'mkdir' })).toBe(template);
    expect(setIn(template, ['install', -1], { type: 'mkdir' })).toBe(template);
    expect(setIn(template, ['install', 0.5], { type: 'mkdir' })).toBe(template);
    expect(setIn({}, ['run', 'pre', 3, 'type'], 'mkdir')).toEqual({});
    expect(JSON.stringify(setIn(template, ['install', 1, 'target'], 'x'))).not.toContain('null');
  });

  it('reads a number on an object as a key, never turning the object into a list', () => {
    const next = setIn(template, ['data', 0], { type: 'string' });
    expect(next.data).toEqual({ 0: { type: 'string' } });
    expect(Array.isArray(next.data)).toBe(false);
    expect(Object.keys(setIn({ data: { port: {} } }, ['data', 0], 1).data)).toEqual(['0', 'port']);
  });
});

describe('deleteIn', () => {
  const template = { a: 1, b: { c: 2, d: 3 }, list: ['x', 'y', 'z'] };

  it('removes a key and keeps the order of the others', () => {
    const next = deleteIn(template, ['b', 'c']);
    expect(next).toEqual({ a: 1, b: { d: 3 }, list: ['x', 'y', 'z'] });
    expect(Object.keys(deleteIn(template, ['a']))).toEqual(['b', 'list']);
    expect(next.list).toBe(template.list);
  });

  it('removes a list item and moves the next ones up', () => {
    expect(deleteIn(template, ['list', 1]).list).toEqual(['x', 'z']);
  });

  it('leaves the object alone when the path leads nowhere', () => {
    expect(deleteIn(template, ['missing'])).toBe(template);
    expect(deleteIn(template, ['b', 'missing', 'deeper'])).toBe(template);
    expect(deleteIn(template, ['list', 7])).toBe(template);
  });
});

describe('moveIn and renameKeyIn', () => {
  it('reorders a list', () => {
    expect(moveIn({ l: ['a', 'b', 'c'] }, ['l'], 0, 2)).toEqual({ l: ['b', 'c', 'a'] });
    const same = { l: ['a'] };
    expect(moveIn(same, ['l'], 0, 3)).toBe(same);
  });

  it('reorders the keys of an object, values untouched', () => {
    const next = moveIn({ data: { a: 1, b: 2, c: 3 } }, ['data'], 2, 0);
    expect(Object.keys(next.data)).toEqual(['c', 'a', 'b']);
    expect(next.data).toEqual({ a: 1, b: 2, c: 3 });
  });

  it('renames a key in its place', () => {
    const next = renameKeyIn({ data: { a: 1, b: 2, c: 3 } }, ['data'], 'b', 'port');
    expect(Object.keys(next.data)).toEqual(['a', 'port', 'c']);
    expect(getIn(next, ['data', 'port'])).toBe(2);
  });

  it('refuses a rename onto a key that exists', () => {
    const template = { data: { a: 1, b: 2 } };
    expect(renameKeyIn(template, ['data'], 'a', 'b')).toBe(template);
  });
});

describe('editor document', () => {
  const template = { name: 'tpl', display: 'Template', run: { command: 'go' } };

  it('rewrites the JSON text after a form edit, keeping key order', () => {
    const doc = updateAt(createDocument(template), ['display'], 'Renamed');
    expect(doc.jsonText).toBe(JSON.stringify({ name: 'tpl', display: 'Renamed', run: { command: 'go' } }, null, 2));
    expect(isDirty(doc)).toBe(true);
  });

  it('removes a key when the form writes undefined', () => {
    expect(updateAt(createDocument(template), ['display'], undefined).template).toEqual({ name: 'tpl', run: { command: 'go' } });
    expect(removeAt(createDocument(template), ['run', 'command']).template).toEqual({ name: 'tpl', display: 'Template', run: {} });
  });

  it('keeps the typed JSON text and follows it while it parses', () => {
    const text = '{"name":"tpl",   "display":"Other"}';
    const doc = setJsonText(createDocument(template), text);
    expect(doc.jsonText).toBe(text);
    expect(doc.template).toEqual({ name: 'tpl', display: 'Other' });
    expect(doc.jsonProblem).toBeNull();
  });

  it('keeps the last valid template while the JSON is broken', () => {
    const start = createDocument(template);
    const broken = setJsonText(start, '{"name": "tpl",');
    expect(broken.template).toBe(start.template);
    expect(broken.jsonProblem?.code).toBe('syntax');
    expect(isDirty(broken)).toBe(true);
    expect(setJsonText(start, '[1, 2]').jsonProblem).toEqual({ code: 'notObject' });
  });

  it('drops the broken text when the form is edited meanwhile', () => {
    const doc = updateAt(setJsonText(createDocument(template), '{'), ['display'], 'X');
    expect(doc.jsonProblem).toBeNull();
    expect(JSON.parse(doc.jsonText)).toEqual({ ...template, display: 'X' });
  });

  it('counts a copy or an import as unsaved until it is saved', () => {
    const doc = createDocument(template, false);
    expect(isDirty(doc)).toBe(true);
    expect(isDirty(markSaved(doc, doc.template))).toBe(false);
  });

  it('is clean when loaded, after a save, and after an edit is undone', () => {
    const doc = createDocument(template);
    expect(isDirty(doc)).toBe(false);
    const edited = updateAt(doc, ['display'], 'X');
    expect(isDirty(markSaved(edited, edited.template))).toBe(false);
    // an edit made while the save was on its way stays unsaved
    expect(isDirty(markSaved(updateAt(edited, ['display'], 'Y'), edited.template))).toBe(true);
    expect(isDirty(updateAt(edited, ['display'], 'Template'))).toBe(false);
  });
});

describe('template name', () => {
  it.each([
    ['', 'nameRequired'],
    [undefined, 'nameRequired'],
    ['a'.repeat(101), 'nameTooLong'],
    ['games/minecraft', 'nameSlash'],
    ['..', 'nameDots'],
    [' paper', 'nameWhitespace'],
    ['paper\n', 'nameWhitespace'],
    ['pa\u0007per', 'nameControl'],
  ])('refuses %j', (name, problem) => {
    expect(templateNameProblem(name)).toBe(problem);
  });

  it('accepts what the panel stores and serves', () => {
    for (const name of ['paper', 'minecraft-java', 'My Server', 'eco9', 'a'.repeat(100), 'v1.2']) {
      expect(templateNameProblem(name)).toBeUndefined();
    }
  });

  it('refuses a local name that exists, unless it is the one being edited', () => {
    expect(templateNameProblem('paper', { takenNames: ['paper'] })).toBe('nameTaken');
    expect(templateNameProblem('paper', { takenNames: ['paper'], originalName: 'paper' })).toBeUndefined();
  });

  it('compares names as the database does, ignoring case and accents', () => {
    // MariaDB's default collation finds the row "paper" for "Paper" or "pàper" and replaces it
    for (const name of ['Paper', 'PAPER', 'pàper', 'Pápér']) {
      expect(templateNameProblem(name, { takenNames: ['paper'] }), name).toBe('nameTaken');
      expect(sameTemplateName(name, 'paper'), name).toBe(true);
    }
    expect(sameTemplateName('paper-1', 'paper')).toBe(false);
  });

  it('compares the same whatever the browser language (ñ = n, ä = a, I = i, trailing spaces)', () => {
    // a fixed collation: under es, sv or tr the default one would keep these apart, the database does not
    for (const [a, b] of [['españa', 'espana'], ['päper', 'paper'], ['PIPE', 'pipe'], ['Ärger', 'arger'], ['paper ', 'paper']]) {
      expect(sameTemplateName(a, b), `${a} / ${b}`).toBe(true);
      expect(templateNameProblem(a, { takenNames: [b] }), a).toBe(a.trimEnd() === a ? 'nameTaken' : 'nameWhitespace');
    }
    expect(copyName('espana', ['españa'])).toBe('espana-copy');
  });

  it('keeps the stored name when only case or accents change', () => {
    expect(storedTemplateName('Paper', 'paper')).toBe('paper');
    expect(storedTemplateName('pàper', 'paper')).toBe('paper');
    expect(storedTemplateName('velocity', 'paper')).toBe('velocity');
    expect(storedTemplateName('paper', undefined)).toBe('paper');
  });

  it('counts a case or accent change of the opened name as the same template', () => {
    expect(templateNameProblem('Paper', { takenNames: ['paper', 'velocity'], originalName: 'paper' })).toBeUndefined();
    expect(templateNameProblem('Velocity', { takenNames: ['paper', 'velocity'], originalName: 'paper' })).toBe('nameTaken');
  });
});

/** Environments that raise no warning, for tests about something else. */
const ENVIRONMENTS = { environment: { type: 'host' }, supportedEnvironments: [{ type: 'docker', image: 'ubuntu' }] };

describe('validation', () => {
  it('blocks saving on a bad name or a step that is not an object, nothing else', () => {
    const issues = validateTemplate({
      name: '',
      install: [null, { type: 'mkdir' }, { type: 'futureop' }],
      run: { stopCode: 'x' },
    });
    expect(describeIssues(issues)).toEqual([
      'name: templates.editor.validation.nameRequired [error, blocking]',
      'install.0: templates.editor.validation.stepNotObject [error, blocking]',
      'install.1.target: templates.editor.validation.required [error]',
      'install.2.type: templates.editor.validation.unknownOperation [warning]',
      'run.stopCode: templates.editor.validation.invalidInteger [error]',
      'environment.type: templates.editor.warnings.noDefaultEnvironment [warning]',
      'supportedEnvironments: templates.editor.warnings.noDockerEnvironment [warning]',
    ]);
    expect(countIssues(issues)).toEqual({ errors: 4, warnings: 3 });
  });

  it('blocks saving while the local names are unknown', () => {
    const env = ENVIRONMENTS;
    const unknown = validateTemplate({ name: 'paper', ...env }, { takenNames: null });
    expect(describeIssues(unknown)).toEqual(['name: templates.editor.validation.namesUnchecked [error, blocking]']);
    expect(validateTemplate({ name: 'paper', ...env }, { takenNames: [] })).toEqual([]);
  });

  it('lets the opened template save under its own name while the local names are unknown', () => {
    const env = ENVIRONMENTS;
    expect(validateTemplate({ name: 'paper', ...env }, { takenNames: null, originalName: 'paper' })).toEqual([]);
    expect(describeIssues(validateTemplate({ name: 'velocity', ...env }, { takenNames: null, originalName: 'paper' }))).toEqual([
      'name: templates.editor.validation.namesUnchecked [error, blocking]',
    ]);
  });

  it('warns that a change of case or accents alone is not saved', () => {
    const issues = validateTemplate({ name: 'Paper', ...ENVIRONMENTS }, { takenNames: ['paper'], originalName: 'paper' });
    expect(describeIssues(issues)).toEqual(['name: templates.editor.warnings.nameCaseOnly [warning]']);
    expect(issues[0].params).toEqual({ name: 'paper' });
  });

  it('finds no errors in any community template', () => {
    const files = readdirSync(FIXTURES).filter((f) => f.endsWith('.json'));
    expect(files.length).toBeGreaterThan(0);
    for (const file of files) {
      const template = prepareLoadedTemplate(JSON.parse(readFileSync(new URL(file, FIXTURES), 'utf8')) as JsonObject, file.replace(/\.json$/, ''));
      const errors = validateTemplate(template).filter((i) => i.severity === 'error');
      expect(describeIssues(errors), file).toEqual([]);
    }
  });

  it('tells which paths lie under a section', () => {
    expect(isUnderPath('run.pre.0.type', 'run')).toBe(true);
    expect(isUnderPath('run', 'run')).toBe(true);
    expect(isUnderPath('runner', 'run')).toBe(false);
  });
});

describe('warnings', () => {
  it('flag an integer variable the node would turn into 0 or cut down', () => {
    const data = {
      ok1: { type: 'integer', value: 25565 },
      ok2: { type: 'integer', value: '-3' },
      ok3: { type: 'integer', value: '' },
      ok4: { type: 'integer' },
      text: { type: 'integer', value: 'abc' },
      decimal: { type: 'integer', value: 1.5 },
      bool: { type: 'integer', value: true },
      plain: { type: 'string', value: 'abc' },
    };
    expect(integerVariableValue({ data }).map((w) => w.path.join('.'))).toEqual(['data.text.value', 'data.decimal.value', 'data.bool.value']);
  });

  it('flag a template without a default environment', () => {
    expect(defaultEnvironment({ environment: { type: 'docker', image: 'x' } })).toEqual([]);
    for (const environment of [undefined, null, {}, { type: '' }]) {
      expect(defaultEnvironment({ environment })).toHaveLength(1);
    }
  });
});

describe('starting points', () => {
  it('puts the name first and drops the README of a loaded template', () => {
    const loaded = prepareLoadedTemplate({ type: 'game', display: 'Paper', readme: '# Paper', name: 'paper' }, 'paper');
    expect(Object.keys(loaded)).toEqual(['name', 'type', 'display']);
  });

  it('names a copy so it overwrites nothing', () => {
    expect(copyName('paper', [])).toBe('paper');
    expect(copyName('paper', ['paper'])).toBe('paper-copy');
    expect(copyName('paper', ['paper', 'paper-copy'])).toBe('paper-copy-2');
    expect(copyName('a'.repeat(100), ['a'.repeat(100)])).toHaveLength(100);
    // taken the way the database compares names
    expect(copyName('paper', ['Paper'])).toBe('paper-copy');
    expect(copyName('paper', ['PÀPER', 'Paper-Copy'])).toBe('paper-copy-2');
  });

  it('marks a renamed copy in its display name only', () => {
    expect(duplicateTemplate({ name: 'paper', display: 'Paper' }, 'paper', ['paper'], 'copia')).toEqual({ name: 'paper-copy', display: 'Paper (copia)' });
    expect(duplicateTemplate({ name: 'paper', display: 'Paper' }, 'paper', [], 'copia')).toEqual({ name: 'paper', display: 'Paper' });
  });

  it('imports a .json file, naming it after the file when it has no name', () => {
    expect(importTemplate('{"display":"X"}', 'minecraft.JSON')).toEqual({ template: { name: 'minecraft', display: 'X' } });
    expect(importTemplate('{"name":"keep"}', 'other.json')).toEqual({ template: { name: 'keep' } });
    expect(importTemplate('{', 'x.json')).toMatchObject({ problem: { code: 'syntax' } });
    expect(importTemplate('"text"', 'x.json')).toEqual({ problem: { code: 'notObject' } });
  });
});

describe('editor address', () => {
  it.each([
    ['', { kind: 'blank' }],
    ['?name=paper', { kind: 'local', name: 'paper' }],
    ['?name=my%20server', { kind: 'local', name: 'my server' }],
    ['?from=3:minecraft', { kind: 'copy', repoId: 3, name: 'minecraft' }],
    ['?from=0:a:b', { kind: 'copy', repoId: 0, name: 'a:b' }],
    ['?from=x:minecraft', { kind: 'blank' }],
    ['?from=3:', { kind: 'blank' }],
    ['?import', { kind: 'import' }],
  ])('%s opens %j', (search, source) => {
    expect(editorSource(search)).toEqual(source);
  });
});

describe('warnings on the community templates', () => {
  // the only warnings a community template may raise, each a real finding (see the report)
  const EXPECTED: Record<string, readonly string[]> = {
    // variables nothing uses: cstrike tvport, fivem fxbuild, terraria-tmodloader tversion,
    // terraria-tshock tshockversion
    'cstrike.json': ['unusedVariable'],
    'fivem.json': ['unusedVariable'],
    'terraria-tmodloader.json': ['unusedVariable'],
    'terraria-tshock.json': ['unusedVariable'],
    // no supported environments: the create form falls back to the generic image
    'dontstarvetogether.json': ['noDockerEnvironment'],
    'eco.json': ['noDockerEnvironment'],
  };

  it('raise only the expected ones', () => {
    const files = readdirSync(FIXTURES).filter((f) => f.endsWith('.json'));
    for (const file of files) {
      const template = JSON.parse(readFileSync(new URL(file, FIXTURES), 'utf8')) as JsonObject;
      const codes = templateWarnings(template).map((w) => w.code).sort();
      expect(codes, file).toEqual([...(EXPECTED[file] ?? [])].sort());
    }
  });
});
