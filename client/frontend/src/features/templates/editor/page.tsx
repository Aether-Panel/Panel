import { useEffect, useState } from 'react';
import { useAuth } from '@/contexts/providers';
import { useTranslations } from '@/contexts/translations-context';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { EmptyState } from '@/components/empty-state';
import { ArrowLeft, LayoutTemplate } from '@/components/icons';
import { TemplateEditorProvider } from './editor-context';
import { editorSource } from './initial';
import { loadEditorStart, type EditorStart } from './load';
import { EditorWorkspace } from './workspace';

// /templates/editor/: a new local template, or ?name= to edit one, ?from=<repo>:<name> to copy
// one, ?import to open the file the list page read. Saving needs templates.local.edit, so the
// page does too.

function EditorSkeleton() {
  return (
    <div className="flex flex-col gap-6" aria-busy="true">
      <div className="space-y-3">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-9 w-72 max-w-full" />
        <Skeleton className="h-4 w-80 max-w-full" />
      </div>
      <Skeleton className="h-12 w-full rounded-lg" />
      <div className="grid gap-6 lg:grid-cols-[13.5rem_minmax(0,1fr)]">
        <div className="hidden space-y-2 lg:block">
          {Array.from({ length: 7 }, (_, i) => <Skeleton key={i} className="h-9 w-full rounded-lg" />)}
        </div>
        <Card className="space-y-4 p-5 md:p-6">
          <Skeleton className="h-5 w-48" />
          <Skeleton className="h-4 w-72 max-w-full" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </Card>
      </div>
    </div>
  );
}

function LoadProblem({ start }: Readonly<{ start: Extract<EditorStart, { ok: false }> }>) {
  const { t } = useTranslations();
  return (
    <EmptyState
      icon={LayoutTemplate}
      tone="error"
      as="h1"
      title={t(`templates.editor.load.${start.problem}.title`)}
      description={t(`templates.editor.load.${start.problem}.description`, { name: start.detail ?? '', message: start.detail ?? '' })}
      action={
        <Button variant="outline" asChild>
          <a href="/templates/">
            <ArrowLeft aria-hidden="true" />
            {t('templates.editor.back')}
          </a>
        </Button>
      }
    />
  );
}

export default function TemplateEditorPage() {
  const { t } = useTranslations();
  const { role, hasScope } = useAuth();
  const [start, setStart] = useState<EditorStart | null>(null);
  const allowed = hasScope('templates.local.edit');

  useEffect(() => {
    if (role && !allowed) window.location.href = '/templates/';
  }, [role, allowed]);

  // the breadcrumb reads "Editor" instead of the raw path segment
  useEffect(() => {
    window.dispatchEvent(new CustomEvent('aether:breadcrumb', { detail: { id: 'editor', label: t('templates.editor.breadcrumb') } }));
  }, [t]);

  useEffect(() => {
    if (!allowed) return;
    let cancelled = false;
    void loadEditorStart(editorSource(window.location.search), t('templates.editor.copySuffix')).then((result) => {
      if (!cancelled) setStart(result);
    });
    return () => {
      cancelled = true;
    };
    // loaded once per visit; a language change must not reload (and drop edits)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [allowed]);

  if (!allowed || !start) return <EditorSkeleton />;
  if (!start.ok) return <LoadProblem start={start} />;

  return (
    <TemplateEditorProvider initialTemplate={start.template} saved={start.saved} originalName={start.originalName} takenNames={start.takenNames}>
      <EditorWorkspace />
    </TemplateEditorProvider>
  );
}
