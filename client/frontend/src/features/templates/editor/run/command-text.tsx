import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import { INVALID_CONTROL, TemplateField, textValue } from '../field';
import { InsertVariableButton } from '../insert-variable';
import { FieldLabel, isEditableScalar, NotEditableHere } from '../steps/field-controls';
import { useInsertTarget } from '../steps/use-insert-target';
import type { EditorIssue } from '../validation';

// One command line (the server's start command, one entry of a conditional list). Start commands
// are long (a java line with its flags), so the box grows with the text and wraps it instead of
// scrolling sideways; Enter does not break the line, since the node runs it as one.

export function CommandText({ id, label, required = false, hint, placeholder, value, issues, onChange }: Readonly<{
  id: string;
  label: string;
  required?: boolean;
  hint?: string;
  placeholder?: string;
  value: unknown;
  issues: readonly EditorIssue[];
  onChange: (text: string) => void;
}>) {
  const { ref, onFocus, insert } = useInsertTarget(onChange);
  if (!isEditableScalar(value)) return <NotEditableHere id={id} label={label} issues={issues} />;
  return (
    <TemplateField id={id} label={<FieldLabel label={label} required={required} />} hint={hint} issues={issues}>
      {(props) => (
        <div className="flex items-start gap-2">
          <Textarea
            {...props}
            ref={ref}
            onFocus={onFocus}
            // as tall as its text where the browser can size it so (field-sizing), else two lines
            className={cn('min-h-[2.75rem] min-w-0 flex-1 resize-y font-mono text-[13px] leading-relaxed [field-sizing:content]', INVALID_CONTROL)}
            rows={2}
            value={textValue(value)}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.nativeEvent.isComposing) e.preventDefault();
            }}
            placeholder={placeholder}
            autoComplete="off"
            autoCapitalize="off"
            spellCheck={false}
          />
          <InsertVariableButton compact onInsert={insert} className="h-10 w-10 shrink-0" />
        </div>
      )}
    </TemplateField>
  );
}
