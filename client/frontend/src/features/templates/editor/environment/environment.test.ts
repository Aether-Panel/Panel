import { readdirSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import type { JsonObject } from '../paths';
import { imageSource } from './images';
import { dockerEnvironment, STEPPER_FALLBACK_IMAGE, thirdPartyImages } from './rules';
import { DEFAULT_DOCKER_IMAGE } from './images';

const FIXTURES = new URL('../../schema/__fixtures__/community/', import.meta.url);
const community = readdirSync(FIXTURES)
  .filter((f) => f.endsWith('.json'))
  .map((f) => JSON.parse(readFileSync(new URL(f, FIXTURES), 'utf8')) as JsonObject);

describe('where a Docker image comes from', () => {
  it.each([
    ['ghcr.io/aether-panel/steamcmd', 'project'],
    ['ghcr.io/aether-panel/php:${phpversion}', 'project'],
    ['GHCR.IO/Aether-Panel/ubuntu:latest', 'project'],
    ['ubuntu', 'official'],
    ['ubuntu:24.04', 'official'],
    ['eclipse-temurin:${javaversion}', 'official'],
    ['python:${pyversion}-slim', 'official'],
    ['library/redis:7', 'official'],
    ['docker.io/library/mariadb:11', 'official'],
    ['docker.io/node:22', 'official'],
    ['postgres@sha256:abc', 'official'],
    // vendors' own images
    ['steamcmd/steamcmd', 'official'],
    ['steamcmd/steamcmd:ubuntu-20', 'official'],
    ['oven/bun:${bunversion}', 'official'],
    ['docker.io/oven/bun', 'official'],
    ['n8nio/n8n:${version}', 'official'],
    ['n8nio/other', 'thirdParty'],
    ['mcr.microsoft.com/dotnet/runtime:${dotnetversion}', 'official'],
    ['mcr.microsoft.com/dotnet/sdk:8.0', 'official'],
    ['someone/steamcmd', 'thirdParty'],
    ['oven/other', 'thirdParty'],
    ['steamcmd/steamcmd-fork', 'thirdParty'],
    ['ghcr.io/someone/image', 'thirdParty'],
    ['ghcr.io/steamcmd/steamcmd', 'thirdParty'],
    ['ghcr.io/aether-panel', 'thirdParty'],
    ['localhost:5000/game', 'thirdParty'],
    ['${image}', 'unknown'],
    ['${registry}/game:1', 'unknown'],
    ['', 'unknown'],
    ['   ', 'unknown'],
  ])('%s is %s', (image, source) => {
    expect(imageSource(image)).toBe(source);
  });

  it('cannot judge what is not text', () => {
    expect(imageSource(undefined)).toBe('unknown');
    expect(imageSource(3)).toBe('unknown');
  });
});

describe('the third-party image warning', () => {
  it('sits on the image of each Docker environment that uses one', () => {
    const template = {
      environment: { type: 'docker', image: 'someone/steamcmd' },
      supportedEnvironments: [
        { type: 'host' },
        { type: 'docker', image: 'ghcr.io/aether-panel/steamcmd' },
        { type: 'docker', image: 'someone/bun' },
        // only Docker reads the image
        { type: 'host', image: 'someone/bun' },
      ],
    };
    expect(thirdPartyImages(template)).toEqual([
      { path: ['environment', 'image'], code: 'thirdPartyImage' },
      { path: ['supportedEnvironments', 2, 'image'], code: 'thirdPartyImage' },
    ]);
  });

  it('finds no image in the community templates that breaks the rule', () => {
    for (const template of community) expect(thirdPartyImages(template)).toEqual([]);
  });
});

describe('the missing Docker environment warning', () => {
  it('names the image the create form will fall back to', () => {
    expect(dockerEnvironment({ supportedEnvironments: [{ type: 'docker', image: 'ubuntu' }] })).toEqual([]);
    expect(dockerEnvironment({ supportedEnvironments: [{ type: 'host' }, { type: 'docker' }] })).toEqual([]);
    // a list without Docker: create-server-stepper sends ubuntu:22.04
    expect(dockerEnvironment({ supportedEnvironments: [{ type: 'host' }] })).toEqual([
      { path: ['supportedEnvironments'], code: 'noDockerEnvironment', params: { image: STEPPER_FALLBACK_IMAGE } },
    ]);
    expect(dockerEnvironment({ supportedEnvironments: [] })[0].params).toEqual({ image: STEPPER_FALLBACK_IMAGE });
    // no list: a bare Docker environment, which the node runs with its default image
    expect(dockerEnvironment({ environment: { type: 'docker', image: 'ubuntu' } })[0].params).toEqual({ image: DEFAULT_DOCKER_IMAGE });
    expect(dockerEnvironment({ supportedEnvironments: null })[0].params).toEqual({ image: DEFAULT_DOCKER_IMAGE });
  });
});
