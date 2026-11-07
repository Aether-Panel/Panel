import { readdirSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import type { JsonObject } from '../paths';
import { imageSource } from './images';
import { thirdPartyImages } from './rules';

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
    ['steamcmd/steamcmd', 'thirdParty'],
    ['steamcmd/steamcmd:ubuntu-20', 'thirdParty'],
    ['oven/bun:${bunversion}', 'thirdParty'],
    ['docker.io/oven/bun', 'thirdParty'],
    ['mcr.microsoft.com/dotnet/runtime:${dotnetversion}', 'thirdParty'],
    ['ghcr.io/someone/image', 'thirdParty'],
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
      environment: { type: 'docker', image: 'steamcmd/steamcmd' },
      supportedEnvironments: [
        { type: 'host' },
        { type: 'docker', image: 'ghcr.io/aether-panel/steamcmd' },
        { type: 'docker', image: 'oven/bun' },
        // only Docker reads the image
        { type: 'host', image: 'oven/bun' },
      ],
    };
    expect(thirdPartyImages(template)).toEqual([
      { path: ['environment', 'image'], code: 'thirdPartyImage' },
      { path: ['supportedEnvironments', 2, 'image'], code: 'thirdPartyImage' },
    ]);
  });

  it('flags exactly the community images that break the rule', () => {
    const flagged = new Set<string>();
    for (const template of community) {
      for (const warning of thirdPartyImages(template)) {
        const [root, index] = warning.path;
        const environment = root === 'environment' ? template.environment : (template.supportedEnvironments as JsonObject[])[index as number];
        flagged.add(String((environment as JsonObject).image));
      }
    }
    expect([...flagged].sort()).toEqual([
      'mcr.microsoft.com/dotnet/runtime:${dotnetversion}',
      'mcr.microsoft.com/dotnet/sdk:${dotnetversion}',
      'oven/bun:${bunversion}',
      'steamcmd/steamcmd',
      'steamcmd/steamcmd:ubuntu-20',
    ]);
  });
});
