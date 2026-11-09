import { describe, expect, it } from 'vitest';
import { formatGib } from './mib';

describe('formatGib', () => {
  it('shows the GiB a MiB value holds, in the panel language', () => {
    expect(formatGib(2048, 'es-ES')).toBe('2 GiB');
    expect(formatGib(512, 'es-ES')).toBe('0,5 GiB');
    expect(formatGib(5000, 'es-ES')).toBe('4,88 GiB');
    expect(formatGib(5000, 'en-US')).toBe('4.88 GiB');
    expect(formatGib(51200, 'en-US')).toBe('50 GiB');
  });
});
