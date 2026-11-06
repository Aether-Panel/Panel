import type { ComponentType } from 'react';
import { Download, Info, ListChecks, Package, Play, Settings2, SlidersHorizontal, type AppIcon } from '@/components/icons';
import { InfoSection } from './info-section';
import { PlaceholderSection } from './placeholder-section';
import { countIssues, isUnderPath, type EditorIssue, type IssueCounts } from './validation';

// The editor's sections, in menu order. A section is a component that edits its part of the
// template through useTemplateEditor(); the page draws the frame (title, description, counts)
// around it. To fill a section, swap its component; its texts are
// templates.editor.sections.<id>.label|description.

export type EditorSectionId = 'info' | 'variables' | 'install' | 'run' | 'environment' | 'requirements' | 'advanced';

export interface EditorSection {
  id: EditorSectionId;
  labelKey: string;
  descriptionKey: string;
  icon: AppIcon;
  component: ComponentType<{ section: EditorSection }>;
  /** The errors and warnings the menu shows next to the section. */
  counts: (issues: readonly EditorIssue[]) => IssueCounts;
}

/** Counts the issues under any of the template keys a section edits. */
function countUnder(...keys: string[]) {
  return (issues: readonly EditorIssue[]) => countIssues(issues.filter((i) => keys.some((key) => isUnderPath(i.path, key))));
}

function section(id: EditorSectionId, icon: AppIcon, component: EditorSection['component'], counts: EditorSection['counts']): EditorSection {
  return {
    id,
    labelKey: `templates.editor.sections.${id}.label`,
    descriptionKey: `templates.editor.sections.${id}.description`,
    icon,
    component,
    counts,
  };
}

export const EDITOR_SECTIONS: readonly EditorSection[] = [
  // an issue about the template as a whole ("") is shown with its identity
  section('info', Info, InfoSection, (issues) => countIssues(issues.filter((i) => i.path === '' || ['name', 'display', 'type', 'icon'].some((key) => isUnderPath(i.path, key))))),
  section('variables', SlidersHorizontal, PlaceholderSection, countUnder('data')),
  section('install', Download, PlaceholderSection, countUnder('install', 'uninstall')),
  section('run', Play, PlaceholderSection, countUnder('run')),
  section('environment', Package, PlaceholderSection, countUnder('environment', 'supportedEnvironments')),
  section('requirements', ListChecks, PlaceholderSection, countUnder('requirements')),
  section('advanced', Settings2, PlaceholderSection, countUnder('groups', 'stats', 'query', 'keepAlive')),
];

export function getSection(id: EditorSectionId): EditorSection {
  return EDITOR_SECTIONS.find((s) => s.id === id) ?? EDITOR_SECTIONS[0];
}
