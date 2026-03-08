import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Folder, File as FileIcon, MoreHorizontal, ChevronLeft, Loader2, Plus, Upload, FolderPlus, FilePlus, Copy, Scissors, Clipboard, Trash2, Edit2, Download } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger, DropdownMenuSeparator } from '@/components/ui/dropdown-menu';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useState, useEffect, lazy, Suspense, useRef } from 'react';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { useToast } from '@/hooks/use-toast';

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

export default function FileManagerView({ serverId }: { serverId: string }) {
  const [currentPath, setCurrentPath] = useState('');
  const [files, setFiles] = useState<FileItemResource[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [editingFile, setEditingFile] = useState<FileItemResource | null>(null);
  const [editedContent, setEditedContent] = useState<string>('');
  const [isSaving, setIsSaving] = useState(false);

  // New State for operations
  const [newItemDialog, setNewItemDialog] = useState<{ open: boolean; type: 'file' | 'folder' }>({ open: false, type: 'file' });
  const [newItemName, setNewItemName] = useState('');
  const [isCreating, setIsCreating] = useState(false);

  const [clipboard, setClipboard] = useState<ClipboardItem | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const { t } = useTranslations();
  const { toast } = useToast();

  const fetchFiles = async (path: string) => {
    setIsLoading(true);
    try {
      const data = await api.get(`/api/servers/${serverId}/file/${path}`);
      if (Array.isArray(data)) {
        setFiles(data);
      }
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Failed to load files.', variant: 'destructive' });
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
        setEditedContent(typeof content === 'string' ? content : JSON.stringify(content, null, 2));
        setEditingFile(file);
      } catch (e: any) {
        toast({ title: t('common.error'), description: e.message || 'Failed to load file content.', variant: 'destructive' });
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

      toast({ title: t('common.success'), description: `${newItemDialog.type === 'file' ? 'Archivo' : 'Carpeta'} creado correctamente.` });

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
      toast({ title: t('common.error'), description: e.message || 'Error al crear el elemento.', variant: 'destructive' });
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

      toast({ title: t('common.success'), description: 'Archivo subido correctamente.' });
      fetchFiles(currentPath);
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Error al subir el archivo.', variant: 'destructive' });
    } finally {
      setIsLoading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const handleCopy = (file: FileItemResource, mode: 'copy' | 'cut') => {
    const path = currentPath ? `${currentPath}/${file.name}` : file.name;
    setClipboard({ name: file.name, path, isFile: file.isFile, mode });
    toast({ description: `${mode === 'copy' ? 'Copiado' : 'Cortado'}: ${file.name}` });
  };

  const handlePaste = async () => {
    if (!clipboard) return;

    setIsLoading(true);
    try {
      const destPath = currentPath ? `${currentPath}/${clipboard.name}` : clipboard.name;

      // Since PufferPanel doesn't have a direct "copy" API in daemon for single files easily 
      // without using specialized archive/extract, we'll use the file rename/move if it's 'cut'
      // If it's 'copy', we would need to read and write. 
      // For simplicity and following PufferPanel common patterns:

      if (clipboard.mode === 'cut') {
        // This is technically a move. We can use the rename/move logic if available or re-upload.
        // Actually, let's just alert that move is easier.
        toast({ title: "Próximamente", description: "La función de mover/copiar entre carpetas se está integrando con el backend." });
      }
    } catch (e: any) {
      toast({ title: t('common.error'), description: 'Error en la operación.', variant: 'destructive' });
    } finally {
      setIsLoading(false);
    }
  };

  const handleDelete = async (file: FileItemResource) => {
    if (!confirm(`¿Estás seguro de que quieres eliminar ${file.name}?`)) return;
    setIsLoading(true);
    try {
      const filePath = currentPath ? `${currentPath}/${file.name}` : file.name;
      await api.delete(`/api/servers/${serverId}/file/${filePath}`);
      toast({ title: t('common.success'), description: 'Eliminado correctamente.' });
      fetchFiles(currentPath);
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Error al eliminar.', variant: 'destructive' });
    } finally {
      setIsLoading(false);
    }
  };

  const handleSaveChanges = async () => {
    if (!editingFile) return;
    setIsSaving(true);
    try {
      const filePath = currentPath ? `${currentPath}/${editingFile.name}` : editingFile.name;
      await fetch(`/api/servers/${serverId}/file/${filePath}`, {
        method: 'PUT',
        body: editedContent,
        credentials: 'include'
      });
      toast({ title: t('common.success'), description: 'Archivo guardado correctamente.' });
      setEditingFile(null);
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message || 'Error al guardar.', variant: 'destructive' });
    } finally {
      setIsSaving(false);
    }
  };

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
      case 'php': return 'php';
      case 'go': return 'go';
      case 'sh': return 'shell';
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

  return (
    <>
      <div className="mt-6 flex flex-col gap-4">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <Button variant="outline" size="icon" onClick={() => setCurrentPath('')} disabled={!currentPath}>
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <div className="flex items-center gap-1 text-sm font-mono bg-muted p-2 rounded-md overflow-x-auto max-w-[300px] md:max-w-md">
              <span className="text-muted-foreground mr-1">/</span>
              {currentPath || '(root)'}
            </div>
          </div>

          <div className="flex items-center gap-2">
            <input type="file" ref={fileInputRef} className="hidden" onChange={handleFileChange} />
            <Button variant="outline" size="sm" onClick={handleUploadClick}>
              <Upload className="h-4 w-4 mr-2" />
              Subir
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="default" size="sm">
                  <Plus className="h-4 w-4 mr-2" />
                  Nuevo
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent>
                <DropdownMenuItem onClick={() => setNewItemDialog({ open: true, type: 'file' })}>
                  <FilePlus className="h-4 w-4 mr-2" />
                  Archivo
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => setNewItemDialog({ open: true, type: 'folder' })}>
                  <FolderPlus className="h-4 w-4 mr-2" />
                  Carpeta
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            {clipboard && (
              <Button variant="secondary" size="sm" onClick={handlePaste}>
                <Clipboard className="h-4 w-4 mr-2" />
                Pegar
              </Button>
            )}
          </div>
        </div>

        <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
          <Card className="border-0">
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>{t('servers.fileManager.title')}</CardTitle>
                  <CardDescription>{t('servers.fileManager.description')}</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('servers.fileManager.table.name')}</TableHead>
                    <TableHead className="hidden sm:table-cell">{t('servers.fileManager.table.size')}</TableHead>
                    <TableHead className="hidden md:table-cell">{t('servers.fileManager.table.modified')}</TableHead>
                    <TableHead className="text-right">{t('servers.fileManager.table.actions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {isLoading ? (
                    <TableRow>
                      <TableCell colSpan={4} className="h-24 text-center">
                        <Loader2 className="h-6 w-6 animate-spin mx-auto text-primary" />
                      </TableCell>
                    </TableRow>
                  ) : files.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4} className="h-24 text-center text-muted-foreground">
                        Esta carpeta está vacía.
                      </TableCell>
                    </TableRow>
                  ) : (
                    files.map((file) => (
                      <TableRow key={file.name} className="group">
                        <TableCell
                          className={`font-medium flex items-center gap-2 cursor-pointer group-hover:text-primary transition-colors`}
                          onClick={() => handleFileClick(file)}
                        >
                          {file.isFile ? <FileIcon className="h-4 w-4 text-muted-foreground" /> : <Folder className="h-4 w-4 text-yellow-500 fill-yellow-500" />}
                          {file.name}
                        </TableCell>
                        <TableCell className="hidden sm:table-cell text-muted-foreground">{file.isFile ? formatSize(file.size) : '-'}</TableCell>
                        <TableCell className="hidden md:table-cell text-muted-foreground">{formatDate(file.modifyTime)}</TableCell>
                        <TableCell className="text-right">
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button variant="ghost" className="h-8 w-8 p-0">
                                <span className="sr-only">Open menu</span>
                                <MoreHorizontal className="h-4 w-4" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              {file.isFile ? (
                                <DropdownMenuItem onClick={() => handleFileClick(file)}>
                                  <Edit2 className="h-4 w-4 mr-2" />
                                  Editar
                                </DropdownMenuItem>
                              ) : null}
                              <DropdownMenuItem asChild>
                                <a href={`/api/servers/${serverId}/file/${currentPath ? `${currentPath}/${file.name}` : file.name}`} download>
                                  <Download className="h-4 w-4 mr-2" />
                                  Descargar
                                </a>
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem onClick={() => handleCopy(file, 'copy')}>
                                <Copy className="h-4 w-4 mr-2" />
                                Copiar
                              </DropdownMenuItem>
                              <DropdownMenuItem onClick={() => handleCopy(file, 'cut')}>
                                <Scissors className="h-4 w-4 mr-2" />
                                Cortar
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem className="text-red-500" onClick={() => handleDelete(file)}>
                                <Trash2 className="h-4 w-4 mr-2" />
                                Eliminar
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Editor Dialog */}
      <Dialog open={!!editingFile} onOpenChange={(open) => !open && setEditingFile(null)}>
        <DialogContent className="w-[95vw] max-w-[95vw] h-[90vh] flex flex-col p-6">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <FileIcon className="h-5 w-5" />
              Editando: {editingFile?.name}
            </DialogTitle>
          </DialogHeader>
          <div className="flex-grow min-h-0 border rounded-md mt-4 overflow-hidden">
            {editingFile && (
              <Suspense fallback={<div className="flex items-center justify-center h-full"><Loader2 className="h-8 w-8 animate-spin" /></div>}>
                <CodeEditor
                  key={editingFile.name}
                  language={getLanguageFromFilename(editingFile.name)}
                  value={editedContent}
                  onChange={(value) => setEditedContent(value || '')}
                />
              </Suspense>
            )}
          </div>
          <DialogFooter className="mt-4 gap-2">
            <Button variant="outline" onClick={() => setEditingFile(null)}>
              Cancelar
            </Button>
            <Button onClick={handleSaveChanges} disabled={isSaving}>
              {isSaving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Guardar Cambios
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* New Item Dialog */}
      <Dialog open={newItemDialog.open} onOpenChange={(open) => !open && closeNewItemDialog()}>
        <DialogContent className="sm:max-w-[425px]">
          <DialogHeader>
            <DialogTitle>Crear {newItemDialog.type === 'file' ? 'Archivo' : 'Carpeta'}</DialogTitle>
            <DialogDescription>
              Introduce el nombre del nuevo elemento que quieres crear en el directorio actual.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid grid-cols-4 items-center gap-4">
              <Label htmlFor="name" className="text-right">
                Nombre
              </Label>
              <Input
                id="name"
                value={newItemName}
                onChange={(e) => setNewItemName(e.target.value)}
                className="col-span-3"
                placeholder={newItemDialog.type === 'file' ? 'nuevo-archivo.txt' : 'nueva-carpeta'}
                autoFocus
                onKeyDown={(e) => e.key === 'Enter' && handleCreateItem()}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={closeNewItemDialog}>
              Cancelar
            </Button>
            <Button onClick={handleCreateItem} disabled={isCreating || !newItemName}>
              {isCreating && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Crear
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
