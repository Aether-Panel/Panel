'use client';
import { lazy, Suspense, useCallback, useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { AlertTriangle, ArrowLeft, GitCompareArrows, Loader2, RotateCcw, RotateCw } from 'lucide-react';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { EmptyState } from '@/components/empty-state';
import { useTranslations } from '@/contexts/translations-context';
import { formatExact, formatRelative, localeFor, type FileVersion } from './file-history-sheet';

const FileDiffEditor = lazy(() => import('./file-diff-editor'));

interface FileDiffViewProps {
  version: FileVersion;
  language: string;
  /** The editor's current text, unsaved edits included. */
  current: string;
  /** Whether the editor holds edits that are not saved yet; restoring replaces them. */
  dirty: boolean;
  loadContent: (id: string) => Promise<string>;
  /** Restores the version on the server and loads it into the editor; throws on failure. */
  onRestore: (version: FileVersion, text: string) => Promise<void>;
  /** Called once the confirmation has closed after a successful restore. */
  onRestored: () => void;
  onBack: () => void;
}

type ContentState = { status: 'loading' } | { status: 'error'; message: string } | { status: 'ready'; text: string };

/** Shown in place of the editor: a stored version next to the current text, with a way back. */
export default function FileDiffView({ version, language, current, dirty, loadContent, onRestore, onRestored, onBack }: Readonly<FileDiffViewProps>) {
  const { t, language: uiLanguage } = useTranslations();
  const locale = localeFor(uiLanguage);
  const headingId = useId();
  const headingRef = useRef<HTMLHeadingElement>(null);
  const [content, setContent] = useState<ContentState>({ status: 'loading' });
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const restoredRef = useRef(false);
  const restoreButtonRef = useRef<HTMLButtonElement>(null);

  const load = useCallback(async () => {
    setContent({ status: 'loading' });
    try {
      const text = await loadContent(version.id);
      setContent({ status: 'ready', text });
    } catch (e: any) {
      setContent({ status: 'error', message: e?.message || '' });
    }
  }, [loadContent, version.id]);

  useEffect(() => {
    let cancelled = false;
    setContent({ status: 'loading' });
    loadContent(version.id).then(
      (text) => !cancelled && setContent({ status: 'ready', text }),
      (e: any) => !cancelled && setContent({ status: 'error', message: e?.message || '' }),
    );
    return () => {
      cancelled = true;
    };
  }, [loadContent, version.id]);

  // the view replaces the editor, so the focus moves to its heading for screen readers and keys
  useEffect(() => {
    const frame = requestAnimationFrame(() => headingRef.current?.focus());
    return () => cancelAnimationFrame(frame);
  }, [version.id]);

  const relative = formatRelative(version.savedAt, locale);
  const exact = formatExact(version.savedAt, locale);
  const identical = content.status === 'ready' && content.text === current;

  const confirmRestore = async () => {
    if (content.status !== 'ready' || restoring) return;
    setRestoring(true);
    try {
      await onRestore(version, content.text);
      restoredRef.current = true;
      setConfirmOpen(false);
    } catch {
      // the editor reports the failure; the confirmation stays open to try again
    } finally {
      setRestoring(false);
    }
  };

  let body: ReactNode;
  if (content.status === 'loading') {
    body = (
      <div className="flex h-full items-center justify-center" role="status" aria-live="polite">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-hidden="true" />
        <span className="sr-only">{t('servers.fileHistory.diff.loading')}</span>
      </div>
    );
  } else if (content.status === 'error') {
    body = (
      <div className="flex h-full items-center justify-center p-4">
        <EmptyState
          size="sm"
          tone="error"
          icon={AlertTriangle}
          title={t('servers.fileHistory.diff.errorTitle')}
          description={content.message || t('servers.fileHistory.errorDescription')}
          action={
            <Button variant="outline" size="sm" onClick={() => void load()}>
              <RotateCw aria-hidden="true" />
              {t('common.retry')}
            </Button>
          }
          className="w-full max-w-md"
        />
      </div>
    );
  } else {
    body = (
      <Suspense
        fallback={
          <div className="flex h-full items-center justify-center">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-label={t('servers.fileHistory.diff.loading')} />
          </div>
        }
      >
        <FileDiffEditor key={version.id} language={language} original={content.text} modified={current} />
      </Suspense>
    );
  }

  return (
    <section aria-labelledby={headingId} className="flex h-full min-h-0 flex-col animate-in fade-in-0 duration-200 ease-panel">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-border/50 bg-card/80 px-3 py-2 sm:px-4">
        <div className="flex min-w-0 flex-1 basis-56 items-center gap-2.5">
          <span aria-hidden="true" className="grid h-7 w-7 shrink-0 place-items-center rounded-md border border-primary/30 bg-primary/10 text-primary-bright">
            <GitCompareArrows className="h-3.5 w-3.5" />
          </span>
          <div className="min-w-0">
            <h3 id={headingId} ref={headingRef} tabIndex={-1} className="truncate rounded-sm text-sm font-medium text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring/70" title={exact}>
              {t('servers.fileHistory.diff.comparing', { when: relative })}
            </h3>
            <p className="truncate text-xs text-muted-foreground" role="status" aria-live="polite">
              {identical ? t('servers.fileHistory.diff.identical') : (
                <>
                  <span className="hidden md:inline">{t('servers.fileHistory.diff.legendSides')}</span>
                  <span className="md:hidden">{t('servers.fileHistory.diff.legendInline')}</span>
                </>
              )}
            </p>
          </div>
        </div>
        <div className="ml-auto flex shrink-0 items-center gap-2">
          <Button variant="ghost" size="sm" onClick={onBack} className="text-muted-foreground hover:text-foreground">
            <ArrowLeft aria-hidden="true" />
            <span className="sm:hidden">{t('servers.fileHistory.diff.backShort')}</span>
            <span className="hidden sm:inline">{t('servers.fileHistory.diff.back')}</span>
          </Button>
          <Button ref={restoreButtonRef} variant="outline" size="sm" onClick={() => setConfirmOpen(true)} disabled={content.status !== 'ready' || restoring || identical}>
            {restoring ? <Loader2 className="animate-spin" aria-hidden="true" /> : <RotateCcw aria-hidden="true" />}
            <span className="sm:hidden">{t('servers.fileHistory.diff.restoreShort')}</span>
            <span className="hidden sm:inline">{t('servers.fileHistory.diff.restore')}</span>
          </Button>
        </div>
      </div>

      <div className="relative min-h-0 flex-1">
        {body}
      </div>

      <AlertDialog open={confirmOpen} onOpenChange={(open) => !restoring && setConfirmOpen(open)}>
        <AlertDialogContent
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            // after a restore the editor comes back and takes the focus itself
            if (restoredRef.current) {
              restoredRef.current = false;
              onRestored();
              return;
            }
            restoreButtonRef.current?.focus();
          }}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>{t('servers.fileHistory.confirm.title')}</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div className="space-y-2">
                <p>{t('servers.fileHistory.confirm.description', { when: relative })}</p>
                {dirty && <p className="font-medium text-foreground">{t('servers.fileHistory.confirm.unsaved')}</p>}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={restoring}>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              disabled={restoring}
              onClick={(event) => {
                // stay open while the restore runs, so the button can show it is busy
                event.preventDefault();
                void confirmRestore();
              }}
            >
              {restoring ? <Loader2 className="animate-spin" aria-hidden="true" /> : <RotateCcw aria-hidden="true" />}
              {t('servers.fileHistory.confirm.action')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  );
}
