'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import {
  Folder,
  File,
  FileImage,
  FileCode,
  FileJson,
  FileCog,
  FileText,
  FileArchive,
  FileTerminal,
  FileVideo,
  FileAudio,
  FileSpreadsheet,
  BookOpenText,
  Coffee,
  Database,
  Palette,
  Globe,
  FolderOpen,
  Home,
  PencilLine,
  ArrowUpToLine,
  ChevronRight,
  MoreHorizontal,
  Loader2,
  Plus,
  Upload,
  FolderPlus,
  FilePlus,
  Copy,
  Scissors,
  X,
  Trash2,
  Edit2,
  Download,
  Check,
  History,
  type LucideIcon,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger, DropdownMenuSeparator } from '@/components/ui/dropdown-menu';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useState, useEffect, lazy, Suspense, useRef, Fragment, useCallback } from 'react';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from "@/lib/toast";
import { cn } from '@/lib/utils';
import { EmptyState } from '@/components/empty-state';
import { FileHistorySheet, formatRelative, isHistoryTracked, localeFor, useFileHistory, type FileVersion } from './file-history-sheet';
import FileDiffView from './file-diff-view';

const CodeEditor = lazy(() => import('./code-editor'));

type FileItemResource = {
  name: string;
  modifyTime: number;
  size: number;
  isFile: boolean;
  extension?: string;
};

type ClipboardItem = {
  name: string;
  path: string;
  isFile: boolean;
  mode: 'copy' | 'cut';
};

type FileMeta = {
  icon: LucideIcon;
  className: string;
};

// File-type tint, from the semantic chart palette only: blue for code, teal for
// config and data, violet for media, amber for folders, neutral for plain text.
const FILE_META: { match: RegExp; icon: LucideIcon; className: string }[] = [
  { match: /^(png|jpe?g|gif|webp|svg|bmp|ico)$/, icon: FileImage, className: 'text-chart-3' },
  { match: /^(js|mjs|cjs|jsx|ts|tsx)$/, icon: FileCode, className: 'text-chart-4' },
  { match: /^(json|yaml|yml|toml|ini|conf|cfg|properties)$/, icon: FileCog, className: 'text-chart-2' },
  { match: /^(md|mdx)$/, icon: BookOpenText, className: 'text-muted-foreground' },
  { match: /^(txt|log|text)$/, icon: FileText, className: 'text-muted-foreground' },
  { match: /^(zip|tar|gz|bz2|rar|7z|xz)$/, icon: FileArchive, className: 'text-chart-1' },
  { match: /^(java|class|jar)$/, icon: Coffee, className: 'text-chart-1' },
  { match: /^(sh|bash|zsh|bat|cmd|ps1)$/, icon: FileTerminal, className: 'text-chart-4' },
  { match: /^(db|sqlite|sqlite3|sql)$/, icon: Database, className: 'text-chart-2' },
  { match: /^(css|scss|sass|less)$/, icon: Palette, className: 'text-chart-4' },
  { match: /^(html|htm|xml)$/, icon: Globe, className: 'text-chart-4' },
  { match: /^(mp4|webm|mkv|avi|mov|flv)$/, icon: FileVideo, className: 'text-chart-3' },
  { match: /^(mp3|wav|ogg|flac|m4a)$/, icon: FileAudio, className: 'text-chart-3' },
  { match: /^(csv|xlsx|xls|ods)$/, icon: FileSpreadsheet, className: 'text-chart-2' },
  { match: /^(py|pyw)$/, icon: FileCode, className: 'text-chart-4' },
  { match: /^(geojson|json5)$/, icon: FileJson, className: 'text-chart-2' },
];

const FOLDER_META: FileMeta = { icon: Folder, className: 'fill-chart-5/15 text-chart-5' };
const PARENT_META: FileMeta = { icon: ArrowUpToLine, className: 'text-muted-foreground' };

function getFileMeta(name: string): FileMeta {
  const ext = name.split('.').pop()?.toLowerCase() || '';
  const match = FILE_META.find((m) => m.match.test(ext));
  return match || { icon: File, className: 'text-muted-foreground' };
}

// Encode each path segment so names with #, ?, % or spaces survive the URL; '/' stays a separator
function encodePath(path: string) {
  return path.split('/').map(encodeURIComponent).join('/');
}

// Raw fetch does not reject on HTTP errors, so surface the server's message (or a fallback) instead
async function assertOk(res: Response, fallback: string) {
  if (res.ok) return;
  const data = await res.json().catch(() => ({}));
  const error = data?.error;
  throw new Error(typeof error === 'string' ? error : error?.msg || fallback);
}

function Breadcrumbs({ path, onNavigate, label }: { path: string; onNavigate: (p: string) => void; label: string }) {
  const parts = path.split('/').filter(Boolean);
  const crumbs = [{ label: '~', path: '' }, ...parts.map((p, i) => ({ label: p, path: parts.slice(0, i + 1).join('/') }))];

  return (
    <nav className="flex min-w-0 flex-1 items-center gap-0.5 overflow-x-auto font-mono text-sm" aria-label={label}>
      {crumbs.map((crumb, i) => {
        const isLast = i === crumbs.length - 1;
        return (
          <Fragment key={`${crumb.path}-${i}`}>
            {i > 0 && <ChevronRight className="h-3.5 w-3.5 shrink-0 text-muted-foreground/50" aria-hidden="true" />}
            <button
              type="button"
              onClick={() => onNavigate(crumb.path)}
              aria-current={isLast ? 'page' : undefined}
              className={cn(
                'focus-ring shrink-0 rounded-md px-1.5 py-1 transition-colors duration-150 hover:bg-accent/50 hover:text-foreground',
                isLast ? 'font-medium text-foreground' : 'text-muted-foreground'
              )}
            >
              {crumb.label}
            </button>
          </Fragment>
        );
      })}
    </nav>
  );
}

export default function FileManagerView({ serverId }: { serverId: string }) {
  const [currentPath, setCurrentPath] = useState('');
  const [files, setFiles] = useState<FileItemResource[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [editingFile, setEditingFile] = useState<FileItemResource | null>(null);
  const [editedContent, setEditedContent] = useState<string>('');
  const [isSaving, setIsSaving] = useState(false);
  const [savedFlash, setSavedFlash] = useState(false);
  // the text last read from or written to disk, to tell whether the editor holds unsaved edits
  const [savedContent, setSavedContent] = useState<string>('');
  const [historyOpen, setHistoryOpen] = useState(false);
  const [compareVersion, setCompareVersion] = useState<FileVersion | null>(null);
  const editorInstanceRef = useRef<any>(null);
  const historyButtonRef = useRef<HTMLButtonElement>(null);

  const closeEditor = () => {
    setIsEditorOpen(false);
    setHistoryOpen(false);
    editorCloseTimeoutRef.current = window.setTimeout(() => {
      editorCloseTimeoutRef.current = null;
      setEditingFile(null);
      setCompareVersion(null);
    }, 300);
  };

  // New State for operations
  const [newItemDialog, setNewItemDialog] = useState<{ open: boolean; type: 'file' | 'folder' }>({ open: false, type: 'file' });
  const [newItemName, setNewItemName] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  const [pendingFile, setPendingFile] = useState<FileItemResource | null>(null);
  const [renameTarget, setRenameTarget] = useState<FileItemResource | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const [isRenaming, setIsRenaming] = useState(false);

  const [clipboard, setClipboard] = useState<ClipboardItem | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const editorCloseTimeoutRef = useRef<number | null>(null);
  const savingRef = useRef(false);

  const { t, language } = useTranslations();

  const fileUrl = (path: string) => `/api/servers/${serverId}/file/${encodePath(path)}`;

  const editingPath = editingFile ? (currentPath ? `${currentPath}/${editingFile.name}` : editingFile.name) : null;
  const historyTracked = !!editingFile && isHistoryTracked(editingFile.name);
  const fileHistory = useFileHistory(serverId, historyTracked ? editingPath : null);
  const historyCount = fileHistory.status === 'ready' ? fileHistory.versions.length : 0;

  const fetchFiles = async (path: string) => {
    setIsLoading(true);
    try {
      const data = await api.get(fileUrl(path));
      if (Array.isArray(data)) {
        setFiles(data);
      }
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('servers.fileManager.toast.loadError') });
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void fetchFiles(currentPath);
  }, [currentPath, serverId]);

  const handleFileClick = async (file: FileItemResource) => {
    if (!file.isFile) {
      if (file.name === '..') {
        const parts = currentPath.split('/').filter(Boolean);
        parts.pop();
        setCurrentPath(parts.join('/'));
      } else {
        const newPath = currentPath ? `${currentPath}/${file.name}` : file.name;
        setCurrentPath(newPath);
      }
    } else {
      try {
        const filePath = currentPath ? `${currentPath}/${file.name}` : file.name;
        const content = await api.get(fileUrl(filePath));
        if (editorCloseTimeoutRef.current !== null) {
          clearTimeout(editorCloseTimeoutRef.current);
          editorCloseTimeoutRef.current = null;
        }
        const text = typeof content === 'string' ? content : JSON.stringify(content, null, 2);
        setEditedContent(text);
        setSavedContent(text);
        setCompareVersion(null);
        setHistoryOpen(false);
        setEditingFile(file);
        setIsEditorOpen(true);
      } catch (e: any) {
        sileo.error({ title: t('common.error'), description: e.message || t('servers.fileManager.toast.openError') });
      }
    }
  };

  const closeNewItemDialog = () => {
    setNewItemDialog((prev) => ({ ...prev, open: false }));
    setNewItemName('');
  };

  const handleCreateItem = async () => {
    if (!newItemName || isCreating) return;
    setIsCreating(true);
    try {
      const filePath = currentPath ? `${currentPath}/${newItemName}` : newItemName;
      const url = `${fileUrl(filePath)}${newItemDialog.type === 'folder' ? '?folder' : ''}`;

      const response = await fetch(url, {
        method: 'PUT',
        body: newItemDialog.type === 'file' ? '' : undefined,
        credentials: 'include'
      });

      await assertOk(response, t('servers.fileManager.toast.createError'));

      sileo.success({
        title: t('common.success'),
        description: newItemDialog.type === 'file' ? t('servers.fileManager.toast.createdFile') : t('servers.fileManager.toast.createdFolder')
      });

      // Reset dialog state
      setNewItemDialog(prev => ({ ...prev, open: false }));
      setNewItemName('');

      // Wait for Radix UI to finish closing and cleanup the body
      setTimeout(() => {
        void fetchFiles(currentPath);
        setIsCreating(false);
        // Safety cleanup to ensure UI is not blocked
        document.body.style.pointerEvents = 'auto';
        document.body.style.overflow = 'auto';
      }, 300);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('servers.fileManager.toast.createError') });
      setIsCreating(false);
    }
  };

  const handleUploadClick = () => {
    fileInputRef.current?.click();
  };

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setIsLoading(true);
    try {
      const filePath = currentPath ? `${currentPath}/${file.name}` : file.name;
      const formData = new FormData();
      formData.append('file', file);

      const res = await fetch(fileUrl(filePath), {
        method: 'PUT',
        body: formData,
        credentials: 'include'
      });
      await assertOk(res, t('servers.fileManager.toast.uploadError'));

      sileo.success({ title: t('common.success'), description: t('servers.fileManager.toast.uploaded') });
      void fetchFiles(currentPath);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('servers.fileManager.toast.uploadError') });
    } finally {
      setIsLoading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const handleCopy = (file: FileItemResource, mode: 'copy' | 'cut') => {
    const path = currentPath ? `${currentPath}/${file.name}` : file.name;
    setClipboard({ name: file.name, path, isFile: file.isFile, mode });
    sileo.success({
      title: t('common.success'),
      description: mode === 'copy' ? t('servers.fileManager.toast.copied', { name: file.name }) : t('servers.fileManager.toast.cut', { name: file.name })
    });
  };

  const readFileBlob = async (path: string): Promise<Blob> => {
    const res = await fetch(fileUrl(path), { credentials: 'include' });
    if (!res.ok) throw new Error(`Error al leer: ${path}`);
    return res.blob();
  };

  const makeFolder = async (path: string) => {
    const res = await fetch(`${fileUrl(path)}?folder`, {
      method: 'PUT',
      credentials: 'include'
    });
    if (!res.ok) throw new Error(`Error al crear carpeta: ${path}`);
  };

  const parentPath = (path: string) => {
    const idx = path.lastIndexOf('/');
    return idx > 0 ? path.slice(0, idx) : '';
  };

  const writeFileBlob = async (path: string, blob: Blob) => {
    // The daemon's PUT does not create parent directories, so ensure they exist first
    const parent = parentPath(path);
    if (parent) {
      await makeFolder(parent);
    }
    const res = await fetch(fileUrl(path), {
      method: 'PUT',
      body: blob,
      credentials: 'include'
    });
    if (!res.ok) throw new Error(`Error al escribir: ${path}`);
  };

  const listFolderForCopy = async (path: string): Promise<FileItemResource[]> => {
    const data = await api.get(fileUrl(path));
    return Array.isArray(data) ? data : [];
  };

  const copyFolderRecursive = async (src: string, dest: string) => {
    await makeFolder(dest);
    const items = await listFolderForCopy(src);
    for (const item of items) {
      if (item.name === '.' || item.name === '..') continue;
      const srcChild = `${src}/${item.name}`;
      const destChild = `${dest}/${item.name}`;
      if (item.isFile) {
        const blob = await readFileBlob(srcChild);
        await writeFileBlob(destChild, blob);
      } else {
        await copyFolderRecursive(srcChild, destChild);
      }
    }
  };

  const handlePaste = async () => {
    if (!clipboard) return;

    const sourcePath = clipboard.path;
    const destPath = currentPath ? `${currentPath}/${clipboard.name}` : clipboard.name;

    // Reject pasting onto itself or a folder into its own subtree
    if (sourcePath === destPath || (!clipboard.isFile && destPath.startsWith(`${sourcePath}/`))) {
      sileo.error({ title: t('common.error'), description: t('servers.fileManager.toast.pasteInvalid') });
      return;
    }

    setIsLoading(true);
    try {
      if (clipboard.isFile) {
        const blob = await readFileBlob(sourcePath);
        await writeFileBlob(destPath, blob);
      } else {
        await copyFolderRecursive(sourcePath, destPath);
      }

      if (clipboard.mode === 'cut') {
        await api.delete(fileUrl(sourcePath));
        setClipboard(null);
      }

      sileo.success({
        title: t('common.success'),
        description: clipboard.mode === 'cut'
          ? t('servers.fileManager.toast.moved')
          : t('servers.fileManager.toast.pasted')
      });
      void fetchFiles(currentPath);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('servers.fileManager.toast.pasteError') });
    } finally {
      setIsLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!pendingFile) return;
    setIsLoading(true);
    try {
      const filePath = currentPath ? `${currentPath}/${pendingFile.name}` : pendingFile.name;
      await api.delete(fileUrl(filePath));
      sileo.success({ title: t('common.success'), description: t('servers.fileManager.toast.deleted') });
      void fetchFiles(currentPath);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('servers.fileManager.toast.deleteError') });
    } finally {
      setIsLoading(false);
      setPendingFile(null);
      // Ensure Radix dropdown/alert-dialog cleanup finished so the UI is not left blocked
      setTimeout(() => {
        document.body.style.pointerEvents = 'auto';
        document.body.style.overflow = 'auto';
      }, 100);
    }
  };

  const openRename = (file: FileItemResource) => {
    setRenameTarget(file);
    setRenameValue(file.name);
  };

  const closeRename = () => {
    setRenameTarget(null);
    setRenameValue('');
    setIsRenaming(false);
  };

  const handleRename = async () => {
    if (!renameTarget || !renameValue.trim() || isRenaming) return;
    const newName = renameValue.trim();
    if (newName === renameTarget.name) {
      closeRename();
      return;
    }
    setIsRenaming(true);
    try {
      const srcPath = currentPath ? `${currentPath}/${renameTarget.name}` : renameTarget.name;
      const destPath = currentPath ? `${currentPath}/${newName}` : newName;
      const response = await fetch(`${fileUrl(srcPath)}?destination=${encodeURIComponent(destPath)}`, {
        method: 'POST',
        credentials: 'include'
      });
      await assertOk(response, t('servers.fileManager.toast.renameError'));
      sileo.success({ title: t('common.success'), description: t('servers.fileManager.toast.renamed') });
      closeRename();
      // Ensure Radix Dialog/DropdownMenu cleanup finished so the UI is not left blocked
      setTimeout(() => {
        document.body.style.pointerEvents = 'auto';
        document.body.style.overflow = 'auto';
      }, 100);
      void fetchFiles(currentPath);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('servers.fileManager.toast.renameError') });
      setIsRenaming(false);
    }
  };

  const handleExtract = async (file: FileItemResource) => {
    setIsLoading(true);
    try {
      const filePath = currentPath ? `${currentPath}/${file.name}` : file.name;
      await api.post(`/api/servers/${serverId}/extract/${encodePath(filePath)}?destination=.&skipRoot`, {});
      sileo.success({ title: t('common.success'), description: t('servers.fileManager.toast.extracted') });
      void fetchFiles(currentPath);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('servers.fileManager.toast.extractError') });
    } finally {
      setIsLoading(false);
    }
  };

  const handleSaveChanges = async () => {
    if (!editingFile || savingRef.current) return;
    savingRef.current = true;
    setIsSaving(true);
    const text = editedContent;
    try {
      const filePath = currentPath ? `${currentPath}/${editingFile.name}` : editingFile.name;
      const res = await fetch(fileUrl(filePath), {
        method: 'PUT',
        body: text,
        credentials: 'include'
      });
      await assertOk(res, t('servers.fileManager.toast.saveError'));
      setSavedContent(text);
      setSavedFlash(true);
      setTimeout(() => setSavedFlash(false), 2000);
      // the save kept the previous text as a version, so the count moves
      if (historyTracked) void fileHistory.refresh(true);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || t('servers.fileManager.toast.saveError') });
    } finally {
      savingRef.current = false;
      setIsSaving(false);
    }
  };

  useEffect(() => {
    if (!editingFile) return;
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
        e.preventDefault();
        void handleSaveChanges();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editingFile, editedContent, isSaving]);

  const getLanguageFromFilename = (filename: string) => {
    const extension = filename.split('.').pop()?.toLowerCase();
    switch (extension) {
      case 'js': return 'javascript';
      case 'ts': return 'typescript';
      case 'tsx': return 'typescript';
      case 'jsx': return 'javascript';
      case 'json': return 'json';
      case 'md': return 'markdown';
      case 'py': return 'python';
      case 'yaml':
      case 'yml': return 'yaml';
      case 'css': return 'css';
      case 'html':
      case 'htm': return 'html';
      case 'xml': return 'xml';
      case 'php': return 'php';
      case 'go': return 'go';
      case 'sh': return 'shell';
      case 'java': return 'java';
      case 'sql': return 'sql';
      case 'rb': return 'ruby';
      case 'c': return 'c';
      case 'cpp':
      case 'cc': return 'cpp';
      case 'cs': return 'csharp';
      case 'ini':
      case 'conf':
      case 'cfg':
      case 'env':
      case 'properties': return 'ini';
      default: return 'plaintext';
    }
  };

  const formatSize = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Number.parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const formatDate = (epoch: number) => {
    if (!epoch) return '-';
    return new Date(epoch * 1000).toLocaleString();
  };

  const focusEditor = () => {
    requestAnimationFrame(() => {
      const editor = editorInstanceRef.current;
      if (!editor) return;
      try {
        editor.layout();
        editor.focus();
      } catch {
        /* disposed while switching */
      }
    });
  };

  const handleEditorReady = useCallback((editor: any) => {
    editorInstanceRef.current = editor;
  }, []);

  const selectVersion = (version: FileVersion) => {
    setCompareVersion(version);
    setHistoryOpen(false);
  };

  const leaveCompare = () => {
    setCompareVersion(null);
    focusEditor();
  };

  const handleRestoreVersion = async (version: FileVersion, text: string) => {
    try {
      await fileHistory.restore(version.id);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e?.message || t('servers.fileHistory.toast.restoreError') });
      throw e;
    }
    setEditedContent(text);
    setSavedContent(text);
    void fileHistory.refresh(true);
    sileo.success({
      title: t('servers.fileHistory.toast.restoredTitle'),
      description: t('servers.fileHistory.toast.restored', { when: formatRelative(version.savedAt, localeFor(language)) }),
    });
  };

  const editingMeta = editingFile ? getFileMeta(editingFile.name) : { icon: File, className: 'text-muted-foreground' };
  const editingLanguage = editingFile ? getLanguageFromFilename(editingFile.name) : 'plaintext';
  const EditingIcon = editingMeta.icon;

  const renderRowMenu = (file: FileItemResource) => (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="h-9 w-9 text-muted-foreground hover:text-foreground"
          aria-label={t('servers.fileManager.rowActions', { name: file.name })}
        >
          <MoreHorizontal aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {file.isFile ? (
          <DropdownMenuItem onClick={() => setTimeout(() => handleFileClick(file), 50)}>
            <Edit2 className="mr-2 h-4 w-4" aria-hidden="true" />
            {t('servers.fileManager.actions.edit')}
          </DropdownMenuItem>
        ) : null}
        <DropdownMenuItem asChild>
          <a href={fileUrl(currentPath ? `${currentPath}/${file.name}` : file.name)} download>
            <Download className="mr-2 h-4 w-4" aria-hidden="true" />
            {t('servers.fileManager.actions.download')}
          </a>
        </DropdownMenuItem>
        {file.isFile && file.name.match(/\.(zip|tar(\.gz)?|tgz|gz|bz2|rar|7z|xz)$/i) ? (
          <DropdownMenuItem onClick={() => handleExtract(file)}>
            <FileArchive className="mr-2 h-4 w-4" aria-hidden="true" />
            {t('servers.fileManager.actions.unarchive')}
          </DropdownMenuItem>
        ) : null}
        <DropdownMenuItem onClick={() => setTimeout(() => openRename(file), 50)}>
          <PencilLine className="mr-2 h-4 w-4" aria-hidden="true" />
          {t('servers.fileManager.actions.rename')}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => handleCopy(file, 'copy')}>
          <Copy className="mr-2 h-4 w-4" aria-hidden="true" />
          {t('servers.fileManager.actions.copy')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => handleCopy(file, 'cut')}>
          <Scissors className="mr-2 h-4 w-4" aria-hidden="true" />
          {t('servers.fileManager.actions.cut')}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <AlertDialog open={pendingFile?.name === file.name} onOpenChange={(open) => !open && setPendingFile(null)}>
          <AlertDialogTrigger asChild>
            <DropdownMenuItem
              className="text-destructive-bright focus:bg-destructive/10 focus:text-destructive-bright"
              onSelect={(e) => { e.preventDefault(); setPendingFile(file); }}
            >
              <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
              {t('servers.fileManager.actions.delete')}
            </DropdownMenuItem>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t('servers.fileManager.deleteDialog.title')}</AlertDialogTitle>
              <AlertDialogDescription>
                {t('servers.fileManager.deleteDialog.description', { name: file.name })}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
              <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                {t('servers.fileManager.actions.delete')}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </DropdownMenuContent>
    </DropdownMenu>
  );

  return (
    <>
      <Card className="mt-6">
        <CardHeader className="flex flex-col gap-4 space-y-0 border-b border-border/50 p-5 sm:flex-row sm:items-center sm:justify-between md:p-6">
          <div className="min-w-0 space-y-1">
            <CardTitle className="text-lg font-semibold tracking-[-0.02em]">{t('servers.fileManager.title')}</CardTitle>
            <CardDescription>{t('servers.fileManager.description')}</CardDescription>
          </div>
          <div className="flex shrink-0 items-center gap-2 self-start sm:self-auto">
            <input type="file" ref={fileInputRef} className="hidden" onChange={handleFileChange} />
            <Button variant="outline" onClick={handleUploadClick}>
              <Upload aria-hidden="true" />
              {t('servers.fileManager.upload')}
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button>
                  <Plus aria-hidden="true" />
                  {t('servers.fileManager.new')}
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={() => setNewItemDialog({ open: true, type: 'file' })}>
                  <FilePlus className="mr-2 h-4 w-4" aria-hidden="true" />
                  {t('servers.fileManager.newFile')}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => setNewItemDialog({ open: true, type: 'folder' })}>
                  <FolderPlus className="mr-2 h-4 w-4" aria-hidden="true" />
                  {t('servers.fileManager.newFolder')}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </CardHeader>

        <CardContent className="space-y-4 p-5 md:p-6">
          {/* Path bar, plus the pending copy/cut item when there is one */}
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
            <div className="flex min-w-0 flex-1 items-center gap-1 rounded-lg border border-border/60 bg-background/30 py-1 pl-1 pr-2">
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground"
                onClick={() => setCurrentPath('')}
                disabled={!currentPath}
                aria-label={t('servers.fileManager.goRoot')}
                title={t('servers.fileManager.goRoot')}
              >
                <Home aria-hidden="true" />
              </Button>
              <Breadcrumbs path={currentPath} onNavigate={setCurrentPath} label={t('servers.fileManager.pathLabel')} />
            </div>

            {clipboard && (
              <div className="flex min-w-0 items-center gap-1 rounded-lg border border-border/60 bg-background/30 p-1">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handlePaste}
                  disabled={isLoading}
                  className="h-8 min-w-0 flex-1 justify-start px-2 text-xs sm:flex-none"
                  title={clipboard.mode === 'cut' ? t('servers.fileManager.actions.cut') : t('servers.fileManager.actions.copy')}
                >
                  {clipboard.mode === 'cut'
                    ? <Scissors className="text-muted-foreground" aria-hidden="true" />
                    : <Copy className="text-muted-foreground" aria-hidden="true" />}
                  <span className="shrink-0">{t('servers.fileManager.paste')}</span>
                  <span className="min-w-0 max-w-[160px] truncate font-mono text-muted-foreground">{clipboard.name}</span>
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground"
                  onClick={() => setClipboard(null)}
                  aria-label={t('servers.fileManager.clearClipboard')}
                >
                  <X aria-hidden="true" />
                </Button>
              </div>
            )}
          </div>

          {isLoading ? (
            <div className="flex justify-center rounded-xl border border-border/60 bg-background/30 py-16">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-label={t('common.loading')} />
            </div>
          ) : files.length === 0 ? (
            <EmptyState
              icon={FolderOpen}
              title={t('servers.fileManager.empty')}
              description={t('servers.fileManager.emptyHint')}
              action={
                <Button variant="outline" onClick={handleUploadClick}>
                  <Upload aria-hidden="true" />
                  {t('servers.fileManager.upload')}
                </Button>
              }
            />
          ) : (
            <div className="overflow-hidden rounded-xl border border-border/60 bg-background/30">
              <Table>
                <TableHeader className="bg-transparent">
                  <TableRow className="border-border/50 hover:bg-transparent">
                    <TableHead>{t('servers.fileManager.table.name')}</TableHead>
                    <TableHead className="hidden w-28 sm:table-cell">{t('servers.fileManager.table.size')}</TableHead>
                    <TableHead className="hidden w-48 lg:table-cell">{t('servers.fileManager.table.modified')}</TableHead>
                    <TableHead className="w-14 pr-2 text-right">
                      <span className="sr-only">{t('servers.fileManager.table.actions')}</span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {files.map((file) => {
                    const isParent = !file.isFile && file.name === '..';
                    const meta = isParent ? PARENT_META : file.isFile ? getFileMeta(file.name) : FOLDER_META;
                    const MetaIcon = meta.icon;
                    return (
                      <TableRow key={file.name} className="group border-border/50 hover:bg-accent/30">
                        {/* w-full + max-w-0 lets the name column absorb the free width and truncate */}
                        <TableCell className="w-full max-w-0 py-2.5">
                          <button
                            type="button"
                            onClick={() => handleFileClick(file)}
                            className="focus-ring flex w-full min-w-0 items-center gap-3 rounded-md text-left"
                          >
                            <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-border/60 bg-card">
                              <MetaIcon className={cn('h-4 w-4', meta.className)} aria-hidden="true" />
                            </span>
                            <span className="min-w-0">
                              <span
                                className={cn(
                                  'block truncate font-medium transition-colors duration-150',
                                  isParent ? 'text-muted-foreground' : 'group-hover:text-primary-bright'
                                )}
                              >
                                {file.name}
                              </span>
                              {!isParent && (
                                <span className="mt-0.5 block truncate text-xs tabular-nums text-muted-foreground lg:hidden">
                                  {file.isFile && <span className="sm:hidden">{formatSize(file.size)} · </span>}
                                  {formatDate(file.modifyTime)}
                                </span>
                              )}
                            </span>
                          </button>
                        </TableCell>
                        <TableCell className="hidden whitespace-nowrap text-xs tabular-nums text-muted-foreground sm:table-cell">
                          {file.isFile ? formatSize(file.size) : '—'}
                        </TableCell>
                        <TableCell className="hidden whitespace-nowrap text-xs tabular-nums text-muted-foreground lg:table-cell">
                          {formatDate(file.modifyTime)}
                        </TableCell>
                        <TableCell className="py-2.5 pr-2 text-right">
                          {renderRowMenu(file)}
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Editor Dialog */}
      <Dialog open={isEditorOpen} onOpenChange={(open) => !open && closeEditor()}>
        <DialogContent className="flex h-[90dvh] w-[calc(100vw-1rem)] max-w-[calc(100vw-1rem)] flex-col gap-0 overflow-hidden p-0 sm:w-[95vw] sm:max-w-[95vw]">
          <div className="flex items-center justify-between gap-3 border-b border-border/50 bg-card py-3 pl-4 pr-12 sm:pl-5">
            <div className="flex min-w-0 items-center gap-3">
              <span className="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-border/60 bg-background/30">
                <EditingIcon className={cn('h-4 w-4', editingMeta.className)} aria-hidden="true" />
              </span>
              <div className="min-w-0">
                <DialogTitle className="truncate text-sm font-semibold leading-tight">{editingFile?.name}</DialogTitle>
                <DialogDescription className="truncate font-mono text-xs text-muted-foreground">
                  ~{currentPath ? `/${currentPath}` : ''}
                </DialogDescription>
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-2 sm:gap-3">
              {savedFlash && (
                <span className="flex items-center gap-1.5 text-xs font-medium text-success" role="status">
                  <Check className="h-3.5 w-3.5" aria-hidden="true" />
                  <span className="hidden sm:inline">{t('common.saved')}</span>
                  <span className="sr-only sm:hidden">{t('common.saved')}</span>
                </span>
              )}
              <span className="hidden rounded-md border border-border/60 px-1.5 py-0.5 font-mono text-xs text-muted-foreground sm:inline">
                {editingLanguage}
              </span>
              {historyTracked && (
                <Button
                  ref={historyButtonRef}
                  variant="outline"
                  size="sm"
                  onClick={() => setHistoryOpen(true)}
                  aria-haspopup="dialog"
                  aria-expanded={historyOpen}
                  aria-label={historyCount > 0 ? t('servers.fileHistory.openWithCount', { count: historyCount }) : t('servers.fileHistory.open')}
                  title={t('servers.fileHistory.open')}
                  className={cn('h-8 gap-1.5 px-2 sm:px-2.5', compareVersion && 'border-primary/40 bg-primary/[0.07] text-primary-bright')}
                >
                  <History aria-hidden="true" />
                  <span className="hidden sm:inline">{t('servers.fileHistory.button')}</span>
                  {historyCount > 0 && (
                    <span
                      aria-hidden="true"
                      className="min-w-[1.25rem] rounded-full bg-secondary px-1.5 text-center text-[11px] font-semibold leading-5 tabular-nums text-muted-foreground animate-in fade-in-0 zoom-in-95 duration-200 ease-panel"
                    >
                      {historyCount}
                    </span>
                  )}
                </Button>
              )}
            </div>
          </div>

          <div className="min-h-0 flex-1 [&_.monaco-editor-host]:rounded-none [&_.monaco-editor-host]:border-0">
            {/* the editor stays mounted under the comparison, so its cursor and undo history survive */}
            <div className={cn('h-full', compareVersion && 'hidden')}>
              {editingFile && (
                <Suspense fallback={<div className="flex h-full items-center justify-center"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></div>}>
                  <CodeEditor
                    key={editingFile.name}
                    language={editingLanguage}
                    value={editedContent}
                    onChange={(value) => setEditedContent(value || '')}
                    onSave={handleSaveChanges}
                    onEditorReady={handleEditorReady}
                  />
                </Suspense>
              )}
            </div>
            {editingFile && compareVersion && (
              <FileDiffView
                key={compareVersion.id}
                version={compareVersion}
                language={editingLanguage}
                current={editedContent}
                dirty={editedContent !== savedContent}
                loadContent={fileHistory.getContent}
                onRestore={handleRestoreVersion}
                onRestored={leaveCompare}
                onBack={leaveCompare}
              />
            )}
          </div>

          <div className="flex items-center justify-between gap-3 border-t border-border/50 bg-card px-4 py-3 sm:px-5">
            <p className="hidden text-xs text-muted-foreground sm:block">
              {t('servers.fileManager.editor.saveHint')}
            </p>
            <div className="flex flex-1 items-center justify-end gap-2 sm:flex-none">
              <Button variant="outline" onClick={closeEditor}>
                {t('servers.fileManager.editor.cancel')}
              </Button>
              <Button onClick={handleSaveChanges} disabled={isSaving}>
                {isSaving ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Check aria-hidden="true" />}
                {t('servers.fileManager.editor.save')}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {editingFile && historyTracked && (
        <FileHistorySheet
          open={historyOpen}
          onOpenChange={setHistoryOpen}
          fileName={editingPath ?? editingFile.name}
          versions={fileHistory.versions}
          status={fileHistory.status}
          error={fileHistory.error}
          selectedId={compareVersion?.id ?? null}
          onRetry={() => void fileHistory.refresh()}
          onSelect={selectVersion}
          returnFocusRef={historyButtonRef}
        />
      )}

      {/* New Item Dialog */}
      <Dialog open={newItemDialog.open} onOpenChange={(open) => !open && closeNewItemDialog()}>
        <DialogContent className="sm:max-w-[425px]">
          <DialogHeader>
            <DialogTitle>
              {newItemDialog.type === 'file' ? t('servers.fileManager.createFileTitle') : t('servers.fileManager.createFolderTitle')}
            </DialogTitle>
            <DialogDescription>{t('servers.fileManager.createDescription')}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="file-manager-new-name">{t('servers.fileManager.nameLabel')}</Label>
              <Input
                id="file-manager-new-name"
                value={newItemName}
                onChange={(e) => setNewItemName(e.target.value)}
                placeholder={newItemDialog.type === 'file' ? t('servers.fileManager.createFilePlaceholder') : t('servers.fileManager.createFolderPlaceholder')}
                className="font-mono"
                autoFocus
                onKeyDown={(e) => e.key === 'Enter' && handleCreateItem()}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={closeNewItemDialog}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleCreateItem} disabled={isCreating || !newItemName}>
              {isCreating && <Loader2 className="animate-spin" aria-hidden="true" />}
              {t('servers.fileManager.create')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Rename Dialog */}
      <Dialog open={!!renameTarget} onOpenChange={(open) => !open && closeRename()}>
        <DialogContent className="sm:max-w-[425px]">
          <DialogHeader>
            <DialogTitle>{t('servers.fileManager.renameDialog.title')}</DialogTitle>
            <DialogDescription>
              {t('servers.fileManager.renameDialog.description', { name: renameTarget?.name ?? '' })}
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="file-manager-rename">{t('servers.fileManager.nameLabel')}</Label>
              <Input
                id="file-manager-rename"
                value={renameValue}
                onChange={(e) => setRenameValue(e.target.value)}
                placeholder={t('servers.fileManager.renameDialog.placeholder')}
                className="font-mono"
                autoFocus
                onKeyDown={(e) => e.key === 'Enter' && handleRename()}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={closeRename}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleRename} disabled={isRenaming || !renameValue.trim()}>
              {isRenaming && <Loader2 className="animate-spin" aria-hidden="true" />}
              {t('servers.fileManager.actions.rename')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
