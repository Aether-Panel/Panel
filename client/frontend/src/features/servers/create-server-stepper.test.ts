import { describe, expect, it } from 'vitest';
import { initialTemplateValues, RESOURCE_DEFAULTS, shownTemplateVariables } from './create-server-stepper';

// "Crear servidor" and the template editor's preview both start from these: they must keep
// giving the wizard exactly the values it always had.

describe('initialTemplateValues', () => {
  it.each([
    ['a string value', { type: 'string', value: 'hello' }, 'hello'],
    ['the default when there is no value', { default: 'fallback' }, 'fallback'],
    ['the value over the default', { value: 'v', default: 'd' }, 'v'],
    ['nothing at all', { display: 'Port' }, ''],
    ['a null value', { value: null }, ''],
    ['a boolean true', { type: 'boolean', value: true }, true],
    ['a boolean written "false"', { type: 'boolean', value: 'false' }, false],
    ['a boolean written "true"', { type: 'boolean', value: 'true' }, true],
    ['an integer written as text', { type: 'integer', value: '25565' }, 25565],
    ['an integer that is not a number', { type: 'integer', value: 'abc' }, 0],
    ['an object value, as JSON', { value: { a: 1 } }, '{"a":1}'],
    ['a list value, as JSON', { value: ['x', 'y'] }, '["x","y"]'],
  ])('an object variable: %s', (_, variable, expected) => {
    expect(initialTemplateValues({ data: { key: variable } }).values).toEqual({ key: expected });
  });

  it('keeps a scalar variable as text and skips a null one', () => {
    expect(initialTemplateValues({ data: { port: 25565, motd: 'hi', gone: null } }).values).toEqual({ port: '25565', motd: 'hi' });
  });

  it('gives internal variables a value too (the wizard sends them as they are)', () => {
    expect(initialTemplateValues({ data: { secret: { internal: true, value: 's' } } }).values).toEqual({ secret: 's' });
  });

  it.each([
    ['a value', { memory: { value: '4096' } }, { memory: 4096 }],
    ['an empty value: the wizard default', { memory: { value: '' } }, { memory: RESOURCE_DEFAULTS.memory }],
    ['no value at all: the wizard default', { cpu: { display: 'CPU' } }, { cpu: RESOURCE_DEFAULTS.cpu }],
    ['a scalar', { disk: 20480 }, { disk: 20480 }],
    ['all four', { cpu: { value: 200 }, memory: { value: 2048 }, disk: { value: 0 }, io: { value: '750' } }, { cpu: 200, memory: 2048, disk: RESOURCE_DEFAULTS.disk, io: 750 }],
  ])('a resource variable with %s fills the limits, not the values', (_, data, limits) => {
    const start = initialTemplateValues({ data });
    expect(start.limits).toEqual(limits);
    expect(start.values).toEqual({});
  });

  it.each([
    ['the memory variable', { memory: { value: 2048 } }, 2048],
    ['the memory variable as text', { memory: { type: 'integer', value: '3072' } }, 3072],
    ['an empty memory variable', { memory: { value: '' } }, null],
    ['no memory variable', { port: { value: 1 } }, null],
    ['a scalar memory variable', { memory: 3000 }, null],
  ])('recommendedMemory from %s', (_, data, expected) => {
    expect(initialTemplateValues({ data }).recommendedMemory).toBe(expected);
  });

  it('reads `variables` when there is no `data`', () => {
    expect(initialTemplateValues({ variables: { a: { value: 'x' } } }).values).toEqual({ a: 'x' });
  });

  it('starts empty for a template without variables', () => {
    expect(initialTemplateValues({})).toEqual({ values: {}, limits: {}, recommendedMemory: null });
  });
});

describe('shownTemplateVariables', () => {
  it('lists every variable but the internal and the resource ones, in order', () => {
    const data = {
      version: { value: 'latest' },
      memory: { value: 2048 },
      secret: { internal: true },
      port: { value: 25565 },
      cpu: { value: 100 },
      gone: null,
    };
    expect(shownTemplateVariables({ data }).map(([key]) => key)).toEqual(['version', 'port']);
  });

  it('is empty when the template asks for nothing', () => {
    expect(shownTemplateVariables({})).toEqual([]);
    expect(shownTemplateVariables({ data: { memory: { value: 1 }, hidden: { internal: true } } })).toEqual([]);
  });
});
