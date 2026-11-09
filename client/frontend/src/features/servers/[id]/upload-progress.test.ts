import { describe, expect, it } from 'vitest';
import { formatBytes, formatDuration } from './upload-progress';

describe('formatBytes', () => {
  it('picks the unit and the decimals', () => {
    expect(formatBytes(512, 'en-US')).toBe('512 B');
    expect(formatBytes(1536, 'en-US')).toBe('1.5 KB');
    expect(formatBytes(1536, 'es-ES')).toBe('1,5 KB');
    expect(formatBytes(150 * 1024 * 1024, 'en-US')).toBe('150 MB');
    expect(formatBytes(3 * 1024 ** 3, 'en-US')).toBe('3.0 GB');
  });
});

describe('formatDuration', () => {
  it('reads as seconds, minutes or hours', () => {
    expect(formatDuration(0.2)).toBe('1 s');
    expect(formatDuration(42)).toBe('42 s');
    expect(formatDuration(75)).toBe('1 min 15 s');
    expect(formatDuration(120)).toBe('2 min');
    expect(formatDuration(3725)).toBe('1 h 2 min');
  });
});
