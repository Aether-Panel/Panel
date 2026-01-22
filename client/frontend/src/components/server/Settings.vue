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
  <div class="settings-container">
    <!-- Header con título -->
    <div class="settings-header">
      <div class="settings-header-content">
        <div class="settings-header-icon">
          <icon name="settings" />
        </div>
        <div>
          <h2 class="settings-title" v-text="t('servers.Settings')" />
          <p class="settings-subtitle">Configuración y variables del servidor</p>
        </div>
      </div>
    </div>
    
    <!-- Variables -->
    <div v-if="Object.keys(vars.data || {}).length > 0" class="settings-section">
      <div class="settings-card">
        <div class="settings-card-header">
          <icon name="variables" class="settings-card-icon" />
          <h3 class="settings-card-title">{{ t('templates.Variables') }}</h3>
        </div>
        <div class="settings-card-body">
        <variables v-model="vars" :disabled="!server.hasScope('server.data.edit')" />
        </div>
      </div>
    </div>
    
    <!-- Flags -->
    <div v-if="Object.keys(flags).length > 0" class="settings-section">
      <div class="settings-card">
        <div class="settings-card-header">
          <icon name="flags" class="settings-card-icon" />
          <h3 class="settings-card-title" v-text="t('servers.FlagsHeader')" />
        </div>
        <div class="settings-card-body">
          <div class="settings-list">
          <toggle
            v-for="(_, name) in flags"
            :key="name"
            v-model="flags[name]"
            :disabled="!server.hasScope('server.flags.edit')"
            :label="t(`servers.flags.${name}`)"
            :hint="getFlagHint(name)"
              class="setting-item"
          />
          </div>
        </div>
      </div>
    </div>
    
    <!-- Plugins Settings -->
    <div v-if="isMinecraftJava" class="settings-section">
      <div class="settings-card">
        <div class="settings-card-header">
          <icon name="plugins" class="settings-card-icon" />
          <h3 class="settings-card-title" v-text="t('plugins.PluginsSettings')" />
        </div>
        <div class="settings-card-body">
          <div class="settings-list">
          <toggle 
            v-model="pluginsEnabled" 
            :disabled="!server.hasScope('server.data.edit')" 
            :label="t('plugins.EnablePluginsTab')" 
            :hint="t('plugins.EnablePluginsTabHint')"
              class="setting-item"
          />
          </div>
        </div>
      </div>
    </div>
    
    <!-- Estado vacío -->
    <div v-if="!anyItems" class="empty-state">
      <div class="empty-state-icon">
        <icon name="settings" />
      </div>
      <h3 class="empty-state-title" v-text="t('servers.NoSettings')" />
      <p class="empty-state-text">No hay configuraciones disponibles para este servidor</p>
    </div>
    
    <!-- Botón de guardar -->
    <div v-if="anyItems" class="settings-actions">
      <btn color="primary" size="lg" @click="save()" class="save-button">
        <icon name="save" />
        {{ t('servers.SaveSettings') }}
      </btn>
    </div>
  </div>
</template>

<style scoped>
.settings-container {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  padding: 2rem;
  max-width: 1400px;
  margin: 0 auto;
}

/* Header */
.settings-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 1.5rem;
  border-bottom: 2px solid #475569;
}

.settings-header-content {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.settings-header-icon {
  width: 2.5rem;
  height: 2.5rem;
  padding: 0.5rem;
  background: #3b82f6;
  border-radius: 0.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.settings-header-icon :deep(svg),
.settings-header-icon :deep(svg path),
.settings-header-icon :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
  width: 1.5rem;
  height: 1.5rem;
}

.settings-title {
  font-size: 1.875rem;
  font-weight: 700;
  color: #f1f5f9;
  margin: 0;
  line-height: 1.2;
}

.settings-subtitle {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0.25rem 0 0;
}

/* Secciones */
.settings-section {
  margin-top: 0;
}

.settings-card {
  background: #1e293b;
  border: 2px solid #475569;
  border-radius: 1rem;
  padding: 2rem;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.settings-card:hover {
  border-color: #3b82f6;
}

.settings-card-header {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 2px solid #334155;
}

.settings-card-icon {
  width: 1.5rem;
  height: 1.5rem;
  color: #3b82f6;
}

.settings-card-icon :deep(svg),
.settings-card-icon :deep(svg path),
.settings-card-icon :deep(svg *) {
  color: #3b82f6 !important;
  fill: #3b82f6 !important;
  stroke: #3b82f6 !important;
  width: 1.5rem;
  height: 1.5rem;
}

.settings-card-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0;
}

.settings-card-body {
  padding-top: 0.5rem;
}

.settings-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.setting-item {
  padding: 1rem;
  background: #0f172a;
  border: 2px solid #334155;
  border-radius: 0.5rem;
  transition: all 0.2s;
}

.setting-item:hover {
  border-color: #475569;
  background: #1e293b;
}

/* Asegurar que todos los textos dentro de las tarjetas sean visibles */
.settings-card :deep(*) {
  color: #f1f5f9;
}

.settings-card :deep(label) {
  color: #f1f5f9 !important;
}

.settings-card :deep(.text-muted-foreground) {
  color: #cbd5e1 !important;
}

.settings-card :deep(input),
.settings-card :deep(select),
.settings-card :deep(textarea) {
  color: #f1f5f9;
  background-color: #0f172a;
  border-color: #475569;
}

.settings-card :deep(input::placeholder),
.settings-card :deep(select::placeholder),
.settings-card :deep(textarea::placeholder) {
  color: #94a3b8;
}

/* Estilos para multiselect dentro de las tarjetas */
.settings-card :deep(.multiselect-single-label),
.settings-card :deep(.multiselect-placeholder) {
  color: #f1f5f9 !important;
}

.settings-card :deep(.multiselect-option) {
  color: #f1f5f9 !important;
  background: #1e293b !important;
}

.settings-card :deep(.multiselect-option:hover) {
  background: #2d3e52 !important;
}

/* Estado vacío */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 4rem 2rem;
  text-align: center;
  background: #1e293b;
  border: 2px dashed #475569;
  border-radius: 1rem;
}

.empty-state-icon {
  width: 4rem;
  height: 4rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #334155;
  border-radius: 1rem;
  margin-bottom: 1.5rem;
}

.empty-state-icon :deep(svg),
.empty-state-icon :deep(svg path),
.empty-state-icon :deep(svg *) {
  width: 2rem;
  height: 2rem;
  color: #94a3b8 !important;
  fill: #94a3b8 !important;
  stroke: #94a3b8 !important;
}

.empty-state-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0 0 0.5rem;
}

.empty-state-text {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0;
}

/* Acciones */
.settings-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 1rem;
  border-top: 2px solid #475569;
}

.save-button {
  min-width: 200px;
}

/* Responsive */
@media (max-width: 768px) {
  .settings-container {
    padding: 1rem;
  }
  
  .settings-title {
    font-size: 1.5rem;
  }
  
  .settings-actions {
    justify-content: stretch;
  }
  
  .save-button {
    width: 100%;
  }
}
</style>
