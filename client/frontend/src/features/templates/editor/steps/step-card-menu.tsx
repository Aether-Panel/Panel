import { useRef } from 'react';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { ArrowDown, ArrowUp, Copy, FileJson, LayoutTemplate, MoreHorizontal, Plus, Trash2 } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { useAnnounceMove } from '../sortable';
import type { StepActions } from './step-card';

/** The card's "…" button, where the focus returns when an action from its menu is cancelled. */
export const stepMenuId = (id: string) => `step-${id}-menu`;

interface StepCardMenuProps {
  /** The card's id. */
  id: string;
  index: number;
  count: number;
  /** "Paso 2: Descargar", what the menu and the move announcements name the step by. */
  numbered: string;
  /** The step has a form, so the menu offers to switch between it and the JSON. */
  hasForm: boolean;
  /** The step shows as JSON now. */
  json: boolean;
  /** Switches between the form and the JSON, opening the card and focusing the editor shown. */
  onToggleJson: () => void;
  actions: StepActions;
}

/** A step card's "…" menu: form or JSON, insert, duplicate, move and delete. */
export function StepCardMenu({ id, index, count, numbered, hasForm, json, onToggleJson, actions }: Readonly<StepCardMenuProps>) {
  const { t } = useTranslations();
  const announceMove = useAnnounceMove();
  // a menu action that moves the focus elsewhere keeps the menu from handing it back as it closes
  const focusMoved = useRef(false);
  const movingFocus = (action: () => void) => () => {
    focusMoved.current = true;
    action();
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          id={stepMenuId(id)}
          variant="ghost"
          size="icon"
          className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground"
          aria-label={t('templates.editor.steps.menu', { step: numbered })}
        >
          <MoreHorizontal aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        onCloseAutoFocus={(e) => {
          if (focusMoved.current) e.preventDefault();
          focusMoved.current = false;
        }}
      >
        {hasForm && (
          <DropdownMenuItem onSelect={movingFocus(onToggleJson)}>
            {json ? <LayoutTemplate className="mr-2 h-4 w-4" aria-hidden="true" /> : <FileJson className="mr-2 h-4 w-4" aria-hidden="true" />}
            {t(json ? 'templates.editor.steps.useForm' : 'templates.editor.steps.editJson')}
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onSelect={movingFocus(() => actions.insertAfter(index))}>
          <Plus className="mr-2 h-4 w-4" aria-hidden="true" />
          {t('templates.editor.steps.insertAfter')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={movingFocus(() => actions.duplicate(index))}>
          <Copy className="mr-2 h-4 w-4" aria-hidden="true" />
          {t('templates.editor.steps.duplicate')}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          disabled={index === 0}
          onSelect={() => {
            actions.move(index, index - 1);
            announceMove(numbered, index, count);
          }}
        >
          <ArrowUp className="mr-2 h-4 w-4" aria-hidden="true" />
          {t('templates.editor.steps.moveUp')}
        </DropdownMenuItem>
        <DropdownMenuItem
          disabled={index === count - 1}
          onSelect={() => {
            actions.move(index, index + 1);
            announceMove(numbered, index + 2, count);
          }}
        >
          <ArrowDown className="mr-2 h-4 w-4" aria-hidden="true" />
          {t('templates.editor.steps.moveDown')}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="text-destructive-bright focus:bg-destructive/10 focus:text-destructive-bright"
          onSelect={movingFocus(() => actions.remove(index))}
        >
          <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
          {t('common.delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
