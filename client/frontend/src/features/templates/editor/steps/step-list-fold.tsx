import { useLayoutEffect } from 'react';
import { ChevronRight } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { useTemplateEditor } from '../editor-context';
import { pathKey, type Path } from '../paths';
import { IssueCountBadges } from '../section-nav';
import { countIssues, isUnderPath } from '../validation';
import { stepsAt } from './model';
import { StepList } from './step-list';
import { useLatchedOpen } from './use-latched-open';

// A list of steps most templates do without (uninstall, run.pre, run.post), folded under a
// heading with its step count and issues. It is folded unless it has steps or something to fix;
// then it stays open until the user closes it, so deleting the last step keeps it open and the
// focus can go to its "Añadir paso".

export function StepListFold({ path, title, note, description, emptyTitle, emptyDescription }: Readonly<{
  path: Path;
  title: string;
  /** A word after the title, such as "(opcional)". */
  note?: string;
  description: string;
  emptyTitle: string;
  emptyDescription: string;
}>) {
  const { t } = useTranslations();
  const { template, issues, focusRequest } = useTemplateEditor();
  const key = pathKey(path);
  const count = stepsAt(template, path).length;
  const counts = countIssues(issues.filter((i) => isUnderPath(i.path, key)));
  const { isOpen, toggle, open } = useLatchedOpen(count > 0 || counts.errors + counts.warnings > 0);
  const toggleId = `template-${key.replaceAll('.', '-')}-toggle`;
  const bodyId = `template-${key.replaceAll('.', '-')}-body`;

  // goTo() on one of its steps opens the fold before the step's card opens and the field is focused
  useLayoutEffect(() => {
    const target = focusRequest?.path;
    if (target && target.length >= path.length && isUnderPath(pathKey(target), key)) open();
  }, [focusRequest, open, key, path.length]);

  return (
    <section aria-labelledby={toggleId}>
      <button
        id={toggleId}
        type="button"
        aria-expanded={isOpen}
        aria-controls={isOpen ? bodyId : undefined}
        onClick={toggle}
        className="flex w-full items-start gap-3 px-5 py-4 text-left transition-colors duration-150 hover:bg-accent/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/70 md:px-6"
      >
        <ChevronRight
          className={cn('mt-0.5 h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200 ease-panel motion-reduce:transition-none', isOpen && 'rotate-90')}
          aria-hidden="true"
        />
        <span className="min-w-0 flex-1 space-y-1">
          <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <span className="text-sm font-semibold text-foreground">{title}</span>
            {note && <span className="text-xs text-muted-foreground">{note}</span>}
            {count > 0 && (
              <span className="text-xs tabular-nums text-muted-foreground">
                · {count === 1 ? t('templates.editor.steps.countOne') : t('templates.editor.steps.countMany', { count })}
              </span>
            )}
          </span>
          <span className="block text-pretty text-sm text-muted-foreground">{description}</span>
        </span>
        <IssueCountBadges counts={counts} className="mt-0.5" />
      </button>
      {isOpen && (
        <div id={bodyId} className="px-3 pb-4 sm:px-5 sm:pb-5 md:px-6 md:pb-6">
          <StepList path={path} label={title} emptyTitle={emptyTitle} emptyDescription={emptyDescription} />
        </div>
      )}
    </section>
  );
}
