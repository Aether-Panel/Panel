import { useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { EmptyState } from '@/components/empty-state';
import { TemplateLogo } from '@/components/template-logo';
import { ArrowLeft, ChevronRight, Copy, FileJson, FilePlus, Gamepad2, Layers, LayoutTemplate, Package, Search, type AppIcon } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { sileo } from '@/lib/toast';
import { templateLogo } from '@/lib/template-logos';
import { matchesTemplate } from '@/lib/template-search';
import { importTemplate, stashImportedTemplate } from './editor/initial';
import { presetHref, TEMPLATE_PRESETS } from './editor/presets';
import { useSettled } from './editor/use-settled';

/** A template of the list page, as the duplicate search shows it. */
export type ListedTemplate = { name: string; display?: string; type?: string; icon?: string | null; repoId: number; repoName?: string };

const EDITOR = '/templates/editor/';

/** The editor opened on a copy of `template`. */
export function duplicateHref(template: Pick<ListedTemplate, 'repoId' | 'name'>): string {
  return `${EDITOR}?${new URLSearchParams({ from: `${template.repoId}:${template.name}` })}`;
}

type Step = 'choose' | 'preset' | 'duplicate';

const OPTION_CLASS =
  'flex w-full items-start gap-3.5 rounded-xl border border-border/60 bg-background/30 p-4 text-left transition-colors duration-150 hover:border-primary/40 hover:bg-accent/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70';

/** The icon of a base template without a logo of its own. */
const PRESET_ICONS: Record<string, AppIcon> = { steam: Gamepad2, docker: Package };

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

const ITEM = '[data-chooser-item]';

/**
 * Arrow keys move between a list's links (Home and End to its ends); ArrowUp on the first one
 * goes back to `before` (the search field) when there is one.
 */
function moveBetweenItems(event: KeyboardEvent<HTMLElement>, before?: HTMLElement | null) {
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;
  const items = [...event.currentTarget.querySelectorAll<HTMLElement>(ITEM)];
  const at = items.indexOf(document.activeElement as HTMLElement);
  if (items.length === 0 || at < 0) return;
  event.preventDefault();
  if (event.key === 'ArrowUp' && at === 0 && before) before.focus();
  else if (event.key === 'Home') items[0].focus();
  else if (event.key === 'End') items[items.length - 1].focus();
  else items[Math.min(Math.max(at + (event.key === 'ArrowDown' ? 1 : -1), 0), items.length - 1)].focus();
}

/** A logo tile, or the icon when there is no logo (or it fails to load). */
function LogoTile({ logo, icon: Icon }: Readonly<{ logo: ReturnType<typeof templateLogo>; icon: AppIcon }>) {
  return (
    <span className="grid h-9 w-9 shrink-0 place-items-center overflow-hidden rounded-lg border border-border/60 bg-secondary text-muted-foreground" aria-hidden="true">
      <TemplateLogo logo={logo} className="h-full w-full rounded-none ring-0" fallback={<Icon className="h-4 w-4" />} />
    </span>
  );
}

/** The base templates, each opening the editor on a copy of it without a name. */
function PresetList() {
  const { t } = useTranslations();
  return (
    <ul className="space-y-1.5" aria-label={t('templates.chooser.preset.listLabel')} onKeyDown={(e) => moveBetweenItems(e)}>
      {TEMPLATE_PRESETS.map((preset) => (
        <li key={preset.id}>
          <a
            href={presetHref(preset.id)}
            data-chooser-item=""
            className="flex items-center gap-3 rounded-xl border border-border/60 bg-background/30 px-3 py-2.5 transition-colors duration-150 hover:border-primary/40 hover:bg-accent/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
          >
            <LogoTile logo={templateLogo({ icon: preset.logo })} icon={PRESET_ICONS[preset.id] ?? LayoutTemplate} />
            <span className="min-w-0 flex-1">
              <span className="block text-sm font-medium text-foreground">{t(`templates.presets.${preset.id}.title`)}</span>
              <span className="mt-0.5 block text-pretty text-xs text-muted-foreground">{t(`templates.presets.${preset.id}.description`)}</span>
            </span>
            <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          </a>
        </li>
      ))}
    </ul>
  );
}

function DuplicateList({ templates, loading }: Readonly<{ templates: ListedTemplate[]; loading: boolean }>) {
  const { t } = useTranslations();
  const [query, setQuery] = useState('');
  const searchRef = useRef<HTMLInputElement>(null);
  const hintId = useId();
  const listRef = useRef<HTMLUListElement>(null);
  const shown = templates.filter((tpl) => matchesTemplate(tpl, query));
  // how many match, said once the user stops typing
  const count = loading ? '' : t(shown.length === 1 ? 'templates.chooser.duplicate.resultsOne' : 'templates.chooser.duplicate.resultsMany', { count: shown.length });
  const announced = useSettled(count, 600);

  const onSearchKey = (event: KeyboardEvent<HTMLInputElement>) => {
    const first = listRef.current?.querySelector<HTMLElement>(ITEM);
    if (!first) return;
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      first.focus();
    } else if (event.key === 'Enter' && query.trim()) {
      // the best match is the first one: Enter opens it, as in the panel's other searches. With
      // nothing typed the first one is just the top of the list, so a stray Enter does nothing.
      event.preventDefault();
      first.click();
    }
  };

  let body: ReactNode;
  if (loading) {
    body = (
      <div className="space-y-2" aria-busy="true">
        {Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-14 w-full rounded-lg" />)}
      </div>
    );
  } else if (templates.length === 0) {
    body = <EmptyState icon={LayoutTemplate} size="sm" title={t('templates.chooser.duplicate.noneTitle')} description={t('templates.chooser.duplicate.noneDescription')} />;
  } else if (shown.length === 0) {
    body = <EmptyState icon={Search} size="sm" title={t('templates.chooser.duplicate.emptyTitle')} description={t('templates.chooser.duplicate.emptyDescription')} />;
  } else {
    body = (
      <ul ref={listRef} className="space-y-1" onKeyDown={(e) => moveBetweenItems(e, searchRef.current)}>
        {shown.map((tpl) => (
          <li key={`${tpl.repoId}-${tpl.name}`}>
            <a
              href={duplicateHref(tpl)}
              data-chooser-item=""
              className="flex items-center gap-3 rounded-lg px-2.5 py-2 transition-colors duration-150 hover:bg-accent/60 focus-visible:bg-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
            >
              <LogoTile logo={templateLogo(tpl)} icon={LayoutTemplate} />
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
          ref={searchRef}
          type="search"
          autoFocus
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={onSearchKey}
          placeholder={t('templates.search.placeholder')}
          aria-label={t('templates.chooser.duplicate.searchLabel')}
          aria-describedby={hintId}
          className="pl-9"
        />
      </div>
      <p id={hintId} className="sr-only">
        {t('templates.chooser.duplicate.searchHint')}
      </p>
      <p className="sr-only" role="status" aria-live="polite">
        {announced}
      </p>
      <div className="custom-scrollbar -mx-1 min-h-0 flex-1 overflow-y-auto px-1 py-0.5">{body}</div>
    </div>
  );
}

/**
 * "Nueva plantilla": start blank, from a base template, from a copy of a local or community
 * template, or from a .json file. Each choice opens the editor; nothing is saved until it is
 * saved there.
 */
export function NewTemplateDialog({ open, onOpenChange, templates, loading }: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  templates: ListedTemplate[];
  loading: boolean;
}>) {
  const { t } = useTranslations();
  const [step, setStep] = useState<Step>('choose');
  const fileRef = useRef<HTMLInputElement>(null);
  // back from a list, the focus returns to the option that opened it
  const optionRefs = useRef<Partial<Record<Step, HTMLButtonElement | null>>>({});
  const cameFrom = useRef<Step | null>(null);
  const presetListRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (step === 'choose' && cameFrom.current) {
      optionRefs.current[cameFrom.current]?.focus();
      cameFrom.current = null;
    }
    // the base templates have no search field to take the focus: the first one does
    if (step === 'preset') presetListRef.current?.querySelector<HTMLElement>(ITEM)?.focus();
  }, [step]);

  const changeOpen = (next: boolean) => {
    onOpenChange(next);
    if (!next) setStep('choose');
  };

  const goBack = () => {
    cameFrom.current = step;
    setStep('choose');
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

  let title = t('templates.chooser.title');
  let description = t('templates.chooser.description');
  if (step === 'preset') {
    title = t('templates.chooser.preset.title');
    description = t('templates.chooser.preset.description');
  } else if (step === 'duplicate') {
    title = t('templates.chooser.duplicate.title');
    description = t('templates.chooser.duplicate.description');
  }

  const back = (
    <div>
      <Button variant="ghost" size="sm" onClick={goBack}>
        <ArrowLeft aria-hidden="true" />
        {t('templates.chooser.back')}
      </Button>
    </div>
  );
  const listTrailing = <ChevronRight className="mt-2.5 h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />;

  return (
    <Dialog open={open} onOpenChange={changeOpen}>
      <DialogContent className="flex max-h-[min(90svh,40rem)] flex-col gap-5 sm:max-w-lg">
        <DialogHeader className="text-left">
          <DialogTitle className="text-xl font-semibold tracking-[-0.02em]">{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>

        {step === 'choose' && (
          <div className="custom-scrollbar -mx-1 min-h-0 space-y-2.5 overflow-y-auto px-1 py-0.5">
            <a href={EDITOR} className={OPTION_CLASS}>
              <OptionBody icon={FilePlus} title={t('templates.chooser.blank.title')} description={t('templates.chooser.blank.description')} />
            </a>
            <button ref={(el) => { optionRefs.current.preset = el; }} type="button" className={OPTION_CLASS} onClick={() => setStep('preset')}>
              <OptionBody icon={Layers} title={t('templates.chooser.preset.option')} description={t('templates.chooser.preset.optionDescription')} trailing={listTrailing} />
            </button>
            <button ref={(el) => { optionRefs.current.duplicate = el; }} type="button" className={OPTION_CLASS} onClick={() => setStep('duplicate')}>
              <OptionBody icon={Copy} title={t('templates.chooser.duplicate.option')} description={t('templates.chooser.duplicate.optionDescription')} trailing={listTrailing} />
            </button>
            <button type="button" className={OPTION_CLASS} onClick={() => fileRef.current?.click()}>
              <OptionBody icon={FileJson} title={t('templates.chooser.import.title')} description={t('templates.chooser.import.description')} />
            </button>
            <input ref={fileRef} type="file" accept=".json,application/json" className="hidden" onChange={(e) => void openFile(e.target.files?.[0])} />
          </div>
        )}
        {step === 'preset' && (
          <>
            <div ref={presetListRef} className="custom-scrollbar -mx-1 min-h-0 flex-1 overflow-y-auto px-1 py-0.5">
              <PresetList />
            </div>
            {back}
          </>
        )}
        {step === 'duplicate' && (
          <>
            <DuplicateList templates={templates} loading={loading} />
            {back}
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
