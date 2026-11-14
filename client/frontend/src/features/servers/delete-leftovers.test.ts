import { afterEach, describe, expect, it, vi } from 'vitest';
import { describeLeftovers, leftoversFrom, rememberLeftovers, takeLeftovers } from './delete-leftovers';

const world = { hostId: 2, hostName: 'mariadb-1', database: 's_a1b2c3d4_world', username: 'u_x7k2' };

describe('leftoversFrom', () => {
  it.each([
    ['a 204 (empty text)', ''],
    ['nothing', undefined],
    ['an answer without the list', { ok: true }],
    ['a list that is not one', { leftoverDatabases: 'x' }],
  ])('finds none in %s', (_, answer) => {
    expect(leftoversFrom(answer)).toEqual([]);
  });

  it('reads the list and drops malformed entries', () => {
    expect(leftoversFrom({ leftoverDatabases: [world, { database: 1 }, null] })).toEqual([world]);
  });
});

describe('describeLeftovers', () => {
  it('names the database, its user and the host', () => {
    expect(describeLeftovers([world, { ...world, hostName: '', database: 's_b', username: 'u_b' }])).toBe(
      's_a1b2c3d4_world (u_x7k2) @ mariadb-1, s_b (u_b)'
    );
  });
});

describe('remember and take', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('hands the list over once', () => {
    const store = new Map<string, string>();
    vi.stubGlobal('sessionStorage', {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => void store.set(k, v),
      removeItem: (k: string) => void store.delete(k),
    });

    rememberLeftovers([world]);
    expect(takeLeftovers()).toEqual([world]);
    expect(takeLeftovers()).toEqual([]);
  });

  it('works without storage', () => {
    vi.stubGlobal('sessionStorage', undefined);
    expect(() => rememberLeftovers([world])).not.toThrow();
    expect(takeLeftovers()).toEqual([]);
  });
});
