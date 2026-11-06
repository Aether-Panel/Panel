import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import * as Document from './document';
import { localTemplateNames } from './load';
import { getIn, pathKey, type JsonObject, type Path } from './paths';
import { issuesByPath, validateTemplate, type EditorIssue } from './validation';
import type { EditorSectionId } from './sections';

// The editor's shared state. Every section reads and writes the one template object through
// useTemplateEditor(); nothing keeps its own copy, so the form, the JSON view and the validation
// always agree. The template is the raw object (see document.ts), never the schema's parse output.

export type EditorView = 'form' | 'json' | 'split';

/** A text field (or the JSON editor) that "Insertar variable" and similar helpers type into. */
export interface InsertTarget {
  insert: (text: string) => void;
}

export interface TemplateEditorValue {
  template: JsonObject;
  jsonText: string;
  jsonProblem: Document.JsonProblem | null;
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

  /** Called by a field on focus; helpers then insert into the field last focused. */
  setInsertTarget: (target: InsertTarget) => void;
  /** Called by a field that goes away; it stops being the target unless another took over. */
  releaseInsertTarget: (target: InsertTarget) => void;
  /** Inserts `text` at the caret of the field last focused; false when there is none. */
  insertIntoField: (text: string) => boolean;
}

const TemplateEditorContext = createContext<TemplateEditorValue | null>(null);

export function useTemplateEditor(): TemplateEditorValue {
  const value = useContext(TemplateEditorContext);
  if (!value) throw new Error('useTemplateEditor must be used inside TemplateEditorProvider');
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
  const [view, setView] = useState<EditorView>('form');
  const [section, setSection] = useState<EditorSectionId>('info');
  const insertTarget = useRef<InsertTarget | null>(null);

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

  const goTo = useCallback((target: EditorSectionId, path?: Path) => {
    setSection(target);
    setView((v) => (v === 'json' ? 'form' : v));
    if (!path) return;
    // after the section has rendered
    requestAnimationFrame(() => {
      const el = document.getElementById(fieldId(path));
      el?.scrollIntoView({ block: 'center', behavior: 'smooth' });
      el?.focus({ preventScroll: true });
    });
  }, []);

  const setInsertTarget = useCallback((target: InsertTarget) => {
    insertTarget.current = target;
  }, []);
  const releaseInsertTarget = useCallback((target: InsertTarget) => {
    if (insertTarget.current === target) insertTarget.current = null;
  }, []);
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
      dirty: Document.isDirty(doc),
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
      setInsertTarget,
      releaseInsertTarget,
      insertIntoField,
    }),
    [doc, originalName, takenNames, recheckNames, update, remove, change, setJsonText, markSaved, issues, issuesAt, view, section, goTo, setInsertTarget, releaseInsertTarget, insertIntoField],
  );

  return <TemplateEditorContext.Provider value={value}>{children}</TemplateEditorContext.Provider>;
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
  const { update, setInsertTarget, releaseInsertTarget } = useTemplateEditor();
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
