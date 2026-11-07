import { useTranslations } from '@/contexts/translations-context';
import { StepList } from './step-list';
import { StepListFold } from './step-list-fold';

// Instalación: the steps that set up a server's files when it is installed (or reinstalled), and,
// folded below, the optional ones that clean up when it is uninstalled.

export function InstallSection() {
  const { t } = useTranslations();
  return (
    <div className="divide-y divide-border/50">
      <div className="space-y-3 px-3 py-4 sm:p-5 md:p-6">
        <StepList
          path={['install']}
          label={t('templates.editor.install.title')}
          emptyTitle={t('templates.editor.install.empty.title')}
          emptyDescription={t('templates.editor.install.empty.description')}
        />
      </div>
      <StepListFold
        path={['uninstall']}
        title={t('templates.editor.install.uninstall.title')}
        note={t('templates.editor.install.uninstall.optional')}
        description={t('templates.editor.install.uninstall.description')}
        emptyTitle={t('templates.editor.install.uninstall.empty.title')}
        emptyDescription={t('templates.editor.install.uninstall.empty.description')}
      />
    </div>
  );
}
