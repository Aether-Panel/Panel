import { DiffEditor } from '@monaco-editor/react';
import { useEffect } from 'react';
import { ensureMonacoStylesheet, markMonacoInUse } from '@/lib/monaco-setup';
import { useTranslations } from '@/contexts/translations-context';
// importing the editor module sets up the bundled Monaco and its workers once for both editors
import { EDITOR_BACKGROUND, EDITOR_FONT_FAMILY, EditorLoading, registerTokyoNightTheme } from './code-editor';

interface FileDiffEditorProps {
  language: string;
  /** The stored version, on the left. */
  original: string;
  /** The editor's current text, unsaved edits included, on the right. */
  modified: string;
}

/**
 * Read-only comparison of a stored version with the current text. Side by side when there is
 * room, a single inline column on narrow screens; long runs of unchanged lines fold away so
 * the changes in a big config stand out.
 */
export default function FileDiffEditor({ language, original, modified }: FileDiffEditorProps) {
  const { t } = useTranslations();

  useEffect(() => {
    markMonacoInUse();
    void ensureMonacoStylesheet();
  }, []);

  const handleMount = (editor: any, monaco: any) => {
    registerTokyoNightTheme(monaco);
    monaco.editor.setTheme('tokyo-night');

    // The models are kept by the wrapper (keepCurrent*Model) and released here, after the
    // widget lets go of them; disposing them first makes Monaco throw on close.
    const models = editor.getModel();
    editor.onDidDispose(() => {
      window.setTimeout(() => {
        models?.original?.dispose();
        models?.modified?.dispose();
      }, 0);
    });

    // the surrounding view slides in, so measure again once it has settled
    requestAnimationFrame(() => {
      try {
        editor.layout();
      } catch {
        /* already disposed */
      }
    });
  };

  return (
    <div className="monaco-editor-host relative h-full w-full overflow-hidden" style={{ backgroundColor: EDITOR_BACKGROUND }}>
      <DiffEditor
        height="100%"
        width="100%"
        language={language}
        original={original}
        modified={modified}
        theme="tokyo-night"
        beforeMount={registerTokyoNightTheme}
        onMount={handleMount}
        keepCurrentOriginalModel
        keepCurrentModifiedModel
        loading={<EditorLoading label={t('servers.fileHistory.diff.loading')} />}
        options={{
          readOnly: true,
          originalEditable: false,
          domReadOnly: true,
          automaticLayout: true,
          renderSideBySide: true,
          useInlineViewWhenSpaceIsLimited: true,
          renderSideBySideInlineBreakpoint: 760,
          hideUnchangedRegions: { enabled: true, contextLineCount: 3, minimumLineCount: 6, revealLineCount: 20 },
          renderOverviewRuler: false,
          ignoreTrimWhitespace: false,
          enableSplitViewResizing: true,
          renderIndicators: true,
          diffWordWrap: 'on',
          wordWrap: 'on',
          fontSize: 14,
          fontFamily: EDITOR_FONT_FAMILY,
          fontLigatures: true,
          minimap: { enabled: false },
          lineNumbers: 'on',
          scrollBeyondLastLine: false,
          smoothScrolling: true,
          renderLineHighlight: 'none',
          padding: { top: 8, bottom: 8 },
          scrollbar: {
            verticalScrollbarSize: 8,
            horizontalScrollbarSize: 8,
            useShadows: false,
          },
        }}
      />
    </div>
  );
}
