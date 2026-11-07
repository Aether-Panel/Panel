import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { EmptyState } from '@/components/empty-state';
import ErrorBoundary from '@/components/ErrorBoundary';
import { AlertCircle, Check, CheckCircle2, Eye, Info, RotateCcw } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import {
  initialTemplateValues, isResourceKey, RESOURCE_DEFAULTS, Step3Configuration, useTemplateFieldErrors, variableFieldId,
} from '@/features/servers/create-server-stepper';
import { cn } from '@/lib/utils';
import { isPlainObject, type JsonObject } from '../paths';

// "Vista previa": the "Crear servidor" wizard's configuration step for the template being
// edited, drawn by the wizard's own component (Step3Configuration) with its own starting values
// and checks, so it is exactly what a server creator gets. Everything typed stays in this
// sheet: nothing is sent and nothing is created. Loaded with React.lazy the first time it opens.

type Limit = number | '';

/** Whether the create form asks anything about the template: a variable that is not internal nor a resource. */
function asksForVariables(template: JsonObject): boolean {
  const variables = isPlainObject(template.data) ? template.data : {};
  return Object.entries(variables).some(([key, v]) => isPlainObject(v) && !v.internal && !isResourceKey(key));
}

/** The form with the wizard's state, started from the template as the wizard starts it. */
function PreviewForm({ template, onReset }: Readonly<{ template: JsonObject; onReset: () => void }>) {
  const { t } = useTranslations();
  const [start] = useState(() => initialTemplateValues(template));
  const limits = { ...RESOURCE_DEFAULTS, ...start.limits };
  const [configData, setConfigData] = useState<Record<string, unknown>>(start.values);
  const [cpuLimit, setCpuLimit] = useState<Limit>(limits.cpu);
  const [memoryLimit, setMemoryLimit] = useState<Limit>(limits.memory);
  const [diskLimit, setDiskLimit] = useState<Limit>(limits.disk);
  const [ioLimit, setIoLimit] = useState<Limit>(limits.io);
  const [extraPorts, setExtraPorts] = useState<string[]>([]);
  const { fieldErrors, check } = useTemplateFieldErrors(template, configData);
  // the result of the last check, shown while the values it checked are still the ones in the form
  const [checked, setChecked] = useState<{ values: Record<string, unknown>; errors: number } | null>(null);

  const runCheck = () => {
    // the same rules as "Crear servidor", which also moves the focus to the first field to fix
    const invalid = check();
    setChecked({ values: configData, errors: invalid.length });
    if (invalid.length > 0) document.getElementById(variableFieldId(invalid[0]))?.focus();
  };

  const result = checked && checked.values === configData ? checked : null;
  let resultText = '';
  if (result) {
    if (result.errors === 0) resultText = t('templates.editor.preview.checkOk');
    else resultText = result.errors === 1 ? t('templates.editor.preview.checkErrorsOne') : t('templates.editor.preview.checkErrorsMany', { count: result.errors });
  }

  return (
    <>
      <div className="custom-scrollbar min-h-0 flex-1 space-y-5 overflow-y-auto px-5 py-5 md:px-6">
        <p className="flex items-start gap-2 rounded-lg border border-border/60 bg-background/40 px-3 py-2.5 text-sm text-muted-foreground">
          <Info className="mt-0.5 h-4 w-4 shrink-0 text-primary-bright" aria-hidden="true" />
          <span className="text-pretty">
            {t('templates.editor.preview.note')}
            {!asksForVariables(template) && <> {t('templates.editor.preview.noVariables')}</>}
          </span>
        </p>
        <ErrorBoundary
          name="template-preview"
          fallback={
            <EmptyState
              icon={AlertCircle}
              tone="error"
              size="sm"
              title={t('templates.editor.preview.failedTitle')}
              description={t('templates.editor.preview.failedDescription')}
            />
          }
        >
          <Step3Configuration
            templateDetails={template}
            templateError={null}
            configData={configData}
            setConfigData={setConfigData}
            fieldErrors={fieldErrors}
            cpuLimit={cpuLimit}
            setCpuLimit={setCpuLimit}
            memoryLimit={memoryLimit}
            setMemoryLimit={setMemoryLimit}
            diskLimit={diskLimit}
            setDiskLimit={setDiskLimit}
            ioLimit={ioLimit}
            setIoLimit={setIoLimit}
            extraPorts={extraPorts}
            setExtraPorts={setExtraPorts}
            // no node: the wizard's checks against a node (free ports, memory, disk) stay off
            nodeId=""
            nodeInfo={null}
            recommendedMemory={start.recommendedMemory}
            setCurrentStep={() => undefined}
            t={t}
          />
        </ErrorBoundary>
      </div>
      <div className="flex flex-wrap items-center justify-end gap-x-3 gap-y-2 border-t border-border/50 px-5 py-3 md:px-6">
        <p aria-live="polite" className={cn('mr-auto flex min-w-0 items-center gap-1.5 text-sm', result?.errors ? 'text-destructive-bright' : 'text-muted-foreground')}>
          {result && (result.errors ? <AlertCircle className="h-4 w-4 shrink-0" aria-hidden="true" /> : <CheckCircle2 className="h-4 w-4 shrink-0 text-success" aria-hidden="true" />)}
          <span className="text-pretty">{resultText}</span>
        </p>
        <div className="flex shrink-0 items-center gap-2">
          <Button variant="ghost" onClick={onReset}>
            <RotateCcw aria-hidden="true" />
            {t('templates.editor.preview.reset')}
          </Button>
          <Button onClick={runCheck}>
            <Check aria-hidden="true" />
            {t('templates.editor.preview.check')}
          </Button>
        </div>
      </div>
    </>
  );
}

export default function TemplatePreviewSheet({ open, onOpenChange, template }: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The template as it is in the editor (the last valid one while the JSON is broken). */
  template: JsonObject;
}>) {
  const { t } = useTranslations();
  // a new key starts the form over from the template's values
  const [resetKey, setResetKey] = useState(0);
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      {/* as wide as the wizard's dialog, so its grid lays out the same; full width on a phone */}
      <SheetContent side="right" closeLabel={t('common.close')} className="flex w-full flex-col gap-0 p-0 sm:max-w-3xl">
        <SheetHeader className="space-y-1 border-b border-border/50 px-5 pb-4 pt-5 text-left md:px-6">
          <SheetTitle className="flex items-center gap-2.5 pr-8 text-base">
            <span aria-hidden="true" className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-border/60 bg-background/30 text-muted-foreground">
              <Eye className="h-4 w-4" />
            </span>
            {t('templates.editor.preview.title')}
          </SheetTitle>
          <SheetDescription>{t('templates.editor.preview.description')}</SheetDescription>
        </SheetHeader>
        <PreviewForm key={resetKey} template={template} onReset={() => setResetKey((k) => k + 1)} />
      </SheetContent>
    </Sheet>
  );
}
