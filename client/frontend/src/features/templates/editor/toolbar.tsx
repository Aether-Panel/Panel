import { Button } from '@/components/ui/button';
import { AlertCircle, AlertTriangle, CheckCircle2, Download, Loader2, Save } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { useTemplateEditor, type EditorView } from './editor-context';
import { issueCountsLabel } from './section-nav';
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

/** One line on the state of the template: what blocks saving, else how many issues are left. */
function StatusLine({ className }: Readonly<{ className?: string }>) {
  const { t } = useTranslations();
  const { issues, jsonProblem, blocked } = useTemplateEditor();
  const counts = countIssues(issues);

  let tone = 'text-muted-foreground';
  let Icon = CheckCircle2;
  let text = t('templates.editor.status.clean');
  if (jsonProblem) {
    tone = 'text-destructive-bright';
    Icon = AlertCircle;
    text = t('templates.editor.status.jsonInvalid');
  } else if (blocked) {
    tone = 'text-destructive-bright';
    Icon = AlertCircle;
    text = t('templates.editor.status.blocked');
  } else if (counts.errors || counts.warnings) {
    tone = counts.errors ? 'text-destructive-bright' : 'text-warning';
    Icon = counts.errors ? AlertCircle : AlertTriangle;
    text = issueCountsLabel(t, counts);
  }

  return (
    <p className={cn('flex min-w-0 items-center gap-1.5 text-sm', tone, className)}>
      <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
      <span className="truncate">{text}</span>
    </p>
  );
}

/**
 * The bar that stays at the top while the form scrolls: the view switcher, the template's state
 * and its actions.
 */
export function EditorToolbar({ view, splitAvailable, saving, canSave, onSave, onDownload }: Readonly<{
  view: EditorView;
  splitAvailable: boolean;
  saving: boolean;
  canSave: boolean;
  onSave: () => void;
  onDownload: () => void;
}>) {
  const { t } = useTranslations();
  const { jsonProblem } = useTemplateEditor();
  return (
    <div className="sticky top-16 z-30 -mx-4 flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-border/60 bg-background/80 px-4 py-3 backdrop-blur-xl md:-mx-6 md:px-6 lg:-mx-8 lg:px-8">
      <ViewSwitcher view={view} splitAvailable={splitAvailable} />
      <StatusLine className="order-last w-full sm:order-none sm:w-auto sm:flex-1" />
      <div className="ml-auto flex shrink-0 items-center gap-2">
        <Button variant="outline" onClick={onDownload} disabled={jsonProblem !== null} aria-label={t('templates.editor.download')} title={t('templates.editor.download')}>
          <Download aria-hidden="true" />
          <span className="hidden md:inline">{t('templates.editor.download')}</span>
        </Button>
        <Button onClick={onSave} disabled={!canSave}>
          {saving ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Save aria-hidden="true" />}
          {saving ? t('common.saving') : t('templates.editor.save')}
        </Button>
      </div>
    </div>
  );
}
