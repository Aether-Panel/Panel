import { readdirSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { issueMessageKey, issuePath } from './issues';
import { templateJsonSchema } from './json-schema';
import { getOperation } from './operations';
import { templateSchema } from './template';

// __fixtures__/community holds every template listed at https://templates.aetherpanel.es/
// templates.json (community-index.json), byte for byte. To refresh them, download the index and
// each entry's url (relative to that host) over the files there.
const FIXTURES = new URL('./__fixtures__/community/', import.meta.url);
const INDEX = new URL('./__fixtures__/community-index.json', import.meta.url);

type Json = Record<string, unknown>;

const fixtures: [string, Json][] = readdirSync(FIXTURES)
  .filter((f) => f.endsWith('.json'))
  .map((f): [string, Json] => [f.replace(/\.json$/, ''), JSON.parse(readFileSync(new URL(f, FIXTURES), 'utf8')) as Json])
  .sort(([a], [b]) => (a < b ? -1 : 1));

function stepTypes(template: Json): string[] {
  const run = (template.run ?? {}) as Json;
  return [template.install, template.uninstall, run.pre, run.post]
    .flatMap((list) => (Array.isArray(list) ? list : []))
    .map((step) => String((step as Json).type));
}

/** The issues of a parse, as "path: i18n key", for readable failures and assertions. */
function issues(value: unknown): string[] {
  const result = templateSchema.safeParse(value);
  return result.success ? [] : result.error.issues.map((i) => `${issuePath(i)}: ${issueMessageKey(i)}`);
}

describe('community templates', () => {
  it('are all present', () => {
    const index = JSON.parse(readFileSync(INDEX, 'utf8')) as Record<string, { url: string }>;
    expect(Object.keys(index).length).toBeGreaterThan(0);
    expect(fixtures.map(([name]) => name)).toEqual(Object.keys(index).sort());
  });

  it.each(fixtures)('%s parses without issues', (_, template) => {
    expect(issues(template)).toEqual([]);
  });

  it.each(fixtures)('%s parses losslessly', (_, template) => {
    // a deep copy, so nothing the parse does to its input can make the two look alike
    const original = structuredClone(template);
    expect(templateSchema.parse(template)).toStrictEqual(original);
  });

  it.each(fixtures)('%s uses only known operations', (_, template) => {
    expect(stepTypes(template).filter((type) => !getOperation(type))).toEqual([]);
  });
});

describe('template schema', () => {
  const base = { type: 'game', display: 'Test', data: {}, run: { command: 'run' }, environment: { type: 'host' } };

  it('reads a template as the node writes it back', () => {
    // nil lists are written as null, empty MetadataTypes as { "type": "" }: a template saved
    // without an environment comes back with { "type": "" }
    const saved = {
      ...base,
      name: 'test',
      environment: { type: '' },
      install: [{ type: 'mkdir', target: 'plugins' }],
      uninstall: null,
      run: { command: null, stdin: {}, autostart: false, autorecover: false, autorestart: false },
      requirements: {},
      stats: { type: '' },
      query: { type: '' },
      keepAlive: { frequency: '', command: '' },
    };
    expect(issues(saved)).toEqual([]);
    expect(templateSchema.parse(saved)).toStrictEqual(saved);
  });

  it('reads null wherever Go reads it as not given', () => {
    const nulls = { type: 'game', data: null, run: null, environment: null, requirements: null, stats: null, query: null, keepAlive: null };
    expect(issues(nulls)).toEqual([]);
    expect(templateSchema.parse(nulls)).toStrictEqual(nulls);
    const nested = {
      ...base,
      run: {
        command: [{ if: null, command: 'run', stdin: null }],
        stdin: null,
        pre: [{ type: 'mkdir', target: 'logs', if: null }, { type: 'command', commands: ['true'], stdin: null }],
      },
    };
    expect(issues(nested)).toEqual([]);
    expect(templateSchema.parse(nested)).toStrictEqual(nested);
  });

  it('takes "" where the node reads it as not given', () => {
    expect(issues({ ...base, install: [{ type: 'curseforge', projectId: '${projectId}', fileId: '' }] })).toEqual([]);
    expect(issues({ ...base, install: [{ type: 'backup', offsite: '' }] })).toEqual([]);
    // but not in a required argument
    expect(issues({ ...base, install: [{ type: 'spongedl', spongeType: '' }] })).toEqual(['install.0.spongeType: templates.editor.validation.required']);
    expect(issues({ ...base, install: [{ type: 'curseforge', projectId: '' }] })).toEqual(['install.0.projectId: templates.editor.validation.required']);
    expect(issues({ ...base, install: [{ type: 'javadl', version: '' }] })).toEqual(['install.0.version: templates.editor.validation.required']);
    expect(issues({ ...base, install: [{ type: 'sleep', duration: '' }] })).toEqual(['install.0.duration: templates.editor.validation.required']);
  });

  it('flags "" where the node casts it to 0 or refuses it', () => {
    // AutoBackupKeep only defaults when keep is missing: "" becomes 0, outside 1-50
    expect(issues({ ...base, install: [{ type: 'backup', keep: '' }] })).toEqual(['install.0.keep: templates.editor.validation.invalidNumber']);
    expect(issues({ ...base, install: [{ type: 'backup', offsiteKeep: '' }] })).toEqual(['install.0.offsiteKeep: templates.editor.validation.invalidInteger']);
    // an optional field given "" has a wrong value, it is not missing
    expect(issues({ ...base, run: { command: 'run', stopCode: '', autostart: '' } })).toEqual([
      'run.stopCode: templates.editor.validation.invalidInteger',
      'run.autostart: templates.editor.validation.invalidType',
    ]);
  });

  it('keeps unknown keys at every level', () => {
    const template = {
      ...base,
      future: { a: 1 },
      data: { port: { type: 'integer', value: 25565, display: 'Port', extra: true } },
      install: [{ type: 'download', files: ['https://example.com/a.jar'], note: 'kept' }],
      environment: { type: 'docker', image: 'eclipse-temurin:21', tmpfs: ['/tmp'] },
    };
    expect(templateSchema.parse(template)).toStrictEqual(template);
  });

  it('flags an unknown operation on the step type', () => {
    expect(issues({ ...base, install: [{ type: 'teleport' }] })).toEqual(['install.0.type: templates.editor.validation.unknownOperation']);
  });

  it('flags a step that is not an object', () => {
    expect(issues({ ...base, install: [null] })).toEqual(['install.0: templates.editor.validation.stepNotObject']);
    expect(issues({ ...base, install: ['mkdir'] })).toEqual(['install.0: templates.editor.validation.stepNotObject']);
  });

  it('flags a missing required argument', () => {
    expect(issues({ ...base, run: { command: 'run', pre: [{ type: 'writefile', text: 'x' }] } })).toEqual([
      'run.pre.0.target: templates.editor.validation.required',
    ]);
    expect(issues({ ...base, install: [{ type: 'download', files: [] }] })).toEqual(['install.0.files: templates.editor.validation.required']);
  });

  it('accepts the older spelling of an argument and flags when neither is set', () => {
    expect(issues({ ...base, install: [{ type: 'steamgamedl', appID: '730' }] })).toEqual([]);
    expect(issues({ ...base, install: [{ type: 'steamgamedl' }] })).toEqual(['install.0.appId: templates.editor.validation.required']);
    // the older spelling is still checked like the current one
    expect(issues({ ...base, install: [{ type: 'curseforge', projectId: 1, fileID: 'abc' }] })).toEqual([
      'install.0.fileID: templates.editor.validation.invalidNumber',
    ]);
  });

  it('asks forgedl for a version or a Minecraft version', () => {
    expect(issues({ ...base, install: [{ type: 'forgedl', target: 'installer.jar' }] })).toEqual([
      'install.0.minecraftVersion: templates.editor.validation.versionOrMinecraftVersion',
    ]);
    expect(issues({ ...base, install: [{ type: 'forgedl', target: 'installer.jar', minecraftVersion: '1.20.1' }] })).toEqual([]);
  });

  it('takes numbers as numbers, digits or variables where the node casts them', () => {
    for (const version of [21, '21', '${javaversion}']) {
      expect(issues({ ...base, install: [{ type: 'javadl', version }] })).toEqual([]);
    }
    expect(issues({ ...base, install: [{ type: 'curseforge', projectId: 'abc' }] })).toEqual([
      'install.0.projectId: templates.editor.validation.invalidNumber',
    ]);
  });

  it('checks durations the way the node parses them', () => {
    for (const duration of ['30s', '1h30m', '${wait}', 5000]) {
      expect(issues({ ...base, install: [{ type: 'sleep', duration }] })).toEqual([]);
    }
    expect(issues({ ...base, install: [{ type: 'sleep', duration: 'soon' }] })).toEqual(['install.0.duration: templates.editor.validation.invalidDuration']);
  });

  it('flags JSON types the node cannot decode', () => {
    expect(issues({ ...base, run: { command: 'run', stopCode: '2' } })).toEqual(['run.stopCode: templates.editor.validation.invalidInteger']);
    expect(issues({ ...base, run: { command: 'run', environmentVars: { A: 1 } } })).toEqual(['run.environmentVars.A: templates.editor.validation.invalidType']);
    expect(issues({ ...base, run: { command: 'run', stdin: { type: 'rcon', port: 25575 } } })).toEqual(['run.stdin.port: templates.editor.validation.invalidType']);
    expect(issues({ ...base, environment: { type: 'vm' } })).toEqual(['environment.type: templates.editor.validation.invalidEnvironment']);
  });

  it('checks a boolean variable value as the node casts it', () => {
    const data = (value: unknown, type: string) => ({ ...base, data: { v: { type, value } } });
    expect(issues(data('true', 'boolean'))).toEqual([]);
    expect(issues(data(1, 'boolean'))).toEqual([]);
    expect(issues(data('yes', 'boolean'))).toEqual(['data.v.value: templates.editor.validation.invalidBoolean']);
    // a missing value is read as "", which strconv.ParseBool refuses
    expect(issues({ ...base, data: { v: { type: 'boolean' } } })).toEqual(['data.v.value: templates.editor.validation.invalidBoolean']);
    // an integer that does not cast becomes 0 on the node, without an error
    expect(issues(data('lots', 'integer'))).toEqual([]);
  });

  it('checks variable keys', () => {
    expect(issues({ ...base, data: { 'server-port': { value: '1' }, java_version: { value: '21' } } })).toEqual([]);
    expect(issues({ ...base, data: { 'bad key': { value: '1' } } })).toEqual(['data.bad key: templates.editor.validation.invalidVariableKey']);
  });

  it('reads a conditional command list', () => {
    const run = { command: [{ if: "env == 'host'", command: 'java21 -jar server.jar' }, { command: 'java -jar server.jar' }] };
    expect(issues({ ...base, run })).toEqual([]);
    expect(issues({ ...base, run: { command: 5 } })).toEqual(['run.command: templates.editor.validation.invalidCommand']);
  });
});

describe('JSON Schema for the JSON view', () => {
  it('is generated', () => {
    const schema = templateJsonSchema() as { properties?: Record<string, unknown> };
    expect(schema.properties).toHaveProperty('install');
    expect(schema.properties).toHaveProperty('run');
    // the step types are offered by name
    expect(JSON.stringify(schema)).toContain('"const":"steamgamedl"');
  });

  it('does not suggest the older argument spellings', () => {
    const json = JSON.stringify(templateJsonSchema());
    expect(json).not.toContain('"appID"');
    expect(json).not.toContain('"fileID"');
    expect(json).toContain('"appId"');
  });
});
