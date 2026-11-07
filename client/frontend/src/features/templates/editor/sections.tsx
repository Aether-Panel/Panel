import type { ComponentType } from 'react';
import { Download, Info, ListChecks, Package, Play, Settings2, SlidersHorizontal, type AppIcon } from '@/components/icons';
import { AdvancedSection } from './advanced/advanced-section';
import { EnvironmentSection } from './environment/environment-section';
import { InfoSection } from './info-section';
import type { JsonObject } from './paths';
import { RequirementsSection } from './requirements/requirements-section';
import { RunSection } from './run/run-section';
import { InstallSection } from './steps/install-section';
import { stepsAt } from './steps/model';
import { countIssues, isUnderPath, type EditorIssue, type IssueCounts } from './validation';
import { templateVariables } from './variables/usage';
import { VariablesSection } from './variables/variables-section';

// The editor's sections, in menu order. A section is a component that edits its part of the
// template through useTemplateEditor(); the page draws the frame (title, description, counts)
// around it. To fill a section, swap its component; its texts are
// templates.editor.sections.<id>.label|description (and .countLabelOne|countLabelMany with an item count).

export type EditorSectionId = 'info' | 'variables' | 'install' | 'run' | 'environment' | 'requirements' | 'advanced';

export interface EditorSection {
  id: EditorSectionId;
  labelKey: string;
  descriptionKey: string;
  icon: AppIcon;
  component: ComponentType<{ section: EditorSection }>;
  /** The errors and warnings the menu shows next to the section. */
  counts: (issues: readonly EditorIssue[]) => IssueCounts;
  /** How many things the section holds (variables, steps), shown in the menu; none when absent. */
  itemCount?: (template: JsonObject) => number;
}

/** Counts the issues under any of the template keys a section edits. */
function countUnder(...keys: string[]) {
  return (issues: readonly EditorIssue[]) => countIssues(issues.filter((i) => keys.some((key) => isUnderPath(i.path, key))));
}

function section(
  id: EditorSectionId,
  icon: AppIcon,
  component: EditorSection['component'],
  counts: EditorSection['counts'],
  itemCount?: EditorSection['itemCount'],
): EditorSection {
  return {
    id,
    labelKey: `templates.editor.sections.${id}.label`,
    descriptionKey: `templates.editor.sections.${id}.description`,
    icon,
    component,
    counts,
    itemCount,
  };
}

export const EDITOR_SECTIONS: readonly EditorSection[] = [
  // an issue about the template as a whole ("") is shown with its identity
  section('info', Info, InfoSection, (issues) => countIssues(issues.filter((i) => i.path === '' || ['name', 'display', 'type', 'icon'].some((key) => isUnderPath(i.path, key))))),
  section('variables', SlidersHorizontal, VariablesSection, countUnder('data'), (template) => Object.keys(templateVariables(template)).length),
  section('install', Download, InstallSection, countUnder('install', 'uninstall'), (template) => stepsAt(template, ['install']).length + stepsAt(template, ['uninstall']).length),
  section('run', Play, RunSection, countUnder('run')),
  section('environment', Package, EnvironmentSection, countUnder('environment', 'supportedEnvironments')),
  section('requirements', ListChecks, RequirementsSection, countUnder('requirements')),
  section('advanced', Settings2, AdvancedSection, countUnder('groups', 'stats', 'query', 'keepAlive')),
];

export function getSection(id: EditorSectionId): EditorSection {
  return EDITOR_SECTIONS.find((s) => s.id === id) ?? EDITOR_SECTIONS[0];
}
