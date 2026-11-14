import { describe, expect, it } from 'vitest';
import { groupIssues, pathFromKey } from './issue-groups';
import { sectionOfPath } from './section-keys';
import type { EditorIssue } from './validation';

const issue = (path: string, severity: EditorIssue['severity'], blocking = false): EditorIssue => ({
  path,
  messageKey: `k.${path}.${severity}`,
  severity,
  blocking,
});

describe('which section an issue belongs to', () => {
  it.each([
    ['', 'info'],
    ['name', 'info'],
    ['icon', 'info'],
    ['data.port.value', 'variables'],
    ['install.2.target', 'install'],
    ['uninstall.0', 'install'],
    ['run.pre.1.if', 'run'],
    ['environment.image', 'environment'],
    ['supportedEnvironments.0.image', 'environment'],
    ['requirements.os', 'requirements'],
    ['groups.0.variables.1', 'advanced'],
    ['keepAlive.frequency', 'advanced'],
    // no section edits these: a key next to "run", and one the form doesn't know
    ['runner', null],
    ['custom.thing', null],
  ])('%s → %s', (path, section) => {
    expect(sectionOfPath(path)).toBe(section);
  });
});

describe('the summary list', () => {
  it('puts sections that block saving first, then follows the menu, unknown keys last', () => {
    const groups = groupIssues([
      issue('run.stop', 'warning'),
      issue('custom', 'error'),
      issue('install.0.target', 'error'),
      issue('name', 'error', true),
      issue('data.x', 'warning'),
    ]);
    expect(groups.map((g) => g.section)).toEqual(['info', 'variables', 'install', 'run', null]);
  });

  it('lists blocking errors, then errors, then warnings inside a section, keeping their order', () => {
    const [install] = groupIssues([
      issue('install.0', 'warning'),
      issue('install.1.files', 'error'),
      issue('install.2', 'error', true),
      issue('install.3', 'warning'),
      issue('install.4', 'error'),
    ]);
    expect(install.issues.map((i) => i.path)).toEqual(['install.2', 'install.1.files', 'install.4', 'install.0', 'install.3']);
  });

  it('a section with a blocking issue comes before the menu order', () => {
    const groups = groupIssues([issue('data.a', 'error'), issue('install.0', 'error', true)]);
    expect(groups.map((g) => g.section)).toEqual(['install', 'variables']);
  });

  it('is empty without issues', () => {
    expect(groupIssues([])).toEqual([]);
  });
});

describe('the field an issue points to', () => {
  const template = {
    data: { 10: { value: 'x' }, port: { value: 1 } },
    install: [{ type: 'mkdir' }, { type: 'download', files: ['a', 'b'] }],
    run: { command: [{ command: 'a' }, { command: 'b', if: '' }] },
  };

  it.each([
    ['', []],
    ['name', ['name']],
    ['install.1.files.0', ['install', 1, 'files', 0]],
    ['run.command.1.if', ['run', 'command', 1, 'if']],
    // a variable named with digits is a key, not an index
    ['data.10.value', ['data', '10', 'value']],
    ['data.port', ['data', 'port']],
    // past what the template holds, digits are taken as an index
    ['uninstall.0.target', ['uninstall', 0, 'target']],
  ])('%s → %j', (key, path) => {
    expect(pathFromKey(template, key)).toEqual(path);
  });
});
