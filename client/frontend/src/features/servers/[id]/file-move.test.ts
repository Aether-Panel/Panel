import { describe, expect, it } from 'vitest';
import { moveProblem, normalizeFolder, parentOf, planMove, runMoves } from './file-move';

describe('normalizeFolder', () => {
  it.each([
    ['~/', ''],
    ['~', ''],
    ['/', ''],
    ['', ''],
    ['.', ''],
    ['~/plugins/', 'plugins'],
    ['/plugins//old/', 'plugins/old'],
    ['plugins\\old', 'plugins/old'],
    ['./a/./b', 'a/b'],
    ['~user/x', '~user/x'],
  ])('%s → %s', (input, expected) => {
    expect(normalizeFolder(input)).toBe(expected);
  });

  it('refuses a way out of the server', () => {
    expect(normalizeFolder('../etc')).toBeNull();
    expect(normalizeFolder('~/a/../../b')).toBeNull();
  });
});

describe('parentOf', () => {
  it('goes one folder up, to the root at most', () => {
    expect(parentOf('bot/src')).toBe('bot');
    expect(parentOf('bot')).toBe('');
    expect(parentOf('')).toBe('');
  });
});

describe('moveProblem', () => {
  it('stops moves onto themselves and folders into themselves', () => {
    expect(moveProblem('bot/a.js', 'bot/a.js', true)).toBe('same');
    expect(moveProblem('bot', 'bot/src/bot', false)).toBe('inside');
    // a sibling that only shares the start of the name is fine
    expect(moveProblem('bot', 'bots/bot', false)).toBeNull();
    expect(moveProblem('bot/a.js', 'a.js', true)).toBeNull();
  });
});

describe('planMove', () => {
  it('takes everything out of a folder to the root', () => {
    const { steps, problem } = planMove('bot', [{ name: 'src', isFile: false }, { name: 'package.json', isFile: true }], '');
    expect(problem).toBeNull();
    expect(steps).toEqual([
      { name: 'src', from: 'bot/src', to: 'src' },
      { name: 'package.json', from: 'bot/package.json', to: 'package.json' },
    ]);
  });

  it('moves nothing when one of them cannot go', () => {
    expect(planMove('', [{ name: 'a.txt', isFile: true }, { name: 'bot', isFile: false }], 'bot/inner')).toEqual({ steps: [], problem: 'inside' });
    expect(planMove('bot', [{ name: 'a', isFile: true }], 'bot').problem).toBe('same');
  });
});

describe('runMoves', () => {
  it('stops at the first failure with how many went', async () => {
    const steps = planMove('bot', [{ name: 'a', isFile: true }, { name: 'b', isFile: true }, { name: 'c', isFile: true }], '').steps;
    const done: string[] = [];
    const boom = new Error('exists');
    const result = await runMoves(steps, async (step) => {
      if (step.name === 'b') throw boom;
      done.push(step.name);
    });
    expect(done).toEqual(['a']);
    expect(result).toEqual({ done: 1, error: boom });
  });
});
