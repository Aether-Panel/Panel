import { useRef, useState } from 'react';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { AlertCircle } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { FieldIssues, INVALID_CONTROL, issuesId } from './field';
import { isPlainObject } from './paths';
import type { EditorIssue } from './validation';

// One object of the template written as JSON, where a form would only copy Docker's own API
// (hostConfig, config). Every edit that parses to an object is applied at once; while the text
// does not parse, the template keeps the last valid object and the text says why. Emptying the
// text removes the key.

const stringify = (value: unknown) => (value === undefined ? '' : (JSON.stringify(value, null, 2) ?? ''));

type Problem = { code: 'syntax'; detail: string } | { code: 'notObject' };

export function JsonBlock({ id, label, hint, value, issues, onApply }: Readonly<{
  id: string;
  label: string;
  hint?: string;
  value: unknown;
  issues: readonly EditorIssue[];
  /** The new object; undefined when the text was emptied. */
  onApply: (value: Record<string, unknown> | undefined) => void;
}>) {
  const { t } = useTranslations();
  const [text, setText] = useState(() => stringify(value));
  const [problem, setProblem] = useState<Problem | null>(null);
  // the value this block wrote last: when it comes back, the text stays as typed
  const written = useRef<unknown>(value);
  const [shown, setShown] = useState(value);
  if (shown !== value) {
    setShown(value);
    // changed elsewhere (the template's JSON view): the text follows
    if (value !== written.current) {
      setText(stringify(value));
      setProblem(null);
    }
  }

  const onText = (next: string) => {
    setText(next);
    if (next.trim() === '') {
      setProblem(null);
      written.current = undefined;
      onApply(undefined);
      return;
    }
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

  const hintId = `${id}-hint`;
  const problemId = `${id}-problem`;
  const lines = text.split('\n').length;

  return (
    <div className="space-y-2">
      <Label htmlFor={id} className="text-sm font-medium">{label}</Label>
      <Textarea
        id={id}
        className={cn('min-h-[5rem] font-mono text-[13px] leading-relaxed', INVALID_CONTROL)}
        rows={Math.min(16, Math.max(3, lines + 1))}
        value={text}
        onChange={(e) => onText(e.target.value)}
        placeholder="{ }"
        aria-invalid={problem !== null || issues.some((i) => i.severity === 'error')}
        aria-describedby={[hint ? hintId : '', problemId, issuesId(id)].filter(Boolean).join(' ')}
        spellCheck={false}
        autoCapitalize="off"
        autoComplete="off"
      />
      {hint && <p id={hintId} className="text-pretty text-xs text-muted-foreground">{hint}</p>}
      {/* only whether the text parses is announced; the detail moves with every keystroke */}
      <p className="sr-only" aria-live="polite">
        {problem ? t('templates.editor.jsonBlock.invalidState') : ''}
      </p>
      <p id={problemId} className="empty:hidden">
        {problem && (
          <span className="flex items-start gap-1.5 text-pretty text-xs text-destructive-bright">
            <AlertCircle className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
            <span className="min-w-0 break-words">
              {problem.code === 'syntax' ? t('templates.editor.jsonBlock.syntax', { detail: problem.detail }) : t('templates.editor.jsonBlock.notObject')}
            </span>
          </span>
        )}
      </p>
      <FieldIssues id={issuesId(id)} issues={issues} />
    </div>
  );
}
