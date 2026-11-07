import type { z } from 'zod';

// Turns a zod issue from the template schema into an i18n key, so the form and the summary show
// it translated. The schemas write their own messages as keys (common.ts msg()); issues zod
// raises by itself (a wrong JSON type deep in a loose object, an unknown step type) are mapped
// by their code.

const PREFIX = 'templates.editor.validation.';

export function issueMessageKey(issue: z.core.$ZodIssue): string {
  if (issue.message.startsWith(PREFIX)) return issue.message;
  switch (issue.code) {
    case 'invalid_union':
      // a step whose type is no operation the node knows
      return 'discriminator' in issue && issue.discriminator === 'type' ? `${PREFIX}unknownOperation` : `${PREFIX}invalidValue`;
    case 'invalid_key':
      return `${PREFIX}invalidVariableKey`;
    case 'invalid_type':
      return `${PREFIX}invalidType`;
    case 'invalid_value':
      return `${PREFIX}invalidOption`;
    case 'too_small':
    case 'too_big':
      return `${PREFIX}outOfRange`;
    default:
      return `${PREFIX}invalidValue`;
  }
}

/** Where an issue is, as the dotted path the form names its fields by ("run.pre.2.target"). */
export function issuePath(issue: z.core.$ZodIssue): string {
  return issue.path.map(String).join('.');
}
