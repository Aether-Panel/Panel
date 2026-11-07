import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import * as Document from './document';
import { localTemplateNames } from './load';
import { createPendingCommits } from './pending-commits';
import { getIn, pathKey, type JsonObject, type Path } from './paths';
import { issuesByPath, validateTemplate, type EditorIssue } from './validation';
import type { EditorSectionId } from './sections';

// The editor's shared state. Every section reads and writes the one template object through
// useTemplateEditor(); nothing keeps its own copy, so the form, the JSON view and the validation
// always agree. The template is the raw object (see document.ts), never the schema's parse output.

export type EditorView = 'form' | 'json' | 'split';

/**
 * The field goTo() is bringing into view. A section that folds parts away (the variable cards,
 * a step's advanced options) opens the one holding it in a layout effect; `seq` tells two
 * requests for the same path apart. The request is withdrawn (null) once the field has the
 * focus, so a part mounted later (a card the user opens) does not act on it again.
 */
export interface FocusRequest {
  path: Path;
  seq: number;
}

/** A text field (or the JSON editor) that "Insertar variable" and similar helpers type into. */
export interface InsertTarget {
  insert: (text: string) => void;
}

export interface TemplateEditorValue {
  template: JsonObject;
  jsonText: string;
  jsonProblem: Document.JsonProblem | null;
  /** Unsaved changes, counting edits a field holds back until it is left (see registerPendingCommit). */
  dirty: boolean;
  /** The name the template was opened or last saved under; undefined until a new one is saved. */
  originalName: string | undefined;
  /** The local templates' names could not be read, so saving waits for recheckNames(). */
  namesUnchecked: boolean;
  /** Reads the local templates' names again; resolves to whether it worked. */
  recheckNames: () => Promise<boolean>;

  /** Writes `value` at `path` (a key or index list); undefined removes the key. */
  update: (path: Path, value: unknown) => void;
  /** Removes the key or list item at `path`. */
  remove: (path: Path) => void;
  /** Any other immutable change, e.g. moveIn or renameKeyIn from paths.ts. */
  change: (change: (template: JsonObject) => JsonObject) => void;
  setJsonText: (text: string) => void;
  /** Records that `saved` (the template sent) was saved under `name`. */
  markSaved: (name: string, saved: JsonObject) => void;

  issues: readonly EditorIssue[];
  /** The issues about exactly `path`, for the field that edits it. */
  issuesAt: (path: Path) => readonly EditorIssue[];
  /** Whether anything blocks saving: a broken JSON text or a blocking issue. */
  blocked: boolean;

  view: EditorView;
  setView: (view: EditorView) => void;
  section: EditorSectionId;
  /** Shows a section in the form and, given a path, focuses the field editing it. */
  goTo: (section: EditorSectionId, path?: Path) => void;
  focusRequest: FocusRequest | null;

  /** Called by a field on focus; helpers then insert into the field last focused. */
  setInsertTarget: (target: InsertTarget) => void;
  /** Called by a field that goes away; it stops being the target unless another took over. */
  releaseInsertTarget: (target: InsertTarget) => void;
  /** Inserts `text` at the caret of the field last focused; false when there is none. */
  insertIntoField: (text: string) => boolean;

  /**
   * For a field that writes its value only when it is left (a variable's key): while it holds a
   * valid edit not written yet, it registers the function that writes it, and unregisters with
   * the function returned. The edit then counts as unsaved, and saving writes it first.
   */
  registerPendingCommit: (commit: () => void) => () => void;
  /** Writes every held-back edit now. Wrap it in flushSync to read the result right after. */
  flushPendingCommits: () => void;
}

const TemplateEditorContext = createContext<TemplateEditorValue | null>(null);

export function useTemplateEditor(): TemplateEditorValue {
  const value = useContext(TemplateEditorContext);
  if (!value) throw new Error('useTemplateEditor must be used inside TemplateEditorProvider');
  return value;
}

/** The editor's functions that never change, apart from the state they act on. */
export type TemplateEditorActions = Pick<
  TemplateEditorValue,
  'update' | 'remove' | 'change' | 'setView' | 'goTo' | 'setInsertTarget' | 'releaseInsertTarget' | 'insertIntoField' | 'registerPendingCommit' | 'flushPendingCommits'
>;

const TemplateEditorActionsContext = createContext<TemplateEditorActions | null>(null);
const FocusRequestContext = createContext<FocusRequest | null>(null);

/**
 * The pending goTo() request alone: it changes only when goTo() is called, so a memoized part
 * (a step card's advanced options) can react to it without re-rendering on every edit.
 */
export function useFocusRequest(): FocusRequest | null {
  return useContext(FocusRequestContext);
}

/**
 * The editor's stable functions without its state: a component that only writes (a field in a
 * memoized step card) reads these and is not re-rendered by every edit of the template, as a
 * reader of useTemplateEditor() is.
 */
export function useTemplateEditorActions(): TemplateEditorActions {
  const value = useContext(TemplateEditorActionsContext);
  if (!value) throw new Error('useTemplateEditorActions must be used inside TemplateEditorProvider');
  return value;
}

/** The element id of the control editing `path`; goTo() focuses it. */
export function fieldId(path: Path): string {
  return `tpl-${pathKey(path)}`;
}

const NO_ISSUES: readonly EditorIssue[] = [];

export function TemplateEditorProvider({ initialTemplate, saved = true, originalName: openedAs, takenNames: initialTakenNames, children }: Readonly<{
  initialTemplate: JsonObject;
  /** False for a copy or an import: unsaved from the start (see document.ts createDocument). */
  saved?: boolean;
  /** The local template being edited; undefined for a new one. */
  originalName?: string;
  /** Names of the local templates, so a new name cannot overwrite one; null when unknown. */
  takenNames: readonly string[] | null;
  children: ReactNode;
}>) {
  const [doc, setDoc] = useState(() => Document.createDocument(initialTemplate, saved));
  const [originalName, setOriginalName] = useState(openedAs);
  const [takenNames, setTakenNames] = useState(initialTakenNames);
  const [view, setViewState] = useState<EditorView>('form');
  const [section, setSection] = useState<EditorSectionId>('info');
  const [focusRequest, setFocusRequest] = useState<FocusRequest | null>(null);
  const focusSeq = useRef(0);
  const insertTarget = useRef<InsertTarget | null>(null);
  // how many fields hold an edit back, so `dirty` follows them
  const [pendingCount, setPendingCount] = useState(0);
  const [pendingCommits] = useState(() => createPendingCommits(setPendingCount));

  const update = useCallback((path: Path, value: unknown) => setDoc((d) => Document.updateAt(d, path, value)), []);
  const remove = useCallback((path: Path) => setDoc((d) => Document.removeAt(d, path)), []);
  const change = useCallback((fn: (t: JsonObject) => JsonObject) => setDoc((d) => Document.changeTemplate(d, fn)), []);
  const setJsonText = useCallback((text: string) => setDoc((d) => Document.setJsonText(d, text)), []);
  const markSaved = useCallback((name: string, saved: JsonObject) => {
    setDoc((d) => Document.markSaved(d, saved));
    setOriginalName(name);
    // saved under a new name, the template opened before still exists under its old one
    setTakenNames((names) => (names === null || names.includes(name) ? names : [...names, name]));
  }, []);
  const recheckNames = useCallback(async () => {
    const names = await localTemplateNames();
    if (names) setTakenNames(names);
    return names !== null;
  }, []);

  const issues = useMemo(
    () => validateTemplate(doc.template, { takenNames, originalName }),
    [doc.template, takenNames, originalName],
  );
  const byPath = useMemo(() => issuesByPath(issues), [issues]);
  const issuesAt = useCallback((path: Path) => byPath.get(pathKey(path)) ?? NO_ISSUES, [byPath]);

  // leaving a view or a section unmounts its fields, so what they hold back is written first;
  // the writes are state updates queued before the switch, so the next render has both
  const setView = useCallback(
    (next: EditorView) => {
      pendingCommits.flush();
      setViewState(next);
    },
    [pendingCommits],
  );

  const goTo = useCallback((target: EditorSectionId, path?: Path) => {
    pendingCommits.flush();
    setSection(target);
    setViewState((v) => (v === 'json' ? 'form' : v));
    if (!path) return;
    setFocusRequest({ path, seq: ++focusSeq.current });
  }, [pendingCommits]);

  // The field is focused once the request has been rendered: the section is shown and what holds
  // the field has opened (in its layout effects). Doing it from goTo() itself would race React's
  // render when goTo() is not called from a user event. Then the request is withdrawn.
  useEffect(() => {
    if (!focusRequest) return;
    const frame = requestAnimationFrame(() => {
      const el = document.getElementById(fieldId(focusRequest.path));
      el?.scrollIntoView({ block: 'center', behavior: 'smooth' });
      el?.focus({ preventScroll: true });
      setFocusRequest((current) => (current === focusRequest ? null : current));
    });
    return () => cancelAnimationFrame(frame);
  }, [focusRequest]);

  const setInsertTarget = useCallback((target: InsertTarget) => {
    insertTarget.current = target;
  }, []);
  const releaseInsertTarget = useCallback((target: InsertTarget) => {
    if (insertTarget.current === target) insertTarget.current = null;
  }, []);
  const registerPendingCommit = pendingCommits.register;
  const flushPendingCommits = pendingCommits.flush;

  const insertIntoField = useCallback((text: string) => {
    if (!insertTarget.current) return false;
    insertTarget.current.insert(text);
    return true;
  }, []);

  const value = useMemo<TemplateEditorValue>(
    () => ({
      template: doc.template,
      jsonText: doc.jsonText,
      jsonProblem: doc.jsonProblem,
      dirty: Document.isDirty(doc) || pendingCount > 0,
      originalName,
      namesUnchecked: takenNames === null,
      recheckNames,
      update,
      remove,
      change,
      setJsonText,
      markSaved,
      issues,
      issuesAt,
      blocked: doc.jsonProblem !== null || issues.some((i) => i.blocking),
      view,
      setView,
      section,
      goTo,
      focusRequest,
      setInsertTarget,
      releaseInsertTarget,
      insertIntoField,
      registerPendingCommit,
      flushPendingCommits,
    }),
    [doc, originalName, takenNames, recheckNames, update, remove, change, setJsonText, markSaved, issues, issuesAt, view, section, goTo, focusRequest, setInsertTarget, releaseInsertTarget, insertIntoField, pendingCount, registerPendingCommit, flushPendingCommits],
  );

  const actions = useMemo<TemplateEditorActions>(
    () => ({ update, remove, change, setView, goTo, setInsertTarget, releaseInsertTarget, insertIntoField, registerPendingCommit, flushPendingCommits }),
    [update, remove, change, setView, goTo, setInsertTarget, releaseInsertTarget, insertIntoField, registerPendingCommit, flushPendingCommits],
  );

  return (
    <TemplateEditorActionsContext.Provider value={actions}>
      <FocusRequestContext.Provider value={focusRequest}>
        <TemplateEditorContext.Provider value={value}>{children}</TemplateEditorContext.Provider>
      </FocusRequestContext.Provider>
    </TemplateEditorActionsContext.Provider>
  );
}

/** What a form control needs for the value at `path`: its id, value, setter and issues. */
export function useTemplateField<T = unknown>(path: Path) {
  const { template, update, issuesAt } = useTemplateEditor();
  return {
    id: fieldId(path),
    value: getIn(template, path) as T | undefined,
    set: (value: T | undefined) => update(path, value),
    issues: issuesAt(path),
  };
}

/**
 * Makes a text input or textarea a target for inserted text (a `${variable}`): spread `ref` and
 * `onFocus` on it. `insert` types into this field directly; helpers that do not know the field
 * use the editor's insertIntoField(), which goes to the field focused last.
 */
export function useInsertableField(path: Path) {
  // only the stable functions: a field must not re-render for every edit elsewhere
  const { update, setInsertTarget, releaseInsertTarget } = useTemplateEditorActions();
  const ref = useRef<HTMLInputElement & HTMLTextAreaElement>(null);
  const key = pathKey(path);
  const pathRef = useRef(path);
  pathRef.current = path;

  const target = useMemo<InsertTarget>(
    () => ({
      insert(text) {
        const el = ref.current;
        if (!el) return;
        // the DOM value is the controlled value, and the caret survives the click on the helper
        const start = el.selectionStart ?? el.value.length;
        const end = el.selectionEnd ?? start;
        update(pathRef.current, el.value.slice(0, start) + text + el.value.slice(end));
        const caret = start + text.length;
        requestAnimationFrame(() => {
          el.focus();
          el.setSelectionRange(caret, caret);
        });
      },
    }),
    // a new target per field path, so a reused component inserts into the right value
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [key, update],
  );

  useEffect(() => () => releaseInsertTarget(target), [target, releaseInsertTarget]);

  return { ref, onFocus: () => setInsertTarget(target), insert: target.insert };
}
