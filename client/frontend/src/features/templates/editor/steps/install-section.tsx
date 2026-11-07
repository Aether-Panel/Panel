import { useLayoutEffect, useState } from 'react';
import { ChevronRight } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { useTemplateEditor } from '../editor-context';
import { IssueCountBadges } from '../section-nav';
import { countIssues, isUnderPath } from '../validation';
import { stepsAt } from './model';
import { StepList } from './step-list';

// Instalación: the steps that set up a server's files when it is installed (or reinstalled), and,
// folded below, the optional ones that clean up when it is uninstalled.

export function InstallSection() {
  const { t } = useTranslations();
  const { template, issues, focusRequest } = useTemplateEditor();
  const uninstallCount = stepsAt(template, ['uninstall']).length;
  const uninstallIssues = countIssues(issues.filter((i) => isUnderPath(i.path, 'uninstall')));
  // folded unless it has steps or something to fix; the user's own choice wins after that
  const [uninstallChoice, setUninstallChoice] = useState<boolean | null>(null);
  const uninstallOpen = uninstallChoice ?? (uninstallCount > 0 || uninstallIssues.errors + uninstallIssues.warnings > 0);

  // goTo() on an uninstall step opens the fold before its card opens and the field is focused
  useLayoutEffect(() => {
    if (focusRequest?.path[0] === 'uninstall') setUninstallChoice(true);
  }, [focusRequest]);

  return (
    <div className="divide-y divide-border/50">
      <div className="space-y-3 px-3 py-4 sm:p-5 md:p-6">
        <StepList
          path={['install']}
          label={t('templates.editor.install.title')}
          emptyTitle={t('templates.editor.install.empty.title')}
          emptyDescription={t('templates.editor.install.empty.description')}
        />
      </div>
      <section aria-labelledby="template-uninstall-toggle">
        <button
          id="template-uninstall-toggle"
          type="button"
          aria-expanded={uninstallOpen}
          aria-controls={uninstallOpen ? 'template-uninstall-body' : undefined}
          onClick={() => setUninstallChoice(!uninstallOpen)}
          className="flex w-full items-start gap-3 px-5 py-4 text-left transition-colors duration-150 hover:bg-accent/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/70 md:px-6"
        >
          <ChevronRight
            className={cn('mt-0.5 h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200 ease-panel motion-reduce:transition-none', uninstallOpen && 'rotate-90')}
            aria-hidden="true"
          />
          <span className="min-w-0 flex-1 space-y-1">
            <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
              <span className="text-sm font-semibold text-foreground">{t('templates.editor.install.uninstall.title')}</span>
              <span className="text-xs text-muted-foreground">{t('templates.editor.install.uninstall.optional')}</span>
              {uninstallCount > 0 && (
                <span className="text-xs tabular-nums text-muted-foreground">
                  · {uninstallCount === 1 ? t('templates.editor.steps.countOne') : t('templates.editor.steps.countMany', { count: uninstallCount })}
                </span>
              )}
            </span>
            <span className="block text-pretty text-sm text-muted-foreground">{t('templates.editor.install.uninstall.description')}</span>
          </span>
          <IssueCountBadges counts={uninstallIssues} className="mt-0.5" />
        </button>
        {uninstallOpen && (
          <div id="template-uninstall-body" className="px-3 pb-4 sm:px-5 sm:pb-5 md:px-6 md:pb-6">
            <StepList
              path={['uninstall']}
              label={t('templates.editor.install.uninstall.title')}
              emptyTitle={t('templates.editor.install.uninstall.empty.title')}
              emptyDescription={t('templates.editor.install.uninstall.empty.description')}
            />
          </div>
        )}
      </section>
    </div>
  );
}
