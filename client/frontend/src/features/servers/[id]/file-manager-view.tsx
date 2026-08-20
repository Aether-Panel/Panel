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
  FolderTree,
  ArrowUpToLine,
  ChevronRight,
  TerminalSquare,
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
  type LucideIcon,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger, DropdownMenuSeparator } from '@/components/ui/dropdown-menu';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useState, useEffect, lazy, Suspense, useRef, Fragment } from 'react';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from "@/lib/toast";
import { cn } from '@/lib/utils';

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

const FILE_META: { match: RegExp; icon: LucideIcon; className: string }[] = [
  { match: /^(png|jpe?g|gif|webp|svg|bmp|ico)$/, icon: FileImage, className: 'text-violet-400' },
  { match: /^(js|mjs|cjs|jsx|ts|tsx)$/, icon: FileCode, className: 'text-sky-400' },
  { match: /^(json|yaml|yml|toml|ini|conf|cfg|properties)$/, icon: FileCog, className: 'text-orange-400' },
  { match: /^(md|mdx)$/, icon: BookOpenText, className: 'text-sky-300' },
  { match: /^(txt|log|text)$/, icon: FileText, className: 'text-zinc-400' },
  { match: /^(zip|tar|gz|bz2|rar|7z|xz)$/, icon: FileArchive, className: 'text-amber-400' },
  { match: /^(java|class|jar)$/, icon: Coffee, className: 'text-red-400' },
  { match: /^(sh|bash|zsh|bat|cmd|ps1)$/, icon: FileTerminal, className: 'text-emerald-400' },
  { match: /^(db|sqlite|sqlite3|sql)$/, icon: Database, className: 'text-teal-400' },
  { match: /^(css|scss|sass|less)$/, icon: Palette, className: 'text-pink-400' },
  { match: /^(html|htm|xml)$/, icon: Globe, className: 'text-orange-400' },
  { match: /^(mp4|webm|mkv|avi|mov|flv)$/, icon: FileVideo, className: 'text-rose-400' },
  { match: /^(mp3|wav|ogg|flac|m4a)$/, icon: FileAudio, className: 'text-fuchsia-400' },
  { match: /^(csv|xlsx|xls|ods)$/, icon: FileSpreadsheet, className: 'text-emerald-400' },
  { match: /^(py|pyw)$/, icon: FileCode, className: 'text-blue-400' },
  { match: /^(geojson|json5)$/, icon: FileJson, className: 'text-yellow-400' },
];

const FOLDER_META: FileMeta = { icon: Folder, className: 'text-amber-400' };
const PARENT_META: FileMeta = { icon: ArrowUpToLine, className: 'text-muted-foreground' };

function getFileMeta(name: string): FileMeta {
  const ext = name.split('.').pop()?.toLowerCase() || '';
  const match = FILE_META.find((m) => m.match.test(ext));
  return match || { icon: File, className: 'text-muted-foreground' };
}

function Breadcrumbs({ path, onNavigate }: { path: string; onNavigate: (p: string) => void }) {
  const parts = path.split('/').filter(Boolean);
  const crumbs = [{ label: '~', path: '' }, ...parts.map((p, i) => ({ label: p, path: parts.slice(0, i + 1).join('/') }))];

  return (
    <nav className="flex min-w-0 flex-1 items-center gap-0.5 overflow-x-auto font-mono text-sm" aria-label="Breadcrumb">
      {crumbs.map((crumb, i) => {
        const isLast = i === crumbs.length - 1;
        return (
          <Fragment key={`${crumb.path}-${i}`}>
            {i > 0 && <ChevronRight className="h-3.5 w-3.5 shrink-0 text-muted-foreground/40" />}
            <button
              type="button"
              onClick={() => onNavigate(crumb.path)}
              className={cn(
                'shrink-0 rounded-md px-1.5 py-1 transition-colors hover:bg-accent/10 hover:text-primary',
                isLast ? 'font-semibold text-primary' : 'text-muted-foreground'
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

  const closeEditor = () => {
    setIsEditorOpen(false);
    editorCloseTimeoutRef.current = window.setTimeout(() => {
      editorCloseTimeoutRef.current = null;
      setEditingFile(null);
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

  const { t } = useTranslations();
  

  const fetchFiles = async (path: string) => {
    setIsLoading(true);
    try {
      const data = await api.get(`/api/servers/${serverId}/file/${path}`);
      if (Array.isArray(data)) {
        setFiles(data);
      }
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || 'Failed to load files.' });
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchFiles(currentPath);
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
        const content = await api.get(`/api/servers/${serverId}/file/${filePath}`);
        if (editorCloseTimeoutRef.current !== null) {
          clearTimeout(editorCloseTimeoutRef.current);
          editorCloseTimeoutRef.current = null;
        }
        setEditedContent(typeof content === 'string' ? content : JSON.stringify(content, null, 2));
        setEditingFile(file);
        setIsEditorOpen(true);
      } catch (e: any) {
        sileo.error({ title: t('common.error'), description: e.message || 'Failed to load file content.' });
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
      const url = `/api/servers/${serverId}/file/${filePath}${newItemDialog.type === 'folder' ? '?folder' : ''}`;

      const response = await fetch(url, {
        method: 'PUT',
        body: newItemDialog.type === 'file' ? '' : undefined,
        credentials: 'include'
      });

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error || 'No se pudo crear el elemento en el servidor');
      }

      sileo.success({
        title: t('common.success'),
        description: newItemDialog.type === 'file' ? t('servers.fileManager.toast.createdFile') : t('servers.fileManager.toast.createdFolder')
      });

      // Reset dialog state
      setNewItemDialog(prev => ({ ...prev, open: false }));
      setNewItemName('');

      // Wait for Radix UI to finish closing and cleanup the body
      setTimeout(() => {
        fetchFiles(currentPath);
        setIsCreating(false);
        // Safety cleanup to ensure UI is not blocked
        document.body.style.pointerEvents = 'auto';
        document.body.style.overflow = 'auto';
      }, 300);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || 'Error al crear el elemento.' });
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

      await fetch(`/api/servers/${serverId}/file/${filePath}`, {
        method: 'PUT',
        body: formData,
        credentials: 'include'
      });

      sileo.success({ title: t('common.success'), description: t('servers.fileManager.toast.uploaded') });
      fetchFiles(currentPath);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || 'Error al subir el archivo.' });
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
    const res = await fetch(`/api/servers/${serverId}/file/${path}`, { credentials: 'include' });
    if (!res.ok) throw new Error(`Error al leer: ${path}`);
    return res.blob();
  };

  const makeFolder = async (path: string) => {
    const res = await fetch(`/api/servers/${serverId}/file/${path}?folder`, {
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
    const res = await fetch(`/api/servers/${serverId}/file/${path}`, {
      method: 'PUT',
      body: blob,
      credentials: 'include'
    });
    if (!res.ok) throw new Error(`Error al escribir: ${path}`);
  };

  const listFolderForCopy = async (path: string): Promise<FileItemResource[]> => {
    const data = await api.get(`/api/servers/${serverId}/file/${path}`);
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
      sileo.error({ title: t('common.error'), description: 'No puedes pegar aquí.' });
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
        await api.delete(`/api/servers/${serverId}/file/${sourcePath}`);
        setClipboard(null);
      }

      sileo.success({
        title: t('common.success'),
        description: clipboard.mode === 'cut'
          ? t('servers.fileManager.toast.moved')
          : t('servers.fileManager.toast.pasted')
      });
      fetchFiles(currentPath);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || 'Error en la operación.' });
    } finally {
      setIsLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!pendingFile) return;
    setIsLoading(true);
    try {
      const filePath = currentPath ? `${currentPath}/${pendingFile.name}` : pendingFile.name;
      await api.delete(`/api/servers/${serverId}/file/${filePath}`);
      sileo.success({ title: t('common.success'), description: t('servers.fileManager.toast.deleted') });
      fetchFiles(currentPath);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || 'Error al eliminar.' });
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
      const response = await fetch(`/api/servers/${serverId}/file/${srcPath}?destination=${encodeURIComponent(destPath)}`, {
        method: 'POST',
        credentials: 'include'
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error || 'No se pudo renombrar el elemento');
      }
      sileo.success({ title: t('common.success'), description: t('servers.fileManager.toast.renamed') });
      closeRename();
      // Ensure Radix Dialog/DropdownMenu cleanup finished so the UI is not left blocked
      setTimeout(() => {
        document.body.style.pointerEvents = 'auto';
        document.body.style.overflow = 'auto';
      }, 100);
      fetchFiles(currentPath);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || 'Error al renombrar.' });
      setIsRenaming(false);
    }
  };

  const handleExtract = async (file: FileItemResource) => {
    setIsLoading(true);
    try {
      const filePath = currentPath ? `${currentPath}/${file.name}` : file.name;
      await api.post(`/api/servers/${serverId}/extract/${filePath}?destination=.&skipRoot`, {});
      sileo.success({ title: t('common.success'), description: t('servers.fileManager.toast.extracted') });
      fetchFiles(currentPath);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || 'Error al descomprimir.' });
    } finally {
      setIsLoading(false);
    }
  };

  const handleSaveChanges = async () => {
    if (!editingFile || savingRef.current) return;
    savingRef.current = true;
    setIsSaving(true);
    try {
      const filePath = currentPath ? `${currentPath}/${editingFile.name}` : editingFile.name;
      await fetch(`/api/servers/${serverId}/file/${filePath}`, {
        method: 'PUT',
        body: editedContent,
        credentials: 'include'
      });
      setSavedFlash(true);
      setTimeout(() => setSavedFlash(false), 2000);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message || 'Error al guardar.' });
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
        handleSaveChanges();
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
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const formatDate = (epoch: number) => {
    if (!epoch) return '-';
    return new Date(epoch * 1000).toLocaleString();
  };

  const editingMeta = editingFile ? getFileMeta(editingFile.name) : { icon: File, className: 'text-muted-foreground' };
  const editingLanguage = editingFile ? getLanguageFromFilename(editingFile.name) : 'plaintext';
  const EditingIcon = editingMeta.icon;

  return (
    <>
      <div className="mt-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
        <Card className="border-0 bg-transparent shadow-none">
          <CardHeader className="flex flex-row items-center gap-4 px-0">
            <div className="flex items-center gap-4">
              <div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl border border-primary/30 bg-gradient-to-br from-primary/25 via-accent/15 to-transparent text-primary shadow-[0_0_20px_rgb(0_0_0/0.3)]">
                <FolderTree className="h-5 w-5" />
              </div>
              <div>
                <CardTitle className="font-headline text-2xl">{t('servers.fileManager.title')}</CardTitle>
                <CardDescription>{t('servers.fileManager.description')}</CardDescription>
              </div>
            </div>
          </CardHeader>

          <CardContent className="mt-4 space-y-4 px-0">
            {/* Path bar + toolbar */}
            <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div className="flex min-w-0 flex-1 items-center gap-2 rounded-xl border border-border/80 bg-card px-2 py-1.5">
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 shrink-0 text-muted-foreground hover:text-primary"
                  onClick={() => setCurrentPath('')}
                  disabled={!currentPath}
                  title="~/"
                >
                  <ArrowUpToLine className="h-4 w-4" />
                </Button>
                <TerminalSquare className="h-4 w-4 shrink-0 text-primary/70" />
                <Breadcrumbs path={currentPath} onNavigate={setCurrentPath} />
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <input type="file" ref={fileInputRef} className="hidden" onChange={handleFileChange} />
                <Button variant="outline" size="sm" onClick={handleUploadClick}>
                  <Upload className="h-4 w-4 mr-2" />
                  {t('servers.fileManager.upload')}
                </Button>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="default" size="sm">
                      <Plus className="h-4 w-4 mr-2" />
                      {t('servers.fileManager.new')}
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onClick={() => setNewItemDialog({ open: true, type: 'file' })}>
                      <FilePlus className="h-4 w-4 mr-2" />
                      {t('servers.fileManager.newFile')}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => setNewItemDialog({ open: true, type: 'folder' })}>
                      <FolderPlus className="h-4 w-4 mr-2" />
                      {t('servers.fileManager.newFolder')}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
                {clipboard && (
                  <div className="flex items-center gap-1 rounded-lg border border-border/80 bg-card px-1 py-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={handlePaste}
                      disabled={isLoading}
                      className="h-8 text-xs"
                      title={clipboard.mode === 'cut' ? t('servers.fileManager.actions.cut') : t('servers.fileManager.actions.copy')}
                    >
                      {clipboard.mode === 'cut'
                        ? <Scissors className="h-3.5 w-3.5 mr-1.5 text-amber-400" />
                        : <Copy className="h-3.5 w-3.5 mr-1.5 text-primary" />}
                      <span className="max-w-[120px] truncate font-mono">{clipboard.name}</span>
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-muted-foreground hover:text-destructive"
                      onClick={() => setClipboard(null)}
                    >
                      <X className="h-4 w-4" />
                    </Button>
                  </div>
                )}
              </div>
            </div>

            {/* File table */}
            <div className="relative overflow-hidden rounded-xl border border-border/80 bg-card">
              <div className="pointer-events-none h-0.5 bg-gradient-to-r from-primary via-accent to-transparent" />
              <Table>
                <TableHeader>
                  <TableRow className="border-border/60">
                    <TableHead>{t('servers.fileManager.table.name')}</TableHead>
                    <TableHead className="hidden sm:table-cell">{t('servers.fileManager.table.size')}</TableHead>
                    <TableHead className="hidden md:table-cell">{t('servers.fileManager.table.modified')}</TableHead>
                    <TableHead className="text-right">{t('servers.fileManager.table.actions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {isLoading ? (
                    <TableRow className="border-0">
                      <TableCell colSpan={4} className="h-40 text-center">
                        <Loader2 className="h-6 w-6 animate-spin mx-auto text-primary" />
                      </TableCell>
                    </TableRow>
                  ) : files.length === 0 ? (
                    <TableRow className="border-0">
                      <TableCell colSpan={4} className="h-40 text-center">
                        <div className="flex flex-col items-center justify-center gap-3 py-4">
                          <div className="grid h-12 w-12 place-items-center rounded-xl border border-primary/20 bg-gradient-to-br from-primary/15 via-accent/10 to-transparent text-muted-foreground">
                            <FolderTree className="h-5 w-5" />
                          </div>
                          <div>
                            <p className="font-headline font-semibold">{t('servers.fileManager.empty')}</p>
                            <p className="mt-0.5 text-sm text-muted-foreground">{t('servers.fileManager.emptyHint')}</p>
                          </div>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : (
                    files.map((file) => {
                      const isParent = !file.isFile && file.name === '..';
                      const meta = isParent ? PARENT_META : file.isFile ? getFileMeta(file.name) : FOLDER_META;
                      const MetaIcon = meta.icon;
                      return (
                        <TableRow key={file.name} className="group border-border/60 transition-colors hover:bg-accent/5">
                          <TableCell
                            className="cursor-pointer"
                            onClick={() => handleFileClick(file)}
                          >
                            <div className="flex items-center gap-3">
                              <div className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-border/70 bg-background/50 backdrop-blur-sm transition-colors group-hover:border-primary/30">
                                <MetaIcon className={cn('h-4 w-4', meta.className, !file.isFile && !isParent && 'fill-amber-400/15')} />
                              </div>
                              <span
                                className={cn(
                                  'truncate font-medium',
                                  isParent
                                    ? 'text-muted-foreground'
                                    : 'transition-colors group-hover:text-primary'
                                )}
                              >
                                {file.name}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell className="hidden font-mono text-xs text-muted-foreground sm:table-cell">
                            {file.isFile ? formatSize(file.size) : '-'}
                          </TableCell>
                          <TableCell className="hidden text-xs text-muted-foreground md:table-cell">
                            {formatDate(file.modifyTime)}
                          </TableCell>
                          <TableCell className="text-right">
                            <DropdownMenu>
                              <DropdownMenuTrigger asChild>
                                <Button variant="ghost" className="h-8 w-8 p-0 opacity-60 transition-opacity group-hover:opacity-100">
                                  <span className="sr-only">Open menu</span>
                                  <MoreHorizontal className="h-4 w-4" />
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end">
                                {file.isFile ? (
                                  <DropdownMenuItem onClick={() => setTimeout(() => handleFileClick(file), 50)}>
                                    <Edit2 className="h-4 w-4 mr-2" />
                                    {t('servers.fileManager.actions.edit')}
                                  </DropdownMenuItem>
                                ) : null}
                                <DropdownMenuItem asChild>
                                  <a href={`/api/servers/${serverId}/file/${currentPath ? `${currentPath}/${file.name}` : file.name}`} download>
                                    <Download className="h-4 w-4 mr-2" />
                                    {t('servers.fileManager.actions.download')}
                                  </a>
                                </DropdownMenuItem>
                                {file.isFile && file.name.match(/\.(zip|tar(\.gz)?|tgz|gz|bz2|rar|7z|xz)$/i) ? (
                                  <DropdownMenuItem onClick={() => handleExtract(file)}>
                                    <FileArchive className="h-4 w-4 mr-2 text-amber-400" />
                                    {t('servers.fileManager.actions.unarchive')}
                                  </DropdownMenuItem>
                                ) : null}
                                <DropdownMenuItem onClick={() => setTimeout(() => openRename(file), 50)}>
                                  <Edit2 className="h-4 w-4 mr-2" />
                                  {t('servers.fileManager.actions.rename')}
                                </DropdownMenuItem>
                                <DropdownMenuSeparator />
                                <DropdownMenuItem onClick={() => handleCopy(file, 'copy')}>
                                  <Copy className="h-4 w-4 mr-2" />
                                  {t('servers.fileManager.actions.copy')}
                                </DropdownMenuItem>
                                <DropdownMenuItem onClick={() => handleCopy(file, 'cut')}>
                                  <Scissors className="h-4 w-4 mr-2" />
                                  {t('servers.fileManager.actions.cut')}
                                </DropdownMenuItem>
                                <DropdownMenuSeparator />
                                <AlertDialog open={pendingFile?.name === file.name} onOpenChange={(open) => !open && setPendingFile(null)}>
                                  <AlertDialogTrigger asChild>
                                    <DropdownMenuItem className="text-destructive focus:bg-destructive/10" onSelect={(e) => { e.preventDefault(); setPendingFile(file); }}>
                                      <Trash2 className="h-4 w-4 mr-2" />
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
                                      <AlertDialogAction onClick={handleDelete} className="bg-red-500 hover:bg-red-600">
                                        {t('servers.fileManager.actions.delete')}
                                      </AlertDialogAction>
                                    </AlertDialogFooter>
                                  </AlertDialogContent>
                                </AlertDialog>
                              </DropdownMenuContent>
                            </DropdownMenu>
                          </TableCell>
                        </TableRow>
                      );
                    })
                  )}
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Editor Dialog */}
      <Dialog open={isEditorOpen} onOpenChange={(open) => !open && closeEditor()}>
        <DialogContent className="flex h-[90vh] w-[95vw] max-w-[95vw] flex-col gap-0 overflow-hidden p-0">
          <div className="flex items-center justify-between gap-3 border-b border-border/80 bg-card px-5 py-4 pr-12">
            <div className="flex min-w-0 items-center gap-3">
              <div className="grid h-10 w-10 shrink-0 place-items-center rounded-lg border border-primary/25 bg-gradient-to-br from-primary/20 via-accent/10 to-transparent text-primary">
                <EditingIcon className={cn('h-5 w-5', editingMeta.className)} />
              </div>
              <div className="min-w-0">
                <p className="truncate font-headline text-base font-semibold leading-tight">{editingFile?.name}</p>
                <p className="truncate font-mono text-[11px] text-muted-foreground">
                  ~{currentPath ? `/${currentPath}` : ''}
                </p>
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <Badge variant="outline" className="font-mono text-[10px] uppercase tracking-wider text-primary">
                {editingLanguage}
              </Badge>
              {savedFlash && (
                <span className="flex items-center gap-1 text-xs font-semibold text-emerald-400">
                  <Check className="h-3.5 w-3.5" />
                  {t('common.saved') || 'Saved'}
                </span>
              )}
            </div>
          </div>

          <div className="min-h-0 flex-1">
            {editingFile && (
              <Suspense fallback={<div className="flex h-full items-center justify-center"><Loader2 className="h-8 w-8 animate-spin text-primary" /></div>}>
                <CodeEditor
                  key={editingFile.name}
                  language={editingLanguage}
                  value={editedContent}
                  onChange={(value) => setEditedContent(value || '')}
                  onSave={handleSaveChanges}
                />
              </Suspense>
            )}
          </div>

          <div className="flex items-center justify-between gap-3 border-t border-border/80 bg-card px-5 py-3">
            <p className="hidden items-center gap-1.5 text-xs text-muted-foreground sm:flex">
              <TerminalSquare className="h-3.5 w-3.5" />
              {t('servers.fileManager.editor.saveHint')}
            </p>
            <div className="flex items-center gap-2">
              <Button variant="outline" onClick={closeEditor}>
                {t('servers.fileManager.editor.cancel')}
              </Button>
              <Button onClick={handleSaveChanges} disabled={isSaving} className="min-w-[120px]">
                {isSaving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {!isSaving && <Check className="mr-2 h-4 w-4" />}
                {t('servers.fileManager.editor.save')}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

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
              <Label htmlFor="name">{t('servers.fileManager.nameLabel')}</Label>
              <Input
                id="name"
                value={newItemName}
                onChange={(e) => setNewItemName(e.target.value)}
                placeholder={newItemDialog.type === 'file' ? t('servers.fileManager.createFilePlaceholder') : t('servers.fileManager.createFolderPlaceholder')}
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
              {isCreating && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
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
              <Label htmlFor="rename">{t('servers.fileManager.nameLabel')}</Label>
              <Input
                id="rename"
                value={renameValue}
                onChange={(e) => setRenameValue(e.target.value)}
                placeholder={t('servers.fileManager.renameDialog.placeholder')}
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
              {isRenaming && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t('servers.fileManager.actions.rename')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
