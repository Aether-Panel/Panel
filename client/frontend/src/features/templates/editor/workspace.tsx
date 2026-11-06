import { lazy, Suspense, useState } from 'react';
import { PageHeader } from '@/components/page-header';
import { Badge } from '@/components/ui/badge';
import { Card } from '@/components/ui/card';
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { AlertCircle, ArrowLeft, CheckCircle2, Loader2 } from '@/components/icons';
import { SettingsCardHeader } from '@/features/settings/general-tab';
import { useTranslations } from '@/contexts/translations-context';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';
import { useTemplateEditor } from './editor-context';
import { textValue } from './field';
import { saveLocalTemplate } from './load';
import { setIn } from './paths';
import { storedTemplateName } from './template-name';
import { IssueCountBadges, SectionNav, SectionSelect } from './section-nav';
import { getSection } from './sections';
import { EditorToolbar } from './toolbar';
import { useLeaveGuard, useMediaQuery, useSaveShortcut } from './use-page-guards';
import type { JsonProblem } from './document';

// Monaco is large: its chunk loads the first time the JSON view opens, not with the page.
const TemplateJsonView = lazy(() => import('./json-view'));

function jsonProblemText(t: (key: string) => string, problem: JsonProblem): string {
  return problem.code === 'notObject' ? t('templates.editor.json.notObject') : t('templates.editor.json.invalid');
}

/** The active section, in a card headed by its title, description and counts. */
function SectionFrame() {
  const { t } = useTranslations();
  const { section: id, issues } = useTemplateEditor();
  const section = getSection(id);
  const Body = section.component;
  return (
    <Card className="min-w-0 overflow-hidden">
      <SettingsCardHeader
        title={t(section.labelKey)}
        description={t(section.descriptionKey)}
        action={<IssueCountBadges counts={section.counts(issues)} />}
      />
      <Body section={section} />
    </Card>
  );
}

/** The form: the section menu (a select when `compact` or on narrow screens) and the active section. */
function FormPanel({ compact, showJsonNotice }: Readonly<{ compact: boolean; showJsonNotice: boolean }>) {
  const { t } = useTranslations();
  const { jsonProblem } = useTemplateEditor();
  return (
    <div
      className={cn(
        'grid min-w-0 items-start gap-4',
        !compact && 'lg:grid-cols-[13.5rem_minmax(0,1fr)] lg:gap-6',
        // next to the JSON the column is narrow: section blocks stack their title over their fields
        compact && 'lg:[&_section]:grid-cols-1 lg:[&_section]:gap-5',
      )}
    >
      {!compact && <SectionNav className="sticky top-[9.5rem] hidden lg:block" />}
      <div className="min-w-0 space-y-4">
        <SectionSelect className={compact ? undefined : 'lg:hidden'} />
        {showJsonNotice && jsonProblem && (
          <p role="status" className="flex items-start gap-2 rounded-lg border border-warning/30 bg-warning/10 px-3 py-2.5 text-sm text-warning">
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <span className="text-pretty">{t('templates.editor.json.formNotice')}</span>
          </p>
        )}
        <SectionFrame />
      </div>
    </div>
  );
}

/** The JSON view in a card: its state on top, Monaco below. */
function JsonPanel({ split }: Readonly<{ split: boolean }>) {
  const { t } = useTranslations();
  const { jsonProblem } = useTemplateEditor();
  return (
    <Card className={cn('flex min-w-0 flex-col overflow-hidden', split ? 'sticky top-[9.5rem] h-[calc(100svh-11rem)] min-h-[24rem]' : 'h-[72svh] min-h-[24rem]')}>
      <div className="flex items-center justify-between gap-3 border-b border-border/50 px-4 py-2.5">
        <h2 className="text-sm font-semibold">{t('templates.editor.json.title')}</h2>
        {/* not a live region: the toolbar's status line already announces that the JSON broke */}
        <span className="text-xs">
          {jsonProblem ? (
            <span className="flex items-center gap-1.5 text-destructive-bright">
              <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />
              {t('templates.editor.json.invalidShort')}
            </span>
          ) : (
            <span className="flex items-center gap-1.5 text-muted-foreground">
              <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />
              {t('templates.editor.json.valid')}
            </span>
          )}
        </span>
      </div>
      {jsonProblem && (
        <div className="space-y-1 border-b border-destructive/30 bg-destructive/[0.07] px-4 py-2.5 text-sm text-destructive-bright">
          <p className="text-pretty">
            {jsonProblemText(t, jsonProblem)} {t('templates.editor.json.keptForm')}
          </p>
          {jsonProblem.code === 'syntax' && <p className="break-words font-mono text-xs opacity-90">{jsonProblem.detail}</p>}
        </div>
      )}
      <div className="min-h-0 flex-1 [&_.monaco-editor-host]:rounded-none">
        <Suspense
          fallback={
            <div className="flex h-full items-center justify-center gap-2 text-xs text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
              {t('templates.editor.json.loading')}
            </div>
          }
        >
          <TemplateJsonView ariaLabel={t('templates.editor.json.title')} />
        </Suspense>
      </div>
    </Card>
  );
}

/** The editor page once the template is loaded: header, toolbar, and the form, the JSON or both. */
export function EditorWorkspace() {
  const { t } = useTranslations();
  const editor = useTemplateEditor();
  const { template, jsonText, dirty, originalName, issues, blocked, markSaved, update } = editor;
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);

  // split needs two columns (1280px and up); on a narrower screen it shows as the form
  const wide = useMediaQuery('(min-width: 1280px)');
  const view = editor.view === 'split' && !wide ? 'form' : editor.view;

  const name = textValue(template.name);
  const errorCount = issues.filter((i) => i.severity === 'error').length;
  // an existing template saved as it is would change nothing; a new one can always be saved
  const canSave = !blocked && !saving && (dirty || originalName === undefined);

  useLeaveGuard(dirty && !saving);

  const save = async () => {
    // a change of case or accents alone is not a rename: the row keeps its name (template-name.ts)
    const storedName = storedTemplateName(name, originalName);
    const saved = storedName === name ? template : setIn(template, ['name'], storedName);
    setConfirmOpen(false);
    setSaving(true);
    setSaveError(null);
    try {
      await saveLocalTemplate(storedName, saved);
      const created = storedName !== originalName;
      if (storedName !== name) update(['name'], storedName);
      markSaved(storedName, saved);
      // the address opens what was saved, so reloading edits it instead of starting over; built
      // from the editor's own path, since after a cancelled back or forward the address bar may
      // already show another page
      const url = new URL('/templates/editor/', window.location.origin);
      url.search = new URLSearchParams({ name: storedName }).toString();
      window.history.replaceState(window.history.state, '', url);
      sileo.success({ title: t('common.success'), description: t(created ? 'templates.toast.created' : 'templates.toast.updated') });
    } catch (e) {
      // the panel's own answer when there is one (it validates again on save)
      const reason = e instanceof Error ? e.message : '';
      const message = reason ? t('templates.editor.saveFailed', { message: reason }) : t('templates.toast.saveFailed');
      setSaveError(message);
      sileo.error({ title: t('common.error'), description: message });
    } finally {
      setSaving(false);
    }
  };

  // errors the node may reject (or fail on) ask first; a blocking one never gets here
  const requestSave = () => {
    if (!canSave) return;
    if (errorCount > 0) setConfirmOpen(true);
    else void save();
  };
  useSaveShortcut(requestSave);

  const download = () => {
    const url = URL.createObjectURL(new Blob([jsonText], { type: 'application/json' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = `${name || 'template'}.json`;
    link.click();
    // the click has handed the blob to the download by the next task
    setTimeout(() => URL.revokeObjectURL(url), 0);
  };

  const title = textValue(template.display) || name || t('templates.editor.newTitle');

  return (
    <div className="flex flex-col gap-6 pb-12">
      <div className="space-y-3">
        <a
          href="/templates/"
          className="inline-flex items-center gap-1.5 rounded-md text-sm text-muted-foreground transition-colors duration-150 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          {t('templates.editor.back')}
        </a>
        <PageHeader
          title={title}
          titleClassName="break-words"
          description={originalName ? t('templates.editor.descriptionLocal', { name: originalName }) : t('templates.editor.descriptionNew')}
          badges={
            dirty ? (
              <Badge variant="secondary" className="gap-1.5 font-normal">
                <span className="h-1.5 w-1.5 rounded-full bg-warning" aria-hidden="true" />
                {t('templates.editor.unsaved')}
              </Badge>
            ) : undefined
          }
        />
      </div>

      <EditorToolbar view={view} splitAvailable={wide} saving={saving} canSave={canSave} onSave={requestSave} onDownload={download} />

      {saveError && (
        <p role="alert" className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2.5 text-sm text-destructive-bright">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <span className="min-w-0 text-pretty break-words">{saveError}</span>
        </p>
      )}

      {view === 'form' && <FormPanel compact={false} showJsonNotice />}
      {view === 'json' && <JsonPanel split={false} />}
      {view === 'split' && (
        <div className="grid grid-cols-2 items-start gap-6">
          <FormPanel compact showJsonNotice={false} />
          <JsonPanel split />
        </div>
      )}

      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('templates.editor.confirmErrors.title')}</AlertDialogTitle>
            <AlertDialogDescription>
              {errorCount === 1
                ? t('templates.editor.confirmErrors.descriptionOne')
                : t('templates.editor.confirmErrors.descriptionMany', { count: errorCount })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void save()}>{t('templates.editor.confirmErrors.confirm')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
