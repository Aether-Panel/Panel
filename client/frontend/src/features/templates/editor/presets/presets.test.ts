import { describe, expect, it } from 'vitest';
import en from '@/lib/locales/en.json';
import es from '@/lib/locales/es.json';
import { TEMPLATE_LOGO_KEYS } from '@/lib/template-logos';
import { getOperation } from '../../schema/operations';
import { templateSchema } from '../../schema/template';
import { imageSource } from '../environment/images';
import { getIn, isPlainObject, type JsonObject, type Path } from '../paths';
import { validateTemplate } from '../validation';
import { templateWarnings } from '../warnings';
import { getPreset, presetHref, TEMPLATE_PRESETS } from './index';

// Every base template is a template the editor finds nothing wrong with (apart from the name the
// user still has to give it), in both languages, using only operations the panel knows and only
// the project's or official images.

type Tree = { [key: string]: string | Tree };

/** t() over a locale file that fails the test on a missing text instead of showing the key. */
function strictText(tree: Tree, language: string) {
  return (key: string) => {
    const value = key.split('.').reduce<unknown>((node, part) => (node as Tree | undefined)?.[part], tree);
    if (typeof value !== 'string') throw new Error(`missing ${language} text ${key}`);
    return value;
  };
}

const LANGUAGES = [
  ['es', strictText(es as unknown as Tree, 'es')],
  ['en', strictText(en as unknown as Tree, 'en')],
] as const;

const CASES = TEMPLATE_PRESETS.flatMap((preset) => LANGUAGES.map(([language, t]) => ({ id: preset.id, language, template: preset.build(t) })));

const STEP_LISTS: Path[] = [['install'], ['uninstall'], ['run', 'pre'], ['run', 'post']];

function steps(template: JsonObject): JsonObject[] {
  return STEP_LISTS.flatMap((path) => {
    const list = getIn(template, path);
    return Array.isArray(list) ? list.filter(isPlainObject) : [];
  });
}

function environments(template: JsonObject): JsonObject[] {
  const supported = Array.isArray(template.supportedEnvironments) ? template.supportedEnvironments : [];
  return [template.environment, ...supported].filter(isPlainObject);
}

describe('base templates', () => {
  it('have distinct ids the editor finds again', () => {
    const ids = TEMPLATE_PRESETS.map((p) => p.id);
    expect(new Set(ids).size).toBe(ids.length);
    for (const id of ids) {
      expect(getPreset(id)?.id).toBe(id);
      expect(presetHref(id)).toBe(`/templates/editor/?preset=${encodeURIComponent(id)}`);
    }
    expect(getPreset('nope')).toBeUndefined();
  });

  it.each(TEMPLATE_PRESETS.map((p) => p.id))('%s has its chooser texts in both languages', (id) => {
    for (const [, t] of LANGUAGES) {
      expect(t(`templates.presets.${id}.title`)).not.toBe('');
      expect(t(`templates.presets.${id}.description`)).not.toBe('');
    }
  });

  it('the chooser (outside the editor folder the i18n test reads) has its texts in both languages', () => {
    const keys = [
      ...['option', 'optionDescription', 'title', 'description', 'listLabel'].map((k) => `templates.chooser.preset.${k}`),
      ...['noneTitle', 'noneDescription', 'resultsOne', 'resultsMany'].map((k) => `templates.chooser.duplicate.${k}`),
    ];
    for (const [, t] of LANGUAGES) for (const key of keys) expect(t(key)).not.toBe('');
  });

  it.each(CASES)('$id ($language) matches the schema', ({ template }) => {
    const result = templateSchema.safeParse(template);
    expect(result.success ? [] : result.error.issues).toEqual([]);
  });

  it.each(CASES)('$id ($language) raises no warning', ({ template }) => {
    expect(templateWarnings(template)).toEqual([]);
  });

  it.each(CASES)('$id ($language) only waits for a name', ({ template }) => {
    expect(template.name).toBe('');
    expect(Object.keys(template)[0]).toBe('name');
    const issues = validateTemplate(template, { takenNames: [] });
    expect(issues.map((i) => [i.path, i.messageKey])).toEqual([['name', 'templates.editor.validation.nameRequired']]);
  });

  it.each(CASES)('$id ($language) uses operations the panel knows', ({ template }) => {
    const list = steps(template);
    expect(list.length).toBeGreaterThan(0);
    for (const step of list) expect(getOperation(String(step.type)), String(step.type)).toBeDefined();
  });

  it.each(CASES)('$id ($language) runs on the project\'s or official images only', ({ template }) => {
    const docker = environments(template).filter((env) => env.type === 'docker');
    expect(docker.length).toBeGreaterThan(0);
    for (const env of docker) expect(['project', 'official']).toContain(imageSource(env.image));
  });

  it.each(CASES)('$id ($language) has a display name, a category and a known icon', ({ id, template }) => {
    expect(template.display).toEqual(expect.any(String));
    expect(template.display).not.toBe('');
    expect(template.type).toEqual(expect.any(String));
    const { logo } = getPreset(id)!;
    expect(template.icon).toBe(logo);
    if (logo) expect(TEMPLATE_LOGO_KEYS).toContain(logo);
  });

  it.each(CASES)('$id ($language) describes every variable', ({ template }) => {
    const variables = Object.values(template.data as Record<string, JsonObject>);
    expect(variables.length).toBeGreaterThan(0);
    for (const variable of variables) {
      expect(variable.display).not.toBe('');
      expect(variable.desc).not.toBe('');
      expect(typeof variable.userEdit).toBe('boolean');
      expect(typeof variable.required).toBe('boolean');
    }
  });

  it('build a new object every time', () => {
    const [, t] = LANGUAGES[0];
    for (const preset of TEMPLATE_PRESETS) {
      const a = preset.build(t);
      const b = preset.build(t);
      expect(a).toEqual(b);
      expect(a).not.toBe(b);
      expect(a.run).not.toBe(b.run);
    }
  });
});
