import { isPlainObject, type Path } from '../paths';
import type { TemplateWarning, WarningRule } from '../warnings';
import { imageSource } from './images';

// Warnings about where the server runs (see warnings.ts).

function imageWarning(environment: unknown, path: Path): TemplateWarning[] {
  // only Docker reads the image: a host environment that keeps one ignores it
  if (!isPlainObject(environment) || environment.type !== 'docker') return [];
  return imageSource(environment.image) === 'thirdParty' ? [{ path: [...path, 'image'], code: 'thirdPartyImage' }] : [];
}

/** A Docker image that is neither the project's own nor an official one (see images.ts). */
export const thirdPartyImages: WarningRule = (template) => {
  const supported = Array.isArray(template.supportedEnvironments) ? template.supportedEnvironments : [];
  return [
    ...imageWarning(template.environment, ['environment']),
    ...supported.flatMap((environment, index) => imageWarning(environment, ['supportedEnvironments', index])),
  ];
};

export const ENVIRONMENT_WARNING_RULES: readonly WarningRule[] = [thirdPartyImages];
