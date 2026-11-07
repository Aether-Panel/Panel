import { ADVANCED_WARNING_RULES } from './advanced/model';
import { ENVIRONMENT_WARNING_RULES } from './environment/rules';
import { isPlainObject, type JsonObject, type Path } from './paths';
import { RUN_WARNING_RULES } from './run/rules';
import { VARIABLE_WARNING_RULES } from './variables/rules';

// Warnings: what the node accepts but probably not as the author meant. The template schema
// only has errors (what the node rejects or fails on), so these live here, apart from it, and
// never block saving. Each rule reads the raw template and returns what it finds; a new check is
// one more function in WARNING_RULES.

/** A warning found by a rule; its message is templates.editor.warnings.<code>. */
export interface TemplateWarning {
  path: Path;
  code: string;
  params?: Record<string, string | number>;
}

export type WarningRule = (template: JsonObject) => TemplateWarning[];

/**
 * An integer variable whose value the node cannot read as a whole number. Variable.UnmarshalJSON
 * casts it with cast.ToIntE and, when that fails, stores 0 without an error; a number with
 * decimals is cut down instead. Empty is fine: it means "no default", which is 0 as well.
 */
export const integerVariableValue: WarningRule = (template) => {
  const variables = template.data;
  if (!isPlainObject(variables)) return [];
  return Object.entries(variables).flatMap(([key, variable]) => {
    if (!isPlainObject(variable) || variable.type !== 'integer') return [];
    const value = variable.value;
    const whole =
      value === undefined ||
      value === null ||
      value === '' ||
      (typeof value === 'number' && Number.isInteger(value)) ||
      (typeof value === 'string' && /^[+-]?\d+$/.test(value));
    return whole ? [] : [{ path: ['data', key, 'value'], code: 'integerVariableValue' }];
  });
};

/**
 * No default environment. The node writes a template saved without one back as { "type": "" },
 * which loads. The panel's "Crear servidor" form does not read it (it always sends a Docker
 * environment), but a server created from the template as it is, through the API, is refused
 * without one (Server.valid).
 */
export const defaultEnvironment: WarningRule = (template) => {
  const environment = template.environment;
  const type = isPlainObject(environment) ? environment.type : undefined;
  return typeof type === 'string' && type !== '' ? [] : [{ path: ['environment', 'type'], code: 'noDefaultEnvironment' }];
};

export const WARNING_RULES: readonly WarningRule[] = [
  integerVariableValue,
  defaultEnvironment,
  ...VARIABLE_WARNING_RULES,
  ...RUN_WARNING_RULES,
  ...ENVIRONMENT_WARNING_RULES,
  ...ADVANCED_WARNING_RULES,
];

/** Every warning the rules find in `template`, in rule order. */
export function templateWarnings(template: JsonObject, rules: readonly WarningRule[] = WARNING_RULES): TemplateWarning[] {
  return rules.flatMap((rule) => rule(template));
}
