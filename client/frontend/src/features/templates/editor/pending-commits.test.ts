import { describe, expect, it } from 'vitest';
import { createPendingCommits } from './pending-commits';

describe('pending commits', () => {
  it('writes every held-back edit on flush and reports how many there are', () => {
    const sizes: number[] = [];
    const pending = createPendingCommits((size) => sizes.push(size));
    const written: string[] = [];
    const offA = pending.register(() => written.push('a'));
    pending.register(() => written.push('b'));
    expect(pending.size).toBe(2);
    pending.flush();
    expect(written).toEqual(['a', 'b']);
    offA();
    offA();
    expect(pending.size).toBe(1);
    expect(sizes).toEqual([1, 2, 1]);
  });

  it('lets a commit unregister itself while flushing', () => {
    const pending = createPendingCommits();
    const written: string[] = [];
    const off = pending.register(() => {
      written.push('key');
      off();
    });
    pending.register(() => written.push('other'));
    pending.flush();
    pending.flush();
    expect(written).toEqual(['key', 'other', 'other']);
    expect(pending.size).toBe(1);
  });

  it('keeps two registrations of the same function apart', () => {
    const pending = createPendingCommits();
    let count = 0;
    const commit = () => count++;
    const offFirst = pending.register(commit);
    pending.register(commit);
    offFirst();
    pending.flush();
    expect(count).toBe(1);
  });
});
