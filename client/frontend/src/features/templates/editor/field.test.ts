import { describe, expect, it } from 'vitest';
import { textValue } from './field';

describe('a value as a text input shows it', () => {
  it.each([
    ['text', 'text'],
    [undefined, ''],
    [null, ''],
    [25565, '25565'],
    [false, 'false'],
    // a variable's value may be a list: it reads as the panel always showed it
    [['a', 'b'], 'a,b'],
    [{ a: 1 }, '{"a":1}'],
  ])('%j reads %j', (value, text) => {
    expect(textValue(value)).toBe(text);
  });
});
