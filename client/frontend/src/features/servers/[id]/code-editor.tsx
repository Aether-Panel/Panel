import { Editor, loader } from '@monaco-editor/react';
import * as monaco from 'monaco-editor';
import EditorWorker from 'monaco-editor/editor/editor.worker?worker';
import JsonWorker from 'monaco-editor/language/json/json.worker?worker';
import CssWorker from 'monaco-editor/language/css/css.worker?worker';
import HtmlWorker from 'monaco-editor/language/html/html.worker?worker';
import TsWorker from 'monaco-editor/language/typescript/ts.worker?worker';
import { useEffect, useRef } from 'react';
import { Loader2 } from 'lucide-react';
import { ensureMonacoStylesheet, markMonacoInUse } from '@/lib/monaco-setup';
import { useTranslations } from '@/contexts/translations-context';

// Monaco is bundled with the panel instead of loaded from a CDN, so the editor works
// without internet access or with a strict CSP. This chunk only loads when an editor opens.
self.MonacoEnvironment = {
  getWorker(_workerId: string, label: string) {
    switch (label) {
      case 'json':
        return new JsonWorker();
      case 'css':
      case 'scss':
      case 'less':
        return new CssWorker();
      case 'html':
      case 'handlebars':
      case 'razor':
        return new HtmlWorker();
      case 'typescript':
      case 'javascript':
        return new TsWorker();
      default:
        return new EditorWorker();
    }
  },
};

loader.config({ monaco });

const tokyoNight = {
  base: 'vs-dark' as const,
  inherit: true,
  rules: [
    { token: '', foreground: 'c0caf5', background: '1a1b26' },
    { token: 'comment', foreground: '565f89', fontStyle: 'italic' },
    { token: 'key', foreground: '7aa2f7', fontStyle: 'bold' },
    { token: 'metatag', foreground: 'bb9af7', fontStyle: 'bold' },
    { token: 'delimiter', foreground: '89ddff' },
    { token: 'value', foreground: '9ece6a' },
    { token: 'keyword', foreground: 'bb9af7', fontStyle: 'bold' },
    { token: 'keyword.control', foreground: 'f7768e' },
    { token: 'operator', foreground: '89ddff' },
    { token: 'identifier', foreground: '7aa2f7' },
    { token: 'variable', foreground: 'e0af68' },
    { token: 'string', foreground: '9ece6a' },
    { token: 'string.escape', foreground: 'ff9e64' },
    { token: 'number', foreground: 'ff9e64' },
    { token: 'constant', foreground: 'ff9e64' },
    { token: 'type', foreground: '2ac3de' },
    { token: 'function', foreground: '7aa2f7' },
    { token: 'tag', foreground: 'f7768e' },
    { token: 'attribute.name', foreground: 'bb9af7' },
    { token: 'attribute.value', foreground: '9ece6a' },
    { token: 'string.yaml', foreground: '9ece6a' },
    { token: 'string.key.json', foreground: '7aa2f7' },
    { token: 'string.value.json', foreground: '9ece6a' },
    { token: 'number.json', foreground: 'ff9e64' },
    { token: 'keyword.json', foreground: 'ff9e64' },
  ],
  colors: {
    'editor.background': '#1a1b26',
    'editor.foreground': '#c0caf5',
    'editor.lineHighlightBackground': '#1e202e',
    'editor.selectionBackground': '#33467c',
    'editor.inactiveSelectionBackground': '#33467c80',
    'editorCursor.foreground': '#c0caf5',
    'editorWhitespace.foreground': '#414868',
    'editorLineNumber.foreground': '#565f89',
    'editorLineNumber.activeForeground': '#7aa2f7',
    'editorGutter.background': '#1a1b26',
    'editorIndentGuide.background': '#292e42',
    'editorIndentGuide.activeBackground': '#3b4261',
    'editorBracketMatch.background': '#33467c40',
    'editorBracketMatch.border': '#7aa2f780',
    'scrollbar.shadow': '#00000000',
    'scrollbarSlider.background': '#41486840',
    'scrollbarSlider.hoverBackground': '#41486860',
    'scrollbarSlider.activeBackground': '#41486880',
  },
};

// Monaco themes only accept hex, so the editor surface colour lives here with
// the rest of the theme; the host and loading state reuse it to avoid a flash.
const EDITOR_BACKGROUND = tokyoNight.colors['editor.background'];

function registerTokyoNightTheme(monaco: any) {
  monaco.editor.defineTheme('tokyo-night', tokyoNight as any);
}

interface CodeEditorProps {
  language: string;
  value: string;
  onChange: (value: string | undefined) => void;
  onSave?: () => void;
}

export default function CodeEditor({ language, value, onChange, onSave }: CodeEditorProps) {
  const editorRef = useRef<any>(null);
  const { t } = useTranslations();

  useEffect(() => {
    markMonacoInUse();
    void ensureMonacoStylesheet();
  }, []);

  const handleBeforeMount = (monaco: any) => {
    registerTokyoNightTheme(monaco);
  };

  const handleOnMount = (editor: any, monaco: any) => {
    editorRef.current = editor;
    registerTokyoNightTheme(monaco);
    monaco.editor.setTheme('tokyo-night');

    const forceLayout = () => {
      try {
        editor.layout();
      } catch {
        /* editor may already be disposed */
      }
    };

    // The dialog animates open, so the initial size may be 0; re-measure once
    // after the frame settles. `automaticLayout` covers all later resizes.
    requestAnimationFrame(forceLayout);

    const container = editor.getDomNode()?.parentElement;
    let resizeObserver: ResizeObserver | undefined;

    if (container) {
      resizeObserver = new ResizeObserver(forceLayout);
      resizeObserver.observe(container);
    }

    editor.onDidDispose(() => {
      resizeObserver?.disconnect();
      editorRef.current = null;
    });

    if (onSave) {
      editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
        onSave();
      });
    }
  };

  const loadingFallback = (
    <div className="flex h-full w-full flex-col items-center justify-center gap-3" style={{ backgroundColor: EDITOR_BACKGROUND }}>
      <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-hidden="true" />
      <p className="text-xs text-muted-foreground">{t('servers.fileManager.editor.loading')}</p>
    </div>
  );

  return (
    <div className="monaco-editor-host relative h-full w-full overflow-hidden rounded-lg" style={{ backgroundColor: EDITOR_BACKGROUND }}>
      <Editor
        height="100%"
        width="100%"
        language={language}
        value={value}
        onChange={onChange}
        theme="tokyo-night"
        beforeMount={handleBeforeMount}
        onMount={handleOnMount}
        loading={loadingFallback}
        options={{
            automaticLayout: true,
            fontSize: 14,
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace',
            fontLigatures: true,
            tabSize: 2,
            wordWrap: 'on',
            minimap: {
              enabled: false,
            },
            lineNumbers: 'on',
            roundedSelection: false,
            scrollBeyondLastLine: false,
            smoothScrolling: true,
            cursorBlinking: 'smooth',
            cursorSmoothCaretAnimation: 'on',
            cursorWidth: 2,
            padding: {
              top: 8,
              bottom: 8,
            },
            bracketPairColorization: {
              enabled: true,
            },
            guides: {
              bracketPairs: true,
              indentation: true,
            },
            renderLineHighlight: 'gutter',
            unusualLineTerminators: 'off',
            readOnly: false,
            overviewRulerBorder: false,
            hideCursorInOverviewRuler: true,
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