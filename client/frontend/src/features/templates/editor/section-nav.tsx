import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { AlertCircle, AlertTriangle } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { useTemplateEditor } from './editor-context';
import type { JsonObject } from './paths';
import { EDITOR_SECTIONS, type EditorSection, type EditorSectionId } from './sections';
import type { IssueCounts } from './validation';

/** "2 errors, 1 warning" for screen readers; the badges show the numbers with their icons. */
export function issueCountsLabel(t: (key: string, options?: Record<string, string | number>) => string, { errors, warnings }: IssueCounts): string {
  const parts: string[] = [];
  if (errors) parts.push(errors === 1 ? t('templates.editor.issues.errorsOne') : t('templates.editor.issues.errorsMany', { count: errors }));
  if (warnings) parts.push(warnings === 1 ? t('templates.editor.issues.warningsOne') : t('templates.editor.issues.warningsMany', { count: warnings }));
  return parts.join(', ');
}

/** The errors and warnings of a section as two small counts, each with its icon. */
export function IssueCountBadges({ counts, className }: Readonly<{ counts: IssueCounts; className?: string }>) {
  const { t } = useTranslations();
  if (!counts.errors && !counts.warnings) return null;
  return (
    <span className={cn('flex shrink-0 items-center gap-1', className)}>
      <span className="sr-only">{issueCountsLabel(t, counts)}</span>
      {counts.errors > 0 && (
        <span aria-hidden="true" className="inline-flex items-center gap-0.5 rounded-md bg-destructive/15 px-1.5 py-px text-[11px] font-medium tabular-nums text-destructive-bright">
          <AlertCircle className="h-3 w-3" />
          {counts.errors}
        </span>
      )}
      {counts.warnings > 0 && (
        <span aria-hidden="true" className="inline-flex items-center gap-0.5 rounded-md bg-warning/15 px-1.5 py-px text-[11px] font-medium tabular-nums text-warning">
          <AlertTriangle className="h-3 w-3" />
          {counts.warnings}
        </span>
      )}
    </span>
  );
}

/** How many things a section holds, in muted figures; screen readers get it in words. */
function ItemCount({ section, template }: Readonly<{ section: EditorSection; template: JsonObject }>) {
  const { t } = useTranslations();
  const count = section.itemCount?.(template) ?? 0;
  if (!count) return null;
  return (
    <span className="shrink-0 text-xs tabular-nums text-muted-foreground">
      <span aria-hidden="true">{count}</span>
      <span className="sr-only">{t(`templates.editor.sections.${section.id}.countLabel`, { count })}</span>
    </span>
  );
}

/** The section menu on wide screens: one button per section, with its counts. */
export function SectionNav({ className }: Readonly<{ className?: string }>) {
  const { t } = useTranslations();
  const { section: active, goTo, issues, template } = useTemplateEditor();
  return (
    <nav aria-label={t('templates.editor.sections.navLabel')} className={className}>
      <ul className="space-y-0.5">
        {EDITOR_SECTIONS.map((section) => {
          const isActive = section.id === active;
          return (
            <li key={section.id}>
              <button
                type="button"
                onClick={() => goTo(section.id)}
                aria-current={isActive ? 'true' : undefined}
                className={cn(
                  'flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70',
                  isActive ? 'bg-accent font-medium text-foreground' : 'text-muted-foreground hover:bg-accent/60 hover:text-foreground',
                )}
              >
                <section.icon className={cn('h-4 w-4 shrink-0', isActive && 'text-primary-bright')} aria-hidden="true" />
                <span className="min-w-0 flex-1 truncate">{t(section.labelKey)}</span>
                <ItemCount section={section} template={template} />
                <IssueCountBadges counts={section.counts(issues)} />
              </button>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}

/** The section menu on narrow screens (and next to the JSON in split view): a select. */
export function SectionSelect({ className }: Readonly<{ className?: string }>) {
  const { t } = useTranslations();
  const { section: active, goTo, issues, template } = useTemplateEditor();
  return (
    <div className={className}>
      <Select value={active} onValueChange={(value) => goTo(value as EditorSectionId)}>
        <SelectTrigger aria-label={t('templates.editor.sections.selectLabel')}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {EDITOR_SECTIONS.map((section) => (
            <SelectItem key={section.id} value={section.id}>
              <span className="flex items-center gap-2">
                <section.icon className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                {t(section.labelKey)}
                <ItemCount section={section} template={template} />
                <IssueCountBadges counts={section.counts(issues)} />
              </span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
