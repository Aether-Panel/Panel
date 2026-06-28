'use client';

import { Editor, useMonaco } from '@monaco-editor/react';
import { useEffect } from 'react';
import { Loader2 } from 'lucide-react';

const tokyoNight = {
	"base": "vs-dark",
	"inherit": true,
	"rules": [
		{ "foreground": "565f89", "token": "comment" },
		{ "foreground": "565f89", "token": "punctuation.definition.comment" },
		{ "foreground": "a9b1d6", "token": "variable.other.reading" },
		{ "foreground": "c0caf5", "token": "string" },
		{ "foreground": "c0caf5", "token": "punctuation.definition.string" },
		{ "foreground": "c0caf5", "token": "text.html.derivative" },
		{ "foreground": "ff9e64", "token": "constant.numeric" },
		{ "foreground": "ff9e64", "token": "keyword.other.unit" },
		{ "foreground": "7aa2f7", "token": "constant.language" },
		{ "foreground": "7aa2f7", "token": "keyword.operator" },
		{ "foreground": "f7768e", "token": "keyword" },
		{ "foreground": "f7768e", "token": "keyword.control" },
		{ "foreground": "f7768e", "token": "keyword.control.flow" },
		{ "foreground": "f7768e", "token": "keyword.control.new" },
		{ "foreground": "f7768e", "token": "storage.type" },
		{ "foreground": "f7768e", "token": "storage.modifier" },
		{ "foreground": "bb9af7", "token": "variable.language.this" },
		{ "foreground": "bb9af7", "token": "support" },
		{ "foreground": "bb9af7", "token": "support.function" },
		{ "foreground": "bb9af7", "token": "support.type" },
		{ "foreground": "bb9af7", "token": "support.class" },
		{ "foreground": "bb9af7", "token": "support.variable" },
		{ "foreground": "bb9af7", "token": "entity.name.function" },
		{ "foreground": "bb9af7", "token": "entity.name.type" },
		{ "foreground": "bb9af7", "token": "entity.name.class" },
		{ "foreground": "bb9af7", "token": "entity.name.module.js" },
		{ "foreground": "bb9af7", "token": "entity.name.method.js" },
		{ "foreground": "2ac3de", "token": "entity.name.tag" },
		{ "foreground": "7dcfff", "token": "entity.other.attribute-name" },
		{ "foreground": "e0af68", "token": "string.regexp" },
		{ "foreground": "f7768e", "fontStyle": "italic", "token": "entity.other.inherited-class" },
		{ "foreground": "9d7cd8", "token": "variable.parameter" },
		{ "foreground": "414868", "token": "meta.tag" },
		{ "foreground": "414868", "token": "punctuation.definition.tag" },
		{ "foreground": "9ece6a", "token": "meta.function-call.generic" },
		{ "foreground": "c0caf5", "token": "meta.function-call.object" },
		{ "foreground": "c0caf5", "token": "meta.function-call.static" },
		{ "foreground": "c0caf5", "token": "meta.function-call.js" },
		{ "foreground": "7dcfff", "token": "variable.other.property" },
		{ "foreground": "7dcfff", "token": "variable.other.object.property" },
		{ "foreground": "7dcfff", "token": "support.type.property-name.css" },
		{ "foreground": "e0af68", "token": "variable" },
		{ "foreground": "c0caf5", "token": "meta.var.expr" },
		{ "foreground": "c0caf5", "token": "meta.class" },
		{ "foreground": "c0caf5", "token": "meta.class.body" },
		{ "foreground": "e0af68", "token": "variable.other.object" },
		{ "foreground": "e0af68", "token": "variable.other.constant" },
		{ "foreground": "e0af68", "token": "variable.other.readwrite" },
		{ "foreground": "c0caf5", "token": "meta.object-literal.key" },
		{ "foreground": "9ece6a", "token": "entity.name.function" },
		{ "foreground": "e0af68", "token": "variable.other.property" },
		{ "foreground": "bb9af7", "token": "support.constant" },
		{ "foreground": "ff9e64", "token": "constant.character.escape" },
		{ "foreground": "73daca", "token": "string.quoted.docstring.multi.python" },
		{ "foreground": "73daca", "token": "string.quoted.docstring.python" },
		{ "foreground": "73daca", "token": "string.quoted.single.python" },
		{ "foreground": "73daca", "token": "string.quoted.double.python" },
		{ "foreground": "73daca", "token": "string.quoted.single.js" },
		{ "foreground": "73daca", "token": "string.quoted.double.js" },
		{ "foreground": "73daca", "token": "string.quoted.template.js" },
		{ "foreground": "73daca", "token": "string.quoted.single.ts" },
		{ "foreground": "73daca", "token": "string.quoted.double.ts" },
		{ "foreground": "73daca", "token": "string.quoted.template.ts" },
		{ "foreground": "bb9af7", "token": "keyword.operator.new" },
		{ "fontStyle": "italic", "token": "keyword.operator.logical" },
		{ "foreground": "f7768e", "token": "keyword.operator.comparison" },
		{ "foreground": "f7768e", "token": "keyword.operator.arithmetic" },
		{ "foreground": "bb9af7", "token": "punctuation" },
		{ "foreground": "c0caf5", "token": "punctuation.accessor" },
		{ "foreground": "bb9af7", "token": "punctuation.separator" },
		{ "foreground": "bb9af7", "token": "punctuation.terminator" },
		{ "foreground": "bb9af7", "token": "punctuation.section" },
		{ "foreground": "bb9af7", "token": "punctuation.definition.parameters" },
		{ "foreground": "bb9af7", "token": "punctuation.definition.template-expression" },
		{ "foreground": "e0af68", "token": "variable.other.alias.yaml" }
	],
	"colors": {
		"editor.background": "#1a1b26",
		"editor.foreground": "#a9b1d6",
		"editor.lineHighlightBackground": "#1e202e",
		"editor.selectionBackground": "#33467c",
		"editorCursor.foreground": "#c0caf5",
		"editorWhitespace.foreground": "#414868",
		"editorIndentGuide.background": "#414868",
		"editorIndentGuide.activeBackground": "#737aa2",
		"sideBar.background": "#16161e",
		"sideBar.border": "#16161e",
		"sideBar.foreground": "#a9b1d6",
		"sideBarSectionHeader.background": "#16161e",
		"sideBarSectionHeader.border": "#414868",
		"sideBarTitle.foreground": "#a9b1d6",
		"list.activeSelectionBackground": "#33467c",
		"list.activeSelectionForeground": "#c0caf5",
		"list.inactiveSelectionBackground": "#24283b",
		"list.hoverBackground": "#24283b",
		"list.focusBackground": "#33467c",
		"list.highlightForeground": "#c0caf5",
		"activityBar.background": "#16161e",
		"activityBar.border": "#16161e",
		"activityBar.foreground": "#c0caf5",
		"activityBar.activeBackground": "#33467c",
		"activityBarBadge.background": "#414868",
		"activityBarBadge.foreground": "#c0caf5",
		"statusBar.background": "#16161e",
		"statusBar.border": "#16161e",
		"statusBar.foreground": "#a9b1d6",
		"statusBar.noFolderBackground": "#16161e",
		"statusBarItem.hoverBackground": "#24283b",
		"statusBarItem.activeBackground": "#33467c",
		"statusBarItem.remoteBackground": "#414868",
		"titleBar.activeBackground": "#16161e",
		"titleBar.inactiveBackground": "#16161e",
		"titleBar.activeForeground": "#c0caf5",
		"titleBar.inactiveForeground": "#a9b1d6",
		"titleBar.border": "#16161e",
		"input.background": "#24283b",
		"input.border": "#414868",
		"input.foreground": "#c0caf5",
		"input.placeholderForeground": "#565f89",
		"inputOption.activeBorder": "#7aa2f7",
		"inputOption.activeBackground": "#7aa2f7",
		"inputOption.activeForeground": "#ffffff",
		"inputValidation.infoBackground": "#414868",
		"inputValidation.infoBorder": "#7aa2f7",
		"inputValidation.warningBackground": "#414868",
		"inputValidation.warningBorder": "#e0af68",
		"inputValidation.errorBackground": "#414868",
		"inputValidation.errorBorder": "#f7768e",
		"dropdown.background": "#24283b",
		"dropdown.border": "#414868",
		"dropdown.foreground": "#c0caf5",
		"button.background": "#33467c",
		"button.foreground": "#c0caf5",
		"button.hoverBackground": "#414868",
		"scrollbar.shadow": "#16161e",
		"scrollbarSlider.background": "#414868",
		"scrollbarSlider.hoverBackground": "#565f89",
		"scrollbarSlider.activeBackground": "#737aa2",
		"badge.background": "#414868",
		"badge.foreground": "#c0caf5",
		"progressBar.background": "#7aa2f7",
		"editorWidget.background": "#24283b",
		"editorWidget.border": "#414868",
		"pickerGroup.border": "#414868",
		"pickerGroup.foreground": "#c0caf5",
		"quickInput.background": "#24283b",
		"quickInput.foreground": "#c0caf5",
		"quickInputList.focusBackground": "#33467c",
		"terminal.ansiBlack": "#3d3d3d",
		"terminal.ansiBrightBlack": "#565f89",
		"terminal.ansiRed": "#f7768e",
		"terminal.ansiBrightRed": "#f7768e",
		"terminal.ansiGreen": "#9ece6a",
		"terminal.ansiBrightGreen": "#9ece6a",
		"terminal.ansiYellow": "#e0af68",
		"terminal.ansiBrightYellow": "#e0af68",
		"terminal.ansiBlue": "#7aa2f7",
		"terminal.ansiBrightBlue": "#7aa2f7",
		"terminal.ansiMagenta": "#bb9af7",
		"terminal.ansiBrightMagenta": "#bb9af7",
		"terminal.ansiCyan": "#7dcfff",
		"terminal.ansiBrightCyan": "#7dcfff",
		"terminal.ansiWhite": "#a9b1d6",
		"terminal.ansiBrightWhite": "#c0caf5",
		"debugToolBar.background": "#24283b",
		"peekView.border": "#414868",
		"peekViewEditor.background": "#1a1b26",
		"peekViewEditor.matchHighlightBackground": "#33467c",
		"peekViewResult.background": "#24283b",
		"peekViewResult.fileForeground": "#a9b1d6",
		"peekViewResult.lineForeground": "#a9b1d6",
		"peekViewResult.matchHighlightBackground": "#33467c",
		"peekViewResult.selectionBackground": "#33467c",
		"peekViewResult.selectionForeground": "#c0caf5",
		"peekViewTitle.background": "#24283b",
		"peekViewTitleDescription.foreground": "#565f89",
		"peekViewTitleLabel.foreground": "#c0caf5",
		"gitDecoration.addedResourceForeground": "#9ece6a",
		"gitDecoration.modifiedResourceForeground": "#e0af68",
		"gitDecoration.deletedResourceForeground": "#f7768e",
		"gitDecoration.untrackedResourceForeground": "#7dcfff",
		"gitDecoration.ignoredResourceForeground": "#565f89",
		"gitDecoration.conflictingResourceForeground": "#bb9af7",
		"notificationCenterHeader.background": "#24283b",
		"notificationCenterHeader.foreground": "#c0caf5",
		"notifications.background": "#24283b",
		"notifications.border": "#414868",
		"notifications.foreground": "#c0caf5",
		"notificationLink.foreground": "#7aa2f7",
		"notificationsErrorIcon.foreground": "#f7768e",
		"notificationsWarningIcon.foreground": "#e0af68",
		"notificationsInfoIcon.foreground": "#7aa2f7"
	}
};

interface CodeEditorProps {
  language: string;
  value: string;
  onChange: (value: string | undefined) => void;
}

export default function CodeEditor({ language, value, onChange }: CodeEditorProps) {
  const monaco = useMonaco();
  
  useEffect(() => {
    if (monaco) {
      monaco.editor.defineTheme('tokyo-night', tokyoNight as any);
      monaco.editor.setTheme('tokyo-night');
    }
  }, [monaco]);

  return (
    <div className="relative h-full w-full rounded-md border border-input">
      <Editor
        height="100%"
        language={language}
        value={value}
        onChange={onChange}
        theme="tokyo-night"
        loading={
          <div className="flex h-full w-full flex-col items-center justify-center gap-4">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
            <p className="text-muted-foreground">Loading Editor...</p>
          </div>
        }
        options={{
          fontSize: 14,
          wordWrap: 'on',
          minimap: {
            enabled: true,
          },
          lineNumbers: 'on',
          roundedSelection: false,
          scrollBeyondLastLine: false,
          readOnly: false,
        }}
      />
    </div>
  );
}
