import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { TemplateLogo } from '@/components/template-logo';
import { LayoutTemplate, Loader2, RefreshCw } from '@/components/icons';
import { SettingsSection } from '@/features/settings/general-tab';
import { useTranslations } from '@/contexts/translations-context';
import { TEMPLATE_LOGO_KEYS, templateLogo } from '@/lib/template-logos';
import { INVALID_CONTROL, TemplateField, textValue } from './field';
import { useTemplateEditor, useTemplateField } from './editor-context';

// Categories the community templates use, offered while typing; any other text is fine too.
const CATEGORY_SUGGESTIONS = ['minecraft-java', 'minecraft-bedrock', 'srcds', 'terraria', 'discord', 'nodejs', 'python', 'java', 'go', 'php', 'dotnet', 'mariadb', 'postgresql', 'mongodb', 'redis', 'voice'];

// Radix Select has no empty value, so "no icon field" goes by this one
const AUTOMATIC_ICON = '__automatic__';

function IconPicker() {
  const { t } = useTranslations();
  const { template } = useTemplateEditor();
  const icon = useTemplateField<string>(['icon']);
  const current = textValue(icon.value);
  // an icon set by hand that has no logo here still shows, so picking nothing does not lose it
  const keys = current && !TEMPLATE_LOGO_KEYS.includes(current) ? [current, ...TEMPLATE_LOGO_KEYS] : TEMPLATE_LOGO_KEYS;
  const preview = templateLogo({ name: textValue(template.name), icon: current });

  return (
    <TemplateField id={icon.id} label={t('templates.editor.info.icon.label')} hint={t('templates.editor.info.icon.hint')} issues={icon.issues}>
      {(props) => (
        <div className="flex items-center gap-3">
          <span className="grid h-10 w-10 shrink-0 place-items-center overflow-hidden rounded-lg border border-border/60 bg-secondary text-muted-foreground" aria-hidden="true">
            <TemplateLogo logo={preview} className="h-full w-full rounded-none ring-0" fallback={<LayoutTemplate className="h-5 w-5" />} />
          </span>
          <Select value={current || AUTOMATIC_ICON} onValueChange={(v) => icon.set(v === AUTOMATIC_ICON ? undefined : v)}>
            <SelectTrigger {...props} className="min-w-0 flex-1">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={AUTOMATIC_ICON}>{t('templates.editor.info.icon.automatic')}</SelectItem>
              {keys.map((key) => (
                <SelectItem key={key} value={key}>
                  <span className="flex items-center gap-2">
                    <TemplateLogo logo={templateLogo({ icon: key })} className="h-5 w-5" fallback={<span className="h-5 w-5" />} />
                    <span className="font-mono text-[13px]">{key}</span>
                  </span>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}
    </TemplateField>
  );
}

/** Shown while the local templates' names are unknown: saving waits until they are read again. */
function RecheckNames() {
  const { t } = useTranslations();
  const { recheckNames } = useTemplateEditor();
  const [checking, setChecking] = useState(false);
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      disabled={checking}
      onClick={async () => {
        setChecking(true);
        try {
          await recheckNames();
        } finally {
          setChecking(false);
        }
      }}
    >
      {checking ? <Loader2 className="animate-spin" aria-hidden="true" /> : <RefreshCw aria-hidden="true" />}
      {t('templates.editor.info.name.recheck')}
    </Button>
  );
}

/** Información: the template's name (its id), what users see, its category and its icon. */
export function InfoSection() {
  const { t } = useTranslations();
  const { originalName, namesUnchecked } = useTemplateEditor();
  const name = useTemplateField<string>(['name']);
  const display = useTemplateField<string>(['display']);
  const type = useTemplateField<string>(['type']);

  const nameHint = originalName
    ? t('templates.editor.info.name.hintRename', { name: originalName })
    : t('templates.editor.info.name.hint');

  return (
    <div className="divide-y divide-border/50">
      <SettingsSection title={t('templates.editor.info.identity.title')} description={t('templates.editor.info.identity.description')}>
        <div className="space-y-5">
          <TemplateField id={name.id} label={t('templates.editor.info.name.label')} hint={nameHint} issues={name.issues}>
            {(props) => (
              <Input
                {...props}
                className={`font-mono ${INVALID_CONTROL}`}
                value={textValue(name.value)}
                onChange={(e) => name.set(e.target.value)}
                placeholder={t('templates.editor.info.name.placeholder')}
                autoComplete="off"
                spellCheck={false}
                required
              />
            )}
          </TemplateField>
          {namesUnchecked && <RecheckNames />}
          <TemplateField id={display.id} label={t('templates.editor.info.display.label')} hint={t('templates.editor.info.display.hint')} issues={display.issues}>
            {(props) => (
              <Input
                {...props}
                className={INVALID_CONTROL}
                value={textValue(display.value)}
                onChange={(e) => display.set(e.target.value)}
                placeholder={t('templates.editor.info.display.placeholder')}
                autoComplete="off"
              />
            )}
          </TemplateField>
        </div>
      </SettingsSection>
      <SettingsSection title={t('templates.editor.info.appearance.title')} description={t('templates.editor.info.appearance.description')}>
        <div className="space-y-5">
          <TemplateField id={type.id} label={t('templates.editor.info.type.label')} hint={t('templates.editor.info.type.hint')} issues={type.issues}>
            {(props) => (
              <>
                <Input
                  {...props}
                  className={`font-mono ${INVALID_CONTROL}`}
                  value={textValue(type.value)}
                  onChange={(e) => type.set(e.target.value)}
                  placeholder={t('templates.editor.info.type.placeholder')}
                  list={`${type.id}-suggestions`}
                  autoComplete="off"
                  spellCheck={false}
                />
                <datalist id={`${type.id}-suggestions`}>
                  {CATEGORY_SUGGESTIONS.map((category) => <option key={category} value={category} />)}
                </datalist>
              </>
            )}
          </TemplateField>
          <IconPicker />
        </div>
      </SettingsSection>
    </div>
  );
}
