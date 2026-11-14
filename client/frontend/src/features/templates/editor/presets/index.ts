import { dockerGeneric } from './docker-generic';
import { minecraftPaper } from './minecraft-paper';
import { nodejsApp } from './nodejs-app';
import type { TemplatePreset } from './preset';
import { pythonApp } from './python-app';
import { steamGame } from './steam-game';

export type { PresetText, TemplatePreset } from './preset';

/** The base templates, in the order the chooser shows them. */
export const TEMPLATE_PRESETS: readonly TemplatePreset[] = [minecraftPaper, steamGame, nodejsApp, pythonApp, dockerGeneric];

export function getPreset(id: string): TemplatePreset | undefined {
  return TEMPLATE_PRESETS.find((preset) => preset.id === id);
}

/** The editor opened on a base template. */
export function presetHref(id: string): string {
  return `/templates/editor/?${new URLSearchParams({ preset: id })}`;
}
