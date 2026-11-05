import { Copy } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { useTranslations } from '@/contexts/translations-context';
import { sileo } from '@/lib/toast';

/** A command or snippet to paste somewhere else, with a button that copies it. */
export function CopyableCode({ command, copiedMessage }: Readonly<{ command: string; copiedMessage?: string }>) {
  const { t } = useTranslations();
  const handleCopy = () => {
    void navigator.clipboard.writeText(command);
    sileo.success({ title: copiedMessage ?? t('common.copied') });
  };

  return (
    <div className="relative rounded-lg border border-border/60 bg-background/50">
      <pre className="custom-scrollbar overflow-x-auto p-3 pr-12 font-mono text-xs leading-relaxed text-foreground"><code>{command}</code></pre>
      <Button
        variant="ghost"
        size="icon"
        className="absolute right-1 top-1 h-9 w-9 text-muted-foreground hover:text-foreground"
        onClick={handleCopy}
        aria-label={t('common.copy')}
      >
        <Copy aria-hidden="true" />
      </Button>
    </div>
  );
}
