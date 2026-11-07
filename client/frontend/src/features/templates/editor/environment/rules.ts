import { isPlainObject, type Path } from '../paths';
import type { TemplateWarning, WarningRule } from '../warnings';
import { DEFAULT_DOCKER_IMAGE, imageSource } from './images';

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

/** What create-server-stepper's serverEnvironment uses when no supported environment is Docker. */
export const STEPPER_FALLBACK_IMAGE = 'ubuntu:22.04';

/**
 * No supported environment is Docker. The panel's "Crear servidor" form always creates servers in
 * Docker, with the first Docker entry of supportedEnvironments (create-server-stepper.tsx,
 * serverEnvironment); without one it sends ubuntu:22.04, and without the list at all a bare
 * Docker environment, which the node runs with its default image. Neither is likely to have
 * what the template needs.
 */
export const dockerEnvironment: WarningRule = (template) => {
  const supported = template.supportedEnvironments;
  if (Array.isArray(supported) && supported.some((e) => isPlainObject(e) && e.type === 'docker')) return [];
  const image = Array.isArray(supported) ? STEPPER_FALLBACK_IMAGE : DEFAULT_DOCKER_IMAGE;
  return [{ path: ['supportedEnvironments'], code: 'noDockerEnvironment', params: { image } }];
};

export const ENVIRONMENT_WARNING_RULES: readonly WarningRule[] = [thirdPartyImages, dockerEnvironment];
