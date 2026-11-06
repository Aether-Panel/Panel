import type { ReactNode } from 'react';
import { Label } from '@/components/ui/label';
import { AlertCircle, AlertTriangle } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import type { EditorIssue } from './validation';

/** An invalid control gets the error colour on its border, next to the message under it. */
export const INVALID_CONTROL = 'aria-[invalid=true]:border-destructive/70';

/** A value as a text input shows it: a string as is, nothing for a missing one, else its text. */
export function textValue(value: unknown): string {
  if (typeof value === 'string') return value;
  return value == null ? '' : String(value);
}

/** The id of the element listing a field's issues, for the control's aria-describedby. */
export const issuesId = (fieldId: string) => `${fieldId}-issues`;
const hintId = (fieldId: string) => `${fieldId}-hint`;

/**
 * A field's errors and warnings under it, each with its icon so colour is not the only cue.
 * The list is a polite live region, always present (empty takes no room) so screen readers
 * announce a message as soon as the value turns invalid.
 */
export function FieldIssues({ id, issues }: Readonly<{ id: string; issues: readonly EditorIssue[] }>) {
  const { t } = useTranslations();
  return (
    <ul id={id} aria-live="polite" className="space-y-1 empty:!mt-0">
      {issues.map((issue) => {
        const isError = issue.severity === 'error';
        const Icon = isError ? AlertCircle : AlertTriangle;
        return (
          <li
            key={`${issue.messageKey}-${issue.path}`}
            className={cn('flex items-start gap-1.5 text-pretty text-xs', isError ? 'text-destructive-bright' : 'text-warning')}
          >
            <Icon className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
            <span className="min-w-0 break-words">{t(issue.messageKey, issue.params)}</span>
          </li>
        );
      })}
    </ul>
  );
}

/** What a control inside TemplateField spreads on itself to be labelled and described. */
export interface FieldControlProps {
  id: string;
  'aria-invalid': boolean;
  'aria-describedby': string;
}

/**
 * Label, control, help text and issues of one template value, stacked. The control is given as
 * a function of the props that tie it to the label, the help and the issues.
 */
export function TemplateField({ id, label, hint, issues, className, children }: Readonly<{
  id: string;
  label: ReactNode;
  hint?: ReactNode;
  issues: readonly EditorIssue[];
  className?: string;
  children: (props: FieldControlProps) => ReactNode;
}>) {
  const describedBy = [hint ? hintId(id) : '', issuesId(id)].filter(Boolean).join(' ');
  return (
    <div className={cn('space-y-2', className)}>
      <Label htmlFor={id} className="text-sm font-medium">{label}</Label>
      {children({ id, 'aria-invalid': issues.some((i) => i.severity === 'error'), 'aria-describedby': describedBy })}
      {hint && <p id={hintId(id)} className="text-pretty text-xs text-muted-foreground">{hint}</p>}
      <FieldIssues id={issuesId(id)} issues={issues} />
    </div>
  );
}
