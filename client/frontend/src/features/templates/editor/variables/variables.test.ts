import { readdirSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import type { JsonObject } from '../paths';
import { validateTemplate } from '../validation';
import {
  booleanValue, insertVariable, integerFromText, isPinnedKey, newOption, newVariable, optionValueFromText, sameOptionValue, uniqueVariableKey, usesOptions,
  withVariableType,
} from './model';
import { conditionVariableKeys, undefinedConditionVariables, undefinedVariables, unusedVariables } from './rules';
import { analyzeVariableUsage, deleteVariable, referenceCount, renameVariable, renameVariableUsages } from './usage';

const FIXTURES = new URL('../../schema/__fixtures__/community/', import.meta.url);

const variable = (extra: JsonObject = {}): JsonObject => ({ type: 'string', value: '', ...extra });

/** "path name" for each place a name is referenced. */
function references(template: JsonObject, name: string): string[] {
  return (analyzeVariableUsage(template).references.get(name) ?? []).map((r) => `${r.path.join('.')} ${r.kind}`);
}

describe('variable usage', () => {
  const template: JsonObject = {
    data: { port: variable(), version: variable(), 'server-port': variable(), eula: variable({ type: 'boolean', value: false }) },
    install: [
      { type: 'command', commands: ['echo ${version}', 'echo ${GIT_BRANCH:-HEAD} ${a[@]}'], if: "eula && env == 'host'" },
      { type: 'writefile', target: 'server.properties', text: 'server-port=${server-port}\n' },
      { type: 'scraperdl', url: 'u', regex: 'r', downloadUrl: 'https://x/${match}.zip' },
      { type: 'forgedl', target: 'f', minecraftVersion: '${version}' },
      { type: 'githubdl', repository: 'r', outputVariable: 'asset' },
      { type: 'command', commands: ['rm ${asset} ${opForgeVersion}'] },
    ],
    run: {
      command: [
        { if: "server-port != '' && file_exists('x.jar')", command: 'java -jar x.jar --port ${port}' },
        { command: 'java -jar ${missing}.jar' },
      ],
      environmentVars: { HOME: '${rootDir}' },
      post: [{ type: 'console', message: 'exit ${exitCode} ${success}' }],
    },
    environment: { type: 'docker', image: 'ghcr.io/aether-panel/java:${version}', portBindings: ['${port}:${port}'] },
    // not read by the node: no use
    display: '${port}',
  };

  it('finds tokens in commands, steps, environment variables and Docker settings', () => {
    expect(references(template, 'version')).toEqual([
      'install.0.commands.0 token',
      'install.3.minecraftVersion token',
      'environment.image token',
    ]);
    expect(references(template, 'port')).toEqual(['run.command.0.command token', 'environment.portBindings.0 token', 'environment.portBindings.0 token']);
  });

  it('finds names read by conditions, but not functions, members or literals', () => {
    expect(references(template, 'eula')).toEqual(['install.0.if condition']);
    expect(references({ data: { eula: variable() }, install: [{ type: 'mkdir', target: 'a', if: "a.eula == 'eula' && file_exists('eula')" }] }, 'eula')).toEqual([]);
    const usage = analyzeVariableUsage({ data: {}, install: [{ type: 'mkdir', target: 'a', if: "a.b == 'eula' && file_exists(\"x\") && !in_path('java')" }] });
    expect(usage.undefinedConditionNames.map((r) => r.name)).toEqual(['a']);
  });

  it('matches a hyphenated key whole in a condition and flags it', () => {
    expect(references(template, 'server-port')).toEqual(['install.1.text token', 'run.command.0.if condition']);
    // "port" inside "server-port" is not a use of port, and "server" is not reported as unknown
    expect(analyzeVariableUsage(template).unsafeConditionKeys).toEqual([{ name: 'server-port', path: ['run', 'command', 0, 'if'] }]);
    expect(analyzeVariableUsage(template).undefinedConditionNames).toEqual([]);
  });

  it('knows built-ins, step outputs and shell variables', () => {
    expect(analyzeVariableUsage(template).undefinedTokens).toEqual([{ name: 'missing', path: ['run', 'command', 1, 'command'] }]);
    // ${exitCode} outside run.post is nothing the node fills in
    expect(analyzeVariableUsage({ install: [{ type: 'console', message: '${exitCode}' }] }).undefinedTokens.map((t) => t.name)).toEqual(['exitCode']);
    // a step output becomes a variable of the server, which later conditions read too
    expect(analyzeVariableUsage({ install: [{ type: 'githubdl', repository: 'r', outputVariable: 'asset' }, { type: 'mkdir', target: 'a', if: "asset != ''" }] }).undefinedConditionNames).toEqual([]);
  });

  it('reads tokens in conditions, where os and arch are known', () => {
    const usage = analyzeVariableUsage({ data: { javaversion: variable() }, install: [{ type: 'mkdir', target: 'a', if: "in_path('java${javaversion}') && '${os}' == 'linux' && ${nope} == 1" }] });
    expect(usage.references.get('javaversion')?.map((r) => r.kind)).toEqual(['token']);
    expect(usage.undefinedTokens.map((t) => t.name)).toEqual(['nope']);
    expect(usage.undefinedConditionNames).toEqual([]);
  });

  it('caches the analysis per template object', () => {
    expect(analyzeVariableUsage(template)).toBe(analyzeVariableUsage(template));
    expect(analyzeVariableUsage({ ...template })).not.toBe(analyzeVariableUsage(template));
  });
});

describe('variable warnings', () => {
  it('warn about tokens and condition names that name no variable', () => {
    const template = { data: { port: variable() }, run: { command: 'run ${port} ${prot}' }, install: [{ type: 'mkdir', target: 'a', if: 'enabled' }] };
    expect(undefinedVariables(template)).toEqual([{ path: ['run', 'command'], code: 'undefinedVariable', params: { token: '${prot}' } }]);
    expect(undefinedConditionVariables(template)).toEqual([{ path: ['install', 0, 'if'], code: 'undefinedConditionVariable', params: { name: 'enabled' } }]);
  });

  it('warn about variables nothing uses, except the ones the panel reads', () => {
    const template = {
      data: {
        memory: variable(),
        ip: variable(),
        javaversion: variable(),
        version: variable(),
        unused: variable({ internal: true }),
        used: variable(),
        cond: variable(),
      },
      run: { command: '${used}' },
      install: [{ type: 'mkdir', target: 'a', if: 'cond' }],
    };
    expect(unusedVariables(template)).toEqual([{ path: ['data', 'unused'], code: 'unusedVariable' }]);
  });

  it('warn when a condition names a key it cannot read', () => {
    const template = { data: { 'level-name': variable(), '2fa': variable() }, install: [{ type: 'mkdir', target: 'a', if: "level-name != '' || 2fa" }] };
    expect(conditionVariableKeys(template).map((w) => w.params?.name)).toEqual(['level-name', '2fa']);
    // the token form works in a condition
    expect(conditionVariableKeys({ data: { 'level-name': variable() }, install: [{ type: 'mkdir', target: 'a', if: "${level-name} != ''" }] })).toEqual([]);
  });

  it('show up in the validation under the place they are about', () => {
    const issues = validateTemplate({ name: 't', environment: { type: 'host' }, data: { a: variable() }, run: { command: '${b}' } }, { takenNames: [] });
    expect(issues.map((i) => `${i.path} ${i.messageKey}`)).toEqual([
      'run.command templates.editor.warnings.undefinedVariable',
      'data.a templates.editor.warnings.unusedVariable',
    ]);
  });

  it('find no unknown names in the community templates', () => {
    for (const file of readdirSync(FIXTURES).filter((f) => f.endsWith('.json'))) {
      const template = JSON.parse(readFileSync(new URL(file, FIXTURES), 'utf8')) as JsonObject;
      const found = [...undefinedVariables(template), ...undefinedConditionVariables(template), ...conditionVariableKeys(template)];
      expect(found, file).toEqual([]);
    }
  });
});

describe('renaming a variable', () => {
  const template: JsonObject = {
    data: { a: variable(), port: variable(), z: variable() },
    groups: [{ display: 'Net', variables: ['port', 'a'] }],
    install: [{ type: 'command', commands: ['echo ${port} ${port}', 'echo ${portx} ${ports}'], if: "port != '' && 'port' != sport && x.port == 1" }],
    run: { command: 'run ${port}', environmentVars: { PORT: '${port}' } },
  };

  it('renames the tokens and condition names, nothing else', () => {
    const { template: next, count } = renameVariableUsages(template, 'port', 'serverPort');
    expect(count).toBe(4);
    expect((next.install as JsonObject[])[0]).toEqual({
      type: 'command',
      commands: ['echo ${serverPort} ${serverPort}', 'echo ${portx} ${ports}'],
      if: "serverPort != '' && 'port' != sport && x.port == 1",
    });
    expect(next.run).toEqual({ command: 'run ${serverPort}', environmentVars: { PORT: '${serverPort}' } });
    // untouched branches keep their identity
    expect(next.data).toBe(template.data);
  });

  it('gives the template itself back when nothing uses the name', () => {
    expect(renameVariableUsages(template, 'z', 'y')).toEqual({ template, count: 0 });
  });

  it('renames a hyphenated key whole in conditions', () => {
    const t = { install: [{ type: 'mkdir', target: '${level-name}', if: "level-name != '' && name != ''" }] };
    expect(renameVariableUsages(t, 'level-name', 'levelName').template).toEqual({ install: [{ type: 'mkdir', target: '${levelName}', if: "levelName != '' && name != ''" }] });
    expect(renameVariableUsages(t, 'name', 'title').template.install).toEqual([{ type: 'mkdir', target: '${level-name}', if: "level-name != '' && title != ''" }]);
  });

  it('leaves conditions alone when the new key has a hyphen, which a condition cannot read', () => {
    const t = { install: [{ type: 'mkdir', target: '${level}', if: "level != '' && '${level}' != 'x'" }] };
    const { template: next, count } = renameVariableUsages(t, 'level', 'level-name');
    expect(next.install).toEqual([{ type: 'mkdir', target: '${level-name}', if: "level != '' && '${level-name}' != 'x'" }]);
    expect(count).toBe(2);
    // the bare name left behind is reported, so the author sees what to fix
    expect(undefinedConditionVariables({ data: { 'level-name': variable() }, ...next }).map((w) => w.params?.name)).toEqual(['level']);
  });

  it('keeps the key in place and follows it in the groups', () => {
    const next = renameVariable(template, 'port', 'serverPort', false);
    expect(Object.keys(next.data as JsonObject)).toEqual(['a', 'serverPort', 'z']);
    expect(next.groups).toEqual([{ display: 'Net', variables: ['serverPort', 'a'] }]);
    expect(next.run).toBe(template.run);
    expect((renameVariable(template, 'port', 'serverPort', true).run as JsonObject).command).toBe('run ${serverPort}');
  });

  it('refuses a taken or missing key', () => {
    expect(renameVariable(template, 'port', 'a', true)).toBe(template);
    expect(renameVariable(template, 'nope', 'b', true)).toBe(template);
  });

  it('takes a deleted variable out of its groups', () => {
    const next = deleteVariable(template, 'port');
    expect(Object.keys(next.data as JsonObject)).toEqual(['a', 'z']);
    expect(next.groups).toEqual([{ display: 'Net', variables: ['a'] }]);
    expect(deleteVariable(template, 'nope')).toBe(template);
  });

  it('counts the references of each variable', () => {
    expect(referenceCount(analyzeVariableUsage(template), 'port')).toBe(5);
    expect(referenceCount(analyzeVariableUsage(template), 'z')).toBe(0);
  });
});

describe('variable model', () => {
  it('makes unique, valid keys', () => {
    expect(uniqueVariableKey('variable', [])).toBe('variable');
    expect(uniqueVariableKey('port_copy', ['port_copy', 'port_copy2'])).toBe('port_copy3');
    expect(uniqueVariableKey('bad key!', [])).toBe('badkey');
    expect(uniqueVariableKey('!!', [])).toBe('variable');
  });

  it('inserts a variable last or after another one', () => {
    const t = { data: { a: variable(), b: variable() } };
    expect(Object.keys(insertVariable(t, 'c', newVariable()).data as JsonObject)).toEqual(['a', 'b', 'c']);
    expect(Object.keys(insertVariable(t, 'c', newVariable(), 'a').data as JsonObject)).toEqual(['a', 'c', 'b']);
    expect(insertVariable(t, 'a', newVariable())).toBe(t);
    expect(insertVariable({ data: null }, 'a', newVariable()).data).toEqual({ a: newVariable() });
  });

  it('converts the value when the type changes, keeping the other fields', () => {
    const v = { type: 'string', value: '25', options: [{ value: 'x' }], custom: 1 };
    expect(withVariableType(v, 'integer')).toEqual({ ...v, type: 'integer', value: 25 });
    expect(withVariableType(v, 'boolean')).toEqual({ ...v, type: 'boolean', value: false });
    expect(withVariableType({ type: 'string', value: 'True' }, 'boolean').value).toBe(true);
    expect(withVariableType({ type: 'integer', value: 7 }, 'string').value).toBe('7');
    expect(withVariableType({ type: 'string', value: 'abc' }, 'integer').value).toBe('abc');
    expect(withVariableType({ type: 'string' }, 'option').value).toBe('');
  });

  it('reads integers and booleans the way the node does', () => {
    expect(integerFromText('25565')).toBe(25565);
    expect(integerFromText('-3')).toBe(-3);
    expect(integerFromText('007')).toBe('007');
    expect(integerFromText('-')).toBe('-');
    expect(integerFromText('')).toBe('');
    expect(integerFromText('99999999999999999999')).toBe('99999999999999999999');
    expect(booleanValue('F')).toBe(false);
    expect(booleanValue(1)).toBe(true);
    expect(booleanValue('yes')).toBeUndefined();
  });

  it('shows a list whenever there are options, whatever the type', () => {
    expect(usesOptions({ type: 'option' })).toBe(true);
    expect(usesOptions({ type: 'string', options: [{ value: 'a' }] })).toBe(true);
    expect(usesOptions({ type: 'string', options: [] })).toBe(false);
    expect(usesOptions({ type: 'boolean', options: null })).toBe(false);
  });

  it('knows the keys JavaScript always lists first', () => {
    for (const key of ['0', '1', '25', '4294967294']) expect(isPinnedKey(key), key).toBe(true);
    for (const key of ['01', '-1', '1a', 'port', '4294967295', '']) expect(isPinnedKey(key), key).toBe(false);
    expect(Object.keys({ port: 1, 25: 2 })).toEqual(['25', 'port']);
  });

  it('keeps an option value a number while it is one', () => {
    expect(optionValueFromText('16', 8)).toBe(16);
    expect(optionValueFromText('16a', 8)).toBe('16a');
    expect(optionValueFromText('16', '8')).toBe('16');
    expect(sameOptionValue(1, '1')).toBe(true);
    expect(sameOptionValue('', null)).toBe(false);
    expect(newOption([{ value: 'option2' }])).toEqual({ value: 'option3', display: '' });
  });
});
