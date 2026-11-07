import { useRef, useState } from 'react';
import { Textarea } from '@/components/ui/textarea';
import { AlertCircle, AlertTriangle } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { INVALID_CONTROL } from '../field';
import { isPlainObject, pathKey, type Path } from '../paths';
import type { EditorIssue } from '../validation';

// One step as JSON: for a type the panel has no form for (a newer node's operation, a typo), a
// step that is not even an object, or whoever prefers to write it. Every edit that parses to an
// object is applied at once, as in the template's JSON view; while the text does not parse the
// step keeps its last valid version and the text says why it is not applied.

const stringify = (value: unknown) => JSON.stringify(value, null, 2) ?? '';

type Problem = { code: 'syntax'; detail: string } | { code: 'notObject' };

export function StepJsonEditor({ id, step, stepPath, issues, onApply }: Readonly<{
  id: string;
  step: unknown;
  stepPath: Path;
  /** Every issue under the step, shown with where in the step it is. */
  issues: readonly EditorIssue[];
  onApply: (step: Record<string, unknown>) => void;
}>) {
  const { t } = useTranslations();
  const [text, setText] = useState(() => stringify(step));
  const [problem, setProblem] = useState<Problem | null>(null);
  // the object this editor wrote last: when it comes back as the step, the text stays as typed
  const written = useRef<unknown>(step);
  const [shown, setShown] = useState(step);
  if (shown !== step) {
    setShown(step);
    // changed elsewhere (the form, the template's JSON view): the text follows
    if (step !== written.current) {
      setText(stringify(step));
      setProblem(null);
    }
  }

  const onText = (next: string) => {
    setText(next);
    let parsed: unknown;
    try {
      parsed = JSON.parse(next);
    } catch (e) {
      setProblem({ code: 'syntax', detail: e instanceof Error ? e.message : String(e) });
      return;
    }
    if (!isPlainObject(parsed)) {
      setProblem({ code: 'notObject' });
      return;
    }
    setProblem(null);
    written.current = parsed;
    onApply(parsed);
  };

  const prefix = pathKey(stepPath);
  const problemId = `${id}-problem`;
  const issuesListId = `${id}-issues`;
  const lines = text.split('\n').length;

  return (
    <div className="space-y-2">
      <Textarea
        id={id}
        className={cn('min-h-[8rem] font-mono text-[13px] leading-relaxed', INVALID_CONTROL)}
        rows={Math.min(22, Math.max(6, lines + 1))}
        value={text}
        onChange={(e) => onText(e.target.value)}
        aria-label={t('templates.editor.steps.json.label')}
        aria-invalid={problem !== null}
        aria-describedby={`${problemId} ${issuesListId}`}
        spellCheck={false}
        autoCapitalize="off"
        autoComplete="off"
      />
      {/* only whether the text parses is announced, once per change; the detail (which moves
          with every keystroke) and the issues are read through aria-describedby */}
      <p className="sr-only" aria-live="polite">
        {problem ? t('templates.editor.steps.json.invalidState') : ''}
      </p>
      <p id={problemId} className="empty:hidden">
        {problem && (
          <span className="flex items-start gap-1.5 text-pretty text-xs text-destructive-bright">
            <AlertCircle className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
            <span className="min-w-0 break-words">
              {problem.code === 'syntax' ? t('templates.editor.steps.json.syntax', { detail: problem.detail }) : t('templates.editor.steps.json.notObject')}
            </span>
          </span>
        )}
      </p>
      <ul id={issuesListId} className="space-y-1 empty:hidden">
        {issues.map((issue, index) => {
          const isError = issue.severity === 'error';
          const Icon = isError ? AlertCircle : AlertTriangle;
          // where in the step: "files.0", or nothing for the step itself
          const where = issue.path === prefix ? '' : issue.path.slice(prefix.length + 1);
          return (
            <li key={`${index}-${issue.path}-${issue.messageKey}`} className={cn('flex items-start gap-1.5 text-pretty text-xs', isError ? 'text-destructive-bright' : 'text-warning')}>
              <Icon className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              <span className="min-w-0 break-words">
                {where && <code className="mr-1.5 font-mono">{where}</code>}
                {t(issue.messageKey, issue.params)}
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
