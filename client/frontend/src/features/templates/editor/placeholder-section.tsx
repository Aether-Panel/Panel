import { Button } from '@/components/ui/button';
import { EmptyState } from '@/components/empty-state';
import { FileJson } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { useTemplateEditor } from './editor-context';
import type { EditorSection } from './sections';

/** A section whose form is not built yet: its part of the template is edited in the JSON view. */
export function PlaceholderSection({ section }: Readonly<{ section: EditorSection }>) {
  const { t } = useTranslations();
  const { setView } = useTemplateEditor();
  return (
    <div className="p-5 md:p-6">
      <EmptyState
        icon={section.icon}
        size="sm"
        title={t('templates.editor.placeholder.title')}
        description={t('templates.editor.placeholder.description', { section: t(section.labelKey) })}
        action={
          <Button variant="outline" size="sm" onClick={() => setView('json')}>
            <FileJson aria-hidden="true" />
            {t('templates.editor.placeholder.openJson')}
          </Button>
        }
      />
    </div>
  );
}
