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
import { sectionHolds, type EditorSectionId } from './section-keys';
import { countIssues, type EditorIssue, type IssueCounts } from './validation';
import { templateVariables } from './variables/usage';
import { VariablesSection } from './variables/variables-section';

// The editor's sections, in menu order. A section is a component that edits its part of the
// template through useTemplateEditor(); the page draws the frame (title, description, counts)
// around it. To fill a section, swap its component; its texts are
// templates.editor.sections.<id>.label|description (and .countLabelOne|countLabelMany with an item count).

export type { EditorSectionId } from './section-keys';

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

/** Counts the issues under the template keys a section edits (section-keys.ts). */
function countFor(id: EditorSectionId) {
  return (issues: readonly EditorIssue[]) => countIssues(issues.filter((i) => sectionHolds(id, i.path)));
}

function section(
  id: EditorSectionId,
  icon: AppIcon,
  component: EditorSection['component'],
  itemCount?: EditorSection['itemCount'],
): EditorSection {
  return {
    id,
    labelKey: `templates.editor.sections.${id}.label`,
    descriptionKey: `templates.editor.sections.${id}.description`,
    icon,
    component,
    counts: countFor(id),
    itemCount,
  };
}

export const EDITOR_SECTIONS: readonly EditorSection[] = [
  section('info', Info, InfoSection),
  section('variables', SlidersHorizontal, VariablesSection, (template) => Object.keys(templateVariables(template)).length),
  section('install', Download, InstallSection, (template) => stepsAt(template, ['install']).length + stepsAt(template, ['uninstall']).length),
  section('run', Play, RunSection),
  section('environment', Package, EnvironmentSection),
  section('requirements', ListChecks, RequirementsSection),
  section('advanced', Settings2, AdvancedSection),
];

export function getSection(id: EditorSectionId): EditorSection {
  return EDITOR_SECTIONS.find((s) => s.id === id) ?? EDITOR_SECTIONS[0];
}
