import { Button } from '@/components/ui/button';
import { AlertCircle, AlertTriangle, CheckCircle2, ChevronDown, Download, Eye, Loader2, Save } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { useTemplateEditor, type EditorView } from './editor-context';
import { IssueSummary } from './issue-summary';
import { IssueCountBadges, issueCountsLabel } from './section-nav';
import { useSettled } from './use-settled';
import { countIssues } from './validation';

/** Form | JSON | Split. Split needs two columns, so it is only offered on wide screens. */
function ViewSwitcher({ view, splitAvailable }: Readonly<{ view: EditorView; splitAvailable: boolean }>) {
  const { t } = useTranslations();
  const { setView } = useTemplateEditor();
  const views: EditorView[] = splitAvailable ? ['form', 'json', 'split'] : ['form', 'json'];
  return (
    <div role="group" aria-label={t('templates.editor.views.label')} className="inline-flex h-10 shrink-0 items-center rounded-lg bg-muted p-1 text-muted-foreground">
      {views.map((option) => (
        <button
          key={option}
          type="button"
          aria-pressed={view === option}
          onClick={() => setView(option)}
          className="inline-flex h-8 items-center rounded-md px-3 text-sm font-medium transition-colors duration-150 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70 aria-pressed:bg-background aria-pressed:text-foreground aria-pressed:shadow-sm"
        >
          {t(`templates.editor.views.${option}`)}
        </button>
      ))}
    </div>
  );
}

/** The status text's id, which the Save button points to for why it is disabled. */
const STATUS_ID = 'template-editor-status';

/** How long the status must stay the same before it is announced, so typing does not spam it. */
const ANNOUNCE_DELAY = 1000;

/**
 * One line on the state of the template: what blocks saving, else how many issues are left.
 * With issues it is a button that opens the list of them (IssueSummary). Screen readers hear it
 * from a polite live region once it settles, not on every keystroke that flips a count.
 */
function StatusLine({ className }: Readonly<{ className?: string }>) {
  const { t } = useTranslations();
  const { issues, jsonProblem, blocked } = useTemplateEditor();
  const counts = countIssues(issues);

  let tone = 'text-muted-foreground';
  let Icon = CheckCircle2;
  let text = t('templates.editor.status.clean');
  // when the text says what blocks saving, the counts go next to it
  let showCounts = false;
  if (jsonProblem) {
    tone = 'text-destructive-bright';
    Icon = AlertCircle;
    text = t('templates.editor.status.jsonInvalid');
    showCounts = true;
  } else if (blocked) {
    tone = 'text-destructive-bright';
    Icon = AlertCircle;
    text = t('templates.editor.status.blocked');
    showCounts = true;
  } else if (counts.errors || counts.warnings) {
    tone = counts.errors ? 'text-destructive-bright' : 'text-warning';
    Icon = counts.errors ? AlertCircle : AlertTriangle;
    text = issueCountsLabel(t, counts);
  }
  const announced = useSettled(text, ANNOUNCE_DELAY);

  const status = (
    <span id={STATUS_ID} className="flex min-w-0 items-center gap-1.5">
      <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
      <span className="truncate">{text}</span>
      {showCounts && <IssueCountBadges counts={counts} />}
    </span>
  );

  return (
    <div className={cn('flex min-w-0 items-center text-sm', tone, className)}>
      {issues.length > 0 ? (
        <IssueSummary>
          <button
            type="button"
            className="-mx-2 flex min-w-0 max-w-full items-center gap-1 rounded-md px-2 py-1.5 transition-colors duration-150 hover:bg-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
          >
            {status}
            <span className="sr-only">. {t('templates.editor.summary.open')}</span>
            <ChevronDown className="h-3.5 w-3.5 shrink-0 opacity-70" aria-hidden="true" />
          </button>
        </IssueSummary>
      ) : (
        <p className="min-w-0">{status}</p>
      )}
      <span className="sr-only" role="status" aria-live="polite" aria-atomic="true">
        {announced}
      </span>
    </div>
  );
}

/**
 * The bar that stays at the top while the form scrolls: the view switcher, the template's state
 * and its actions.
 */
export function EditorToolbar({ view, splitAvailable, saving, canSave, onSave, onDownload, onPreview }: Readonly<{
  view: EditorView;
  splitAvailable: boolean;
  saving: boolean;
  canSave: boolean;
  onSave: () => void;
  onDownload: () => void;
  onPreview: () => void;
}>) {
  const { t } = useTranslations();
  const { jsonProblem } = useTemplateEditor();
  const saveLabel = saving ? t('common.saving') : t('templates.editor.save');
  return (
    <div className="sticky top-16 z-30 -mx-4 flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-border/60 bg-background/80 px-4 py-3 backdrop-blur-xl md:-mx-6 md:px-6 lg:-mx-8 lg:px-8">
      <ViewSwitcher view={view} splitAvailable={splitAvailable} />
      {/* its own row until lg: next to the sidebar a tablet leaves it too little room inline */}
      <StatusLine className="order-last w-full lg:order-none lg:w-auto lg:flex-1" />
      <div className="ml-auto flex shrink-0 items-center gap-2">
        {/* square icon buttons until the row is wide enough for their labels (xl, next to the sidebar) */}
        <Button variant="outline" onClick={onPreview} aria-label={t('templates.editor.preview.button')} title={t('templates.editor.preview.button')} className="w-10 px-0 xl:w-auto xl:px-4">
          <Eye aria-hidden="true" />
          <span className="hidden xl:inline">{t('templates.editor.preview.button')}</span>
        </Button>
        <Button variant="outline" onClick={onDownload} disabled={jsonProblem !== null} aria-label={t('templates.editor.download')} title={t('templates.editor.download')} className="w-10 px-0 xl:w-auto xl:px-4">
          <Download aria-hidden="true" />
          <span className="hidden xl:inline">{t('templates.editor.download')}</span>
        </Button>
        {/* on the narrowest phones its label goes too, so the view switcher and the three actions share one row */}
        <Button onClick={onSave} disabled={!canSave} aria-describedby={STATUS_ID} aria-label={saveLabel} title={saveLabel} className="w-10 px-0 min-[420px]:w-auto min-[420px]:px-4">
          {saving ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Save aria-hidden="true" />}
          <span className="hidden min-[420px]:inline">{saveLabel}</span>
        </Button>
      </div>
    </div>
  );
}
