// The project's rule for Docker images in templates: only its own images (ghcr.io/aether-panel/*)
// or official ones: Docker Hub's official images (ubuntu, eclipse-temurin, mariadb…) and the
// images a vendor publishes for its own software (Microsoft's .NET, Valve's SteamCMD, Bun). Any
// other image can change or disappear without notice; a package a template needs goes into our
// images instead.

/** Where the project publishes its own runtime images. */
export const PROJECT_IMAGE_PREFIX = 'ghcr.io/aether-panel/';

/** What the node runs when a Docker environment names no image (internal/servers/docker/factory.go). */
export const DEFAULT_DOCKER_IMAGE = 'ghcr.io/aether-panel/generic';

/** The names Docker Hub goes by, which an image reference may spell out. */
const DOCKER_HUB = new Set(['docker.io', 'index.docker.io', 'registry-1.docker.io']);

/** Registries run by a vendor for its own images: everything there is official. */
const VENDOR_REGISTRIES = new Set(['mcr.microsoft.com']);

/** Docker Hub repositories a vendor publishes its own software in (namespace/name). */
const VENDOR_REPOSITORIES = new Set(['steamcmd/steamcmd', 'oven/bun']);

export type ImageSource = 'project' | 'official' | 'thirdParty' | 'unknown';

/**
 * Where an image reference comes from:
 * - project: ghcr.io/aether-panel/…;
 * - official: a Docker Hub official image, written without a namespace (`ubuntu:24.04`,
 *   `eclipse-temurin:${javaversion}`) or under `library/`, with or without docker.io; or a
 *   vendor's own image (VENDOR_REGISTRIES, VENDOR_REPOSITORIES);
 * - thirdParty: anything else (a user's namespace on Docker Hub, another registry, another
 *   account on ghcr.io);
 * - unknown: no image, or a repository a variable decides (`${image}`), which cannot be judged.
 * The tag and digest do not matter, so a variable there is fine.
 */
export function imageSource(reference: unknown): ImageSource {
  if (typeof reference !== 'string') return 'unknown';
  const image = reference.trim().toLowerCase();
  if (image === '') return 'unknown';
  // the repository: without the digest, and without the tag after the last path component
  const withoutDigest = image.split('@')[0];
  const parts = withoutDigest.split('/');
  parts[parts.length - 1] = parts[parts.length - 1].split(':')[0];
  const repository = parts.join('/');
  if (repository.includes('${')) return 'unknown';
  if (`${repository}/`.startsWith(PROJECT_IMAGE_PREFIX) && repository.length > PROJECT_IMAGE_PREFIX.length) return 'project';

  // a first component with a dot or a port, or localhost, names a registry
  const first = parts[0];
  const hasRegistry = parts.length > 1 && (first.includes('.') || first.includes(':') || first === 'localhost');
  if (hasRegistry && VENDOR_REGISTRIES.has(first) && parts.length > 1 && parts[parts.length - 1] !== '') return 'official';
  if (hasRegistry && !DOCKER_HUB.has(first)) return 'thirdParty';
  const path = hasRegistry ? parts.slice(1) : parts;
  if (VENDOR_REPOSITORIES.has(path.join('/'))) return 'official';
  if (path.length === 1 && path[0] !== '') return 'official';
  if (path.length === 2 && path[0] === 'library' && path[1] !== '') return 'official';
  return 'thirdParty';
}
