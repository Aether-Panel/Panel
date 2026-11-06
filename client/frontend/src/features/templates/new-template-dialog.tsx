import { useRef, useState, type ReactNode } from 'react';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { EmptyState } from '@/components/empty-state';
import { TemplateLogo } from '@/components/template-logo';
import { ArrowLeft, ChevronRight, Copy, FileJson, FilePlus, LayoutTemplate, Search, type AppIcon } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { sileo } from '@/lib/toast';
import { templateLogo } from '@/lib/template-logos';
import { matchesTemplate } from '@/lib/template-search';
import { importTemplate, stashImportedTemplate } from './editor/initial';

/** A template of the list page, as the duplicate search shows it. */
export type ListedTemplate = { name: string; display?: string; type?: string; icon?: string | null; repoId: number; repoName?: string };

const EDITOR = '/templates/editor/';

/** The editor opened on a copy of `template`. */
export function duplicateHref(template: Pick<ListedTemplate, 'repoId' | 'name'>): string {
  return `${EDITOR}?${new URLSearchParams({ from: `${template.repoId}:${template.name}` })}`;
}

const OPTION_CLASS =
  'flex w-full items-start gap-3.5 rounded-xl border border-border/60 bg-background/30 p-4 text-left transition-colors duration-150 hover:border-primary/40 hover:bg-accent/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70';

function OptionBody({ icon: Icon, title, description, trailing }: Readonly<{ icon: AppIcon; title: string; description: string; trailing?: ReactNode }>) {
  return (
    <>
      <span className="mt-0.5 grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-border/60 bg-card text-primary-bright" aria-hidden="true">
        <Icon className="h-4 w-4" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-sm font-medium text-foreground">{title}</span>
        <span className="mt-1 block text-pretty text-xs text-muted-foreground">{description}</span>
      </span>
      {trailing}
    </>
  );
}

function DuplicateList({ templates, loading }: Readonly<{ templates: ListedTemplate[]; loading: boolean }>) {
  const { t } = useTranslations();
  const [query, setQuery] = useState('');
  const shown = templates.filter((tpl) => matchesTemplate(tpl, query));

  let body: ReactNode;
  if (loading) {
    body = (
      <div className="space-y-2" aria-busy="true">
        {Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-14 w-full rounded-lg" />)}
      </div>
    );
  } else if (shown.length === 0) {
    body = <EmptyState icon={LayoutTemplate} size="sm" title={t('templates.chooser.duplicate.emptyTitle')} description={t('templates.chooser.duplicate.emptyDescription')} />;
  } else {
    body = (
      <ul className="space-y-1">
        {shown.map((tpl) => (
          <li key={`${tpl.repoId}-${tpl.name}`}>
            <a href={duplicateHref(tpl)} className="flex items-center gap-3 rounded-lg px-2.5 py-2 transition-colors duration-150 hover:bg-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70">
              <span className="grid h-9 w-9 shrink-0 place-items-center overflow-hidden rounded-lg border border-border/60 bg-secondary text-muted-foreground" aria-hidden="true">
                <TemplateLogo logo={templateLogo(tpl)} className="h-full w-full rounded-none ring-0" fallback={<LayoutTemplate className="h-4 w-4" />} />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium text-foreground">{tpl.display || tpl.name}</span>
                <span className="block truncate font-mono text-xs text-muted-foreground">{tpl.name}</span>
              </span>
              <Badge variant="secondary" className="max-w-[40%] shrink-0 font-normal">
                <span className="truncate">{tpl.repoId === 0 ? t('templates.tabs.local') : tpl.repoName || t('templates.tabs.community')}</span>
              </Badge>
            </a>
          </li>
        ))}
      </ul>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
        <Input
          type="search"
          autoFocus
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t('templates.search.placeholder')}
          aria-label={t('templates.chooser.duplicate.searchLabel')}
          className="pl-9"
        />
      </div>
      <div className="custom-scrollbar -mx-1 min-h-0 flex-1 overflow-y-auto px-1">{body}</div>
    </div>
  );
}

/**
 * "Nueva plantilla": start blank, from a copy of a local or community template, or from a .json
 * file. Each choice opens the editor; nothing is saved until it is saved there.
 */
export function NewTemplateDialog({ open, onOpenChange, templates, loading }: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  templates: ListedTemplate[];
  loading: boolean;
}>) {
  const { t } = useTranslations();
  const [step, setStep] = useState<'choose' | 'duplicate'>('choose');
  const fileRef = useRef<HTMLInputElement>(null);

  const changeOpen = (next: boolean) => {
    onOpenChange(next);
    if (!next) setStep('choose');
  };

  const openFile = async (file: File | undefined) => {
    if (!file) return;
    try {
      const result = importTemplate(await file.text(), file.name);
      if ('problem' in result) {
        sileo.error({ title: t('common.error'), description: t('templates.toast.invalidJson') });
      } else if (stashImportedTemplate(result.template)) {
        window.location.assign(`${EDITOR}?import`);
      } else {
        sileo.error({ title: t('common.error'), description: t('templates.chooser.import.failed') });
      }
    } catch {
      sileo.error({ title: t('common.error'), description: t('templates.chooser.import.failed') });
    } finally {
      if (fileRef.current) fileRef.current.value = '';
    }
  };

  return (
    <Dialog open={open} onOpenChange={changeOpen}>
      <DialogContent className="flex max-h-[min(90svh,40rem)] flex-col gap-5 sm:max-w-lg">
        <DialogHeader className="text-left">
          <DialogTitle className="text-xl font-semibold tracking-[-0.02em]">
            {step === 'duplicate' ? t('templates.chooser.duplicate.title') : t('templates.chooser.title')}
          </DialogTitle>
          <DialogDescription>
            {step === 'duplicate' ? t('templates.chooser.duplicate.description') : t('templates.chooser.description')}
          </DialogDescription>
        </DialogHeader>

        {step === 'choose' ? (
          <div className="space-y-2.5">
            <a href={EDITOR} className={OPTION_CLASS}>
              <OptionBody icon={FilePlus} title={t('templates.chooser.blank.title')} description={t('templates.chooser.blank.description')} />
            </a>
            <button type="button" className={OPTION_CLASS} onClick={() => setStep('duplicate')}>
              <OptionBody
                icon={Copy}
                title={t('templates.chooser.duplicate.option')}
                description={t('templates.chooser.duplicate.optionDescription')}
                trailing={<ChevronRight className="mt-2.5 h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />}
              />
            </button>
            <button type="button" className={OPTION_CLASS} onClick={() => fileRef.current?.click()}>
              <OptionBody icon={FileJson} title={t('templates.chooser.import.title')} description={t('templates.chooser.import.description')} />
            </button>
            <input ref={fileRef} type="file" accept=".json,application/json" className="hidden" onChange={(e) => void openFile(e.target.files?.[0])} />
          </div>
        ) : (
          <>
            <DuplicateList templates={templates} loading={loading} />
            <div>
              <Button variant="ghost" size="sm" onClick={() => setStep('choose')}>
                <ArrowLeft aria-hidden="true" />
                {t('templates.chooser.back')}
              </Button>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
