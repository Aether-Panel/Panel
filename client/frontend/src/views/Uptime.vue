<script setup>
import { ref, inject, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import Dropdown from '@/components/ui/Dropdown.vue'

const { t, locale } = useI18n()
const api = inject('api')
const route = useRoute()
const router = useRouter()

const loading = ref(false)
const uptimeData = ref(null)
const serverId = computed(() => route.params.serverId)
const days = ref(30)
const daysOptions = computed(() => [
  { value: 7, label: t('uptime.Days7') },
  { value: 30, label: t('uptime.Days30') },
  { value: 90, label: t('uptime.Days90') },
  { value: 365, label: t('uptime.Days365') }
])

// Formatear duración en segundos a formato legible
function formatDuration(seconds) {
  if (!seconds || seconds === 0) return '0s'
  
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = seconds % 60
  
  const parts = []
  if (days > 0) parts.push(`${days}d`)
  if (hours > 0) parts.push(`${hours}h`)
  if (minutes > 0) parts.push(`${minutes}m`)
  if (secs > 0 || parts.length === 0) parts.push(`${secs}s`)
  
  return parts.join(' ')
}

// Formatear fecha
function formatDate(dateStr) {
  if (!dateStr) return '-'
  try {
    const date = new Date(dateStr)
    if (isNaN(date.getTime())) return '-'
    const localeStr = locale.value ? locale.value.replace('_', '-') : 'en-US'
    return date.toLocaleString(localeStr, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    })
  } catch {
    return '-'
  }
}

async function loadUptime() {
  loading.value = true
  try {
    if (serverId.value) {
      uptimeData.value = await api.uptime.getServer(serverId.value, days.value, 100)
    } else {
      const data = await api.uptime.getAll(days.value)
      // Siempre mostrar datos, incluso si es un objeto vacío (se mostrará como objeto vacío)
      uptimeData.value = data || {}
    }
  } catch (err) {
    console.error('Error loading uptime:', err)
    uptimeData.value = null
  } finally {
    loading.value = false
  }
}

// Construir segmentos de timeline para la barra de actividad
function buildTimelineSegments() {
  if (!uptimeData.value || !uptimeData.value.history || !uptimeData.value.period) {
    return []
  }

  const history = [...uptimeData.value.history].reverse() // Ordenar de más antiguo a más reciente
  const periodStart = new Date(uptimeData.value.period.since).getTime()
  const periodEnd = new Date(uptimeData.value.period.until).getTime()
  const periodDuration = periodEnd - periodStart
  
  if (periodDuration <= 0) return []

  const segments = []
  const now = Date.now()

  // Procesar cada evento del historial
  for (let i = 0; i < history.length; i++) {
    const event = history[i]
    const startTime = new Date(event.startTime).getTime()
    const endTime = event.endTime ? new Date(event.endTime).getTime() : now

    // Calcular posiciones en porcentaje
    const percentStart = ((startTime - periodStart) / periodDuration) * 100
    const percentEnd = ((endTime - periodStart) / periodDuration) * 100
    const percentWidth = Math.max(0.1, percentEnd - percentStart) // Mínimo 0.1% para visibilidad

    // Formatear tooltip
    const startStr = formatDate(event.startTime)
    const endStr = event.endTime ? formatDate(event.endTime) : t('uptime.Now')
    const durationStr = formatDuration(event.duration || Math.floor((endTime - startTime) / 1000))
    const statusStr = event.isRunning ? t('uptime.Online') : t('uptime.Offline')
    
    const tooltip = `${statusStr}\n${t('uptime.From')}: ${startStr}\n${t('uptime.To')}: ${endStr}\n${t('uptime.Duration')}: ${durationStr}`

    segments.push({
      status: event.isRunning ? 'online' : 'offline',
      percentStart: Math.max(0, percentStart),
      percentWidth: Math.min(100 - Math.max(0, percentStart), percentWidth),
      tooltip: tooltip,
      startTime: startTime,
      endTime: endTime
    })
  }

  return segments
}

onMounted(() => {
  loadUptime()
})
</script>

<template>
  <div 
    :class="[
      'uptime',
      'p-8 max-w-7xl mx-auto space-y-8'
    ]"
  >
    <div 
      :class="[
        'header',
        'flex items-center justify-between mb-8'
      ]"
    >
      <h1 
        :class="[
          'text-3xl font-bold text-foreground m-0',
          'pb-3 border-b-2 border-border/50'
        ]"
        v-text="t('uptime.Uptime')" 
      />
      <div 
        :class="[
          'controls',
          'flex gap-4'
        ]"
      >
        <dropdown v-model="days" :options="daysOptions" @update:modelValue="loadUptime()" />
      </div>
    </div>

    <loader v-if="loading" />
    
    <div 
      v-else-if="uptimeData" 
      :class="[
        'content',
        'space-y-8'
      ]"
    >
      <!-- Vista para servidor específico -->
      <div 
        v-if="serverId && uptimeData.serverId" 
        :class="[
          'server-uptime',
          'space-y-6'
        ]"
      >
        <div 
          v-if="uptimeData.nodeName" 
          :class="[
            'node-info',
            'flex items-center gap-2 p-4 rounded-xl',
            'bg-muted/30 border-2 border-border/50',
            'mb-6'
          ]"
        >
          <span 
            :class="[
              'label',
              'font-medium text-muted-foreground'
            ]"
          >
            {{ t('uptime.Node') }}:
          </span>
          <span 
            :class="[
              'value',
              'font-semibold text-foreground'
            ]"
          >
            {{ uptimeData.nodeName }}
          </span>
        </div>
        
        <div 
          :class="[
            'stats-grid',
            'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6',
            'mb-8'
          ]"
        >
          <div 
            :class="[
              'stat-card',
              'p-6 rounded-xl border-2 border-border/50',
              'bg-muted/30 shadow-sm'
            ]"
          >
            <div 
              :class="[
                'stat-label',
                'text-sm text-muted-foreground mb-2'
              ]"
            >
              {{ t('uptime.CurrentStatus') }}
            </div>
            <div 
              :class="[
                'stat-value',
                'text-3xl font-bold mb-2',
                'flex items-center gap-2',
                uptimeData.currentStatus 
                  ? 'text-success' 
                  : 'text-error'
              ]"
            >
              <icon :name="uptimeData.currentStatus ? 'check' : 'close'" />
              {{ uptimeData.currentStatus ? t('uptime.Online') : t('uptime.Offline') }}
            </div>
            <div 
              v-if="uptimeData.currentStartTime" 
              :class="[
                'stat-detail',
                'text-sm text-muted-foreground'
              ]"
            >
              {{ t('uptime.Since') }}: {{ formatDate(uptimeData.currentStartTime) }}
            </div>
            <div 
              v-if="uptimeData.currentUptime && uptimeData.currentUptime > 0" 
              :class="[
                'stat-detail',
                'text-sm text-muted-foreground'
              ]"
            >
              {{ t('uptime.CurrentUptime') }}: {{ formatDuration(uptimeData.currentUptime) }}
            </div>
          </div>

          <div 
            :class="[
              'stat-card',
              'p-6 rounded-xl border-2 border-border/50',
              'bg-muted/30 shadow-sm'
            ]"
          >
            <div 
              :class="[
                'stat-label',
                'text-sm text-muted-foreground mb-2'
              ]"
            >
              {{ t('uptime.UptimePercentage') }}
            </div>
            <div 
              :class="[
                'stat-value',
                'percentage',
                'text-3xl font-bold mb-2',
                (uptimeData.uptimePercent || 0) >= 99.9 
                  ? 'text-success' 
                  : (uptimeData.uptimePercent || 0) >= 99 
                    ? 'text-primary' 
                    : (uptimeData.uptimePercent || 0) >= 95 
                      ? 'text-warning' 
                      : 'text-error'
              ]"
            >
              {{ (uptimeData.uptimePercent || 0).toFixed(3) }}%
            </div>
            <!-- Barra de progreso horizontal -->
            <div 
              :class="[
                'uptime-progress-bar',
                'w-full h-5 rounded-full overflow-hidden',
                'bg-muted border-2 border-border/50',
                'flex mt-4 mb-2'
              ]"
            >
              <div 
                :class="[
                  'progress-segment',
                  'uptime-progress',
                  'h-full transition-all duration-300',
                  'bg-gradient-to-r from-success to-success/80'
                ]"
                :style="{ width: (uptimeData.uptimePercent || 0) + '%' }"
              ></div>
              <div 
                :class="[
                  'progress-segment',
                  'downtime-progress',
                  'h-full transition-all duration-300',
                  'bg-gradient-to-r from-error to-error/80'
                ]"
                :style="{ width: (100 - (uptimeData.uptimePercent || 0)) + '%' }"
              ></div>
            </div>
            <div 
              v-if="uptimeData.period" 
              :class="[
                'stat-detail',
                'text-sm text-muted-foreground'
              ]"
            >
              {{ t('uptime.Period') }}: {{ uptimeData.period.days }} {{ t('uptime.Days') }}
            </div>
          </div>

          <div 
            :class="[
              'stat-card',
              'p-6 rounded-xl border-2 border-border/50',
              'bg-muted/30 shadow-sm'
            ]"
          >
            <div 
              :class="[
                'stat-label',
                'text-sm text-muted-foreground mb-2'
              ]"
            >
              {{ t('uptime.TotalUptime') }}
            </div>
            <div 
              :class="[
                'stat-value',
                'text-3xl font-bold mb-2',
                'text-foreground'
              ]"
            >
              {{ formatDuration(uptimeData.uptimeSeconds) }}
            </div>
            <div 
              :class="[
                'stat-detail',
                'text-sm text-muted-foreground'
              ]"
            >
              {{ t('uptime.OutOf') }} {{ formatDuration(uptimeData.totalSeconds) }}
            </div>
          </div>

          <div 
            :class="[
              'stat-card',
              'p-6 rounded-xl border-2 border-border/50',
              'bg-muted/30 shadow-sm'
            ]"
          >
            <div 
              :class="[
                'stat-label',
                'text-sm text-muted-foreground mb-2'
              ]"
            >
              {{ t('uptime.TotalDowntime') }}
            </div>
            <div 
              :class="[
                'stat-value',
                'downtime',
                'text-3xl font-bold mb-2',
                'text-error'
              ]"
            >
              {{ formatDuration(uptimeData.downtimeSeconds) }}
            </div>
            <div 
              :class="[
                'stat-detail',
                'text-sm text-muted-foreground'
              ]"
            >
              {{ t('uptime.OutOf') }} {{ formatDuration(uptimeData.totalSeconds) }}
            </div>
          </div>
        </div>

        <!-- Barra de actividad timeline -->
        <div 
          v-if="uptimeData.history && uptimeData.period" 
          :class="[
            'activity-timeline',
            'mt-8 mb-8'
          ]"
        >
          <h2 
            :class="[
              'text-2xl font-bold text-foreground mb-4',
              'pb-2 border-b-2 border-border/50'
            ]"
          >
            {{ t('uptime.ActivityTimeline') }}
          </h2>
          <div 
            :class="[
              'timeline-bar',
              'relative w-full h-10 rounded-xl overflow-hidden',
              'bg-muted/30 border-2 border-border/50',
              'mb-4'
            ]"
            ref="timelineBar"
          >
            <div
              v-for="(segment, idx) in buildTimelineSegments()"
              :key="idx"
              :class="[
                'timeline-segment',
                'absolute top-0 h-full cursor-pointer',
                'transition-opacity duration-200 hover:opacity-80 hover:scale-y-110',
                'rounded',
                segment.status === 'online'
                  ? 'bg-gradient-to-r from-success to-success/80 border border-success/30'
                  : 'bg-gradient-to-r from-error to-error/80 border border-error/30'
              ]"
              :style="{
                left: segment.percentStart + '%',
                width: segment.percentWidth + '%',
                '--tooltip': `'${segment.tooltip}'`
              }"
              :title="segment.tooltip"
            />
          </div>
          <div 
            :class="[
              'timeline-legend',
              'flex gap-6 justify-center text-sm'
            ]"
          >
            <div 
              :class="[
                'legend-item',
                'flex items-center gap-2'
              ]"
            >
              <div 
                :class="[
                  'legend-color',
                  'online',
                  'w-5 h-5 rounded',
                  'bg-gradient-to-r from-success to-success/80'
                ]"
              ></div>
              <span :class="['text-foreground']">{{ t('uptime.Online') }}</span>
            </div>
            <div 
              :class="[
                'legend-item',
                'flex items-center gap-2'
              ]"
            >
              <div 
                :class="[
                  'legend-color',
                  'offline',
                  'w-5 h-5 rounded',
                  'bg-gradient-to-r from-error to-error/80'
                ]"
              ></div>
              <span :class="['text-foreground']">{{ t('uptime.Offline') }}</span>
            </div>
          </div>
        </div>

        <div 
          v-if="uptimeData.history && uptimeData.history.length > 0" 
          :class="[
            'history-section',
            'mt-8'
          ]"
        >
          <h2 
            :class="[
              'text-2xl font-bold text-foreground mb-4',
              'pb-2 border-b-2 border-border/50'
            ]"
          >
            {{ t('uptime.History') }}
          </h2>
          <div 
            :class="[
              'history-table',
              'rounded-xl overflow-hidden',
              'bg-muted/30 border-2 border-border/50'
            ]"
          >
            <div 
              :class="[
                'history-header',
                'grid grid-cols-[1fr_2fr_2fr_1fr] gap-4',
                'p-4 bg-muted/50 font-bold',
                'border-b-2 border-border/50',
                'text-foreground'
              ]"
            >
              <div>{{ t('uptime.Status') }}</div>
              <div>{{ t('uptime.StartTime') }}</div>
              <div>{{ t('uptime.EndTime') }}</div>
              <div>{{ t('uptime.Duration') }}</div>
            </div>
            <div 
              v-for="event in uptimeData.history" 
              :key="event.id" 
              :class="[
                'history-row',
                'grid grid-cols-[1fr_2fr_2fr_1fr] gap-4',
                'p-4 border-b border-border/30 last:border-b-0',
                'transition-colors duration-200',
                event.isRunning 
                  ? 'bg-success/10 hover:bg-success/15' 
                  : 'bg-error/10 hover:bg-error/15'
              ]"
            >
              <div 
                :class="[
                  'status',
                  'flex items-center gap-2 font-medium',
                  event.isRunning ? 'text-success' : 'text-error'
                ]"
              >
                <icon :name="event.isRunning ? 'check' : 'close'" />
                {{ event.isRunning ? t('uptime.Online') : t('uptime.Offline') }}
              </div>
              <div :class="['text-foreground']">{{ formatDate(event.startTime) }}</div>
              <div :class="['text-foreground']">{{ formatDate(event.endTime) }}</div>
              <div :class="['text-foreground']">{{ formatDuration(event.duration) }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Vista para todos los servidores (admin) -->
      <div 
        v-else-if="uptimeData && typeof uptimeData === 'object' && Object.keys(uptimeData).length > 0" 
        :class="[
          'all-servers-uptime',
          'space-y-6'
        ]"
      >
        <div 
          :class="[
            'servers-grid',
            'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6'
          ]"
        >
          <div 
            v-for="(stats, id) in uptimeData" 
            :key="id" 
            :class="[
              'server-card',
              'p-6 rounded-xl border-2 border-border/50',
              'bg-muted/30 shadow-sm',
              'cursor-pointer transition-all duration-200',
              'hover:shadow-lg hover:-translate-y-1',
              'hover:border-primary/30'
            ]"
            @click="router.push({ name: 'UptimeServer', params: { serverId: id } })"
          >
            <div 
              :class="[
                'server-header',
                'flex justify-between items-center mb-4'
              ]"
            >
              <h3 
                :class="[
                  'text-xl font-bold text-foreground m-0'
                ]"
              >
                {{ stats.serverName || id }}
              </h3>
              <div 
                :class="[
                  'status-badge',
                  'w-8 h-8 rounded-full flex items-center justify-center',
                  (stats.currentStatus === true) 
                    ? 'bg-success/20 text-success' 
                    : 'bg-error/20 text-error'
                ]"
              >
                <icon :name="(stats.currentStatus === true) ? 'check' : 'close'" />
              </div>
            </div>
            <div 
              v-if="stats.nodeName" 
              :class="[
                'server-node',
                'mb-4 pb-4 border-b border-border/30',
                'text-sm'
              ]"
            >
              <span 
                :class="[
                  'node-label',
                  'text-muted-foreground font-medium'
                ]"
              >
                {{ t('uptime.Node') }}:
              </span>
              <span 
                :class="[
                  'node-value',
                  'text-foreground font-semibold ml-2'
                ]"
              >
                {{ stats.nodeName }}
              </span>
            </div>
            <div 
              :class="[
                'server-stats',
                'flex flex-col gap-4'
              ]"
            >
              <!-- Barra de progreso horizontal -->
              <div 
                :class="[
                  'uptime-bar-container',
                  'w-full'
                ]"
              >
                <div 
                  :class="[
                    'uptime-bar-label',
                    'text-sm mb-2',
                    'flex justify-between items-center',
                    'text-muted-foreground'
                  ]"
                >
                  <span>
                    {{ t('uptime.UptimePercentage') }}: 
                    <strong 
                      :class="['text-foreground font-semibold']"
                    >
                      {{ (stats.uptimePercent || 0).toFixed(3) }}%
                    </strong>
                  </span>
                </div>
                <div 
                  :class="[
                    'uptime-bar',
                    'w-full h-8 rounded-lg overflow-hidden',
                    'bg-muted border-2 border-border/50',
                    'flex mb-2'
                  ]"
                >
                  <div 
                    :class="[
                      'uptime-bar-segment',
                      'uptime-segment',
                      'h-full transition-all duration-300',
                      'bg-gradient-to-r from-success to-success/80',
                      'cursor-pointer hover:opacity-90'
                    ]"
                    :style="{ width: (stats.uptimePercent || 0) + '%' }"
                    :title="t('uptime.Uptime') + ': ' + formatDuration(stats.uptime || 0)"
                  ></div>
                  <div 
                    :class="[
                      'uptime-bar-segment',
                      'downtime-segment',
                      'h-full transition-all duration-300',
                      'bg-gradient-to-r from-error to-error/80',
                      'cursor-pointer hover:opacity-90'
                    ]"
                    :style="{ width: (100 - (stats.uptimePercent || 0)) + '%' }"
                    :title="t('uptime.Downtime') + ': ' + formatDuration(stats.downtime || 0)"
                  ></div>
                </div>
                <div 
                  :class="[
                    'uptime-bar-legend',
                    'flex justify-around gap-4',
                    'text-xs text-muted-foreground'
                  ]"
                >
                  <div 
                    :class="[
                      'legend-item',
                      'flex items-center gap-2'
                    ]"
                  >
                    <span 
                      :class="[
                        'legend-color',
                        'online',
                        'w-3 h-3 rounded',
                        'bg-gradient-to-r from-success to-success/80'
                      ]"
                    ></span>
                    <span>{{ formatDuration(stats.uptime || 0) }}</span>
                  </div>
                  <div 
                    :class="[
                      'legend-item',
                      'flex items-center gap-2'
                    ]"
                  >
                    <span 
                      :class="[
                        'legend-color',
                        'offline',
                        'w-3 h-3 rounded',
                        'bg-gradient-to-r from-error to-error/80'
                      ]"
                    ></span>
                    <span>{{ formatDuration(stats.downtime || 0) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div 
      v-else-if="!loading && (!uptimeData || (typeof uptimeData === 'object' && Object.keys(uptimeData).length === 0))" 
      :class="[
        'empty',
        'text-center py-12',
        'text-muted-foreground'
      ]"
    >
      <p :class="['text-lg']">{{ t('uptime.NoData') }}</p>
    </div>
  </div>
</template>


