import { readdirSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import type { JsonObject } from '../paths';
import { canMakeSingle, commandEntries, commandMode, defaultEntryIndex, hasCondition, toConditional, toSingle } from './command';
import { commandFallback, stopMethod } from './rules';
import { hasStopCode, stopMode, switchStopMode } from './stop';

const FIXTURES = new URL('../../schema/__fixtures__/community/', import.meta.url);
const community = readdirSync(FIXTURES)
  .filter((f) => f.endsWith('.json'))
  .map((f) => [f, JSON.parse(readFileSync(new URL(f, FIXTURES), 'utf8')) as JsonObject] as const);

describe('the start command', () => {
  it('tells its shape', () => {
    expect(commandMode('java -jar server.jar')).toBe('single');
    expect(commandMode(undefined)).toBe('single');
    expect(commandMode(null)).toBe('single');
    expect(commandMode([{ command: 'a', if: "env == 'host'" }, { command: 'b' }])).toBe('conditional');
    expect(commandMode([])).toBe('conditional');
    expect(commandMode(['a'])).toBe('other');
    expect(commandMode(7)).toBe('other');
    expect(commandMode({ command: 'a' })).toBe('other');
  });

  it('reads a condition the way the node does: blank and null are none', () => {
    expect(hasCondition({ command: 'a' })).toBe(false);
    expect(hasCondition({ command: 'a', if: '' })).toBe(false);
    expect(hasCondition({ command: 'a', if: '  ' })).toBe(false);
    expect(hasCondition({ command: 'a', if: null })).toBe(false);
    expect(hasCondition({ command: 'a', if: "env == 'host'" })).toBe(true);
  });

  it('turns one line into a list whose fallback runs it, and back, without losing anything', () => {
    const list = toConditional('java -jar server.jar');
    expect(list).toEqual([{ command: 'java -jar server.jar' }]);
    expect(defaultEntryIndex(list)).toBe(0);
    expect(canMakeSingle(list)).toBe(true);
    expect(toSingle(list)).toBe('java -jar server.jar');
    expect(toConditional(undefined)).toEqual([{ command: '' }]);
    expect(toConditional(null)).toEqual([{ command: '' }]);
    // a list stays the list it is
    const two = [{ if: "env == 'host'", command: 'a' }, { command: 'b' }];
    expect(toConditional(two)).toEqual(two);
  });

  it('only goes back to one line when nothing would be lost', () => {
    expect(canMakeSingle([])).toBe(true);
    expect(toSingle([])).toBe('');
    expect(canMakeSingle([{ command: 'a', if: '' }])).toBe(true);
    expect(canMakeSingle([{ command: 'a', if: "env == 'host'" }])).toBe(false);
    expect(canMakeSingle([{ command: 'a' }, { command: 'b' }])).toBe(false);
    expect(canMakeSingle([{ command: 'a', stdin: { type: 'rcon' } }])).toBe(false);
    expect(canMakeSingle([{ command: 3 }])).toBe(false);
    expect(canMakeSingle('a')).toBe(false);
    expect(canMakeSingle([{ command: 'a' }, 'junk'])).toBe(false);
  });

  it('finds the fallback the node uses: the first entry without a condition', () => {
    const entries = commandEntries([{ if: "env == 'host'", command: 'a' }, { command: 'b' }, { command: 'c' }]);
    expect(defaultEntryIndex(entries)).toBe(1);
    expect(defaultEntryIndex(commandEntries([{ if: 'x', command: 'a' }]))).toBe(-1);
  });

  it('warns about a list with no fallback, and about fallbacks after the first', () => {
    expect(commandFallback({ run: { command: [{ if: 'x', command: 'a' }] } })).toEqual([{ path: ['run', 'command'], code: 'noDefaultCommand' }]);
    expect(commandFallback({ run: { command: [] } })).toEqual([{ path: ['run', 'command'], code: 'noDefaultCommand' }]);
    expect(commandFallback({ run: { command: [{ command: 'a' }, { if: 'x', command: 'b' }, { command: 'c' }] } })).toEqual([
      { path: ['run', 'command', 2], code: 'extraDefaultCommand' },
    ]);
    expect(commandFallback({ run: { command: 'a' } })).toEqual([]);
    expect(commandFallback({ run: null })).toEqual([]);
  });
});

describe('stopping', () => {
  it('follows the node: a code other than 0 wins over the command', () => {
    expect(stopMode({ stop: 'stop' })).toBe('command');
    expect(stopMode({ stopCode: 2 })).toBe('signal');
    expect(stopMode({ stop: 'stop', stopCode: 15 })).toBe('signal');
    expect(stopMode({ stop: 'stop', stopCode: 0 })).toBe('command');
    expect(stopMode({})).toBeNull();
    expect(stopMode({ stop: '', stopCode: null })).toBeNull();
    // a code the node cannot read is still meant as a code (its error shows there)
    expect(hasStopCode({ stopCode: 'x' })).toBe(true);
  });

  it('switching removes the other key in place, and switching back restores it', () => {
    const run = { command: 'srv', stop: 'quit', autostart: false };
    const toSignal = switchStopMode(run, 'signal', {});
    // no code was ever written: none is made up
    expect(toSignal.run).toEqual({ command: 'srv', autostart: false });
    expect(toSignal.memory).toEqual({ stop: 'quit' });

    const withCode = { command: 'srv', stopCode: 15, autostart: false };
    const back = switchStopMode(withCode, 'command', toSignal.memory);
    expect(back.run).toEqual({ command: 'srv', stop: 'quit', autostart: false });
    expect(Object.keys(back.run)).toEqual(['command', 'stop', 'autostart']);
    expect(back.memory).toEqual({ stop: 'quit', stopCode: 15 });

    const again = switchStopMode(back.run, 'signal', back.memory);
    expect(again.run).toEqual({ command: 'srv', stopCode: 15, autostart: false });
    expect(Object.keys(again.run)).toEqual(['command', 'stopCode', 'autostart']);
  });

  it('with both keys, choosing one way drops the other', () => {
    // both keys: choosing the command drops the code the node would have used
    const both = { stop: 'stop', stopCode: 2 };
    expect(switchStopMode(both, 'command', {}).run).toEqual({ stop: 'stop' });
    expect(switchStopMode(both, 'signal', {}).run).toEqual({ stopCode: 2 });
  });

  it('warns about a command the code makes useless, and about no way to stop', () => {
    expect(stopMethod({ run: { stop: 'stop', stopCode: 2 } })).toEqual([{ path: ['run', 'stop'], code: 'stopCommandIgnored' }]);
    expect(stopMethod({ run: { command: 'x' } })).toEqual([{ path: ['run', 'stop'], code: 'noStopMethod' }]);
    expect(stopMethod({ run: { stop: 'stop' } })).toEqual([]);
    expect(stopMethod({ run: { stopCode: 15 } })).toEqual([]);
    expect(stopMethod({})).toEqual([]);
  });

  it('finds nothing to warn about in the community templates', () => {
    for (const [file, template] of community) {
      expect([...commandFallback(template), ...stopMethod(template)], file).toEqual([]);
    }
  });
});
