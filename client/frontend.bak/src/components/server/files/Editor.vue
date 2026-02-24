<script>
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Ace from '@/components/ui/Ace.vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import TextField from '@/components/ui/TextField.vue'

const extensions = {}
// image formats
extensions['.jpg']  = { type: 'image', disableSave: true }
extensions['.jpeg'] = { type: 'image', disableSave: true }
extensions['.png']  = { type: 'image', disableSave: true }
extensions['.gif']  = { type: 'image', disableSave: true }
// audio formats
extensions['.mp3']  = { type: 'audio', disableSave: true }
extensions['.wav']  = { type: 'audio', disableSave: true }
extensions['.ogg']  = { type: 'audio', disableSave: true }
extensions['.flac'] = { type: 'audio', disableSave: true }
extensions['.aac']  = { type: 'audio', disableSave: true }
extensions['.alac'] = { type: 'audio', disableSave: true }
// video formats
extensions['.mp4']  = { type: 'video', disableSave: true }
extensions['.webm'] = { type: 'video', disableSave: true }
extensions['.avi']  = { type: 'video', disableSave: true }
extensions['.mkv']  = { type: 'video', disableSave: true }
extensions['.m4a']  = { type: 'video', disableSave: true }

function getType(file) {
  return (extensions[file.extension] || {}).type
}

export function skipDownload(file) {
  return ['image', 'audio', 'video'].indexOf(getType(file)) !== -1
}

export default {
  components: {
    Ace,
    Btn,
    Icon,
    TextField
  },
  props: {
    readOnly: { type: Boolean, default: () => false },
    modelValue: { type: Object, required: true }
  },
  emits: ['update:modelValue', 'save', 'close'],
  setup(props, { emit }) {
    const { t } = useI18n()

    // Separar nombre y extensión
    const fileName = ref('')
    const fileExtension = ref('')

    // Función para separar nombre y extensión
    function parseFileName(fullName) {
      const lastDot = fullName.lastIndexOf('.')
      if (lastDot === -1 || lastDot === 0) {
        return { name: fullName, extension: '' }
      }
      return {
        name: fullName.substring(0, lastDot),
        extension: fullName.substring(lastDot)
      }
    }

    // Inicializar valores
    const parsed = parseFileName(props.modelValue.name)
    fileName.value = parsed.name
    fileExtension.value = parsed.extension

    // Observar cambios en el nombre del archivo
    watch(() => props.modelValue.name, (newName) => {
      const parsed = parseFileName(newName)
      fileName.value = parsed.name
      fileExtension.value = parsed.extension
    })

    // Computed para el nombre completo
    const fullFileName = computed(() => {
      return fileName.value + fileExtension.value
    })

    // Actualizar el nombre cuando cambian los campos
    watch([fileName, fileExtension], () => {
      const newName = fullFileName.value
      if (newName !== props.modelValue.name) {
        emit('update:modelValue', { ...props.modelValue, name: newName })
      }
    })

    function emitUpdate(event) {
      emit('update:modelValue', { ...props.modelValue, content: event })
    }

    return { 
      t, 
      emit, 
      emitUpdate, 
      extensions, 
      getType, 
      skipDownload,
      fileName,
      fileExtension,
      fullFileName
    }
  }
}
</script>

<template>
  <div class="editor-container">
    <div class="editor-header">
      <div class="editor-header-content">
        <div class="editor-title-section">
          <h1 class="editor-title" v-text="fullFileName || modelValue.name" />
          <div v-if="!readOnly && getType(modelValue) !== 'image' && getType(modelValue) !== 'video' && getType(modelValue) !== 'audio'" class="editor-name-fields">
            <text-field 
              v-model="fileName" 
              :label="t('files.FileName') || 'Nombre del archivo'"
              placeholder="nombre"
              class="editor-name-input"
            />
            <text-field 
              v-model="fileExtension" 
              :label="t('files.FileExtension') || 'Extensión'"
              placeholder=".ext"
              class="editor-extension-input"
            />
          </div>
        </div>
        <div class="editor-actions">
          <btn v-if="!readOnly && !((extensions[modelValue.extension] || {}).disableSave)" color="primary" size="lg" @click="emit('save', {close: true})">
            <icon name="save" /> 
            {{ t('common.Save') }}
          </btn>
          <btn v-hotkey="'Escape'" variant="icon" @click="emit('close')">
            <icon name="close" />
          </btn>
        </div>
      </div>
    </div>
    <div class="editor-content">
      <img v-if="getType(modelValue) === 'image'" class="file-viewer" :src="modelValue.url" :alt="modelValue.name" />
      <video v-else-if="getType(modelValue) === 'video'" class="file-viewer" controls>
        <source :src="modelValue.url" />
        <div class="warning unsupported" v-text="t('errors.VideoUnsupported')" />
      </video>
      <audio v-else-if="getType(modelValue) === 'audio'" class="file-viewer" controls>
        <source :src="modelValue.url" />
        <div class="warning unsupported" v-text="t('errors.AudioUnsupported')" />
      </audio>
      <ace 
        v-else 
        id="file-editor" 
        :read-only="readOnly" 
        :model-value="modelValue.content" 
        class="file-editor" 
        :file="fullFileName || modelValue.name" 
        theme="monokai" 
        @update:modelValue="emitUpdate" 
        @save="emit('save', {close: false})" 
      />
    </div>
  </div>
</template>

<style scoped>
.editor-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
  background: #1e293b;
  border-radius: 0.75rem;
  overflow: hidden;
  position: relative;
}

.editor-header {
  flex-shrink: 0;
  background: #0f172a;
  border-bottom: 2px solid #334155;
  padding: 1.5rem;
}

.editor-header-content {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1.5rem;
}

.editor-title-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.editor-title {
  font-size: 1.5rem;
  font-weight: 700;
  color: #f1f5f9;
  margin: 0;
  word-break: break-all;
}

.editor-name-fields {
  display: flex;
  gap: 1rem;
  align-items: flex-end;
}

.editor-name-input {
  flex: 2;
}

.editor-extension-input {
  flex: 1;
  min-width: 120px;
}

.editor-actions {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-shrink: 0;
}

.editor-content {
  flex: 1;
  display: flex;
  overflow: hidden;
  position: relative;
  min-height: 0;
}

.file-viewer {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  margin: auto;
}

.file-editor {
  width: 100%;
  height: 100%;
  font-size: 14px;
  flex: 1;
  min-height: 0;
  position: relative;
  overflow: hidden;
}

.file-editor :deep(.ace_editor) {
  width: 100% !important;
  height: 100% !important;
  font-size: 14px !important;
  position: absolute !important;
  top: 0 !important;
  left: 0 !important;
  right: 0 !important;
  bottom: 0 !important;
}

.file-editor :deep(.ace_scroller) {
  width: 100% !important;
  height: 100% !important;
  overflow-y: auto !important;
  overflow-x: auto !important;
}

.warning.unsupported {
  padding: 2rem;
  text-align: center;
  color: #fbbf24;
  font-weight: 600;
}

@media (max-width: 768px) {
  .editor-name-fields {
    flex-direction: column;
    align-items: stretch;
  }
  
  .editor-extension-input {
    min-width: auto;
  }
  
  .editor-header-content {
    flex-direction: column;
  }
  
  .editor-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
