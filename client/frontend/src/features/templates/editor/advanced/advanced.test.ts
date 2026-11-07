import { readdirSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import type { JsonObject } from '../paths';
import { joinAlternatives, parseAlternatives, toggleAlternative } from '../requirements/model';
import { writeOptional } from '../write';
import { GO_DURATION, groupVariables, groupVariablesRule, isPositiveDuration, keepAliveRule, newGroup } from './model';

const FIXTURES = new URL('../../schema/__fixtures__/community/', import.meta.url);
const community = readdirSync(FIXTURES)
  .filter((f) => f.endsWith('.json'))
  .map((f) => [f, JSON.parse(readFileSync(new URL(f, FIXTURES), 'utf8')) as JsonObject] as const);

describe('requirement rows', () => {
  it('read one value or several joined by ||, as the node splits them', () => {
    expect(parseAlternatives('linux')).toEqual(['linux']);
    expect(parseAlternatives('linux || windows')).toEqual(['linux', 'windows']);
    expect(parseAlternatives('linux||windows')).toEqual(['linux', 'windows']);
    expect(parseAlternatives('')).toEqual([]);
    expect(parseAlternatives(undefined)).toEqual([]);
    expect(parseAlternatives(3)).toEqual([]);
  });

  it('toggle a value, and clear the key when none is left', () => {
    expect(toggleAlternative(undefined, 'linux')).toBe('linux');
    expect(toggleAlternative('linux', 'windows')).toBe('linux || windows');
    expect(toggleAlternative('linux || windows', 'linux')).toBe('windows');
    expect(toggleAlternative('linux', 'linux')).toBeUndefined();
    // a value the form does not list stays
    expect(toggleAlternative('freebsd', 'linux')).toBe('freebsd || linux');
    expect(joinAlternatives([])).toBeUndefined();
  });
});

describe('optional values', () => {
  it('lose their key when cleared, and with prune their emptied object too', () => {
    const template = { requirements: { os: 'linux' }, keepAlive: { frequency: '5m', command: 'save' } };
    expect(writeOptional(template, ['requirements', 'os'], undefined)).toEqual({ requirements: {}, keepAlive: template.keepAlive });
    expect(writeOptional(template, ['requirements', 'os'], undefined, true)).toEqual({ keepAlive: template.keepAlive });
    expect(writeOptional(template, ['keepAlive', 'command'], '', true)).toEqual({ requirements: template.requirements, keepAlive: { frequency: '5m' } });
    expect(writeOptional(template, ['requirements', 'binaries'], [], true)).toBe(template);
    expect(writeOptional(template, ['requirements', 'arch'], 'amd64')).toEqual({ requirements: { os: 'linux', arch: 'amd64' }, keepAlive: template.keepAlive });
  });

  it('leave an object that was empty already as written', () => {
    const template = { requirements: {} };
    expect(writeOptional(template, ['requirements', 'os'], undefined, true)).toBe(template);
  });
});

describe('keep-alive', () => {
  it.each([
    ['30s', true],
    ['5m', true],
    ['1h30m', true],
    ['1.5h', true],
    ['300ms', true],
    ['0s', false],
    ['0', false],
    ['5', false],
    ['5 m', false],
    ['-1s', false],
    [' 5m', false],
    ['${frequency}', false],
    ['', false],
  ])('%s is a usable frequency: %s', (value, ok) => {
    expect(isPositiveDuration(value)).toBe(ok);
  });

  it('builds the duration pattern from its parts without changing what it accepts', () => {
    // the single literal it was written as before being split into named parts
    expect(GO_DURATION.source).toBe(String.raw`^(?:(?:\d+(?:\.\d*)?|\.\d+)(?:ns|us|µs|μs|ms|s|m|h))+$`);
    expect(GO_DURATION.flags).toBe('');
  });

  it('warns about a frequency the node cannot read, or a half filled keep-alive', () => {
    expect(keepAliveRule({ keepAlive: { frequency: '5m', command: 'save-all' } })).toEqual([]);
    expect(keepAliveRule({ keepAlive: { frequency: 'often', command: 'save-all' } })).toEqual([{ path: ['keepAlive', 'frequency'], code: 'keepAliveFrequency' }]);
    expect(keepAliveRule({ keepAlive: { frequency: '5m' } })).toEqual([{ path: ['keepAlive', 'command'], code: 'keepAliveIncomplete' }]);
    expect(keepAliveRule({ keepAlive: { command: 'save-all' } })).toEqual([{ path: ['keepAlive', 'frequency'], code: 'keepAliveIncomplete' }]);
    expect(keepAliveRule({ keepAlive: {} })).toEqual([]);
    expect(keepAliveRule({})).toEqual([]);
  });
});

describe('groups', () => {
  it('list their variables as text', () => {
    expect(groupVariables({ variables: ['a', 3, 'b'] })).toEqual(['a', 'b']);
    expect(groupVariables({})).toEqual([]);
    expect(groupVariables(null)).toEqual([]);
  });

  it('start empty, with the next order when the others have one', () => {
    expect(newGroup([])).toEqual({ display: '', description: '', variables: [] });
    expect(newGroup([{ order: 1 }, { order: 4 }, 'junk'])).toEqual({ display: '', description: '', variables: [], order: 5 });
  });

  it('warn about a listed key no variable has', () => {
    const template = { data: { port: {}, motd: {} }, groups: [{ variables: ['port', 'gone'] }, { variables: ['motd'] }, 'junk'] };
    expect(groupVariablesRule(template)).toEqual([{ path: ['groups', 0, 'variables', 1], code: 'groupUnknownVariable', params: { key: 'gone' } }]);
  });

  it('find nothing to warn about in the community templates', () => {
    for (const [file, template] of community) {
      expect([...groupVariablesRule(template), ...keepAliveRule(template)], file).toEqual([]);
    }
  });
});
