import { isUnderPath } from './validation';

// Which part of the template each section of the editor edits, by top-level key. The section
// menu counts issues with it and the validation summary files each issue under its section, so
// both always agree. Kept apart from sections.tsx (the components) so pure code can use it.

export type EditorSectionId = 'info' | 'variables' | 'install' | 'run' | 'environment' | 'requirements' | 'advanced';

export const SECTION_KEYS: Readonly<Record<EditorSectionId, readonly string[]>> = {
  info: ['name', 'display', 'type', 'icon'],
  variables: ['data'],
  install: ['install', 'uninstall'],
  run: ['run'],
  environment: ['environment', 'supportedEnvironments'],
  requirements: ['requirements'],
  advanced: ['groups', 'stats', 'query', 'keepAlive'],
};

/** Whether the issue at `path` (dotted) belongs to `section`; the whole template ("") is Información's. */
export function sectionHolds(section: EditorSectionId, path: string): boolean {
  if (path === '') return section === 'info';
  return SECTION_KEYS[section].some((key) => isUnderPath(path, key));
}

/** The section that edits `path`, or null for a key no section edits (the JSON view has it). */
export function sectionOfPath(path: string): EditorSectionId | null {
  for (const id of Object.keys(SECTION_KEYS) as EditorSectionId[]) {
    if (sectionHolds(id, path)) return id;
  }
  return null;
}
