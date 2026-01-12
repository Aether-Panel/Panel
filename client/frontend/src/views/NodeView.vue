<script setup>
import { ref, inject, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import markdown from '@/utils/markdown'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import Overlay from '@/components/ui/Overlay.vue'
import TextField from '@/components/ui/TextField.vue'
import Toggle from '@/components/ui/Toggle.vue'

const api = inject('api')
const toast = inject('toast')
const events = inject('events')
const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const deploymentOpen = ref(false)
let deploymentData = {}
const withPrivateHost = ref(false)
const name = ref('')
const publicHost = ref('')
const publicPort = ref('8080')
const privateHost = ref('')
const privatePort = ref('8080')
const sftpPort = ref('5657')
const currentStep = ref(1)
const featuresFetched = ref(null)
const features = ref({})
const systemInfo = ref(null)
const systemInfoFetched = ref(null)
const nodeServers = ref([])
const loadingServers = ref(true)
const nodeStats = ref({
  totalServers: 0,
  onlineServers: 0,
  offlineServers: 0,
  totalMemory: 0,
  usedMemory: 0,
  totalCpu: 0,
  usedCpu: 0
})
let statsInterval = null

onMounted(async () => {
  const node = await api.node.get(route.params.id)
  // Traducir el nombre del nodo local
  if (route.params.id === '0' || route.params.id === 0 || node.name === 'LocalNode') {
    name.value = t('nodes.LocalNode')
  } else {
    name.value = node.name
  }
  publicHost.value = node.publicHost
  publicPort.value = node.publicPort
  privateHost.value = node.privateHost
  privatePort.value = node.privatePort
  sftpPort.value = node.sftpPort
  withPrivateHost.value = !(node.publicHost === node.privateHost && node.publicPort === node.privatePort)
  deploymentData = await api.node.deployment(route.params.id)
  if (route.query.created) {
    deploymentOpen.value = true
  }

  fetchFeatures()
  fetchSystemInfo()
  await loadNodeServers()
  
  // Actualizar estadísticas cada 5 segundos
  statsInterval = setInterval(async () => {
    await updateNodeStats()
  }, 5000)
})

onUnmounted(() => {
  if (statsInterval) {
    clearInterval(statsInterval)
  }
})

async function fetchFeatures() {
  featuresFetched.value = null
  features.value = {}
  try {
    const f = await api.node.features(route.params.id)
    features.value.envs = [ ...new Set(f.environments.map(e => e === 'standard' || e === 'tty' ? 'host' : e)) ].map(e => t(`env.${e}.name`))
    features.value.docker = f.features.indexOf('docker') !== -1
    features.value.os = f.os
    features.value.arch = f.arch
    features.value.version = f.version || t('common.Unknown')
    featuresFetched.value = true
  } catch(e) {
    featuresFetched.value = false
  }
}

async function fetchSystemInfo() {
  systemInfoFetched.value = null
  systemInfo.value = null
  try {
    const info = await api.node.system(route.params.id)
    systemInfo.value = info
    systemInfoFetched.value = true
  } catch(e) {
    console.error('Error fetching system info:', e)
    systemInfoFetched.value = false
  }
}

function canSubmit() {
  if (!name.value) return false
  if (!publicHost.value) return false
  if (!publicPort.value) return false
  if (!sftpPort.value) return false
  if (withPrivateHost.value) {
    if (!privateHost.value) return false
    if (!privatePort.value) return false
  }
  return true
}

async function submit() {
  if (!canSubmit()) return
  const node = {
    name: name.value,
    publicHost: publicHost.value,
    publicPort: publicPort.value,
    sftpPort: sftpPort.value
  }
  if (withPrivateHost.value) {
    node.privateHost = privateHost.value
    node.privatePort = privatePort.value
  } else {
    node.privateHost = publicHost.value
    node.privatePort = publicPort.value
  }
  await api.node.update(route.params.id, node)
  toast.success(t('nodes.Updated'))
}

async function deleteNode() {
  events.emit(
    'confirm',
    t('nodes.ConfirmDelete', { name: name.value }),
    {
      text: t('nodes.Delete'),
      icon: 'remove',
      color: 'error',
      action: async () => {
        await api.node.delete(route.params.id)
        toast.success(t('nodes.Deleted'))
        router.push({ name: 'NodeList' })
      }
    },
    {
      color: 'primary'
    }
  )
}

function getDeployConfig() {
  const config = {
    logs: '/var/log/SkyPanel',
    web: {
      host: `0.0.0.0:${privatePort.value}`
    },
    token: {
      public: location.origin + '/auth/publickey'
    },
    panel: {
      enable: false
    },
    daemon: {
      auth: {
        url: location.origin + '/oauth2/token',
        ...deploymentData
      },
      data: {
        root: '/var/lib/SkyPanel'
      },
      sftp: {
        host: `0.0.0.0:${sftpPort.value}`
      }
    }
  }
  return JSON.stringify(config, undefined, 2)
}

function closeDeploy() {
  deploymentOpen.value = false
  currentStep.value = 1
  fetchFeatures()
}

async function loadNodeServers() {
  loadingServers.value = true
  try {
    const nodeId = route.params.id || '0'
    console.log('Loading servers for node:', nodeId)
    const response = await api.server.search({ node: nodeId, limit: 100 })
    console.log('Server search response:', response)
    nodeServers.value = response.servers || []
    
    // Cargar el estado de cada servidor
    for (const server of nodeServers.value) {
      try {
        const status = await api.server.getStatus(server.id)
        server.online = status === 'online'
      } catch (error) {
        console.error(`Error getting status for server ${server.id}:`, error)
        server.online = false
      }
    }
    
    console.log('Node servers loaded:', nodeServers.value.length)
    await updateNodeStats()
  } catch (error) {
    console.error('Error loading node servers:', error)
    // Si falla, intentar con list()
    try {
      const response = await api.server.list(1, 100)
      nodeServers.value = response.servers || []
      
      // Cargar el estado de cada servidor
      for (const server of nodeServers.value) {
        try {
          const status = await api.server.getStatus(server.id)
          server.online = status === 'online'
        } catch (error) {
          server.online = false
        }
      }
      
      console.log('Fallback: loaded servers via list():', nodeServers.value.length)
    } catch (fallbackError) {
      console.error('Fallback also failed:', fallbackError)
    }
  } finally {
    loadingServers.value = false
  }
}

async function updateNodeStats() {
  if (nodeServers.value.length === 0) return
  
  let onlineCount = 0
  let totalMem = 0
  let usedMem = 0
  let totalCpu = 0
  let usedCpu = 0
  
  for (const server of nodeServers.value) {
    try {
      const status = await api.server.getStatus(server.id)
      if (status === 'online') {
        onlineCount++
        const stats = await api.server.getStats(server.id)
        usedMem += stats.memory || 0
        usedCpu += stats.cpu || 0
      }
      
      // Sumar límites totales
      if (server.memory && server.memory > 0) {
        totalMem += server.memory
      }
      if (server.cpu && server.cpu > 0) {
        totalCpu += server.cpu
      }
    } catch (error) {
      // Ignorar errores de servidores individuales
    }
  }
  
  nodeStats.value = {
    totalServers: nodeServers.value.length,
    onlineServers: onlineCount,
    offlineServers: nodeServers.value.length - onlineCount,
    totalMemory: totalMem,
    usedMemory: usedMem,
    totalCpu: totalCpu,
    usedCpu: usedCpu
  }
}

function formatMemory(bytes) {
  if (!bytes || bytes === 0) return '0 MB'
  const mb = bytes / 1024 / 1024
  if (mb >= 1024) {
    return `${(mb / 1024).toFixed(2)} GB`
  }
  return `${mb.toFixed(0)} MB`
}

function formatCpu(cpu) {
  if (!cpu || cpu === 0) return '0%'
  return `${cpu.toFixed(1)}%`
}

function formatUptime(seconds) {
  if (!seconds) return t('common.Unknown')
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  
  if (days > 0) {
    return `${days}d ${hours}h ${minutes}m`
  } else if (hours > 0) {
    return `${hours}h ${minutes}m`
  } else {
    return `${minutes}m`
  }
}
</script>

<template>
  <div 
    :class="[
      'nodeview',
      'w-full max-w-5xl ml-auto mr-0',
      'space-y-6'
    ]"
    style="padding-left: 10rem;"
  >
    <h1 
      :class="[
        'text-3xl font-bold text-foreground mb-6',
        'pb-3 border-b-2 border-border/50'
      ]"
      v-text="name" 
    />
    
    <!-- Estadísticas del Nodo -->
    <div 
      v-if="!loadingServers && nodeServers.length > 0"
      :class="[
        'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6'
      ]"
    >
      <!-- Total de Servidores -->
      <div 
        :class="[
          'p-5 rounded-xl border-2 border-border/50',
          'bg-muted/30'
        ]"
      >
        <div :class="['flex items-center justify-between mb-2']">
          <span :class="['text-sm text-muted-foreground']" v-text="t('nodes.stats.TotalServers')" />
          <icon name="hi-server" :class="['text-primary']" />
        </div>
        <div :class="['text-3xl font-bold text-foreground']" v-text="nodeStats.totalServers" />
      </div>
      
      <!-- Servidores Online -->
      <div 
        :class="[
          'p-5 rounded-xl border-2 border-border/50',
          'bg-muted/30'
        ]"
      >
        <div :class="['flex items-center justify-between mb-2']">
          <span :class="['text-sm text-muted-foreground']" v-text="t('nodes.stats.OnlineServers')" />
          <icon name="hi-check" :class="['text-success']" />
        </div>
        <div :class="['text-3xl font-bold text-success']" v-text="nodeStats.onlineServers" />
        <div :class="['text-xs text-muted-foreground mt-1']">
          {{ nodeStats.offlineServers }} {{ t('nodes.stats.Offline') }}
        </div>
      </div>
      
      <!-- Uso de RAM -->
      <div 
        :class="[
          'p-5 rounded-xl border-2 border-border/50',
          'bg-muted/30'
        ]"
      >
        <div :class="['flex items-center justify-between mb-2']">
          <span :class="['text-sm text-muted-foreground']" v-text="t('nodes.stats.Memory')" />
          <icon name="hi-chip" :class="['text-primary']" />
        </div>
        <div :class="['text-2xl font-bold text-foreground']">
          {{ formatMemory(nodeStats.usedMemory) }}
        </div>
        <div :class="['text-xs text-muted-foreground mt-1']">
          {{ t('nodes.stats.Of') }} {{ nodeStats.totalMemory > 0 ? formatMemory(nodeStats.totalMemory) : '∞' }}
        </div>
        <div 
          v-if="nodeStats.totalMemory > 0"
          :class="['w-full bg-border/30 rounded-full h-2 mt-2']"
        >
          <div 
            :class="['bg-primary h-2 rounded-full transition-all duration-300']"
            :style="{ width: `${Math.min(100, (nodeStats.usedMemory / nodeStats.totalMemory) * 100)}%` }"
          />
        </div>
      </div>
      
      <!-- Uso de CPU -->
      <div 
        :class="[
          'p-5 rounded-xl border-2 border-border/50',
          'bg-muted/30'
        ]"
      >
        <div :class="['flex items-center justify-between mb-2']">
          <span :class="['text-sm text-muted-foreground']" v-text="t('nodes.stats.CPU')" />
          <icon name="hi-chart-bar" :class="['text-primary']" />
        </div>
        <div :class="['text-2xl font-bold text-foreground']">
          {{ formatCpu(nodeStats.usedCpu) }}
        </div>
        <div :class="['text-xs text-muted-foreground mt-1']">
          {{ t('nodes.stats.Of') }} {{ nodeStats.totalCpu > 0 ? formatCpu(nodeStats.totalCpu) : '∞' }}
        </div>
        <div 
          v-if="nodeStats.totalCpu > 0"
          :class="['w-full bg-border/30 rounded-full h-2 mt-2']"
        >
          <div 
            :class="['bg-primary h-2 rounded-full transition-all duration-300']"
            :style="{ width: `${Math.min(100, (nodeStats.usedCpu / nodeStats.totalCpu) * 100)}%` }"
          />
        </div>
      </div>
    </div>
    
    <loader v-if="featuresFetched === null" />
    <div 
      v-else-if="featuresFetched === false" 
      :class="[
        'features',
        'p-5 rounded-xl border-2 border-error/50',
        'bg-error/10 text-error'
      ]"
    >
      <div 
        :class="[
          'unreachable',
          'font-semibold'
        ]"
        v-text="t('nodes.Unreachable')" 
      />
    </div>
    <div 
      v-else 
      :class="[
        'features',
        'p-5 rounded-xl border-2 border-border/50',
        'bg-muted/30 space-y-3'
      ]"
    >
      <div 
        :class="[
          'reachable',
          'font-semibold text-success'
        ]"
        v-text="t('nodes.Reachable')" 
      />
      <div :class="['grid grid-cols-1 md:grid-cols-2 gap-4']">
        <div :class="['flex items-center justify-between']">
          <span :class="['text-foreground font-medium']" v-text="t('nodes.features.os.label')" />
          <span :class="['text-muted-foreground']" v-text="t('nodes.features.os.' + features.os)" />
        </div>
        <div :class="['flex items-center justify-between']">
          <span :class="['text-foreground font-medium']" v-text="t('nodes.features.arch.label')" />
          <span :class="['text-muted-foreground']" v-text="t('nodes.features.arch.' + features.arch)" />
        </div>
        <div :class="['flex items-center justify-between']">
          <span :class="['text-foreground font-medium']" v-text="t('nodes.features.envs')" />
          <span :class="['text-muted-foreground']" v-text="features.envs.join(', ')" />
        </div>
        <div :class="['flex items-center justify-between']">
          <span :class="['text-foreground font-medium']" v-text="t('env.docker.name')" />
          <span :class="['text-muted-foreground']" v-text="t('nodes.features.docker.' + features.docker)" />
        </div>
        <div :class="['flex items-center justify-between']">
          <span :class="['text-foreground font-medium']" v-text="t('nodes.features.version')" />
          <span :class="['text-muted-foreground']" v-text="features.version || t('common.Unknown')" />
        </div>
        <div :class="['flex items-center justify-between']">
          <span :class="['text-foreground font-medium']" v-text="t('nodes.PublicAddress')" />
          <span :class="['text-muted-foreground']" v-text="`${publicHost}:${publicPort}`" />
        </div>
      </div>
    </div>
    
    <!-- Información del Sistema -->
    <div 
      v-if="systemInfoFetched && systemInfo"
      :class="['mt-6']"
    >
      <h2 
        :class="[
          'text-2xl font-bold text-foreground mb-4',
          'pb-2 border-b-2 border-border/50'
        ]"
        v-text="t('nodes.system.Title')" 
      />
      
      <div :class="['grid grid-cols-1 md:grid-cols-2 gap-4']">
        <!-- Información General -->
        <div 
          :class="[
            'p-5 rounded-xl border-2 border-border/50',
            'bg-muted/30 space-y-3'
          ]"
        >
          <h3 :class="['text-lg font-semibold text-foreground mb-3']" v-text="t('nodes.system.General')" />
          <div :class="['flex items-center justify-between']">
            <span :class="['text-foreground font-medium']" v-text="t('nodes.system.Hostname')" />
            <span :class="['text-muted-foreground']" v-text="systemInfo.hostname" />
          </div>
          <div :class="['flex items-center justify-between']">
            <span :class="['text-foreground font-medium']" v-text="t('nodes.system.Platform')" />
            <span :class="['text-muted-foreground']" v-text="`${systemInfo.platform} ${systemInfo.platformVersion}`" />
          </div>
          <div :class="['flex items-center justify-between']">
            <span :class="['text-foreground font-medium']" v-text="t('nodes.system.Uptime')" />
            <span :class="['text-muted-foreground']" v-text="formatUptime(systemInfo.uptime)" />
          </div>
        </div>
        
        <!-- CPU -->
        <div 
          :class="[
            'p-5 rounded-xl border-2 border-border/50',
            'bg-muted/30 space-y-3'
          ]"
        >
          <h3 :class="['text-lg font-semibold text-foreground mb-3']" v-text="t('nodes.system.CPU')" />
          <div :class="['flex items-center justify-between']">
            <span :class="['text-foreground font-medium']" v-text="t('nodes.system.Model')" />
            <span :class="['text-muted-foreground text-sm']" v-text="systemInfo.cpuModel" />
          </div>
          <div :class="['flex items-center justify-between']">
            <span :class="['text-foreground font-medium']" v-text="t('nodes.system.Cores')" />
            <span :class="['text-muted-foreground']" v-text="systemInfo.cpuCores" />
          </div>
          <div :class="['flex items-center justify-between']">
            <span :class="['text-foreground font-medium']" v-text="t('nodes.system.Threads')" />
            <span :class="['text-muted-foreground']" v-text="systemInfo.cpuThreads" />
          </div>
        </div>
        
        <!-- Memoria -->
        <div 
          :class="[
            'p-5 rounded-xl border-2 border-border/50',
            'bg-muted/30 space-y-3'
          ]"
        >
          <h3 :class="['text-lg font-semibold text-foreground mb-3']" v-text="t('nodes.system.Memory')" />
          <div :class="['flex items-center justify-between']">
            <span :class="['text-foreground font-medium']" v-text="t('nodes.system.Total')" />
            <span :class="['text-muted-foreground']" v-text="formatMemory(systemInfo.memoryTotal)" />
          </div>
          <div :class="['flex items-center justify-between']">
            <span :class="['text-foreground font-medium']" v-text="t('nodes.system.Used')" />
            <span :class="['text-muted-foreground']" v-text="formatMemory(systemInfo.memoryUsed)" />
          </div>
          <div :class="['flex items-center justify-between']">
            <span :class="['text-foreground font-medium']" v-text="t('nodes.system.Free')" />
            <span :class="['text-muted-foreground']" v-text="formatMemory(systemInfo.memoryFree)" />
          </div>
          <div 
            :class="['w-full bg-border/30 rounded-full h-2 mt-2']"
          >
            <div 
              :class="['bg-primary h-2 rounded-full transition-all duration-300']"
              :style="{ width: `${(systemInfo.memoryUsed / systemInfo.memoryTotal) * 100}%` }"
            />
          </div>
        </div>
        
        <!-- Discos -->
        <div 
          :class="[
            'p-5 rounded-xl border-2 border-border/50',
            'bg-muted/30 space-y-3'
          ]"
        >
          <h3 :class="['text-lg font-semibold text-foreground mb-3']" v-text="t('nodes.system.Disks')" />
          <div 
            v-for="(disk, index) in systemInfo.disks"
            :key="index"
            :class="['space-y-2']"
          >
            <div :class="['flex items-center justify-between']">
              <span :class="['text-foreground font-medium']" v-text="disk.path" />
              <span :class="['text-muted-foreground text-sm']" v-text="`${disk.usedPercent.toFixed(1)}%`" />
            </div>
            <div :class="['flex items-center justify-between text-sm']">
              <span :class="['text-muted-foreground']" v-text="`${formatMemory(disk.used)} / ${formatMemory(disk.total)}`" />
            </div>
            <div 
              :class="['w-full bg-border/30 rounded-full h-2']"
            >
              <div 
                :class="[
                  'h-2 rounded-full transition-all duration-300',
                  disk.usedPercent > 90 ? 'bg-error' : disk.usedPercent > 70 ? 'bg-warning' : 'bg-primary'
                ]"
                :style="{ width: `${disk.usedPercent}%` }"
              />
            </div>
          </div>
        </div>
      </div>
    </div>
    
    <!-- Lista de Servidores en el Nodo -->
    <div 
      v-if="!loadingServers"
      :class="['mt-6']"
    >
      <h2 
        :class="[
          'text-2xl font-bold text-foreground mb-4',
          'pb-2 border-b-2 border-border/50'
        ]"
        v-text="t('nodes.stats.ServersOnNode')" 
      />
      <div 
        v-if="nodeServers.length === 0"
        :class="[
          'p-5 rounded-xl border-2 border-border/50',
          'bg-muted/30 text-center text-muted-foreground'
        ]"
      >
        {{ t('nodes.stats.NoServers') }}
      </div>
      <div 
        v-else
        :class="[
          'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4'
        ]"
      >
        <router-link
          v-for="server in nodeServers"
          :key="server.id"
          :to="{ name: 'ServerView', params: { id: server.id } }"
          :class="[
            'p-4 rounded-xl border-2 border-border/50',
            'bg-muted/30 hover:bg-muted/50',
            'transition-all duration-200',
            'cursor-pointer'
          ]"
        >
          <div :class="['flex items-center justify-between mb-2']">
            <span :class="['font-semibold text-foreground']" v-text="server.name" />
            <span 
              :class="[
                'w-2 h-2 rounded-full',
                server.online ? 'bg-success' : 'bg-error'
              ]"
            />
          </div>
          <div :class="['text-sm text-muted-foreground']">
            {{ server.type || 'Generic' }}
          </div>
        </router-link>
      </div>
    </div>
    <h2 
      :class="[
        'text-2xl font-bold text-foreground mb-4',
        'pb-2 border-b-2 border-border/50'
      ]"
      v-text="t('nodes.Edit')" 
    />
    <div 
      v-if="route.params.id > 0" 
      :class="[
        'edit',
        'space-y-5'
      ]"
    >
      <text-field v-model="name" class="name" :label="t('common.Name')" />
      <text-field v-model="publicHost" class="public-host" :label="t('nodes.PublicHost')" />
      <text-field v-model="publicPort" class="public-port" :label="t('nodes.PublicPort')" type="number" />
      <toggle v-model="withPrivateHost" class="private-toggle" :label="t('nodes.WithPrivateAddress')" :hint="t('nodes.WithPrivateAddressHint')" />
      <text-field v-if="withPrivateHost" v-model="privateHost" class="private-host" :label="t('nodes.PrivateHost')" />
      <text-field v-if="withPrivateHost" v-model="privatePort" class="private-port" :label="t('nodes.PrivatePort')" type="number" />
      <text-field v-model="sftpPort" class="sftp-port" :label="t('nodes.SftpPort')" type="number" />
      <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
        <btn :disabled="!canSubmit()" color="primary" @click="submit()"><icon name="save" />{{ t('nodes.Update') }}</btn>
        <btn color="error" @click="deleteNode()"><icon name="remove" />{{ t('nodes.Delete') }}</btn>
        <btn @click="deploymentOpen = true" v-text="t('nodes.Deploy')" />
      </div>
    </div>
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div 
      v-else 
      :class="[
        'edit',
        'p-5 rounded-xl border-2 border-border/50',
        'bg-muted/30 prose prose-sm max-w-none'
      ]"
      v-html="markdown(t('nodes.LocalNodeEdit'))" 
    />
    <overlay v-model="deploymentOpen" closable :title="t('nodes.Deploy')" @close="closeDeploy()">
      <div :class="['space-y-5']">
        <!-- eslint-disable-next-line vue/no-v-html -->
        <div 
          :class="[
            'prose prose-sm max-w-none',
            'p-5 rounded-xl border-2 border-border/50',
            'bg-muted/30'
          ]"
          v-html="markdown(t(`nodes.deploy.Step${currentStep}`, { config: getDeployConfig() }))" 
        />
        <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
          <btn v-if="currentStep < 5" @click="currentStep += 1" v-text="t('common.Next')" />
          <btn v-else @click="closeDeploy()" v-text="t('common.Close')" />
        </div>
      </div>
    </overlay>
  </div>
</template>
