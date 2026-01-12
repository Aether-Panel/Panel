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
  
  // Update server.properties for Minecraft Java servers if MOTD, IP, or Port changed
  if (isMinecraftJava.value && props.server.hasScope('server.files.edit')) {
    try {
      await updateServerProperties(data)
    } catch (error) {
      console.error('Error updating server.properties:', error)
      // No mostrar error al usuario, solo loguear
    }
  }
  
  toast.success(t('servers.SettingsSaved'))
  
  // Emit event to update plugins tab visibility
  if (events) {
    events.emit('server:plugins-enabled-changed', pluginsEnabled.value)
  }
}

async function updateServerProperties(data) {
  try {
    // Leer el archivo server.properties actual
    let propertiesContent = ''
    try {
      propertiesContent = await props.server.getFile('server.properties', true)
    } catch (error) {
      // Si el archivo no existe, crear uno nuevo
      propertiesContent = ''
    }
    
    // Actualizar las propiedades relevantes
    const lines = propertiesContent.split('\n')
    const updatedLines = []
    const propertiesToUpdate = {}
    
    // Solo actualizar propiedades que están en data y son relevantes
    if (data.motd !== undefined) propertiesToUpdate['motd'] = data.motd
    if (data.ip !== undefined) propertiesToUpdate['server-ip'] = data.ip
    if (data.port !== undefined) propertiesToUpdate['server-port'] = data.port
    
    // Si no hay propiedades para actualizar, salir
    if (Object.keys(propertiesToUpdate).length === 0) {
      return
    }
    
    // Mapa de propiedades que ya existen en el archivo
    const existingProperties = new Set()
    
    // Procesar líneas existentes
    for (const line of lines) {
      const trimmedLine = line.trim()
      if (!trimmedLine || trimmedLine.startsWith('#')) {
        updatedLines.push(line)
        continue
      }
      
      const equalIndex = trimmedLine.indexOf('=')
      if (equalIndex === -1) {
        updatedLines.push(line)
        continue
      }
      
      const key = trimmedLine.substring(0, equalIndex).trim()
      const lowerKey = key.toLowerCase()
      
      // Actualizar propiedades que existen
      if (lowerKey === 'motd' && propertiesToUpdate['motd'] !== undefined) {
        // Escapar el valor del MOTD correctamente
        const motdValue = String(propertiesToUpdate['motd']).replace(/\n/g, '\\n')
        updatedLines.push(`motd=${motdValue}`)
        existingProperties.add('motd')
      } else if (lowerKey === 'server-ip' && propertiesToUpdate['server-ip'] !== undefined) {
        updatedLines.push(`server-ip=${propertiesToUpdate['server-ip']}`)
        existingProperties.add('server-ip')
      } else if (lowerKey === 'server-port' && propertiesToUpdate['server-port'] !== undefined) {
        updatedLines.push(`server-port=${propertiesToUpdate['server-port']}`)
        existingProperties.add('server-port')
      } else {
        updatedLines.push(line)
      }
    }
    
    // Agregar propiedades que no existen
    if (propertiesToUpdate['motd'] !== undefined && !existingProperties.has('motd')) {
      const motdValue = String(propertiesToUpdate['motd']).replace(/\n/g, '\\n')
      updatedLines.push(`motd=${motdValue}`)
    }
    if (propertiesToUpdate['server-ip'] !== undefined && !existingProperties.has('server-ip')) {
      updatedLines.push(`server-ip=${propertiesToUpdate['server-ip']}`)
    }
    if (propertiesToUpdate['server-port'] !== undefined && !existingProperties.has('server-port')) {
      updatedLines.push(`server-port=${propertiesToUpdate['server-port']}`)
    }
    
    // Escribir el archivo actualizado
    const updatedContent = updatedLines.join('\n')
    await props.server.uploadFile('server.properties', updatedContent)
  } catch (error) {
    console.error('Error updating server.properties:', error)
    throw error
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
  color: rgb(var(--color-foreground));
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
  background: rgb(var(--color-muted) / 0.3);
  border: 2px solid rgb(var(--color-border) / 0.5);
  border-radius: 0.75rem;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.server-tab-card-content {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.server-setting-item {
  padding: 0.75rem 0;
  border-bottom: 1px solid rgb(var(--color-border) / 0.1);
  color: rgb(var(--color-foreground));
}

.server-setting-item:last-child {
  border-bottom: none;
}

/* Asegurar que todos los textos dentro de las tarjetas sean blancos */
.server-tab-card :deep(*) {
  color: rgb(var(--color-foreground));
}

.server-tab-card :deep(label) {
  color: rgb(var(--color-foreground));
}

.server-tab-card :deep(.text-muted-foreground) {
  color: rgb(var(--color-muted-foreground)) !important;
}

.server-tab-card :deep(input),
.server-tab-card :deep(select),
.server-tab-card :deep(textarea) {
  color: rgb(var(--color-foreground));
  background-color: rgb(var(--color-background));
}

.server-tab-card :deep(input::placeholder),
.server-tab-card :deep(select::placeholder),
.server-tab-card :deep(textarea::placeholder) {
  color: rgb(var(--color-muted-foreground));
}

/* Estilos para multiselect dentro de las tarjetas */
.server-tab-card :deep(.multiselect-single-label),
.server-tab-card :deep(.multiselect-placeholder) {
  color: rgb(var(--color-foreground)) !important;
}

.server-tab-card :deep(.multiselect-option) {
  color: rgb(var(--color-foreground)) !important;
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
