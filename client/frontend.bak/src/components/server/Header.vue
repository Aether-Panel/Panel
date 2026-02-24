<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Overlay from '@/components/ui/Overlay.vue'
import TextField from '@/components/ui/TextField.vue'
import Status from './Status.vue'

const props = defineProps({
  server: { type: Object, required: true }
})

const { t, locale } = useI18n()
const edit = ref(false)
const name = ref(props.server.name)

const stats = ref({ cpu: 0, memory: 0, disk: 0 })
const memoryLimit = ref(null) // En bytes
const cpuLimit = ref(null) // En porcentaje
const diskLimit = ref(null) // En bytes
const diskUsage = ref(0) // En bytes

const numFormat = new Intl.NumberFormat(
  [locale.value.replace('_', '-'), 'en'],
  { maximumFractionDigits: 2 }
)

const formatMemory = (value) => {
  if (!value) return numFormat.format(0) + ' B'
  if (value < Math.pow(2, 10)) return numFormat.format(value) + ' B'
  if (value < Math.pow(2, 20)) return numFormat.format(value / Math.pow(2, 10)) + ' KiB'
  if (value < Math.pow(2, 30)) return numFormat.format(value / Math.pow(2, 20)) + ' MiB'
  if (value < Math.pow(2, 40)) return numFormat.format(value / Math.pow(2, 30)) + ' GiB'
  return numFormat.format(value / Math.pow(2, 40)) + ' TiB'
}

const formatCpu = (value) => {
  return numFormat.format(value) + '%'
}

const serverAddress = computed(() => {
  // Usar la IP pública del nodo
  let ip = props.server.node?.publicHost || '0.0.0.0'
  const port = props.server.port
  return port ? `${ip}:${port}` : ip
})

const memoryUsage = computed(() => {
  if (!stats.value.memory) return '0 B'
  return formatMemory(stats.value.memory)
})

const memoryLimitFormatted = computed(() => {
  if (!memoryLimit.value) return '∞'
  return formatMemory(memoryLimit.value)
})

const memoryDisplay = computed(() => {
  return `${memoryUsage.value} / ${memoryLimitFormatted.value}`
})

const cpuUsage = computed(() => {
  return formatCpu(stats.value.cpu || 0)
})

const cpuLimitFormatted = computed(() => {
  if (!cpuLimit.value) return '∞'
  return formatCpu(cpuLimit.value)
})

const cpuDisplay = computed(() => {
  return `${cpuUsage.value} / ${cpuLimitFormatted.value}`
})

const diskUsageFormatted = computed(() => {
  return formatMemory(diskUsage.value || 0)
})

const diskLimitFormatted = computed(() => {
  if (!diskLimit.value) return '∞'
  return formatMemory(diskLimit.value)
})

const diskDisplay = computed(() => {
  return `${diskUsageFormatted.value} / ${diskLimitFormatted.value}`
})

let task = null
let stopListener = null

onMounted(async () => {
  // Obtener límites del servidor desde los datos
  try {
    let serverData = null
    if (props.server.hasScope('server.data.view')) {
      serverData = await props.server.getData()
    } else if (props.server.hasScope('server.definition.view')) {
      const definition = await props.server.getDefinition()
      if (definition && definition.data) {
        serverData = definition
      }
    }

    if (serverData && serverData.data) {
      // Obtener límite de memoria desde la variable "memory" (en MB para Minecraft)
      if (serverData.data.memory && serverData.data.memory.value) {
        const memoryMB = parseFloat(serverData.data.memory.value)
        if (!isNaN(memoryMB) && memoryMB > 0) {
          memoryLimit.value = memoryMB * Math.pow(2, 20) // Convertir MB a bytes
        }
      }
    }
  } catch (e) {
    console.warn('No se pudieron obtener límites del servidor:', e)
  }

  // Obtener estadísticas iniciales
  if (props.server.hasScope('server.stats')) {
    try {
      const initialStats = await props.server.getStats()
      if (initialStats) {
        stats.value = { 
          cpu: initialStats.cpu || 0, 
          memory: initialStats.memory || 0,
          disk: initialStats.disk || 0
        }
      }
    } catch (e) {
      console.warn('No se pudieron obtener estadísticas iniciales:', e)
    }
  }

  // Escuchar eventos de estadísticas
  stopListener = props.server.on('stat', (data) => {
    if (data) {
      stats.value = { 
        cpu: data.cpu || 0, 
        memory: data.memory || 0,
        disk: data.disk || 0
      }
    }
  })

  // Polling para estadísticas si es necesario
  task = props.server.startTask(async () => {
    if (props.server.needsPolling() && props.server.hasScope('server.stats')) {
      try {
        const newStats = await props.server.getStats()
        if (newStats) {
          stats.value = { 
            cpu: newStats.cpu || 0, 
            memory: newStats.memory || 0,
            disk: newStats.disk || 0
          }
        }
      } catch (e) {
        // Silenciar errores de polling
      }
    }
  }, 5000)
})

onUnmounted(() => {
  if (task) props.server.stopTask(task)
  if (stopListener) stopListener()
})

async function updateName() {
  await props.server.updateName(name.value)
  edit.value = false
}
</script>

<template>
  <div class="server-header-wrapper">
    <div class="server-header">
      <div class="server-header-left">
        <Status :server="server" />
        <h1 class="server-header-title">
          {{ server.name }}
        </h1>
        <btn 
          v-if="server.hasScope('server.name.edit')" 
          variant="icon" 
          size="sm"
          :tooltip="t('servers.EditName')" 
          class="server-edit-btn"
          @click="edit = !edit"
        >
          <icon name="edit" />
        </btn>
      </div>
      <div class="server-header-info">
        <div v-if="server.hasScope('server.stats')" class="server-info-item">
          <icon name="hi-cog" class="server-info-icon" />
          <span class="server-info-label">CPU:</span>
          <span class="server-info-value">{{ cpuDisplay }}</span>
        </div>
        <div v-if="server.hasScope('server.stats')" class="server-info-item">
          <icon name="hi-server" class="server-info-icon" />
          <span class="server-info-label">RAM:</span>
          <span class="server-info-value">{{ memoryDisplay }}</span>
        </div>
        <div v-if="server.hasScope('server.stats')" class="server-info-item">
          <icon name="hi-folder" class="server-info-icon" />
          <span class="server-info-label">Disco:</span>
          <span class="server-info-value">{{ diskDisplay }}</span>
        </div>
        <div class="server-info-item">
          <icon name="hi-globe-alt" class="server-info-icon" />
          <span class="server-info-label">IP:</span>
          <span class="server-info-value">{{ serverAddress }}</span>
        </div>
      </div>
      <div class="server-header-right">
        <slot name="actions" />
      </div>
    </div>
  </div>
  <overlay v-model="edit" :title="t('servers.EditName')" closable class="server-name">
    <text-field v-model="name" />
    <btn color="primary" @click="updateName()"><icon name="save" />{{ t('common.Save') }}</btn>
  </overlay>
</template>

<style scoped>
.server-header-wrapper {
  margin-bottom: 1rem;
}

.server-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.75rem 1rem;
  background: rgb(var(--color-background));
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.5rem;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
  flex-wrap: wrap;
  flex: 1;
}

.server-header-left {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex: 1;
  min-width: 0;
}

.server-header-info {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
}

.server-info-item {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.375rem 0.75rem;
  background: rgb(var(--color-muted) / 0.3);
  border: 1px solid rgb(var(--color-border) / 0.2);
  border-radius: 0.375rem;
  font-size: 0.875rem;
}

.server-info-icon {
  width: 1rem;
  height: 1rem;
  color: rgb(var(--color-primary));
  flex-shrink: 0;
}

.server-info-label {
  color: rgb(var(--color-muted-foreground));
  font-weight: 500;
}

.server-info-value {
  color: rgb(var(--color-foreground));
  font-weight: 600;
}

.server-header-right {
  display: none;
}

.server-header-title {
  flex: 1;
  font-size: 1.25rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.server-edit-btn {
  color: rgb(var(--color-muted-foreground));
  padding: 0.25rem;
}

.server-edit-btn:hover {
  color: rgb(var(--color-primary));
  background: rgb(var(--color-primary) / 0.1);
}

@media (max-width: 768px) {
  .server-header {
    flex-direction: column;
    align-items: flex-start;
  }
  
  .server-header-info {
    width: 100%;
    justify-content: flex-start;
  }
  
  .server-header-right {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
