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
  <div class="server-tab-content">
    <div class="server-tab-section">
      <h2 class="server-tab-title" v-text="t('servers.Admin')" />
      <p class="server-tab-subtitle" v-text="t('servers.AdminDescription') || 'Administración avanzada del servidor'" />
    </div>
    
    <div class="server-tab-section">
      <div class="server-admin-actions">
        <btn
          v-if="server.hasScope('server.definition.view')"
          v-hotkey="'a e'"
          variant="outline"
          @click="editDefinition()"
        >
          <icon name="edit" />
          {{ t('servers.EditDefinition') }}
        </btn>
        <btn
          v-if="server.hasScope('server.delete')"
          color="error"
          variant="outline"
          @click="deleteServer()"
        >
          <icon name="remove" />
          {{ t('servers.Delete') }}
        </btn>
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
.server-tab-content {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  padding: 1.5rem;
  max-width: 100%;
}

.server-tab-title {
  font-size: 1.5rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  margin: 0 0 0.5rem 0;
  padding-bottom: 1rem;
  border-bottom: 2px solid rgb(var(--color-border) / 0.5);
}

.server-tab-subtitle {
  font-size: 0.875rem;
  color: rgb(var(--color-muted-foreground));
  margin: 0;
  padding-top: 0.5rem;
}

.server-tab-section {
  width: 100%;
}

.server-admin-actions {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
}
</style>
