<script setup>
import { ref, inject, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Ace from '@/components/ui/Ace.vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '../ui/Loader.vue'
import Overlay from '@/components/ui/Overlay.vue'
import Tab from '@/components/ui/Tab.vue'
import Tabs from '@/components/ui/Tabs.vue'

import Variables from '@/components/template/Variables.vue'
import Install from '@/components/template/Install.vue'
import Hooks from '@/components/template/Hooks.vue'
import RunConfig from '@/components/template/RunConfig.vue'
import ServerEnvironment from '@/components/template/ServerEnvironment.vue'

const { t } = useI18n()
const toast = inject('toast')
const events = inject('events')
const router = useRouter()

const props = defineProps({
  server: { type: Object, required: true }
})

const def = ref({})
const edit = ref("")
const editorOpen = ref(false)
const serverJson = ref(null)
const deleting = ref(false)

function editDefinition() {
  edit.value = JSON.stringify(def.value, undefined, 4)
  editorOpen.value = true
}

function cancelEdit() {
  editorOpen.value = false
}

function saveDefinition() {
  editorOpen.value = false
  const edited = JSON.parse(edit.value)
  props.server.updateDefinition(edited)
  toast.success(t('settings.Saved'))
  def.value = edited
}

function deleteServer() {
  events.emit(
    'confirm',
    t('servers.ConfirmDelete', { name: props.server.name }),
    {
      text: t('servers.Delete'),
      icon: 'remove',
      color: 'error',
      action: async () => {
        deleting.value = true
        await props.server.delete()
        toast.success(t('servers.Deleted'))
        // delay 500ms to prevent running into sqlite dbs still being locked
        setTimeout(() => {router.push({ name: 'ServerList' })}, 500)
      }
    },
    {
      color: 'primary'
    }
  )
}

function definitionTabChanged(newTab) {
  if (newTab === 'json' && serverJson.value) serverJson.value.refresh()
}

onMounted(async () => {
  if (props.server.hasScope('server.definition.view'))
    def.value = await props.server.getDefinition()
})
</script>

<template>
  <div class="admin-container">
    <!-- Header con título y descripción -->
    <div class="admin-header">
      <div class="admin-header-content">
        <div class="admin-header-icon">
          <icon name="admin" />
        </div>
        <div>
          <h2 class="admin-title" v-text="t('servers.Admin')" />
          <p class="admin-subtitle" v-text="t('servers.AdminDescription') || 'Administración avanzada del servidor'" />
        </div>
      </div>
    </div>
    
    <!-- Acciones de administración -->
    <div class="admin-section">
      <div class="admin-card">
        <div class="admin-card-header">
          <icon name="settings" class="admin-card-icon" />
          <h3 class="admin-card-title">{{ t('servers.AdminActions') || 'Acciones de administración' }}</h3>
        </div>
        <div class="admin-actions">
        <btn
          v-if="server.hasScope('server.definition.view')"
          v-hotkey="'a e'"
          variant="outline"
          @click="editDefinition()"
            class="admin-action-btn"
        >
          <icon name="edit" />
          {{ t('servers.EditDefinition') }}
        </btn>
        <btn
          v-if="server.hasScope('server.delete')"
          color="error"
          variant="outline"
          @click="deleteServer()"
            class="admin-action-btn"
        >
          <icon name="remove" />
          {{ t('servers.Delete') }}
        </btn>
        </div>
      </div>
    </div>

    <!-- Instalación -->
    <div class="admin-section">
      <div class="admin-card">
        <div class="admin-card-header">
          <icon name="install" class="admin-card-icon" />
          <h3 class="admin-card-title">{{ t('servers.Install') || 'Instalación' }}</h3>
        </div>
        <div class="admin-actions">
        <btn
          v-if="server.hasScope('server.install')"
          v-hotkey="'a i'"
          color="warning"
          variant="outline"
          @click="server.install()"
            class="admin-action-btn"
        >
          <icon name="install" />
          {{ t('servers.Install') }}
        </btn>
      </div>
        <div class="admin-hint">
          <icon name="warning" class="hint-icon" />
          <p>{{ t('servers.InstallHint') || 'Reinstala el servidor desde cero. Esto eliminará todos los archivos actuales.' }}</p>
        </div>
      </div>
    </div>

    <overlay v-model="editorOpen" class="server-definition">
      <tabs @tabChanged="definitionTabChanged">
        <tab id="variables" :title="t('templates.Variables')" icon="variables" hotkey="t v">
          <variables v-model="edit" />
        </tab>
        <tab id="install" :title="t('templates.Install')" icon="install" hotkey="t i">
          <install v-model="edit" />
        </tab>
        <tab id="run" :title="t('templates.RunConfig')" icon="start" hotkey="t r">
          <run-config v-model="edit" />
        </tab>
        <tab id="hooks" :title="t('templates.Hooks')" icon="hooks" hotkey="t h">
          <hooks v-model="edit" />
        </tab>
        <tab id="environment" :title="t('templates.Environment')" icon="environment" hotkey="t e">
          <server-environment v-model="edit" />
        </tab>
        <tab id="json" :title="t('templates.Json')" icon="json" hotkey="t j">
          <ace id="server-json" ref="serverJson" v-model="edit" class="server-json-editor" mode="json" />
        </tab>
      </tabs>
      <div class="actions">
        <btn v-hotkey="'Escape'" color="error" @click="cancelEdit()"><icon name="close" />{{ t('common.Cancel') }}</btn>
        <btn :disabled="!server.hasScope('server.definition.edit')" color="primary" @click="saveDefinition()"><icon name="save" />{{ t('common.Save') }}</btn>
      </div>
    </overlay>

    <overlay v-model="deleting" class="deleting">
      <loader :text="t('servers.Deleting')" />
    </overlay>
  </div>
</template>

<style scoped>
.admin-container {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  padding: 2rem;
  max-width: 1400px;
  margin: 0 auto;
}

/* Header */
.admin-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 1.5rem;
  border-bottom: 2px solid #475569;
}

.admin-header-content {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.admin-header-icon {
  width: 2.5rem;
  height: 2.5rem;
  padding: 0.5rem;
  background: #3b82f6;
  border-radius: 0.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.admin-header-icon :deep(svg),
.admin-header-icon :deep(svg path),
.admin-header-icon :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
  width: 1.5rem;
  height: 1.5rem;
}

.admin-title {
  font-size: 1.875rem;
  font-weight: 700;
  color: #f1f5f9;
  margin: 0;
  line-height: 1.2;
}

.admin-subtitle {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0.25rem 0 0;
}

/* Secciones */
.admin-section {
  margin-top: 0;
}

.admin-card {
  background: #1e293b;
  border: 2px solid #475569;
  border-radius: 1rem;
  padding: 2rem;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.admin-card:hover {
  border-color: #3b82f6;
}

.admin-card-header {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 2px solid #334155;
}

.admin-card-icon {
  width: 1.5rem;
  height: 1.5rem;
  color: #3b82f6;
}

.admin-card-icon :deep(svg),
.admin-card-icon :deep(svg path),
.admin-card-icon :deep(svg *) {
  color: #3b82f6 !important;
  fill: #3b82f6 !important;
  stroke: #3b82f6 !important;
  width: 1.5rem;
  height: 1.5rem;
}

.admin-card-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0;
}

.admin-actions {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
}

.admin-action-btn {
  min-width: 150px;
}

.admin-hint {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  margin-top: 1.5rem;
  padding: 1rem;
  background: #0f172a;
  border: 2px solid #f59e0b;
  border-radius: 0.5rem;
}

.hint-icon {
  width: 1.25rem;
  height: 1.25rem;
  flex-shrink: 0;
  margin-top: 0.125rem;
}

.hint-icon :deep(svg),
.hint-icon :deep(svg path),
.hint-icon :deep(svg *) {
  color: #f59e0b !important;
  fill: #f59e0b !important;
  stroke: #f59e0b !important;
  width: 1.25rem;
  height: 1.25rem;
}

.admin-hint p {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0;
  line-height: 1.5;
}

/* Responsive */
@media (max-width: 768px) {
  .admin-container {
    padding: 1rem;
  }
  
  .admin-title {
    font-size: 1.5rem;
  }
  
  .admin-actions {
    flex-direction: column;
  }
  
  .admin-action-btn {
    width: 100%;
  }
}
</style>
