'use client';
import { Fragment, useEffect, useId, useRef, useState, type ReactNode } from 'react';
import {
  AlertTriangle,
  ArrowUp,
  Ban,
  Check,
  Clock,
  Eraser,
  FilePlus,
  FileText,
  Loader2,
  Pencil,
  RotateCcw,
  ShieldAlert,
  Sparkles,
  Trash2,
  X,
  type AppIcon,
} from '@/components/icons';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';
import { keyedByContent } from '@/lib/keyed';

type Translate = (key: string, options?: { [key: string]: string | number }) => string;

type StepKind = 'read' | 'create' | 'modify' | 'destructive';
type StepStatus = 'done' | 'failed' | 'pending' | 'declined' | 'refused';

/** One tool call the assistant made (or wants to make) on the server's files. */
type Step = {
  id: string;
  tool: string;
  kind: StepKind;
  summary: string;
  /** Server-relative paths; the panel never sends host paths. */
  paths?: string[];
  status: StepStatus;
  error?: string;
};

/** Changes waiting for the user's approval; the token answers them once, before expiresAt. */
type Pending = { token: string; actions: Step[]; expiresAt: string };

/** What both /ai/chat and /ai/chat/confirm answer. */
type ChatResponse = { reply: string; sources?: string[]; steps?: Step[]; pending?: Pending | null };

type Message = {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  sources?: string[];
  steps?: Step[];
  error?: boolean;
  /** On a failed question: true when it can be sent again as it was. */
  retry?: boolean;
};

// ids only need to be unique within one conversation; crypto.randomUUID is missing on plain-HTTP panels
let messageSeq = 0;
const messageId = () => `${Date.now().toString(36)}-${messageSeq++}`;

const MAX_LENGTH = 2000;
const SUGGESTIONS = ['slow', 'crash', 'error', 'restart'] as const;

/* ------------------------------------------------------------------ */
/* A small, safe Markdown renderer: the assistant writes **bold**,     */
/* `code`, lists and code blocks. Everything becomes React elements,   */
/* never raw HTML.                                                     */
/* ------------------------------------------------------------------ */

function renderInline(text: string, keyPrefix: string): ReactNode[] {
  const parts: ReactNode[] = [];
  const pattern = /(\*\*[^*]+\*\*|`[^`]+`)/g;
  let last = 0;
  let match: RegExpExecArray | null;
  let i = 0;
  while ((match = pattern.exec(text)) !== null) {
    if (match.index > last) parts.push(text.slice(last, match.index));
    const token = match[0];
    if (token.startsWith('**')) {
      parts.push(<strong key={`${keyPrefix}-${i++}`} className="font-semibold text-foreground">{token.slice(2, -2)}</strong>);
    } else {
      parts.push(<code key={`${keyPrefix}-${i++}`} className="rounded bg-background/70 px-1 py-0.5 font-mono text-[12px] text-foreground">{token.slice(1, -1)}</code>);
    }
    last = match.index + token.length;
  }
  if (last < text.length) parts.push(text.slice(last));
  return parts;
}

const BULLET = /^\s*[-*•]\s+/;
const NUMBERED = /^\s*\d+[.)]\s+/;

function isFence(line: string) {
  return line.trim().startsWith('```');
}

/** A fenced code block opening at `start`; `next` is the line after the closing fence. */
function readFence(lines: string[], start: number): { code: string[]; next: number } {
  const code: string[] = [];
  let i = start + 1;
  while (i < lines.length && !isFence(lines[i])) code.push(lines[i++]);
  return { code, next: i + 1 };
}

/** The run of list items starting at `start`, all of the first item's kind. */
function readList(lines: string[], start: number): { ordered: boolean; items: string[]; next: number } {
  const ordered = NUMBERED.test(lines[start]);
  const marker = ordered ? NUMBERED : BULLET;
  const items: string[] = [];
  let i = start;
  while (i < lines.length && marker.test(lines[i])) {
    items.push(lines[i].replace(marker, ''));
    i++;
  }
  return { ordered, items, next: i };
}

/** Consecutive text lines up to a blank line, a list or a code block. */
function readParagraph(lines: string[], start: number): { paragraph: string[]; next: number } {
  const paragraph: string[] = [];
  let i = start;
  while (i < lines.length && lines[i].trim() !== '' && !BULLET.test(lines[i]) && !NUMBERED.test(lines[i]) && !isFence(lines[i])) {
    paragraph.push(lines[i++]);
  }
  return { paragraph, next: i };
}

function renderList(ordered: boolean, items: string[], key: number): ReactNode {
  const List = ordered ? 'ol' : 'ul';
  return (
    <List key={key} className={cn('space-y-1 pl-5', ordered ? 'list-decimal' : 'list-disc marker:text-primary')}>
      {keyedByContent(items).map(({ key: itemKey, item }, j) => <li key={itemKey}>{renderInline(item, `${key}-${j}`)}</li>)}
    </List>
  );
}

function Markdown({ text }: Readonly<{ text: string }>) {
  const blocks: ReactNode[] = [];
  const lines = text.replaceAll('\r', '').split('\n');
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    const key = blocks.length;
    if (isFence(line)) {
      const { code, next } = readFence(lines, i);
      i = next;
      blocks.push(
        <pre key={key} className="custom-scrollbar overflow-x-auto rounded-lg border border-border/60 bg-background/70 px-3 py-2 font-mono text-[12px] leading-5 text-foreground">
          {code.join('\n')}
        </pre>,
      );
      continue;
    }
    if (BULLET.test(line) || NUMBERED.test(line)) {
      const { ordered, items, next } = readList(lines, i);
      i = next;
      blocks.push(renderList(ordered, items, key));
      continue;
    }
    if (line.trim() === '') {
      i++;
      continue;
    }
    // one whitespace in the pattern and the rest trimmed: the same capture, without backtracking
    const heading = /^#{1,4}\s(.*)$/.exec(line);
    if (heading) {
      blocks.push(<p key={key} className="font-semibold text-foreground">{renderInline(heading[1].trimStart(), String(key))}</p>);
      i++;
      continue;
    }
    const { paragraph, next } = readParagraph(lines, i);
    i = next;
    blocks.push(
      <p key={key}>
        {keyedByContent(paragraph).map(({ key: lineKey, item: l }, j) => (
          <Fragment key={lineKey}>
            {j > 0 && <br />}
            {renderInline(l, `${key}-${j}`)}
          </Fragment>
        ))}
      </p>,
    );
  }
  return <div className="space-y-2.5">{blocks}</div>;
}

/* ------------------------------------------------------------------ */
/* File work: the steps the assistant took and the changes it asks     */
/* to make. Status and kind always read as text, never colour alone.   */
/* ------------------------------------------------------------------ */

const KIND_ICONS: Record<StepKind, AppIcon> = {
  read: FileText,
  create: FilePlus,
  modify: Pencil,
  destructive: Trash2,
};

const STATUS_STYLES: Record<StepStatus, { icon: AppIcon; className: string }> = {
  done: { icon: Check, className: 'border-success/30 bg-success/10 text-success' },
  failed: { icon: X, className: 'border-destructive/30 bg-destructive/10 text-destructive-bright' },
  pending: { icon: Clock, className: 'border-warning/30 bg-warning/10 text-warning' },
  declined: { icon: Ban, className: 'border-border/60 bg-background/50 text-muted-foreground' },
  refused: { icon: ShieldAlert, className: 'border-warning/30 bg-warning/10 text-warning' },
};

/** The server folder itself reaches the panel as "."; people read it as "/". */
function displayPath(path: string): string {
  return path === '.' || path === '' ? '/' : path;
}

/**
 * What a step does, in the reader's language. The backend's summary is English (it also goes
 * to the model and the logs), so other languages get a translated phrase from the tool and its
 * paths; English keeps the backend's more detailed sentence.
 */
function stepText(step: Step, t: Translate, language: string): string {
  if (language.startsWith('en') && step.summary) return step.summary;
  const key = `servers.assistant.tools.${step.tool}`;
  const text = t(key, { path: displayPath(step.paths?.[0] ?? ''), dest: displayPath(step.paths?.[1] ?? '') });
  return text === key ? step.summary : text;
}

function PathList({ paths }: Readonly<{ paths?: string[] }>) {
  if (!paths || paths.length === 0) return null;
  return (
    <span className="flex flex-wrap gap-1">
      {keyedByContent(paths).map(({ key, item }) => (
        <code key={key} className="max-w-full break-all rounded bg-background/70 px-1 py-0.5 font-mono text-[11px] text-foreground">
          {displayPath(item)}
        </code>
      ))}
    </span>
  );
}

function StatusBadge({ status, t }: Readonly<{ status: StepStatus; t: Translate }>) {
  // a status the panel does not know yet still shows, as a neutral badge with its name
  const style = STATUS_STYLES[status] ?? STATUS_STYLES.declined;
  const Icon = style.icon;
  const label = STATUS_STYLES[status] ? t(`servers.assistant.steps.status.${status}`) : status;
  return (
    <span className={cn('inline-flex shrink-0 items-center gap-1 rounded-full border px-1.5 py-0.5 text-[10px] font-medium leading-none', style.className)}>
      <Icon className="h-2.5 w-2.5" aria-hidden="true" />
      {label}
    </span>
  );
}

/** The compact list of what the assistant did, under its answer. */
function StepList({ steps, t }: Readonly<{ steps: Step[]; t: Translate }>) {
  const { language } = useTranslations();
  return (
    <ul aria-label={t('servers.assistant.steps.label')} className="space-y-2 rounded-lg border border-border/50 bg-background/30 px-2.5 py-2">
      {steps.map((step) => {
        const Icon = KIND_ICONS[step.kind] ?? FileText;
        return (
          <li key={step.id} className="flex items-start gap-2 text-xs leading-5">
            <Icon aria-hidden="true" className={cn('mt-0.5 h-3.5 w-3.5 shrink-0', step.kind === 'destructive' ? 'text-destructive-bright' : 'text-muted-foreground')} />
            <span className="min-w-0 flex-1 space-y-1">
              <span className="block break-words text-foreground/90">{stepText(step, t, language)}</span>
              <PathList paths={step.paths} />
              {step.error && <span className="block break-words text-destructive-bright">{step.error}</span>}
            </span>
            <StatusBadge status={step.status} t={t} />
          </li>
        );
      })}
    </ul>
  );
}

/** A failed request, said plainly: that it could not be done, why, and whether changes were
 * made before it stopped (those steps are listed under it). The last one can be asked again. */
function FailureReport({ message, canRetry, onRetry, t }: Readonly<{ message: Message; canRetry: boolean; onRetry: () => void; t: Translate }>) {
  const changed = (message.steps ?? []).some((step) => step.status === 'done' && step.kind !== 'read');
  return (
    <div role="alert" className="space-y-2 rounded-xl border border-destructive/30 bg-destructive/[0.06] px-3.5 py-3">
      <p className="flex items-start gap-2 font-medium text-destructive-bright">
        <AlertTriangle aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0" />
        {t('servers.assistant.errors.title')}
      </p>
      <p className="break-words text-foreground/90">{message.content}</p>
      {changed && <p className="text-xs">{t('servers.assistant.errors.changesKept')}</p>}
      {canRetry && (
        <Button type="button" size="sm" variant="outline" onClick={onRetry}>
          <RotateCcw aria-hidden="true" />
          {t('servers.assistant.errors.retry')}
        </Button>
      )}
    </div>
  );
}

type PendingCardProps = {
  pending: Pending;
  approved: ReadonlySet<string>;
  onToggle: (id: string, checked: boolean) => void;
  onAnswer: (approved: string[]) => void;
  confirming: boolean;
  cardRef: React.RefObject<HTMLElement | null>;
  idPrefix: string;
  t: Translate;
};

/** The changes the assistant wants to make: each one can be unticked; nothing runs until Approve. */
function PendingCard({ pending, approved, onToggle, onAnswer, confirming, cardRef, idPrefix, t }: Readonly<PendingCardProps>) {
  const { language } = useTranslations();
  const titleId = `${idPrefix}-pending-title`;
  return (
    <section
      ref={cardRef}
      tabIndex={-1}
      aria-labelledby={titleId}
      className="focus-ring space-y-3 rounded-xl border border-primary/30 bg-primary/[0.05] p-3.5 outline-none animate-in fade-in-0 slide-in-from-bottom-1 duration-300 ease-panel"
    >
      <div className="space-y-0.5">
        <h3 id={titleId} className="text-sm font-semibold text-foreground">{t('servers.assistant.pending.title')}</h3>
        <p className="text-xs text-muted-foreground">{t('servers.assistant.pending.description')}</p>
      </div>
      <ul className="space-y-2">
        {pending.actions.map((action, index) => {
          const destructive = action.kind === 'destructive';
          const Icon = KIND_ICONS[action.kind] ?? FileText;
          const checkId = `${idPrefix}-approve-${index}`;
          return (
            <li
              key={action.id}
              className={cn('flex items-start gap-2.5 rounded-lg border px-3 py-2.5', destructive ? 'border-destructive/40 bg-destructive/[0.07]' : 'border-border/60 bg-background/40')}
            >
              <Checkbox
                id={checkId}
                checked={approved.has(action.id)}
                onCheckedChange={(value) => onToggle(action.id, value === true)}
                disabled={confirming}
                className="mt-0.5"
              />
              <label htmlFor={checkId} className="min-w-0 flex-1 cursor-pointer space-y-1 text-sm leading-5">
                <span className="flex items-start gap-1.5">
                  <Icon aria-hidden="true" className={cn('mt-0.5 h-3.5 w-3.5 shrink-0', destructive ? 'text-destructive-bright' : 'text-muted-foreground')} />
                  <span className="min-w-0 break-words text-foreground">{stepText(action, t, language)}</span>
                </span>
                {destructive && (
                  <span className="inline-flex items-center gap-1 text-[11px] font-medium text-destructive-bright">
                    <AlertTriangle className="h-3 w-3" aria-hidden="true" />
                    {t('servers.assistant.pending.destructive')}
                  </span>
                )}
                <PathList paths={action.paths} />
              </label>
            </li>
          );
        })}
      </ul>
      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button type="button" size="sm" variant="outline" disabled={confirming} onClick={() => onAnswer([])}>
          {t('servers.assistant.pending.declineAll')}
        </Button>
        <Button
          type="button"
          size="sm"
          disabled={confirming || approved.size === 0}
          aria-busy={confirming}
          // keep the order the assistant listed them in
          onClick={() => onAnswer(pending.actions.filter((a) => approved.has(a.id)).map((a) => a.id))}
        >
          {confirming ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Check aria-hidden="true" />}
          {t('servers.assistant.pending.approve')}
        </Button>
      </div>
    </section>
  );
}

/** The panel's error code, when the API sent one ({error: {code, msg}}). */
function errorCode(err: ApiError): string | undefined {
  const data: unknown = err.data;
  if (data === null || typeof data !== 'object' || !('error' in data)) return undefined;
  const error: unknown = (data as { error: unknown }).error;
  if (error === null || typeof error !== 'object' || !('code' in error)) return undefined;
  const code: unknown = (error as { code: unknown }).code;
  return typeof code === 'string' ? code : undefined;
}

/** The reasons the panel gives for a failed chat ({error, reason}); each has its own text. */
const FAILURE_REASONS = new Set([
  'notConfigured',
  'keyRejected',
  'quota',
  'unavailable',
  'unreachable',
  'rejected',
  'unreadable',
  'empty',
  'tooLong',
  'blocked',
  'malformed',
  'timeout',
  'failed',
]);

/** The reason code a failed chat came with, if any. */
function failureReason(err: ApiError): string | undefined {
  const data: unknown = err.data;
  if (data === null || typeof data !== 'object' || !('reason' in data)) return undefined;
  const reason: unknown = (data as { reason: unknown }).reason;
  return typeof reason === 'string' ? reason : undefined;
}

/** What went wrong with a request, in the reader's language: from the panel's reason when it
 * gave one, else from the status, else (no answer at all) the connection. */
function failureText(err: unknown, t: Translate): string {
  if (!(err instanceof ApiError)) {
    const offline = typeof navigator !== 'undefined' && navigator.onLine === false;
    return t(`servers.assistant.errors.reasons.${offline ? 'offline' : 'connection'}`);
  }
  const reason = failureReason(err);
  if (reason && FAILURE_REASONS.has(reason)) return t(`servers.assistant.errors.reasons.${reason}`);
  if (err.status === 429) return t('servers.assistant.errors.limit');
  if (err.status === 403) return t('servers.assistant.errors.noPermission');
  if (err.status === 401) return t('servers.assistant.errors.reasons.session');
  if (err.status === 400 && /api key/i.test(err.message)) return t('servers.assistant.errors.notConfigured');
  // a proxy in front of the panel answers these when the panel is down or restarting
  if ([502, 503, 504].includes(err.status)) return t('servers.assistant.errors.reasons.panelUnavailable', { status: err.status });
  return t('servers.assistant.errors.reasons.status', { status: err.status });
}

/** Whether asking the same again can work; a missing permission or key will not change by itself. */
function canRetry(err: unknown): boolean {
  if (!(err instanceof ApiError)) return true;
  if ([400, 401, 403].includes(err.status)) return false;
  const reason = failureReason(err);
  return reason !== 'keyRejected' && reason !== 'notConfigured';
}

/** The request can no longer be answered: it expired, was already answered, or belongs to someone else. */
function isExpiredConfirmation(err: unknown): boolean {
  if (!(err instanceof ApiError)) return false;
  return errorCode(err) === 'ErrAIConfirmationInvalid' || err.status === 409;
}

function storageKey(serverId: string) {
  return `assistant_${serverId}`;
}

function pendingStorageKey(serverId: string) {
  return `assistant_${serverId}_pending`;
}

function isPending(value: unknown): value is Pending {
  if (value === null || typeof value !== 'object') return false;
  const p = value as Partial<Pending>;
  return typeof p.token === 'string' && Array.isArray(p.actions) && p.actions.length > 0;
}

/**
 * The server assistant: a side panel with a chat that looks at the server's console,
 * crashes, performance and players to answer in plain words. Opened from the server header.
 * When the panel lets it work with files, it lists what it did under each answer and asks
 * before changing or deleting anything.
 */
export default function AssistantPanel({ serverId, serverName }: Readonly<{ serverId: string; serverName: string }>) {
  const { t, language } = useTranslations();
  const ids = useId();
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);
  const [draft, setDraft] = useState('');
  const [thinking, setThinking] = useState(false);
  const [pending, setPending] = useState<Pending | null>(null);
  const [approved, setApproved] = useState<Set<string>>(new Set());
  const [confirming, setConfirming] = useState(false);
  const listRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const cardRef = useRef<HTMLElement>(null);
  const wasBusy = useRef(false);

  const busy = thinking || confirming;
  // nothing new can be asked while a request runs or the assistant waits for an answer
  const inputLocked = busy || pending !== null;

  // the conversation survives switching tabs in the same browser session
  useEffect(() => {
    try {
      const stored = sessionStorage.getItem(storageKey(serverId));
      // conversations saved before messages had ids get them now
      if (stored) setMessages((JSON.parse(stored) as Message[]).map((m) => (m.id ? m : { ...m, id: messageId() })));
      const storedPending: unknown = JSON.parse(sessionStorage.getItem(pendingStorageKey(serverId)) ?? 'null');
      setPending(isPending(storedPending) ? storedPending : null);
    } catch {
      setMessages([]);
      setPending(null);
    }
  }, [serverId]);
  useEffect(() => {
    try {
      sessionStorage.setItem(storageKey(serverId), JSON.stringify(messages.slice(-20)));
    } catch {
      // storage blocked: the chat simply starts empty next time
    }
  }, [messages, serverId]);
  useEffect(() => {
    try {
      if (pending) sessionStorage.setItem(pendingStorageKey(serverId), JSON.stringify(pending));
      else sessionStorage.removeItem(pendingStorageKey(serverId));
    } catch {
      // storage blocked: an unanswered request is simply gone next time
    }
  }, [pending, serverId]);

  // every action starts ticked; a new request resets the choice
  useEffect(() => {
    setApproved(new Set(pending?.actions.map((a) => a.id) ?? []));
  }, [pending]);

  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages, busy, pending]);

  // a new request takes the focus, so keyboard and screen reader users land on it
  useEffect(() => {
    if (pending) cardRef.current?.focus();
  }, [pending]);

  // back to the question box once an answer arrives (it is disabled while waiting, so this
  // runs after it is enabled again rather than in the request itself)
  useEffect(() => {
    if (wasBusy.current && !busy && !pending) inputRef.current?.focus();
    wasBusy.current = busy;
  }, [busy, pending]);

  /** Reports a failure twice: a toast, seen wherever the chat is scrolled, and a message in
   * the chat that stays, with the steps done before it stopped. */
  const addError = (content: string, err?: unknown, retry = false) => {
    // a task that failed half way still did some work: its steps come with the error and are
    // shown, so the user sees what changed before it stopped
    const steps = err instanceof ApiError && Array.isArray(err.data?.steps) ? (err.data.steps as Step[]) : [];
    sileo.error({ title: t('servers.assistant.errors.title'), description: content });
    setMessages((current) => [...current, { id: messageId(), role: 'assistant', content, error: true, steps, retry }]);
  };

  const addAnswer = (res: ChatResponse) => {
    setMessages((current) => [
      ...current,
      { id: messageId(), role: 'assistant', content: res.reply ?? '', sources: res.sources ?? [], steps: res.steps ?? [] },
    ]);
    setPending(isPending(res.pending) ? res.pending : null);
  };

  /** Asks a question; with `retryOf`, the failed answer of that id is removed and the
   * question, already in the conversation, is asked again. */
  const send = async (text: string, retryOf?: string) => {
    const question = text.trim();
    if (!question || inputLocked) return;
    // only the text of the conversation goes back; steps and confirmations stay in the panel
    const said = messages.filter((m) => !m.error && m.content.trim() !== '');
    const history = retryOf ? said : [...said, { role: 'user' as const, content: question }];
    if (retryOf) {
      setMessages((current) => current.filter((m) => m.id !== retryOf));
    } else {
      setMessages((current) => [...current, { id: messageId(), role: 'user', content: question }]);
      setDraft('');
    }
    setThinking(true);
    try {
      const res = await api.post<ChatResponse>(`/api/servers/${serverId}/ai/chat`, {
        messages: history.slice(-20).map(({ role, content }) => ({ role, content })),
        language,
      });
      addAnswer(res);
    } catch (err) {
      addError(failureText(err, t), err, canRetry(err));
    } finally {
      setThinking(false);
    }
  };

  /** Asks again the question a failed answer belongs to: the last one before it. */
  const retryFailed = (failedId: string) => {
    const index = messages.findIndex((m) => m.id === failedId);
    const question = messages.slice(0, index).findLast((m) => m.role === 'user');
    if (question) void send(question.content, failedId);
  };

  /** Answers the pending request: the IDs to run (an empty list declines them all). */
  const answer = async (approvedIds: string[]) => {
    if (!pending || busy) return;
    setConfirming(true);
    try {
      const res = await api.post<ChatResponse>(`/api/servers/${serverId}/ai/chat/confirm`, {
        token: pending.token,
        approved: approvedIds,
        language,
      });
      addAnswer(res);
    } catch (err) {
      if (isExpiredConfirmation(err)) {
        setPending(null);
        addError(t('servers.assistant.errors.expired'));
      } else {
        // without permission it cannot be answered any more; anything else can be tried again
        if (err instanceof ApiError && err.status === 403) setPending(null);
        addError(failureText(err, t), err);
      }
    } finally {
      setConfirming(false);
    }
  };

  const toggleApproved = (id: string, checked: boolean) => {
    setApproved((current) => {
      const next = new Set(current);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  };

  const onKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
      event.preventDefault();
      void send(draft);
    }
  };

  const clearConversation = () => {
    setMessages([]);
    setPending(null);
  };

  let busyLabel = '';
  if (confirming) busyLabel = t('servers.assistant.pending.working');
  else if (thinking) busyLabel = t('servers.assistant.thinking');

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Button size="sm" variant="outline" className="group gap-2 border-primary/30 bg-primary/[0.06] hover:border-primary/50 hover:bg-primary/10">
          <Sparkles aria-hidden="true" className="text-primary-bright transition-transform duration-300 ease-panel group-hover:rotate-12 group-hover:scale-110" />
          {t('servers.assistant.open')}
        </Button>
      </SheetTrigger>
      <SheetContent
        side="right"
        closeLabel={t('common.close')}
        className="flex w-full flex-col gap-0 p-0 sm:max-w-md"
        // a request waiting for an answer gets the focus; otherwise, on a desktop, the question
        // box does; on phones the keyboard waits for a tap
        onOpenAutoFocus={(event) => {
          if (pending) {
            event.preventDefault();
            cardRef.current?.focus();
          } else if (window.matchMedia('(min-width: 640px)').matches) {
            event.preventDefault();
            inputRef.current?.focus();
          }
        }}
      >
        <SheetHeader className="space-y-1 border-b border-border/50 px-5 pb-4 pt-5 text-left">
          <SheetTitle className="pr-8 text-base">
            {t('servers.assistant.title')}
          </SheetTitle>
          <SheetDescription className="text-xs">{t('servers.assistant.description', { name: serverName })}</SheetDescription>
        </SheetHeader>

        <div ref={listRef} className="custom-scrollbar min-h-0 flex-1 overflow-y-auto px-5 py-5" aria-live="polite" aria-busy={busy}>
          {messages.length === 0 ? (
            <div className="flex h-full flex-col justify-end gap-4">
              <div className="space-y-1.5">
                <p className="text-sm font-semibold text-foreground">{t('servers.assistant.emptyTitle')}</p>
                <p className="text-sm text-muted-foreground">{t('servers.assistant.emptyDescription')}</p>
              </div>
              <div className="flex flex-col gap-2">
                {SUGGESTIONS.map((key, index) => (
                  <button
                    key={key}
                    type="button"
                    onClick={() => void send(t(`servers.assistant.suggestions.${key}`))}
                    className="focus-ring rounded-xl border border-border/60 bg-background/30 px-3.5 py-2.5 text-left text-sm text-foreground transition-colors duration-200 hover:border-primary/40 hover:bg-primary/[0.06] animate-in fade-in-0 slide-in-from-bottom-1 fill-mode-both duration-300 ease-panel"
                    style={{ animationDelay: `${index * 60}ms` }}
                  >
                    {t(`servers.assistant.suggestions.${key}`)}
                  </button>
                ))}
              </div>
            </div>
          ) : (
            <ol className="space-y-4">
              {messages.map((message, index) => (
                <li
                  key={message.id}
                  className={cn('flex animate-in fade-in-0 slide-in-from-bottom-1 duration-300 ease-panel', message.role === 'user' ? 'justify-end' : 'justify-start')}
                >
                  {message.role === 'user' ? (
                    <p className="max-w-[85%] whitespace-pre-wrap break-words rounded-2xl rounded-br-md bg-primary/15 px-3.5 py-2 text-sm text-foreground">
                      {message.content}
                    </p>
                  ) : (
                    <div className="min-w-0 max-w-[92%] space-y-2 text-sm leading-relaxed text-muted-foreground">
                      {message.error && (
                        <FailureReport
                          message={message}
                          canRetry={message.retry === true && index === messages.length - 1 && !inputLocked}
                          onRetry={() => retryFailed(message.id)}
                          t={t}
                        />
                      )}
                      {!message.error && message.content.trim() !== '' && <Markdown text={message.content} />}
                      {message.steps && message.steps.length > 0 && <StepList steps={message.steps} t={t} />}
                      {message.sources && message.sources.length > 0 && (
                        <p className="text-[11px] text-muted-foreground/80">
                          {t('servers.assistant.looked', { sources: message.sources.map((s) => t(`servers.assistant.sources.${s}`)).join(', ') })}
                        </p>
                      )}
                    </div>
                  )}
                </li>
              ))}
              {pending && (
                <li>
                  <PendingCard
                    pending={pending}
                    approved={approved}
                    onToggle={toggleApproved}
                    onAnswer={(ids) => void answer(ids)}
                    confirming={confirming}
                    cardRef={cardRef}
                    idPrefix={ids}
                    t={t}
                  />
                </li>
              )}
              {busy && (
                <li className="flex items-center gap-1.5 py-1" aria-label={busyLabel}>
                  {[0, 1, 2].map((dot) => (
                    <span
                      key={dot}
                      aria-hidden="true"
                      // a soft staggered pulse, not a bounce
                      className="h-1.5 w-1.5 animate-pulse rounded-full bg-primary [animation-duration:1.2s]"
                      style={{ animationDelay: `${dot * 140}ms` }}
                    />
                  ))}
                  <span className="sr-only">{busyLabel}</span>
                </li>
              )}
            </ol>
          )}
        </div>

        <form
          className="border-t border-border/50 p-4"
          onSubmit={(event) => {
            event.preventDefault();
            void send(draft);
          }}
        >
          <label htmlFor={`${ids}-question`} className="sr-only">{t('servers.assistant.placeholder')}</label>
          {/* a row, not an absolutely placed button: the primary button's sheen sets position: relative */}
          <div
            className={cn(
              'flex items-end gap-2 rounded-xl border border-border/60 bg-background/40 pr-2 transition-colors duration-200 focus-within:border-primary/50 focus-within:shadow-[0_0_0_3px_hsl(var(--primary)/0.12)]',
              inputLocked && 'opacity-60',
            )}
          >
            <textarea
              id={`${ids}-question`}
              ref={inputRef}
              rows={2}
              value={draft}
              maxLength={MAX_LENGTH}
              disabled={inputLocked}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={onKeyDown}
              placeholder={pending ? t('servers.assistant.pending.inputLocked') : t('servers.assistant.placeholder')}
              className="block max-h-40 min-h-[3.25rem] min-w-0 flex-1 resize-none bg-transparent px-3.5 py-3 text-sm text-foreground outline-none placeholder:text-muted-foreground/60 disabled:cursor-not-allowed"
            />
            <Button
              type="submit"
              size="icon"
              disabled={inputLocked || draft.trim() === ''}
              aria-label={t('servers.assistant.send')}
              className="mb-2 h-8 w-8 shrink-0 rounded-lg"
            >
              <ArrowUp aria-hidden="true" />
            </Button>
          </div>
          <div className="mt-2 flex items-center justify-between gap-3 text-[11px] text-muted-foreground">
            <span>{t('servers.assistant.hint')}</span>
            {messages.length > 0 && (
              <button
                type="button"
                onClick={clearConversation}
                disabled={busy}
                className="focus-ring inline-flex items-center gap-1 rounded px-1 hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
              >
                <Eraser className="h-3 w-3" aria-hidden="true" />
                {t('servers.assistant.clear')}
              </button>
            )}
          </div>
        </form>
      </SheetContent>
    </Sheet>
  );
}
