import { useEffect, useId, useState } from 'react';
import { ArrowUpToLine, Folder, Home, Loader2 } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { normalizeFolder, parentOf } from './file-move';

type Props = Readonly<{
  /** the names being moved; null while the dialog is closed */
  names: string[] | null;
  /** the folder they are in */
  currentPath: string;
  /** folders inside the current one, offered as destinations */
  subfolders: string[];
  moving: boolean;
  onMove: (destination: string) => void;
  onClose: () => void;
}>;

/**
 * Where to move one item or a selection. The root and the folder above are one click away (the
 * usual move: out of a folder an archive was unpacked into); any other folder is typed.
 */
export function MoveDialog({ names, currentPath, subfolders, moving, onMove, onClose }: Props) {
  const { t } = useTranslations();
  const inputId = useId();
  const listId = useId();
  const [value, setValue] = useState('');

  // each opening starts from the folder above, the most likely destination
  useEffect(() => {
    if (names) setValue(currentPath ? `~/${parentOf(currentPath)}` : '~/');
  }, [names, currentPath]);

  const destination = normalizeFolder(value);
  const unchanged = destination === currentPath;
  const invalid = destination === null;
  const canMove = !!names && !invalid && !unchanged && !moving;

  const quick = [
    currentPath ? { label: t('servers.fileManager.moveDialog.parent'), path: parentOf(currentPath), icon: ArrowUpToLine } : null,
    { label: t('servers.fileManager.moveDialog.root'), path: '', icon: Home },
  ].filter((q): q is { label: string; path: string; icon: typeof Home } => q !== null);

  const title = names?.length === 1
    ? t('servers.fileManager.moveDialog.titleOne', { name: names[0] })
    : t('servers.fileManager.moveDialog.titleMany', { count: names?.length ?? 0 });

  return (
    <Dialog open={!!names} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle className="break-all">{title}</DialogTitle>
          <DialogDescription>{t('servers.fileManager.moveDialog.description')}</DialogDescription>
        </DialogHeader>

        <div className="space-y-3 py-2">
          <div className="flex flex-wrap gap-2">
            {quick.map(({ label, path, icon: Icon }) => {
              const active = destination === path;
              return (
                <Button
                  key={label}
                  type="button"
                  variant="outline"
                  size="sm"
                  aria-pressed={active}
                  onClick={() => setValue(`~/${path}`)}
                  className={cn('h-8 text-xs', active && 'border-primary/50 bg-primary/[0.07] text-primary-bright')}
                >
                  <Icon aria-hidden="true" />
                  {label}
                </Button>
              );
            })}
          </div>

          <div className="space-y-2">
            <Label htmlFor={inputId}>{t('servers.fileManager.moveDialog.label')}</Label>
            <div className="relative">
              <Folder className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-chart-5" aria-hidden="true" />
              <Input
                id={inputId}
                value={value}
                onChange={(e) => setValue(e.target.value)}
                list={listId}
                className="pl-9 font-mono"
                placeholder="~/plugins"
                autoFocus
                aria-invalid={invalid}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && canMove && destination !== null) onMove(destination);
                }}
              />
              <datalist id={listId}>
                {subfolders.map((folder) => <option key={folder} value={`~/${folder}`} />)}
              </datalist>
            </div>
            <p className={cn('text-xs', invalid ? 'text-destructive-bright' : 'text-muted-foreground')}>
              {invalid ? t('servers.fileManager.moveDialog.invalid') : t('servers.fileManager.moveDialog.hint')}
            </p>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>{t('common.cancel')}</Button>
          <Button onClick={() => destination !== null && onMove(destination)} disabled={!canMove}>
            {moving && <Loader2 className="animate-spin" aria-hidden="true" />}
            {t('servers.fileManager.moveDialog.move')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
