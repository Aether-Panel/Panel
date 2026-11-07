import { readdirSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { getOperation, OPERATIONS, templateOperationsByCategory } from '../../schema/operations';
import { widgetSchema, type OperationField } from '../../schema/fields';
import type { JsonObject } from '../paths';
import { conditionPreset, conditionReference, PRESET_EXPRESSIONS } from './condition';
import {
  clearedValue, copyStep, fieldKey, foldAfterContentChange, insertStep, moveEntry, newStep, numberFromText, readField, removeStep, renameEntry, stepHasContent, stepsAt, stepType,
  uniqueEntryKey, withStdinKey, writeField, writeFieldIn,
} from './model';
import { OPERATION_ICONS } from './operation-icons';
import { matchOperation } from './palette-search';
import { fileNameOf, stepSummary } from './summary';

const field = (op: string, name: string): OperationField => {
  const found = getOperation(op)?.fields.find((f) => f.name === name);
  if (!found) throw new Error(`${op}.${name} is not in the registry`);
  return found;
};

const FIXTURES = new URL('../../schema/__fixtures__/community/', import.meta.url);
const community = readdirSync(FIXTURES)
  .filter((f) => f.endsWith('.json'))
  .map((f) => JSON.parse(readFileSync(new URL(f, FIXTURES), 'utf8')) as JsonObject);

describe('step summary', () => {
  it.each([
    [{ type: 'download', files: ['https://piston-data.mojang.com/v1/objects/abc/server.jar'] }, 'piston-data.mojang.com → server.jar'],
    [{ type: 'download', files: ['https://a.com/x/one.jar?raw=1', 'https://b.com/two.zip', '${extra}'] }, 'one.jar +2'],
    [{ type: 'download', files: ['${jarurl}'] }, '${jarurl}'],
    [{ type: 'command', commands: ['java -jar installer.jar', 'rm installer.jar'] }, 'java -jar installer.jar +1'],
    [{ type: 'command', commands: ['echo hi'] }, 'echo hi'],
    [{ type: 'move', source: 'out/*.jar', target: 'server.jar' }, 'out/*.jar → server.jar'],
    [{ type: 'extract', source: 'server.zip' }, 'server.zip'],
    [{ type: 'alterfile', file: 'server.properties', search: 'motd=.*', replace: 'motd=${motd}' }, 'server.properties: motd=.* → motd=${motd}'],
    [{ type: 'mojangdl', version: '${version}', target: 'server.jar' }, '${version} → server.jar'],
    [{ type: 'paperdl', minecraftVersion: '1.21', build: '${build}', target: 'server.jar' }, 'paper 1.21 ${build} → server.jar'],
    [{ type: 'resolveforgeversion', minecraftVersion: '1.20.1' }, '1.20.1 → ${opForgeVersion}'],
    [{ type: 'steamgamedl', appID: '896660' }, 'App 896660'],
    [{ type: 'javadl', version: 21 }, 'Java 21'],
    [{ type: 'writefile', target: 'eula.txt', text: 'eula=true' }, 'eula.txt'],
    [{ type: 'console', message: '\nfirst line\nsecond' }, 'first line'],
    [{ type: 'sleep', duration: '5s' }, '5s'],
  ])('%j → %s', (step, expected) => {
    expect(stepSummary(step)).toBe(expected);
  });

  it('is empty for a step without arguments, an unknown type or a non-object', () => {
    expect(stepSummary({ type: 'fabricdl' })).toBe('');
    expect(stepSummary({ type: 'download' })).toBe('');
    expect(stepSummary({ type: 'nope', a: 1 })).toBe('');
    expect(stepSummary(null)).toBe('');
    expect(stepSummary('mkdir')).toBe('');
  });

  it('reads the file name a download is saved under', () => {
    expect(fileNameOf('https://example.com/a/b/server.jar?x=1#y')).toBe('server.jar');
    expect(fileNameOf('https://example.com/')).toBe('https://example.com/');
    expect(fileNameOf('server.jar')).toBe('server.jar');
  });

  it('has something to say for every step of every community template that has arguments', () => {
    for (const template of community) {
      for (const list of [stepsAt(template, ['install']), stepsAt(template, ['uninstall']), stepsAt(template, ['run', 'pre']), stepsAt(template, ['run', 'post'])]) {
        for (const step of list) {
          if (stepHasContent(step) && Object.keys(step as JsonObject).some((k) => k !== 'type' && k !== 'if')) {
            expect(stepSummary(step), JSON.stringify(step)).not.toBe('');
          }
        }
      }
    }
  });
});

describe('condition presets', () => {
  it.each([
    [undefined, 'none'],
    [null, 'none'],
    ['', 'none'],
    ['   ', 'none'],
    ["env == 'host'", 'host'],
    ['env=="host"', 'host'],
    ["  'docker' == env ", 'docker'],
    ["env == 'docker'", 'docker'],
    ["env != 'host'", 'custom'],
    ["env == 'host' && javaversion != ''", 'custom'],
    ["env == 'tty'", 'custom'],
    ["env == 'host\"", 'custom'],
    [true, 'custom'],
  ])('%j is %s', (value, preset) => {
    expect(conditionPreset(value)).toBe(preset);
  });

  it('writes what the community templates write, and reads it back', () => {
    expect(PRESET_EXPRESSIONS).toEqual({ host: "env == 'host'", docker: "env == 'docker'" });
    expect(conditionPreset(PRESET_EXPRESSIONS.host)).toBe('host');
    expect(conditionPreset(PRESET_EXPRESSIONS.docker)).toBe('docker');
    const used = community.flatMap((t) => stepsAt(t, ['install']).map((s) => (s as JsonObject).if)).filter((v) => typeof v === 'string');
    expect(used).toContain(PRESET_EXPRESSIONS.host);
    expect(used).toContain(PRESET_EXPRESSIONS.docker);
  });

  it('names a variable the way a condition can read it', () => {
    expect(conditionReference('version')).toBe('version');
    expect(conditionReference('server-port')).toBe("'${server-port}'");
    expect(conditionReference('2fa')).toBe("'${2fa}'");
  });
});

describe('writing a field', () => {
  const target = field('mkdir', 'target');
  const replace = field('alterfile', 'replace');
  const regex = field('alterfile', 'regex');
  const fileId = field('curseforge', 'fileId');
  const appId = field('steamgamedl', 'appId');
  const files = field('download', 'files');

  it('sets the value and keeps every other key, unknown ones included', () => {
    const step = { type: 'mkdir', if: "env == 'host'", custom: { keep: true } };
    expect(writeField(step, target, 'data')).toEqual({ type: 'mkdir', if: "env == 'host'", custom: { keep: true }, target: 'data' });
  });

  it('keeps the key in place when it changes', () => {
    const next = writeField({ type: 'alterfile', replace: 'a', file: 'x' }, replace, 'b');
    expect(Object.keys(next)).toEqual(['type', 'replace', 'file']);
  });

  it('returns the same step when nothing changes', () => {
    const step = { type: 'mkdir', target: 'x' };
    expect(writeField(step, target, 'x')).toBe(step);
    const empty = { type: 'alterfile' };
    expect(writeField(empty, replace, '')).toBe(empty);
  });

  it('removes the key when an optional field is cleared, never writing ""', () => {
    expect(writeField({ type: 'alterfile', replace: 'a' }, replace, '')).toEqual({ type: 'alterfile' });
    expect(writeField({ type: 'alterfile', replace: 'a' }, replace, undefined)).toEqual({ type: 'alterfile' });
    expect(clearedValue(replace)).toBeUndefined();
  });

  it('keeps the key of a required field cleared, with its empty value, in its place', () => {
    const next = writeField({ type: 'mkdir', target: 'x', if: 'y' }, target, '');
    expect(next).toEqual({ type: 'mkdir', target: '', if: 'y' });
    expect(Object.keys(next)).toEqual(['type', 'target', 'if']);
    expect(writeField({ type: 'mkdir', target: 'x' }, target, undefined)).toEqual({ type: 'mkdir', target: '' });
    expect(clearedValue(target)).toBe('');
  });

  it('empties a required list instead of removing it, and removes an optional one', () => {
    expect(writeField({ type: 'download', files: ['a'], if: 'x' }, files, [])).toEqual({ type: 'download', files: [], if: 'x' });
    expect(clearedValue(files)).toEqual([]);
    const extraArgs = field('steamgamedl', 'extraArgs');
    expect(writeField({ type: 'steamgamedl', extraArgs: ['-beta'] }, extraArgs, [])).toEqual({ type: 'steamgamedl' });
  });

  it('a required field cleared still reports "required"', () => {
    const issues = getOperation('mkdir')!.stepSchema.safeParse({ type: 'mkdir', target: '' }).error?.issues ?? [];
    expect(issues.map((i) => i.message)).toContain('templates.editor.validation.required');
    const list = getOperation('download')!.stepSchema.safeParse({ type: 'download', files: [] }).error?.issues ?? [];
    expect(list.map((i) => i.message)).toContain('templates.editor.validation.required');
  });

  it('writes "" for a field whose factory reads "" as not given', () => {
    expect(clearedValue(fileId)).toBe('');
    expect(writeField({ type: 'curseforge', fileId: 123, java: 'java' }, fileId, '')).toEqual({ type: 'curseforge', fileId: '', java: 'java' });
  });

  it('removes a switch turned back to the node default', () => {
    expect(writeField({ type: 'alterfile' }, regex, true)).toEqual({ type: 'alterfile', regex: true });
    expect(writeField({ type: 'alterfile', regex: true }, regex, false)).toEqual({ type: 'alterfile' });
  });

  it('reads an older spelling and writes the field name in its place', () => {
    const step = { type: 'steamgamedl', appID: '1', if: 'x' };
    expect(fieldKey(step, appId)).toBe('appID');
    expect(readField(step, appId)).toBe('1');
    const next = writeField(step, appId, '2');
    expect(next).toEqual({ type: 'steamgamedl', appId: '2', if: 'x' });
    expect(Object.keys(next)).toEqual(['type', 'appId', 'if']);
  });

  it('prefers the field name over an older spelling and drops the older one on edit', () => {
    const step = { type: 'steamgamedl', appID: 'old', appId: 'new' };
    expect(readField(step, appId)).toBe('new');
    expect(writeField(step, appId, 'newer')).toEqual({ type: 'steamgamedl', appId: 'newer' });
  });

  it('clearing a field under an older spelling writes the field name in its place', () => {
    expect(writeField({ type: 'curseforge', fileID: '5' }, fileId, '')).toEqual({ type: 'curseforge', fileId: '' });
    expect(writeField({ type: 'steamgamedl', appID: '5' }, appId, '')).toEqual({ type: 'steamgamedl', appId: '' });
    expect(writeField({ type: 'steamgamedl', username: 'u', password: 'p' }, field('steamgamedl', 'username'), '')).toEqual({ type: 'steamgamedl', password: 'p' });
  });

  it('writes into the template at the step path, leaving non-objects alone', () => {
    const template = { install: [{ type: 'mkdir' }, null], other: { a: 1 } };
    const next = writeFieldIn(template, ['install', 0], target, 'x');
    expect(next).toEqual({ install: [{ type: 'mkdir', target: 'x' }, null], other: { a: 1 } });
    expect(next.other).toBe(template.other);
    expect(writeFieldIn(template, ['install', 1], target, 'x')).toBe(template);
  });
});

describe('number fields', () => {
  it.each([
    ['', undefined],
    ['25', 25],
    ['-3', -3],
    ['${port}', '${port}'],
    // half typed or mistyped: kept as typed so its error shows, and a number again once fixed
    ['-', '-'],
    ['12a', '12a'],
    ['99999999999999999999', '99999999999999999999'],
  ])('%j → %j', (text, expected) => {
    expect(numberFromText(text)).toBe(expected);
  });

  it('turns a string typed through back into a number', () => {
    let value: unknown = 5;
    for (const text of ['', '-', '-1', '-12']) value = numberFromText(text) ?? value;
    expect(value).toBe(-12);
  });
});

describe('step lists', () => {
  it('reads a missing or null list as empty', () => {
    expect(stepsAt({}, ['install'])).toEqual([]);
    expect(stepsAt({ uninstall: null }, ['uninstall'])).toEqual([]);
    expect(stepsAt({ run: { pre: [{ type: 'mkdir' }] } }, ['run', 'pre'])).toEqual([{ type: 'mkdir' }]);
  });

  it('inserts, creating the list where missing', () => {
    expect(insertStep({}, ['run', 'pre'], 0, { type: 'mkdir' })).toEqual({ run: { pre: [{ type: 'mkdir' }] } });
    expect(insertStep({ install: [{ type: 'a' }, { type: 'c' }] }, ['install'], 1, { type: 'b' })).toEqual({ install: [{ type: 'a' }, { type: 'b' }, { type: 'c' }] });
    expect(insertStep({ install: null }, ['install'], 5, { type: 'a' })).toEqual({ install: [{ type: 'a' }] });
  });

  it('removes a step, and the key with the last one', () => {
    expect(removeStep({ install: [{ type: 'a' }, { type: 'b' }] }, ['install'], 0)).toEqual({ install: [{ type: 'b' }] });
    expect(removeStep({ run: { pre: [{ type: 'a' }], post: [] } }, ['run', 'pre'], 0)).toEqual({ run: { post: [] } });
    const template = { install: [{ type: 'a' }] };
    expect(removeStep(template, ['install'], 3)).toBe(template);
  });

  it('makes a new step with only its type, and copies deeply', () => {
    for (const op of OPERATIONS) expect(newStep(op)).toEqual({ type: op.key });
    const step = { type: 'command', commands: ['a'] };
    const copy = copyStep(step) as JsonObject;
    expect(copy).toEqual(step);
    expect(copy.commands).not.toBe(step.commands);
  });

  it('knows a step type and whether a step has anything to lose', () => {
    expect(stepType({ type: 'mkdir' })).toBe('mkdir');
    expect(stepType({ type: 3 })).toBeUndefined();
    expect(stepHasContent({ type: 'mkdir' })).toBe(false);
    // a cleared required field keeps its key with an empty value: nothing to lose
    expect(stepHasContent({ type: 'mkdir', target: '' })).toBe(false);
    expect(stepHasContent({ type: 'download', files: [] })).toBe(false);
    expect(stepHasContent({ type: 'mkdir', target: 'world' })).toBe(true);
    expect(stepHasContent({ type: 'mkdir', if: "env == 'host'" })).toBe(true);
    expect(stepHasContent(null)).toBe(true);
  });
});

describe('the palette search', () => {
  const ops = templateOperationsByCategory().flatMap((g) => g.operations);
  const text = (key: string) => ({
    label: `label of ${key}`,
    help: `help of ${key}`,
  });

  it('matches the label, the key and the help, ignoring case and accents', () => {
    expect(matchOperation({ key: 'download', label: 'Descargar archivos', help: 'Descarga cada dirección', category: 'Descargas' }, 'DESCARGAR')).toBeGreaterThan(0);
    expect(matchOperation({ key: 'download', label: 'Descargar archivos', help: 'Descarga cada dirección', category: 'Descargas' }, 'direccion')).toBeGreaterThan(0);
    expect(matchOperation({ key: 'steamgamedl', label: 'Juego de Steam', help: '', category: 'Steam' }, 'steamgamedl')).toBeGreaterThan(0);
    expect(matchOperation({ key: 'mkdir', label: 'Crear carpeta', help: '', category: 'Archivos' }, 'zip')).toBe(0);
  });

  it('needs every word, and ranks a label match above a help match', () => {
    const op = { key: 'writefile', label: 'Escribir archivo', help: 'Crea un archivo con el texto dado', category: 'Archivos' };
    expect(matchOperation(op, 'escribir archivo')).toBeGreaterThan(0);
    expect(matchOperation(op, 'escribir zip')).toBe(0);
    const inLabel = matchOperation(op, 'escribir');
    const inHelp = matchOperation(op, 'texto');
    expect(inLabel).toBeGreaterThan(inHelp);
    expect(matchOperation(op, '')).toBe(1);
  });

  it('every offered operation matches its own key', () => {
    for (const op of ops) expect(matchOperation({ key: op.key, ...text(op.key), category: op.category }, op.key)).toBeGreaterThan(0);
  });
});

describe('operation icons', () => {
  it('has the icon of every operation', () => {
    for (const op of OPERATIONS) expect(OPERATION_ICONS[op.icon], `${op.key}: ${op.icon}`).toBeDefined();
  });
});

describe('maps and console settings', () => {
  it('names a new entry with a free key', () => {
    expect(uniqueEntryKey('KEY', [])).toBe('KEY');
    expect(uniqueEntryKey('KEY', ['KEY', 'KEY_2'])).toBe('KEY_3');
  });

  it('renames an entry in place, never onto another', () => {
    const env = { A: '1', B: '2', C: '3' };
    expect(Object.keys(renameEntry(env, 'B', 'X'))).toEqual(['A', 'X', 'C']);
    expect(renameEntry(env, 'B', 'X').X).toBe('2');
    expect(renameEntry(env, 'B', 'C')).toBe(env);
    expect(renameEntry(env, 'Z', 'Y')).toBe(env);
  });

  it('removes a console setting cleared, keeping the others', () => {
    expect(withStdinKey({ type: 'rcon', port: '1' }, 'port', '')).toEqual({ type: 'rcon' });
    expect(withStdinKey({ type: 'rconws' }, 'secure', false)).toEqual({ type: 'rconws', secure: false });
    expect(withStdinKey({ type: 'rconws', secure: false }, 'secure', undefined)).toEqual({ type: 'rconws' });
  });
});

describe('moving map entries', () => {
  it('moves an entry and keeps the others in order', () => {
    const env = { A: '1', B: '2', C: '3' };
    expect(moveEntry(env, 0, 2)).toEqual({ object: { B: '2', C: '3', A: '1' }, to: 2 });
    expect(Object.keys(moveEntry(env, 2, 0)!.object)).toEqual(['C', 'A', 'B']);
    expect(moveEntry(env, 1, 1)).toBeNull();
    expect(moveEntry(env, 5, 0)).toBeNull();
  });

  it('never moves a number-like name, nor anything above one (JavaScript lists those first)', () => {
    const env = { 10: 'x', 2: 'y', A: '1', B: '2' };
    expect(Object.keys(env)).toEqual(['2', '10', 'A', 'B']);
    expect(moveEntry(env, 0, 3)).toBeNull();
    // a drop among them lands just below them
    expect(moveEntry(env, 3, 0)).toEqual({ object: { 2: 'y', 10: 'x', B: '2', A: '1' }, to: 2 });
    expect(Object.keys(moveEntry(env, 3, 0)!.object)).toEqual(['2', '10', 'B', 'A']);
    expect(moveEntry(env, 2, 1)).toBeNull();
  });

  it('checks a required map for at least one entry', () => {
    const required = widgetSchema({ kind: 'keyValue' }, true, false);
    expect(required.safeParse({}).success).toBe(false);
    expect(required.safeParse({ A: '1' }).success).toBe(true);
    expect(widgetSchema({ kind: 'keyValue' }, false, false).safeParse({}).success).toBe(true);
  });
});

describe('folds opened by their content', () => {
  const closed = { open: false, closedByUser: false };

  it('opens when content needs to be seen, and stays open when it goes away', () => {
    const opened = foldAfterContentChange(closed, true);
    expect(opened.open).toBe(true);
    expect(foldAfterContentChange(opened, false)).toBe(opened);
  });

  it('stays closed once the user closed it, and returns the same state when nothing changes', () => {
    const userClosed = { open: false, closedByUser: true };
    expect(foldAfterContentChange(userClosed, true)).toBe(userClosed);
    expect(foldAfterContentChange(closed, false)).toBe(closed);
  });
});
