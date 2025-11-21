<script setup>
import { ref, inject, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Toggle from '@/components/ui/Toggle.vue'
import Variables from '@/components/ui/Variables.vue'

const { t, te, locale } = useI18n()
const toast = inject('toast')
const events = inject('events', null)

const props = defineProps({
  server: { type: Object, required: true }
})

// Check if this is Minecraft Java Edition (not Bedrock, BungeeCord, etc.)
const isMinecraftJava = computed(() => {
  const type = props.server.type
  // Accept both "minecraft" and "minecraft-java", but exclude "minecraft-bedrock", "minecraft-bungeecord", etc.
  return type === 'minecraft' || type === 'minecraft-java'
})

const vars = ref({})
const flags = ref({})
const pluginsEnabled = ref(true)
const anyItems = computed(() => {
  if (Object.keys(vars.value).length > 0) return true
  if (Object.keys(flags.value).length > 0) return true
  return true // Always true now because we have pluginsEnabled
})

onMounted(async () => {
  if (props.server.hasScope('server.definition.view')) {
    vars.value = (await props.server.getDefinition())
  } else if (props.server.hasScope('server.data.view')) {
    vars.value = (await props.server.getData()) || {}
  }
  if (props.server.hasScope('server.flags.view'))
    flags.value = (await props.server.getFlags()) || {}
  
  // Load plugins enabled setting from localStorage (backend doesn't support custom variables)
  const storedValue = localStorage.getItem(`pluginsEnabled_${props.server.id}`)
  if (storedValue !== null) {
    pluginsEnabled.value = storedValue === 'true'
  } else {
    // Default to true if not set
    pluginsEnabled.value = true
  }
})

async function save() {
  const data = {}
  Object.keys(vars.value.data).map(name => {
    data[name] = vars.value.data[name].value
  })
  
  if (props.server.hasScope('server.data.edit.admin')) {
    await props.server.adminUpdateData(data)
  } else if (props.server.hasScope('server.data.edit')) {
    await props.server.updateData(data)
  }
  if (props.server.hasScope('server.flags.edit'))
    await props.server.setFlags(flags.value)
  
  // Save plugins enabled setting to localStorage (backend doesn't support custom variables)
  localStorage.setItem(`pluginsEnabled_${props.server.id}`, pluginsEnabled.value.toString())
  
  toast.success(t('servers.SettingsSaved'))
  
  // Emit event to update plugins tab visibility
  if (events) {
    events.emit('server:plugins-enabled-changed', pluginsEnabled.value)
  }
}

function getFlagHint(name) {
  if (te(`servers.flags.hint.${name}`, locale))
    return t(`servers.flags.hint.${name}`)
}
</script>

<template>
  <div class="server-tab-content">
    <div class="server-tab-section">
      <h2 class="server-tab-title" v-text="t('servers.Settings')" />
    </div>
    
    <div v-if="Object.keys(vars.data || {}).length > 0" class="server-tab-section">
      <h3 class="server-tab-section-title">{{ t('templates.Variables') }}</h3>
      <div class="server-tab-card">
        <variables v-model="vars" :disabled="!server.hasScope('server.data.edit')" />
      </div>
    </div>
    
    <div v-if="Object.keys(flags).length > 0" class="server-tab-section">
      <h3 class="server-tab-section-title" v-text="t('servers.FlagsHeader')" />
      <div class="server-tab-card">
        <div class="server-tab-card-content">
          <toggle
            v-for="(_, name) in flags"
            :key="name"
            v-model="flags[name]"
            :disabled="!server.hasScope('server.flags.edit')"
            :label="t(`servers.flags.${name}`)"
            :hint="getFlagHint(name)"
            class="server-setting-item"
          />
        </div>
      </div>
    </div>
    
    <div v-if="isMinecraftJava" class="server-tab-section">
      <h3 class="server-tab-section-title" v-text="t('plugins.PluginsSettings')" />
      <div class="server-tab-card">
        <div class="server-tab-card-content">
          <toggle 
            v-model="pluginsEnabled" 
            :disabled="!server.hasScope('server.data.edit')" 
            :label="t('plugins.EnablePluginsTab')" 
            :hint="t('plugins.EnablePluginsTabHint')"
            class="server-setting-item"
          />
        </div>
      </div>
    </div>
    
    <div v-if="!anyItems" class="server-tab-empty-state">
      <p class="server-tab-empty-text" v-text="t('servers.NoSettings')" />
    </div>
    
    <div v-if="anyItems" class="server-tab-actions">
      <btn color="primary" @click="save()">
        <icon name="save" />
        {{ t('servers.SaveSettings') }}
      </btn>
    </div>
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
  margin: 0;
  padding-bottom: 1rem;
  border-bottom: 2px solid rgb(var(--color-border) / 0.5);
}

.server-tab-section {
  width: 100%;
}

.server-tab-section-title {
  font-size: 1.125rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  margin: 0 0 1rem 0;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid rgb(var(--color-border) / 0.3);
}

.server-tab-card {
  background: rgb(var(--color-background));
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
  padding: 1.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.server-tab-card-content {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.server-setting-item {
  padding: 0.75rem 0;
  border-bottom: 1px solid rgb(var(--color-border) / 0.1);
}

.server-setting-item:last-child {
  border-bottom: none;
}

.server-tab-empty-state {
  padding: 3rem 1.5rem;
  text-align: center;
  background: rgb(var(--color-muted) / 0.2);
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
}

.server-tab-empty-text {
  color: rgb(var(--color-muted-foreground));
  margin: 0;
  font-size: 0.875rem;
}

.server-tab-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 1rem;
  border-top: 1px solid rgb(var(--color-border) / 0.3);
}
</style>
