'use client';
import { Fragment, useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { ArrowUp, Bot, Eraser, Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { cn } from '@/lib/utils';

type Message = { role: 'user' | 'assistant'; content: string; sources?: string[]; error?: boolean };

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
      {items.map((item, j) => <li key={j}>{renderInline(item, `${key}-${j}`)}</li>)}
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
        {paragraph.map((l, j) => (
          <Fragment key={j}>
            {j > 0 && <br />}
            {renderInline(l, `${key}-${j}`)}
          </Fragment>
        ))}
      </p>,
    );
  }
  return <div className="space-y-2.5">{blocks}</div>;
}

function storageKey(serverId: string) {
  return `assistant_${serverId}`;
}

/**
 * The server assistant: a side panel with a chat that looks at the server's console,
 * crashes, performance and players to answer in plain words. Opened from the server header.
 */
export default function AssistantPanel({ serverId, serverName }: Readonly<{ serverId: string; serverName: string }>) {
  const { t, language } = useTranslations();
  const ids = useId();
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);
  const [draft, setDraft] = useState('');
  const [thinking, setThinking] = useState(false);
  const listRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  // the conversation survives switching tabs in the same browser session
  useEffect(() => {
    try {
      const stored = sessionStorage.getItem(storageKey(serverId));
      if (stored) setMessages(JSON.parse(stored));
    } catch {
      setMessages([]);
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
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages, thinking]);

  const send = async (text: string) => {
    const question = text.trim();
    if (!question || thinking) return;
    const history = [...messages.filter((m) => !m.error), { role: 'user' as const, content: question }];
    setMessages((current) => [...current, { role: 'user', content: question }]);
    setDraft('');
    setThinking(true);
    try {
      const res = await api.post<{ reply: string; sources?: string[] }>(`/api/servers/${serverId}/ai/chat`, {
        messages: history.slice(-20).map(({ role, content }) => ({ role, content })),
        language,
      });
      setMessages((current) => [...current, { role: 'assistant', content: res.reply, sources: res.sources ?? [] }]);
    } catch (err) {
      let content = t('servers.assistant.errors.generic');
      if (err instanceof ApiError && err.status === 429) content = t('servers.assistant.errors.limit');
      else if (err instanceof ApiError && err.status === 403) content = t('servers.assistant.errors.noPermission');
      else if (err instanceof ApiError && err.status === 400 && /api key/i.test(err.message)) content = t('servers.assistant.errors.notConfigured');
      setMessages((current) => [...current, { role: 'assistant', content, error: true }]);
    } finally {
      setThinking(false);
      inputRef.current?.focus();
    }
  };

  const onKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
      event.preventDefault();
      void send(draft);
    }
  };

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
        // on a desktop the question box gets the focus; on phones the keyboard waits for a tap
        onOpenAutoFocus={(event) => {
          if (window.matchMedia('(min-width: 640px)').matches) {
            event.preventDefault();
            inputRef.current?.focus();
          }
        }}
      >
        <SheetHeader className="space-y-1 border-b border-border/50 px-5 pb-4 pt-5 text-left">
          <SheetTitle className="flex items-center gap-2.5 pr-8 text-base">
            <span
              aria-hidden="true"
              className="flex h-8 w-8 items-center justify-center rounded-lg text-white shadow-[0_6px_20px_-6px_hsl(var(--primary)/0.8)]"
              style={{ background: 'linear-gradient(135deg, hsl(var(--primary)), #8B5CF6)' }}
            >
              <Bot className="h-4 w-4" />
            </span>
            {t('servers.assistant.title')}
          </SheetTitle>
          <SheetDescription className="text-xs">{t('servers.assistant.description', { name: serverName })}</SheetDescription>
        </SheetHeader>

        <div ref={listRef} className="custom-scrollbar min-h-0 flex-1 overflow-y-auto px-5 py-5" aria-live="polite" aria-busy={thinking}>
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
                  key={index}
                  className={cn('flex animate-in fade-in-0 slide-in-from-bottom-1 duration-300 ease-panel', message.role === 'user' ? 'justify-end' : 'justify-start')}
                >
                  {message.role === 'user' ? (
                    <p className="max-w-[85%] whitespace-pre-wrap break-words rounded-2xl rounded-br-md bg-primary/15 px-3.5 py-2 text-sm text-foreground">
                      {message.content}
                    </p>
                  ) : (
                    <div className={cn('max-w-[92%] space-y-2 text-sm leading-relaxed', message.error ? 'text-destructive-bright' : 'text-muted-foreground')}>
                      {message.error ? <p>{message.content}</p> : <Markdown text={message.content} />}
                      {message.sources && message.sources.length > 0 && (
                        <p className="text-[11px] text-muted-foreground/80">
                          {t('servers.assistant.looked', { sources: message.sources.map((s) => t(`servers.assistant.sources.${s}`)).join(', ') })}
                        </p>
                      )}
                    </div>
                  )}
                </li>
              ))}
              {thinking && (
                <li className="flex items-center gap-1.5 py-1" aria-label={t('servers.assistant.thinking')}>
                  {[0, 1, 2].map((dot) => (
                    <span
                      key={dot}
                      aria-hidden="true"
                      // a soft staggered pulse, not a bounce
                      className="h-1.5 w-1.5 animate-pulse rounded-full bg-primary [animation-duration:1.2s]"
                      style={{ animationDelay: `${dot * 140}ms` }}
                    />
                  ))}
                  <span className="sr-only">{t('servers.assistant.thinking')}</span>
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
          <div className="relative rounded-xl border border-border/60 bg-background/40 transition-colors duration-200 focus-within:border-primary/50 focus-within:shadow-[0_0_0_3px_hsl(var(--primary)/0.12)]">
            <textarea
              id={`${ids}-question`}
              ref={inputRef}
              rows={2}
              value={draft}
              maxLength={MAX_LENGTH}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={onKeyDown}
              placeholder={t('servers.assistant.placeholder')}
              className="block max-h-40 min-h-[3.25rem] w-full resize-none bg-transparent px-3.5 py-3 pr-12 text-sm text-foreground outline-none placeholder:text-muted-foreground/60"
            />
            <Button
              type="submit"
              size="icon"
              disabled={thinking || draft.trim() === ''}
              aria-label={t('servers.assistant.send')}
              className="absolute bottom-2 right-2 h-8 w-8 rounded-lg"
            >
              <ArrowUp aria-hidden="true" />
            </Button>
          </div>
          <div className="mt-2 flex items-center justify-between gap-3 text-[11px] text-muted-foreground">
            <span>{t('servers.assistant.hint')}</span>
            {messages.length > 0 && (
              <button type="button" onClick={() => setMessages([])} className="focus-ring inline-flex items-center gap-1 rounded px-1 hover:text-foreground">
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
