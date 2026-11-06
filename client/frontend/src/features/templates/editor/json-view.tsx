import { useCallback, useEffect, useRef } from 'react';
import CodeEditor from '@/features/servers/[id]/code-editor';
import { templateJsonSchema, TEMPLATE_JSON_SCHEMA_URI } from '../schema/json-schema';
import { useTemplateEditor, type InsertTarget } from './editor-context';

// The JSON view: Monaco, loaded only when this view first opens (the page imports this module
// lazily), validating against the template's JSON Schema and completing its keys, operation
// types and arguments. Monaco ties a schema to models through fileMatch, so the model gets a
// fixed URI the schema's fileMatch names.

const TEMPLATE_MODEL_URI = 'inmemory://aether-panel/template.json';

type Monaco = typeof import('monaco-editor');
type Editor = import('monaco-editor').editor.IStandaloneCodeEditor;

function configureJson(monaco: Monaco) {
  monaco.json.jsonDefaults.setDiagnosticsOptions({
    validate: true,
    allowComments: false,
    // the schema is inline; nothing is fetched
    enableSchemaRequest: false,
    schemas: [{ uri: TEMPLATE_JSON_SCHEMA_URI, fileMatch: [TEMPLATE_MODEL_URI], schema: templateJsonSchema() }],
  });
}

export default function TemplateJsonView() {
  const { jsonText, setJsonText, setInsertTarget, releaseInsertTarget } = useTemplateEditor();
  const target = useRef<InsertTarget | null>(null);

  // "Insertar variable" types at Monaco's cursor once the editor has had focus
  const onEditorReady = useCallback(
    (editor: Editor | null) => {
      if (!editor) return;
      const insert: InsertTarget = {
        insert(text) {
          const selection = editor.getSelection();
          if (!selection) return;
          editor.executeEdits('insert-variable', [{ range: selection, text, forceMoveMarkers: true }]);
          editor.focus();
        },
      };
      target.current = insert;
      editor.onDidFocusEditorText(() => setInsertTarget(insert));
    },
    [setInsertTarget],
  );

  useEffect(
    () => () => {
      if (target.current) releaseInsertTarget(target.current);
    },
    [releaseInsertTarget],
  );

  return (
    <CodeEditor
      language="json"
      path={TEMPLATE_MODEL_URI}
      value={jsonText}
      onChange={(value) => setJsonText(value ?? '')}
      onBeforeMount={configureJson}
      onEditorReady={onEditorReady}
    />
  );
}
