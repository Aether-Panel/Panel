import { useMemo, useRef, useState, type KeyboardEvent, type ReactElement } from 'react';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { AlertCircle, AlertTriangle, Braces } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { useTemplateEditor, type EditorView } from './editor-context';
import { groupIssues, pathFromKey, type IssueGroup } from './issue-groups';
import { sectionOfPath } from './section-keys';
import { IssueCountBadges } from './section-nav';
import { getSection } from './sections';
import { countIssues, type EditorIssue } from './validation';

// The validation summary: every error and warning of the template, under the section that edits
// it, the ones that block saving first. Picking one shows its section and focuses its field.

/** Arrow keys, Home and End move between the items; Tab still works as usual. */
function moveFocus(event: KeyboardEvent<HTMLElement>) {
  const keys = ['ArrowDown', 'ArrowUp', 'Home', 'End'];
  if (!keys.includes(event.key)) return;
  const items = [...event.currentTarget.querySelectorAll<HTMLButtonElement>('[data-issue-item]')];
  if (items.length === 0) return;
  event.preventDefault();
  const at = items.indexOf(document.activeElement as HTMLButtonElement);
  let next = 0;
  if (event.key === 'End') next = items.length - 1;
  else if (event.key === 'ArrowDown') next = at < 0 ? 0 : Math.min(at + 1, items.length - 1);
  else if (event.key === 'ArrowUp') next = at < 0 ? items.length - 1 : Math.max(at - 1, 0);
  items[next].focus();
}

function IssueItem({ issue, onPick }: Readonly<{ issue: EditorIssue; onPick: (issue: EditorIssue) => void }>) {
  const { t } = useTranslations();
  const error = issue.severity === 'error';
  const Icon = error ? AlertCircle : AlertTriangle;
  return (
    <li>
      <button
        type="button"
        data-issue-item=""
        onClick={() => onPick(issue)}
        className="flex w-full items-start gap-2.5 rounded-md px-2.5 py-2 text-left transition-colors duration-150 hover:bg-accent/60 focus-visible:bg-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
      >
        <Icon className={cn('mt-0.5 h-4 w-4 shrink-0', error ? 'text-destructive-bright' : 'text-warning')} aria-hidden="true" />
        <span className="min-w-0 flex-1">
          <span className="sr-only">{t(error ? 'templates.editor.summary.error' : 'templates.editor.summary.warning')}: </span>
          <span className="block text-pretty break-words text-sm text-foreground">{t(issue.messageKey, issue.params)}</span>
          <span className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1">
            {issue.blocking && (
              <span className="rounded bg-destructive/15 px-1.5 py-px text-[11px] font-medium text-destructive-bright">{t('templates.editor.summary.blocking')}</span>
            )}
            {issue.path !== '' && <span className="min-w-0 break-all font-mono text-[11px] text-muted-foreground">{issue.path}</span>}
          </span>
        </span>
      </button>
    </li>
  );
}

function GroupHeading({ group, id }: Readonly<{ group: IssueGroup; id: string }>) {
  const { t } = useTranslations();
  const section = group.section ? getSection(group.section) : null;
  const Icon = section ? section.icon : Braces;
  return (
    <div className="flex items-center gap-2 px-2.5 pb-1 pt-2">
      <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
      <h3 id={id} className="min-w-0 flex-1 truncate text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {section ? t(section.labelKey) : t('templates.editor.summary.jsonOnly')}
      </h3>
      <IssueCountBadges counts={countIssues(group.issues)} />
    </div>
  );
}

/**
 * Wraps `children` (the toolbar's status button) so it opens the list. A pick closes the list and
 * goes to the field; the focus then stays there instead of going back to the button. `view` is the
 * view on screen (split shows as the form on a narrow screen).
 */
export function IssueSummary({ view, children }: Readonly<{ view: EditorView; children: ReactElement }>) {
  const { t } = useTranslations();
  const { issues, template, jsonProblem, goTo, setView } = useTemplateEditor();
  const [open, setOpen] = useState(false);
  const picked = useRef(false);
  const groups = useMemo(() => groupIssues(issues), [issues]);

  const pick = (issue: EditorIssue) => {
    picked.current = true;
    setOpen(false);
    const section = sectionOfPath(issue.path);
    if (section) goTo(section, pathFromKey(template, issue.path));
    // a key no section edits is only in the JSON, which the split view already shows
    else if (view !== 'split') setView('json');
  };

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        if (next) picked.current = false;
        setOpen(next);
      }}
    >
      <PopoverTrigger asChild>{children}</PopoverTrigger>
      <PopoverContent
        align="start"
        collisionPadding={16}
        className="flex max-h-[min(70svh,32rem)] w-[min(28rem,calc(100vw-2rem))] flex-col p-0"
        aria-label={t('templates.editor.summary.title')}
        onCloseAutoFocus={(e) => {
          if (picked.current) e.preventDefault();
        }}
      >
        <div className="flex items-center justify-between gap-3 border-b border-border/50 px-4 py-3">
          <div className="min-w-0">
            <h2 className="text-sm font-semibold">{t('templates.editor.summary.title')}</h2>
            <p className="text-xs text-muted-foreground">{t('templates.editor.summary.hint')}</p>
          </div>
          <IssueCountBadges counts={countIssues(issues)} />
        </div>
        {jsonProblem && (
          <p className="border-b border-border/50 bg-warning/10 px-4 py-2 text-xs text-warning">{t('templates.editor.summary.staleJson')}</p>
        )}
        <div className="custom-scrollbar min-h-0 flex-1 overflow-y-auto p-1.5" onKeyDown={moveFocus}>
          {groups.map((group, index) => {
            const headingId = `template-issues-${group.section ?? 'json'}`;
            return (
              <section key={group.section ?? 'json'} aria-labelledby={headingId} className={cn(index > 0 && 'mt-1 border-t border-border/40 pt-1')}>
                <GroupHeading group={group} id={headingId} />
                <ul>
                  {group.issues.map((issue, i) => (
                    <IssueItem key={`${issue.path}|${issue.messageKey}|${i}`} issue={issue} onPick={pick} />
                  ))}
                </ul>
              </section>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}
