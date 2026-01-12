<script setup>
import { computed, ref, onMounted, onUnmounted, inject, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import ContextMenu from '@/components/ui/ContextMenu.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import Overlay from '@/components/ui/Overlay.vue'
import Editor, { skipDownload } from './files/Editor.vue'
import Upload from './files/Upload.vue'
import TextField from '@/components/ui/TextField.vue'

const { t } = useI18n()
const events = inject('events')
const toast = inject('toast')

const props = defineProps({
  server: { type: Object, required: true }
})

const allowDirectoryUpload = 'webkitdirectory' in document.createElement('input')
const canEdit = props.server.hasScope('server.files.edit')

const fileEls = ref([])
const files = ref(null)
const file = ref(null)
const fileSizeWarn = ref(false)
const fileSizeWarnSubject = ref(null)
const currentPath = ref([])
const editorOpen = ref(false)
const loading = ref(false)
const createFileOpen = ref(false)
const createFolderOpen = ref(false)
const archiveSelectedOpen = ref(false)
const newItemName = ref('')
const selection = computed(() => {
  return (files.value || []).filter(f => f.isSelected)
})

let task
let unbindEvent
onMounted(() => {
  refresh()

  unbindEvent = props.server.on('status', () => {
    if (selection.value.length === 0) refresh()
  })

  task = props.server.startTask(() => {
    if (selection.value.length === 0) refresh()
  }, 5 * 60 * 1000)
})

onUnmounted(async () => {
  if (unbindEvent) unbindEvent()
  if (task) props.server.stopTask(task)
})

watch(currentPath, async (newPath) => {
  const res = await props.server.getFile(newPath.map(e => e.name).join('/'))
  files.value = res.sort(sortFiles)
}, {deep: true})

async function refresh(manual = false) {
  try {
    if (manual) files.value = null // cause visual feedback on manual refresh
    loading.value = true
    const res = await props.server.getFile(getCurrentPath())
    files.value = res.sort(sortFiles)
  } catch (error) {
    console.error('Error refreshing files:', error)
    toast.error(t('files.RefreshError') || 'Error al actualizar los archivos')
  } finally {
    loading.value = false
  }
}

function sortFiles(a, b) {
  if (a.isFile && !b.isFile) return 1
  if (!a.isFile && b.isFile) return -1
  if (a.name.toLowerCase() < b.name.toLowerCase()) return -1
  return 1
}

function getCurrentPath() {
  return currentPath.value.map(e => e.name).join('/')
}

async function openFile(f, overrideWarn = false) {
  if (f.isFile) {
    if (!skipDownload(f) && !overrideWarn && f.size > 30 * Math.pow(2, 20)) {
      fileSizeWarnSubject.value = f
      fileSizeWarn.value = true
      return
    }

    fileSizeWarn.value = false
    loading.value = true
    const path = getCurrentPath() + `/${f.name}`
    const content = skipDownload(f) ? null : await props.server.getFile(path, true)
    file.value = { ...f, content, url: props.server.getFileUrl(path) }
    editorOpen.value = true
    loading.value = false
  } else {
    if (f.name === '..') {
      currentPath.value.pop()
    } else {
      currentPath.value.push(f)
    }
  }
}

async function saveFile({close}) {
  await props.server.uploadFile(`${getCurrentPath()}/${file.value.name}`, file.value.content)
  toast.success(t('files.Saved'))
  if (close) {
    editorOpen.value = false
    file.value = null
  }
  refresh()
}

function getIcon(file) {
  if (!file.isFile) return 'hi-folder'
  if (!file.extension) return 'hi-document'
  
  const ext = file.extension.substring(1).toLowerCase()
  
  // Mapeo de extensiones a iconos de Devicon (https://devicon.dev/)
  const deviconMap = {
    // Lenguajes de programación
    'java': 'java',
    'jar': 'java',
    'war': 'java',
    'ear': 'java',
    'class': 'java',
    'js': 'javascript',
    'mjs': 'javascript',
    'cjs': 'javascript',
    'ts': 'typescript',
    'tsx': 'typescript',
    'jsx': 'react',
    'py': 'python',
    'pyc': 'python',
    'pyw': 'python',
    'rb': 'ruby',
    'php': 'php',
    'go': 'go',
    'rs': 'rust',
    'c': 'c',
    'cpp': 'cplusplus',
    'cc': 'cplusplus',
    'cxx': 'cplusplus',
    'h': 'c',
    'hpp': 'cplusplus',
    'cs': 'csharp',
    'kt': 'kotlin',
    'kts': 'kotlin',
    'swift': 'swift',
    'lua': 'lua',
    'r': 'r',
    'dart': 'dart',
    'scala': 'scala',
    'groovy': 'groovy',
    'pl': 'perl',
    'pm': 'perl',
    
    // Frameworks y herramientas
    'vue': 'vuejs',
    'gradle': 'gradle',
    
    // Configuración y datos
    'json': 'json',
    'yml': 'yaml',
    'yaml': 'yaml',
    'toml': 'toml',
    'xml': 'xml',
    'sql': 'mysql',
    'md': 'markdown',
    'properties': 'java',
    
    // Web
    'html': 'html5',
    'htm': 'html5',
    'css': 'css3',
    'scss': 'sass',
    'sass': 'sass',
    'less': 'less',
    
    // Bases de datos
    'db': 'sqlite',
    'sqlite': 'sqlite',
    'sqlite3': 'sqlite',
    
    // Docker y contenedores
    'dockerfile': 'docker',
    
    // Git
    'gitignore': 'git',
    'gitattributes': 'git',
    
    // Node.js
    'npm': 'npm',
    'node': 'nodejs',
  }
  
  // Si hay un icono de Devicon, devolverlo con el prefijo 'devicon:'
  if (deviconMap[ext]) {
    const iconName = 'devicon:' + deviconMap[ext]
    console.log(`File: ${file.name}, Extension: ${ext}, Icon: ${iconName}`)
    return iconName
  }
  
  // Fallback a Heroicons para otros tipos
  const heroiconMap = {
    // Archivos comprimidos
    'zip': 'archive',
    'rar': 'archive',
    'tar': 'archive',
    'gz': 'archive',
    '7z': 'archive',
    'bz2': 'archive',
    'xz': 'archive',
    
    // Ejecutables
    'exe': 'hi-cube',
    'dll': 'hi-cube',
    'so': 'hi-cube',
    'dylib': 'hi-cube',
    
    // Archivos de texto
    'txt': 'hi-document-text',
    'log': 'hi-document-text',
    'csv': 'hi-document-text',
    
    // Imágenes
    'png': 'hi-photo',
    'jpg': 'hi-photo',
    'jpeg': 'hi-photo',
    'gif': 'hi-photo',
    'svg': 'hi-photo',
    'webp': 'hi-photo',
    'ico': 'hi-photo',
    'bmp': 'hi-photo',
    
    // Scripts de shell
    'sh': 'hi-command-line',
    'bash': 'hi-command-line',
    'bat': 'hi-command-line',
    'cmd': 'hi-command-line',
    'ps1': 'hi-command-line',
    
    // Documentos
    'pdf': 'hi-document',
    'doc': 'hi-document',
    'docx': 'hi-document',
    
    // Configuración genérica
    'ini': 'hi-cog-6-tooth',
    'conf': 'hi-cog-6-tooth',
    'config': 'hi-cog-6-tooth',
  }
  
  return heroiconMap[ext] || 'hi-document'
}

function deleteFile(file) {
  events.emit(
    'confirm',
    t('files.ConfirmDelete', { name: file.name }),
    {
      text: t('files.Delete'),
      icon: 'remove',
      color: 'error',
      action: async () => {
        await props.server.deleteFile(getCurrentPath() + '/' + file.name)
        await refresh()
      }
    },
    {
      color: 'primary'
    }
  )
}

const numFormat = new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 })
function formatFileSize(size) {
  if (!size) return '0 B'
  if (size < Math.pow(2, 10)) return numFormat.format(size) + ' B'
  if (size < Math.pow(2, 20)) return numFormat.format(size / Math.pow(2, 10)) + ' KiB'
  if (size < Math.pow(2, 30)) return numFormat.format(size / Math.pow(2, 20)) + ' MiB'
  if (size < Math.pow(2, 40)) return numFormat.format(size / Math.pow(2, 30)) + ' GiB'
  return numFormat.format(size / Math.pow(2, 40)) + ' TiB'
}

function startCreateFile() {
  newItemName.value = ''
  createFileOpen.value = true
}

function startCreateFolder() {
  newItemName.value = ''
  createFolderOpen.value = true
}

async function createFile() {
  if (!newItemName.value || newItemName.value.trim() === '') return
  await props.server.uploadFile(`${getCurrentPath()}/${newItemName.value}`, '')
  const file = { name: newItemName.value, size: 0, isFile: true }
  createFileOpen.value = false
  newItemName.value = ''
  await openFile(file)
  await refresh()
}

async function createFolder() {
  if (!newItemName.value || newItemName.value.trim() === '') return
  await props.server.createFolder(`${getCurrentPath()}/${newItemName.value}`)
  const folder = { name: newItemName.value, isFile: false }
  createFolderOpen.value = false
  newItemName.value = ''
  await openFile(folder)
  await refresh()
}

const archiveExtensions = [
  '.7z',
  '.bz2',
  '.gz',
  '.lz',
  '.lzma',
  '.rar',
  '.tar',
  '.tgz',
  '.xz',
  '.zip',
  '.zipx'
]

function isArchive (file) {
  const filename = file.name.toLowerCase()
  for (let i = 0; i < archiveExtensions.length; i++) {
    if (filename.endsWith(archiveExtensions[i])) return true
  }
  return false
}

async function extract(file) {
  loading.value = true
  try {
    let dest = getCurrentPath()
    if (!dest.startsWith('/')) dest = '/' + dest
    await props.server.extractFile(`${getCurrentPath()}/${file.name}`, dest)
    refresh()
  } finally {
    loading.value = false
  }
}

async function makeArchiveName(fileName) {
  let destination = `${getCurrentPath()}/${fileName}.zip`
  for (let i = 2; await props.server.fileExists(destination); i++) {
    destination = `${getCurrentPath()}/${fileName} (${i}).zip`
  }
  return destination
}

async function archiveCurrentDirectory() {
  loading.value = true
  try {
    const item = currentPath.value[currentPath.value.length - 1];
    let lastPathEntry = props.server.id
    if (item !== undefined) {
      lastPathEntry = currentPath.value[currentPath.value.length - 1].name
    }

    await props.server.archiveFile(
      await makeArchiveName(lastPathEntry),
      `${getCurrentPath()}`
    )
  } finally {
    setTimeout(() => {
      refresh()
      loading.value = false
    }, 500)
  }
}

async function archive(file) {
  loading.value = true
  try {
    await props.server.archiveFile(
      await makeArchiveName(file.name),
      `${getCurrentPath()}/${file.name}`
    )
  } finally {
    setTimeout(() => {
      refresh()
      loading.value = false
    }, 500)
  }
}

function download(file) {
  const a = document.createElement('a')
  a.href = props.server.getFileUrl(getCurrentPath() + '/' + file.name)
  a.download = a.href.split('/').pop()
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

function fileListHotkey() {
  if (fileEls.value[0]) fileEls.value[0].focus()
}

function trackFileEl(index) {
  return (el) => fileEls.value[index] = el
}

function contextActionsForFile(file) {
  const actions = []
  if (file.name !== '..') {
    actions.push({
      icon: file.isSelected ? 'deselect' :'select',
      label: t(file.isSelected ? 'files.Deselect' : 'files.Select'),
      hotkey: 's',
      action: () => file.isSelected = !file.isSelected
    })
  }
  if (canEdit && file.name !== '..' && !file.isFile) {
    actions.push({
      icon: 'archive',
      label: t('files.Archive'),
      hotkey: 'a',
      action: () => archive(file)
    })
  }
  if (canEdit && file.isFile && isArchive(file)) {
    actions.push({
      icon: 'extract',
      label: t('files.Extract'),
      hotkey: 'e',
      action: () => extract(file)
    })
  }
  if (file.isFile) {
    actions.push({
      icon: 'download',
      label: t('files.Download'),
      hotkey: 'd',
      action: () => download(file)
    })
  }
  if (canEdit && file.name !== '..') {
    actions.push({
      icon: 'remove',
      label: t('files.Delete'),
      hotkey: 'Delete',
      class: 'action-delete',
      action: () => deleteFile(file)
    })
  }
  return actions
}

async function archiveSelected() {
  const archiveName = newItemName.value
  newItemName.value = ''
  archiveSelectedOpen.value = false
  loading.value = true
  try {
    await props.server.archiveFile(
      await makeArchiveName(archiveName),
      selection.value.map(f => {
        return `${getCurrentPath()}/${f.name}`
      })
    )
  } finally {
    setTimeout(async () => {
      await refresh()
      loading.value = false
    }, 500)
  }
}

function deleteSelected() {
  events.emit(
    'confirm',
    t('files.ConfirmDeleteSelected', undefined, selection.value.length),
    {
      text: t('files.Delete'),
      icon: 'remove',
      color: 'error',
      action: async () => {
        loading.value = true
        try {
          await Promise.all(
            selection.value.map(f => props.server.deleteFile(getCurrentPath() + '/' + f.name))
          )
        } finally {
          await refresh()
          loading.value = false
        }
      }
    },
    {
      color: 'primary'
    }
  )
}

function deselectAll() {
  files.value = files.value.map(f => {
    f.isSelected = false
    return f
  })
}

function selectAll() {
  if (files.value === null) return
  files.value = files.value.map(f => {
    f.isSelected = f.name !== '..'
    return f
  })
}
</script>

<template>
  <div v-hotkey="'Control+a'" class="space-y-4 p-4" @hotkey="selectAll()">
    <div class="flex flex-col gap-4 pb-4 border-b-2 border-border/50">
      <h2 class="text-2xl font-bold text-foreground" v-text="t('servers.Files')" />
      <div class="flex items-center gap-2 flex-wrap text-sm">
        <a @click="currentPath = []" class="text-primary hover:text-primary-foreground transition-colors cursor-pointer"><icon name="hi-home" /></a>
        <span v-for="segment, index in currentPath" :key="index" class="flex items-center gap-2">
          <icon name="hi-chevron-right" class="text-muted-foreground" />
          <a @click="currentPath.splice(index + 1)" class="text-primary hover:text-primary-foreground transition-colors cursor-pointer" v-text="segment.name" />
        </span>
      </div>
      <div class="flex-1" />
      <div v-if="selection.length === 0" class="flex items-center gap-2 flex-wrap">
        <btn v-if="canEdit" v-hotkey="'f a'" variant="icon" :tooltip="t('files.ArchiveCurrent')" class="" @click="archiveCurrentDirectory()"><icon name="hi-folder" /></btn>
        <upload v-if="canEdit" :path="getCurrentPath()" :server="server" hotkey="f u" @uploaded="refresh()" />
        <upload v-if="canEdit && allowDirectoryUpload" :path="getCurrentPath()" :server="server" folder hotkey="f d" @uploaded="refresh()" />
        <btn v-if="canEdit" v-hotkey="'f c f'" variant="icon" :tooltip="t('files.CreateFile')" class="" @click="startCreateFile()"><icon name="hi-document" /></btn>
        <btn v-if="canEdit" v-hotkey="'f c d'" variant="icon" :tooltip="t('files.CreateFolder')" class="" @click="startCreateFolder()"><icon name="hi-folder" /></btn>
        <btn v-hotkey="'f r'" variant="icon" :tooltip="t('files.Refresh')" class="" @click="refresh(true)" @hotkey="refresh(true)"><icon name="refresh" class="rotate-180" /></btn>
      </div>
      <div v-else class="flex items-center gap-2 flex-wrap">
        <span class="text-sm font-medium text-foreground" v-text="t('files.Selected', undefined, selection.length)" />
        <btn v-if="canEdit" v-hotkey="'f s a'" variant="icon" :tooltip="t('files.ArchiveSelected')" @click="archiveSelectedOpen = true"><icon name="archive" /></btn>
        <btn v-if="canEdit" v-hotkey="'f s d'" variant="icon" :tooltip="t('files.DeleteSelected', undefined, selection.length)" @click="deleteSelected()"><icon name="hi-trash" /></btn>
        <btn v-hotkey="'Escape'" variant="icon" :tooltip="t('files.DeselectAll')" @click="deselectAll()"><icon name="close" /></btn>
      </div>
    </div>
    <div v-hotkey="'f l'" class="files-grid-container" @hotkey="fileListHotkey">
      <loader v-if="!Array.isArray(files)" />
      <div v-else class="files-grid">
        <div 
          v-for="(fileItem, index) in files" 
          :key="fileItem.name" 
          :ref="trackFileEl(index)" 
          tabindex="0" 
          :class="[
            'file-card',
            fileItem.isSelected ? 'file-card-selected' : '',
            fileItem.isFile ? 'file-card-file' : 'file-card-folder'
          ]"
          @click="openFile(fileItem)" 
          @keydown.enter="openFile(fileItem)" 
          @keydown.space.prevent="fileItem.isSelected = !fileItem.isSelected"
        >
          <div class="file-card-header">
            <icon 
              :class="[
                'file-icon',
                fileItem.isFile ? 'file-icon-file' : 'file-icon-folder'
              ]" 
              :name="getIcon(fileItem)" 
            />
            <div 
              v-if="fileItem.name !== '..'"
              class="file-card-checkbox"
              @click.stop="fileItem.isSelected = !fileItem.isSelected"
            >
              <input 
                type="checkbox" 
                :checked="fileItem.isSelected"
                class="file-checkbox-input"
                @change="fileItem.isSelected = $event.target.checked"
                @click.stop
              />
              <div 
                :class="[
                  'file-checkbox-custom',
                  fileItem.isSelected ? 'file-checkbox-checked' : ''
                ]"
              >
                <icon v-if="fileItem.isSelected" name="hi-check" class="file-checkbox-icon" />
              </div>
            </div>
          </div>
          <div class="file-card-body">
            <div class="file-name" :title="fileItem.name">{{ fileItem.name }}</div>
            <div v-if="fileItem.isFile" class="file-size">{{ formatFileSize(fileItem.size) }}</div>
            <div v-else class="file-type">{{ t('files.Folder') }}</div>
        </div>
          <context-menu :title="fileItem.name" :actions="contextActionsForFile(fileItem)">
          <template #title>
            <li class="flex items-center justify-between gap-4 px-4 py-2 border-b-2 border-border/50">
              <span class="font-medium text-foreground" v-text="file.name" />
              <span v-if="file.isFile" class="text-sm text-muted-foreground" v-text="formatFileSize(file.size)" />
            </li>
          </template>
          <template #activator="contextMenu">
              <div 
                v-if="contextMenu.canOpen" 
                class="file-card-menu"
                @click.stop="contextMenu.onClick"
              >
              <icon name="hi-ellipsis-vertical" />
              </div>
          </template>
        </context-menu>
        </div>
      </div>
    </div>
    <overlay v-model="fileSizeWarn" closable :title="t('files.OpenLargeFile')">
      <btn color="error" @click="fileSizeWarn = false"><icon name="close" />{{ t('common.Cancel') }}</btn>
      <btn color="primary" @click="openFile(fileSizeWarnSubject, true)"><icon name="hi-check" />{{ t('files.OpenAnyways') }}</btn>
    </overlay>
    <overlay v-model="createFileOpen" closable :title="t('files.CreateFile')">
      <text-field v-model="newItemName" />
      <btn color="primary" :disabled="!newItemName || newItemName.trim() === ''" @click="createFile()"><icon name="hi-check" />{{ t('files.CreateFile') }}</btn>
    </overlay>
    <overlay v-model="createFolderOpen" closable :title="t('files.CreateFolder')">
      <text-field v-model="newItemName" />
      <btn color="primary" :disabled="!newItemName || newItemName.trim() === ''" @click="createFolder()"><icon name="hi-check" />{{ t('files.CreateFolder') }}</btn>
    </overlay>
    <overlay v-model="archiveSelectedOpen" closable :title="t('files.ArchiveSelectedName')">
      <text-field v-model="newItemName" />
      <btn color="primary" :disabled="!newItemName || newItemName.trim() === ''" @click="archiveSelected()"><icon name="hi-check" />{{ t('files.ArchiveSelected') }}</btn>
    </overlay>
    <overlay v-model="loading" class="loader-overlay">
      <loader />
    </overlay>
    <overlay v-model="editorOpen" class="editor">
      <editor v-if="file" v-model="file" :read-only="!canEdit" @save="saveFile($event)" @close="editorOpen = false" />
    </overlay>
  </div>
</template>

<style scoped>
.files-grid-container {
  width: 100%;
}

.files-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 1rem;
  padding: 0.5rem 0;
}

.file-card {
  position: relative;
  display: flex;
  flex-direction: column;
  background: rgb(var(--color-background));
  border: 2px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
  padding: 1rem;
  cursor: pointer;
  transition: all 0.2s ease-in-out;
  min-height: 140px;
  overflow: hidden;
}

.file-card:hover {
  border-color: rgb(var(--color-primary) / 0.5);
  background: rgb(var(--color-muted) / 0.3);
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.file-card-selected {
  border-color: rgb(var(--color-primary));
  background: rgb(var(--color-primary) / 0.15);
  box-shadow: 0 0 0 2px rgb(var(--color-primary) / 0.2);
}

.file-card-header {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 0.75rem;
  min-height: 64px;
}

.file-icon {
  font-size: 3rem;
  transition: transform 0.2s ease-in-out;
}

.file-icon-folder {
  color: rgb(var(--color-primary));
}

.file-icon-file {
  color: rgb(var(--color-muted-foreground));
}

.file-card:hover .file-icon {
  transform: scale(1.1);
}

.file-card-checkbox {
  position: absolute;
  bottom: 0.5rem;
  right: 0.5rem;
  z-index: 10;
  width: 1.5rem;
  height: 1.5rem;
  cursor: pointer;
}

.file-checkbox-input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
  margin: 0;
  padding: 0;
}

.file-checkbox-custom {
  width: 1.5rem;
  height: 1.5rem;
  border: 2px solid rgb(var(--color-border));
  border-radius: 0.375rem;
  background: rgb(var(--color-background));
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease-in-out;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.file-checkbox-custom:hover {
  border-color: rgb(var(--color-primary));
  background: rgb(var(--color-muted) / 0.5);
}

.file-checkbox-checked {
  background: rgb(var(--color-primary));
  border-color: rgb(var(--color-primary));
}

.file-checkbox-icon {
  color: rgb(var(--color-primary-foreground));
  font-size: 0.875rem;
  font-weight: bold;
}

.file-card-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-height: 0;
}

.file-name {
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 1.4;
}

.file-size,
.file-type {
  font-size: 0.75rem;
  color: rgb(var(--color-muted-foreground));
  line-height: 1.3;
}

.file-card-menu {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  width: 1.75rem;
  height: 1.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgb(var(--color-muted) / 0.5);
  border-radius: 0.375rem;
  opacity: 0;
  transition: opacity 0.2s ease-in-out;
  cursor: pointer;
  z-index: 5;
}

.file-card:hover .file-card-menu {
  opacity: 1;
}

.file-card-menu:hover {
  background: rgb(var(--color-muted) / 0.8);
}

@media (max-width: 768px) {
  .files-grid {
    grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
    gap: 0.75rem;
  }
  
  .file-card {
    min-height: 120px;
    padding: 0.75rem;
  }
  
  .file-icon {
    font-size: 2.5rem;
  }
}
</style>
