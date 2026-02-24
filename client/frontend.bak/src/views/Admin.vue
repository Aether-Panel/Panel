<script setup>
import { ref, inject, onMounted, onUnmounted, nextTick, toRaw } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { Chart, registerables } from 'chart.js'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

Chart.register(...registerables)

const { t } = useI18n()
const api = inject('api')

const loading = ref(true)
const stats = ref({
  totalServers: 0,
  totalUsers: 0,
  totalNodes: 0,
  totalTemplates: 0
})

const resourcesChart = ref(null)
const resourcesChartEl = ref(null)
const networkChart = ref(null)
const networkChartEl = ref(null)

const resourceStats = ref({
  totalMemory: 0,
  usedMemory: 0,
  totalCpu: 0,
  usedCpu: 0,
  totalDisk: 0,
  usedDisk: 0
})

const networkStats = ref({
  bytesSent: 0,
  bytesRecv: 0,
  history: [] // Array para almacenar historial: { time: timestamp, sent: bytes, recv: bytes }
})

const systemInfo = ref(null)

let statsInterval = null
let isUpdatingNetworkChart = false // Flag para evitar actualizaciones simultáneas
let isUpdatingResourcesChart = false // Flag para evitar actualizaciones simultáneas de recursos
let updateQueue = Promise.resolve() // Cola de promesas para actualizaciones secuenciales

onMounted(async () => {
  try {
    // Cargar solo estadísticas generales (muy rápido)
    // NO cargar detalles de templates para evitar 40+ peticiones
    const [servers, users, nodes, repos] = await Promise.all([
      api.server.list().catch(() => ({ servers: [] })),
      api.user.list().catch(() => []),
      api.node.list().catch(() => []),
      api.template.listRepos().catch(() => []) // Solo repos, no todos los templates
    ])
    
    const serverList = servers.servers || servers || []
    
    stats.value = {
      totalServers: serverList.length || 0,
      totalUsers: users.length || 0,
      totalNodes: nodes.length || 0,
      totalTemplates: repos.length || 0 // Número de repositorios como aproximación
    }
    
    // Cargar información del sistema (rápido)
    await loadSystemInfo()
    
    // Mostrar la interfaz inmediatamente
    loading.value = false
    await nextTick()
    createCharts()
    // Esperar un poco más para asegurar que el DOM esté completamente renderizado
    await nextTick()
    setTimeout(() => {
      createNetworkChart()
    }, 100)
    
    // Actualizar información del sistema cada 5 segundos para actualización constante
    statsInterval = setInterval(async () => {
      try {
        console.log('🔄 [ADMIN] Actualizando estadísticas del sistema...')
        
        // Cargar datos del sistema
        await loadSystemInfo()
        
        // Usar una cola de promesas para garantizar actualizaciones secuenciales
        updateQueue = updateQueue.then(() => {
          return new Promise((resolve) => {
            // Actualizar gráfica de recursos primero
            console.log('📊 [ADMIN] Iniciando actualización de gráfica de recursos...')
        updateCharts()
            
            // Esperar a que termine la actualización de recursos antes de continuar
            setTimeout(() => {
              console.log('📊 [ADMIN] Iniciando actualización de gráfica de red...')
              if (networkStats.value.history.length > 0) {
                updateNetworkChart()
              }
              // Dar tiempo para que se complete la actualización de red
              setTimeout(resolve, 100)
            }, 100)
          })
        }).catch(error => {
          console.error('❌ [ADMIN] Error en cola de actualización:', error)
        })
      } catch (error) {
        console.error('Error updating system info:', error)
      }
    }, 5000) // Actualizar cada 5 segundos
    
  } catch (error) {
    console.error('Error loading admin stats:', error)
    loading.value = false
  }
})

onUnmounted(() => {
  if (statsInterval) {
    clearInterval(statsInterval)
  }
  if (resourcesChart.value) {
    resourcesChart.value.destroy()
  }
  if (networkChart.value) {
    networkChart.value.destroy()
  }
})

async function loadSystemInfo() {
  try {
    // Obtener información del sistema del nodo local (ID 0)
    const info = await api.node.system('0')
    systemInfo.value = info
    
    // Calcular uso total de disco (sumar todos los discos)
    let totalDisk = 0
    let usedDisk = 0
    if (info.disks && Array.isArray(info.disks)) {
      info.disks.forEach(disk => {
        totalDisk += disk.total || 0
        usedDisk += disk.used || 0
      })
    }
    
    // Actualizar estadísticas de recursos con datos reales del sistema
    resourceStats.value = {
      totalMemory: info.memoryTotal,
      usedMemory: info.memoryUsed,
      freeMemory: info.memoryFree,
      // Para CPU, usar un porcentaje basado en el uso actual
      totalCpu: 100, // 100% del sistema
      usedCpu: info.cpuUsage || 0,
      totalDisk: totalDisk,
      usedDisk: usedDisk
    }
    
    // Actualizar estadísticas de red si están disponibles
    if (info.networkBytesSent !== undefined && info.networkBytesRecv !== undefined) {
      const now = Date.now()
      
      console.log('📡 [NETWORK] Datos recibidos:', {
        sent: info.networkBytesSent,
        recv: info.networkBytesRecv,
        prevSent: networkStats.value.bytesSent,
        prevRecv: networkStats.value.bytesRecv
      })
      
      // Calcular la diferencia desde la última medición (tráfico en este intervalo)
      let sentDiff = 0
      let recvDiff = 0
      
      if (networkStats.value.bytesSent > 0 && networkStats.value.bytesRecv > 0) {
        // Calcular diferencia (puede ser negativo si hay reinicio del contador)
        sentDiff = info.networkBytesSent >= networkStats.value.bytesSent 
          ? info.networkBytesSent - networkStats.value.bytesSent 
          : 0
        recvDiff = info.networkBytesRecv >= networkStats.value.bytesRecv 
          ? info.networkBytesRecv - networkStats.value.bytesRecv 
          : 0
      } else {
        // Primera vez, no hay diferencia, pero inicializamos con 0
        sentDiff = 0
        recvDiff = 0
      }
      
      // Convertir a MB para el intervalo (aproximadamente 5 segundos)
      const sentMB = (sentDiff / 1024 / 1024).toFixed(2)
      const recvMB = (recvDiff / 1024 / 1024).toFixed(2)
      
      console.log('📡 [NETWORK] Diferencia calculada:', {
        sentMB: parseFloat(sentMB),
        recvMB: parseFloat(recvMB)
      })
      
      // Agregar al historial (mantener solo los últimos 60 puntos = 5 minutos de datos)
      networkStats.value.history.push({
        time: now,
        sent: parseFloat(sentMB),
        recv: parseFloat(recvMB)
      })
      
      if (networkStats.value.history.length > 60) {
        networkStats.value.history.shift()
      }
      
      console.log('📡 [NETWORK] Historial actualizado:', networkStats.value.history.length, 'puntos')
      
      // Guardar valores actuales para la próxima comparación
      networkStats.value.bytesSent = info.networkBytesSent
      networkStats.value.bytesRecv = info.networkBytesRecv
      
      // La actualización de la gráfica se hace desde el intervalo principal con un delay
    } else {
      console.warn('⚠️ [NETWORK] No se recibieron datos de red de la API')
    }
  } catch (error) {
    console.error('Error loading system info:', error)
  }
}

function createCharts() {
  console.log('Creating charts...', {
    resourcesChartEl: !!resourcesChartEl.value,
    systemInfo: systemInfo.value
  })
  
  try {
    // Destruir gráficos existentes si los hay
    if (resourcesChart.value) {
      resourcesChart.value.destroy()
      resourcesChart.value = null
    }
    
    // Gráfico de recursos del sistema (barras) - Usar porcentajes
    if (resourcesChartEl.value) {
    // Calcular porcentajes para la visualización inicial
    const memoryPercent = Number(parseFloat(((resourceStats.value.usedMemory / resourceStats.value.totalMemory) * 100).toFixed(2)))
    const cpuPercent = Number(parseFloat((systemInfo.value?.cpuUsage?.toFixed(2) || 0)))
    const diskPercent = Number(parseFloat(((resourceStats.value.usedDisk / resourceStats.value.totalDisk) * 100).toFixed(2)))
    
    const availableMemoryPercent = Number(parseFloat((100 - memoryPercent).toFixed(2)))
    const availableCpuPercent = Number(parseFloat((100 - cpuPercent).toFixed(2)))
    const availableDiskPercent = Number(parseFloat((100 - diskPercent).toFixed(2)))
    
    resourcesChart.value = new Chart(resourcesChartEl.value, {
      type: 'bar',
      data: {
        labels: [t('admin.charts.Memory'), t('admin.charts.CPU'), t('admin.charts.Disk')],
        datasets: [{
          label: t('admin.charts.Used'),
          data: [
            memoryPercent,
            cpuPercent,
            diskPercent
          ],
          backgroundColor: 'rgba(59, 130, 246, 0.8)',
          borderColor: 'rgba(59, 130, 246, 1)',
          borderWidth: 2
        }, {
          label: t('admin.charts.Available'),
          data: [
            availableMemoryPercent,
            availableCpuPercent,
            availableDiskPercent
          ],
          backgroundColor: 'rgba(34, 197, 94, 0.4)',
          borderColor: 'rgba(34, 197, 94, 1)',
          borderWidth: 2
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: {
            position: 'bottom',
            labels: {
              color: 'rgb(226, 232, 240)',
              font: { size: 12 }
            }
          },
          title: {
            display: true,
            text: t('admin.charts.SystemResources'),
            color: 'rgb(226, 232, 240)',
            font: { size: 16, weight: 'bold' }
          },
          tooltip: {
            callbacks: {
              label: function(context) {
                const label = context.dataset.label || ''
                const value = context.parsed.y
                // Todos los valores son porcentajes
                  return `${label}: ${value}%`
              }
            }
          }
        },
        scales: {
          y: {
            stacked: true,
            beginAtZero: true,
            min: 0,
            max: 100,
            suggestedMin: 0,
            suggestedMax: 100,
            ticks: {
              color: 'rgb(226, 232, 240)',
              stepSize: 20,
              callback: function(value) {
                return value + '%'
              }
            },
            grid: {
              color: 'rgba(148, 163, 184, 0.1)'
            }
          },
          x: {
            stacked: true,
            ticks: {
              color: 'rgb(226, 232, 240)'
            },
            grid: {
              color: 'rgba(148, 163, 184, 0.1)'
            }
          }
        }
      }
    })
    
    console.log('Charts created successfully')
  }
  } catch (error) {
    console.error('Error creating charts:', error)
  }
}

function updateCharts() {
  // Evitar actualizaciones simultáneas
  if (isUpdatingResourcesChart) {
    console.log('⏳ [RESOURCES] Actualización en progreso, saltando...')
    return
  }
  
  try {
    isUpdatingResourcesChart = true
    
    if (!resourcesChartEl.value) {
      console.warn('⚠️ [RESOURCES] resourcesChartEl no existe')
      isUpdatingResourcesChart = false
      return
    }
    
    // Verificar que tenemos datos válidos
    if (!systemInfo.value) {
      console.warn('⚠️ [RESOURCES] systemInfo no disponible')
      isUpdatingResourcesChart = false
      return
    }
    
    // Destruir gráfica existente
    if (resourcesChart.value) {
      try {
        resourcesChart.value.destroy()
    } catch (e) {
        console.warn('⚠️ [RESOURCES] Error al destruir gráfica:', e)
      }
      resourcesChart.value = null
    }
    
    // Calcular porcentajes para todos los recursos (0-100)
    const memoryPercent = Number(parseFloat(((resourceStats.value.usedMemory / resourceStats.value.totalMemory) * 100).toFixed(2)))
    const cpuPercent = Number(parseFloat((systemInfo.value?.cpuUsage?.toFixed(2) || 0)))
    const diskPercent = Number(parseFloat(((resourceStats.value.usedDisk / resourceStats.value.totalDisk) * 100).toFixed(2)))
    
    const availableMemoryPercent = Number(parseFloat((100 - memoryPercent).toFixed(2)))
    const availableCpuPercent = Number(parseFloat((100 - cpuPercent).toFixed(2)))
    const availableDiskPercent = Number(parseFloat((100 - diskPercent).toFixed(2)))
    
    console.log('🔄 [RESOURCES] Recreando gráfica con datos:', {
      usadoMemory: memoryPercent,
      disponibleMemory: availableMemoryPercent,
      sumaMemory: memoryPercent + availableMemoryPercent,
      usadoCPU: cpuPercent,
      disponibleCPU: availableCpuPercent,
      sumaCPU: cpuPercent + availableCpuPercent,
      usadoDisk: diskPercent,
      disponibleDisk: availableDiskPercent,
      sumaDisk: diskPercent + availableDiskPercent
    })
    
    // Obtener traducciones como strings primitivos
    const labelMemory = String(t('admin.charts.Memory'))
    const labelCPU = String(t('admin.charts.CPU'))
    const labelDisk = String(t('admin.charts.Disk'))
    const labelUsed = String(t('admin.charts.Used'))
    const labelAvailable = String(t('admin.charts.Available'))
    const titleText = String(t('admin.charts.SystemResources'))
    
    // Recrear la gráfica completamente con datos en porcentajes
    resourcesChart.value = new Chart(resourcesChartEl.value, {
      type: 'bar',
      data: {
        labels: [labelMemory, labelCPU, labelDisk],
        datasets: [{
          label: labelUsed,
          data: [memoryPercent, cpuPercent, diskPercent],
          backgroundColor: 'rgba(59, 130, 246, 0.8)',
          borderColor: 'rgba(59, 130, 246, 1)',
          borderWidth: 2
        }, {
          label: labelAvailable,
          data: [availableMemoryPercent, availableCpuPercent, availableDiskPercent],
          backgroundColor: 'rgba(34, 197, 94, 0.4)',
          borderColor: 'rgba(34, 197, 94, 1)',
          borderWidth: 2
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: {
          duration: 0 // Sin animación para evitar conflictos
        },
        plugins: {
          legend: {
            position: 'bottom',
            labels: {
              color: 'rgb(226, 232, 240)',
              font: { size: 12 }
            }
          },
          title: {
            display: true,
            text: titleText,
            color: 'rgb(226, 232, 240)',
            font: { size: 16, weight: 'bold' }
          },
          tooltip: {
            callbacks: {
              label: function(context) {
                const label = context.dataset.label || ''
                const value = context.parsed.y
                // Todos los valores son porcentajes ahora
                return `${label}: ${value}%`
              }
            }
          }
        },
        scales: {
          y: {
            stacked: true,
            beginAtZero: true,
            min: 0,
            max: 100,
            suggestedMin: 0,
            suggestedMax: 100,
            ticks: {
              color: 'rgb(226, 232, 240)',
              stepSize: 20,
              callback: function(value) {
                return value + '%'
              }
            },
            grid: {
              color: 'rgba(148, 163, 184, 0.1)'
            }
          },
          x: {
            stacked: true,
            ticks: {
              color: 'rgb(226, 232, 240)'
            },
            grid: {
              color: 'rgba(148, 163, 184, 0.1)'
            }
          }
        }
      }
    })
    
    console.log('✅ [RESOURCES] Gráfica recreada exitosamente')
  } catch (error) {
    console.error('❌ [RESOURCES] Error actualizando gráfica de recursos:', error)
  } finally {
    isUpdatingResourcesChart = false
  }
}

// Función helper para calcular el máximo del eje Y de manera inteligente
function calculateYAxisMax(value) {
  if (value === 0) return 1 // Si no hay datos, mostrar mínimo 1 MB
  
  // Agregar 20% de margen arriba
  const withMargin = value * 1.2
  
  // Redondear a valores "bonitos" para mejor visualización
  if (value < 0.1) {
    // Valores muy pequeños (< 0.1 MB): redondear a 0.1, 0.2, 0.5, 1
    if (withMargin < 0.1) return 0.1
    if (withMargin < 0.2) return 0.2
    if (withMargin < 0.5) return 0.5
    return 1
  } else if (value < 1) {
    // Valores pequeños (0.1 - 1 MB): redondear a 0.5, 1, 2, 5
    if (withMargin < 0.5) return 0.5
    if (withMargin < 1) return 1
    if (withMargin < 2) return 2
    return 5
  } else if (value < 10) {
    // Valores medianos (1 - 10 MB): redondear a 1, 2, 5, 10, 20
    const rounded = Math.ceil(withMargin)
    if (rounded <= 1) return 1
    if (rounded <= 2) return 2
    if (rounded <= 5) return 5
    if (rounded <= 10) return 10
    return 20
  } else if (value < 100) {
    // Valores grandes (10 - 100 MB): redondear a múltiplos de 10
    return Math.ceil(withMargin / 10) * 10
  } else if (value < 1000) {
    // Valores muy grandes (100 - 1000 MB): redondear a múltiplos de 50
    return Math.ceil(withMargin / 50) * 50
  } else {
    // Valores extremos (> 1000 MB = 1 GB): redondear a múltiplos de 100
    return Math.ceil(withMargin / 100) * 100
  }
}

function createNetworkChart() {
  try {
    if (!networkChartEl.value) {
      console.warn('⚠️ [NETWORK] networkChartEl no está disponible')
      return
    }
    
    console.log('📊 [NETWORK] Creando gráfica de red...')
    
    // Destruir gráfico existente si hay
    if (networkChart.value) {
      try {
        networkChart.value.destroy()
      } catch (e) {
        // Ignorar errores al destruir
      }
      networkChart.value = null
    }
    
    // Preparar datos iniciales - si no hay historial, crear datos vacíos
    // Usar toRaw y JSON para crear copias completamente desvinculadas
    const rawHistory = toRaw(networkStats.value.history)
    const historyCopy = rawHistory.length > 0 
      ? JSON.parse(JSON.stringify(rawHistory))
      : []
    
    // Crear arrays completamente nuevos con valores primitivos
    const labels = []
    const sentData = []
    const recvData = []
    
    if (historyCopy.length > 0) {
      for (let i = 0; i < historyCopy.length; i++) {
        const item = historyCopy[i]
        const time = new Date(item.time)
        labels.push(String(time.toLocaleTimeString()))
        sentData.push(Number(parseFloat(item.sent) || 0))
        recvData.push(Number(parseFloat(item.recv) || 0))
      }
    } else {
      // Al menos un punto inicial
      labels.push(String(new Date().toLocaleTimeString()))
      sentData.push(0)
      recvData.push(0)
    }
    
    // Calcular el máximo de los datos para ajustar el eje Y dinámicamente
    const maxSent = sentData.length > 0 ? Math.max(...sentData) : 0
    const maxRecv = recvData.length > 0 ? Math.max(...recvData) : 0
    const maxValue = Math.max(maxSent, maxRecv)
    const yAxisMax = calculateYAxisMax(maxValue)
    
    // Obtener traducciones como strings primitivos (no reactivos)
    const labelSent = String(t('admin.charts.NetworkSent'))
    const labelRecv = String(t('admin.charts.NetworkRecv'))
    const titleText = String(t('admin.charts.NetworkTraffic'))
    
    console.log('📊 [NETWORK] Datos para gráfica:', {
      labels: labels.length,
      sentData: sentData.length,
      recvData: recvData.length,
      maxValue: maxValue,
      yAxisMax: yAxisMax
    })
    
    networkChart.value = new Chart(networkChartEl.value, {
      type: 'line',
      data: {
        labels: labels,
        datasets: [{
          label: labelSent,
          data: sentData,
          borderColor: 'rgba(59, 130, 246, 1)',
          backgroundColor: 'rgba(59, 130, 246, 0.1)',
          borderWidth: 2,
          fill: true,
          tension: 0.4,
          pointRadius: 3,
          pointHoverRadius: 5
        }, {
          label: labelRecv,
          data: recvData,
          borderColor: 'rgba(34, 197, 94, 1)',
          backgroundColor: 'rgba(34, 197, 94, 0.1)',
          borderWidth: 2,
          fill: true,
          tension: 0.4,
          pointRadius: 3,
          pointHoverRadius: 5
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: {
          duration: 0 // Sin animación para evitar problemas
        },
        plugins: {
          legend: {
            position: 'bottom',
            labels: {
              color: 'rgb(226, 232, 240)',
              font: { size: 12 }
            }
          },
          title: {
            display: true,
            text: titleText,
            color: 'rgb(226, 232, 240)',
            font: { size: 16, weight: 'bold' }
          },
          tooltip: {
            callbacks: {
              label: function(context) {
                const label = context.dataset.label || ''
                const value = context.parsed.y
                return `${label}: ${value} MB`
              }
            }
          }
        },
        scales: {
          y: {
            beginAtZero: true,
            max: yAxisMax, // Ajustar el máximo del eje Y basado en los datos
            ticks: {
              color: 'rgb(226, 232, 240)',
              callback: function(value) {
                return value.toFixed(2) + ' MB' // Mostrar 2 decimales
              }
            },
            grid: {
              color: 'rgba(148, 163, 184, 0.1)'
            }
          },
          x: {
            ticks: {
              color: 'rgb(226, 232, 240)',
              maxRotation: 45,
              minRotation: 45
            },
            grid: {
              color: 'rgba(148, 163, 184, 0.1)'
            }
          }
        }
      }
    })
    
    console.log('✅ [NETWORK] Gráfica de red creada exitosamente')
  } catch (error) {
    console.error('❌ [NETWORK] Error creating network chart:', error)
  }
}

function updateNetworkChart() {
  // Evitar actualizaciones simultáneas
  if (isUpdatingNetworkChart) {
    console.log('⏳ [NETWORK] Actualización en progreso, saltando...')
    return
  }
  
  try {
    isUpdatingNetworkChart = true
    
    if (!networkChartEl.value) {
      console.warn('⚠️ [NETWORK] networkChartEl no está disponible')
      isUpdatingNetworkChart = false
      return
    }
    
    if (networkStats.value.history.length === 0) {
      console.log('📊 [NETWORK] No hay datos en el historial aún')
      isUpdatingNetworkChart = false
      return
    }
    
    // Si no existe la gráfica, crearla
    if (!networkChart.value) {
      console.log('📊 [NETWORK] Gráfica no existe, creándola...')
      isUpdatingNetworkChart = false
      createNetworkChart()
      return
    }
    
    // Actualizar datos existentes sin recrear la gráfica (más eficiente)
    const rawHistory = toRaw(networkStats.value.history)
    const historyCopy = JSON.parse(JSON.stringify(rawHistory))
    
    // Crear arrays completamente nuevos con valores primitivos
    const labels = []
    const sentData = []
    const recvData = []
    
    for (let i = 0; i < historyCopy.length; i++) {
      const item = historyCopy[i]
      const time = new Date(item.time)
      labels.push(String(time.toLocaleTimeString()))
      sentData.push(Number(parseFloat(item.sent) || 0))
      recvData.push(Number(parseFloat(item.recv) || 0))
    }
    
    // Calcular el máximo de los datos para ajustar el eje Y dinámicamente
    const maxSent = sentData.length > 0 ? Math.max(...sentData) : 0
    const maxRecv = recvData.length > 0 ? Math.max(...recvData) : 0
    const maxValue = Math.max(maxSent, maxRecv)
    const yAxisMax = calculateYAxisMax(maxValue)
    
    console.log('🔄 [NETWORK] Actualizando gráfica:', {
      puntos: labels.length,
      maxValue: maxValue,
      yAxisMax: yAxisMax
    })
    
    // Actualizar los datos de la gráfica sin recrearla
    if (networkChart.value && networkChart.value.data) {
      networkChart.value.data.labels = labels
      networkChart.value.data.datasets[0].data = sentData
      networkChart.value.data.datasets[1].data = recvData
      networkChart.value.options.scales.y.max = yAxisMax
      
      // Actualizar sin animación para mejor rendimiento
      networkChart.value.update('none')
      console.log('✅ [NETWORK] Gráfica actualizada exitosamente')
    }
  } catch (error) {
    console.error('❌ [NETWORK] Error actualizando network chart:', error)
    // Si hay error, intentar recrear la gráfica
    if (networkChart.value) {
      try {
        networkChart.value.destroy()
      } catch (e) {
        // Ignorar errores al destruir
      }
      networkChart.value = null
    }
    // Recrear en el próximo ciclo
    setTimeout(() => {
      if (!isUpdatingNetworkChart) {
        createNetworkChart()
      }
    }, 100)
  } finally {
    isUpdatingNetworkChart = false
  }
}
</script>

<template>
  <div 
    :class="[
      'admin-view',
      'w-full max-w-5xl ml-auto mr-0',
      'space-y-8'
    ]"
    style="margin-left: auto; margin-right: 0; padding-left: 16rem;"
  >
    <!-- Header -->
    <div 
      :class="[
        'flex items-center justify-between',
        'pb-4 border-b-2 border-border/50'
      ]"
    >
      <div>
        <h1 
          :class="[
            'text-3xl font-bold text-foreground mb-2'
          ]"
        >
          {{ t('admin.Admin') }}
        </h1>
        <p 
          :class="[
            'text-muted-foreground'
          ]"
        >
          {{ t('admin.AdminDescription') }}
        </p>
      </div>
      <div 
        :class="[
          'p-3 rounded-xl',
          'bg-primary/10 text-primary',
          'border-2 border-primary/30'
        ]"
      >
        <icon name="hi-shield-check" class="text-3xl" />
      </div>
    </div>

    <!-- Loading State -->
    <loader v-if="loading" />

    <!-- Content -->
    <div v-else class="space-y-6">
      <!-- Estadísticas -->
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <div 
          :class="[
            'p-6 rounded-xl',
            'bg-muted/30 border-2 border-border/50',
            'shadow-lg hover:shadow-xl transition-shadow'
          ]"
        >
          <div class="flex items-center justify-between mb-4">
            <h3 
              :class="[
                'text-lg font-semibold text-foreground'
              ]"
            >
              {{ t('servers.Servers') }}
            </h3>
            <icon name="hi-server" class="text-2xl text-primary" />
          </div>
          <p 
            :class="[
              'text-3xl font-bold text-primary'
            ]"
          >
            {{ stats.totalServers }}
          </p>
        </div>

        <div 
          :class="[
            'p-6 rounded-xl',
            'bg-muted/30 border-2 border-border/50',
            'shadow-lg hover:shadow-xl transition-shadow'
          ]"
        >
          <div class="flex items-center justify-between mb-4">
            <h3 
              :class="[
                'text-lg font-semibold text-foreground'
              ]"
            >
              {{ t('users.Users') }}
            </h3>
            <icon name="hi-users" class="text-2xl text-primary" />
          </div>
          <p 
            :class="[
              'text-3xl font-bold text-primary'
            ]"
          >
            {{ stats.totalUsers }}
          </p>
        </div>

        <div 
          :class="[
            'p-6 rounded-xl',
            'bg-muted/30 border-2 border-border/50',
            'shadow-lg hover:shadow-xl transition-shadow'
          ]"
        >
          <div class="flex items-center justify-between mb-4">
            <h3 
              :class="[
                'text-lg font-semibold text-foreground'
              ]"
            >
              {{ t('nodes.Nodes') }}
            </h3>
            <icon name="hi-server" class="text-2xl text-primary" />
          </div>
          <p 
            :class="[
              'text-3xl font-bold text-primary'
            ]"
          >
            {{ stats.totalNodes }}
          </p>
        </div>

        <div 
          :class="[
            'p-6 rounded-xl',
            'bg-muted/30 border-2 border-border/50',
            'shadow-lg hover:shadow-xl transition-shadow'
          ]"
        >
          <div class="flex items-center justify-between mb-4">
            <h3 
              :class="[
                'text-lg font-semibold text-foreground'
              ]"
            >
              {{ t('templates.Repositories') || t('templates.Templates') }}
            </h3>
            <icon name="hi-document" class="text-2xl text-primary" />
          </div>
          <p 
            :class="[
              'text-3xl font-bold text-primary'
            ]"
          >
            {{ stats.totalTemplates }}
          </p>
        </div>
      </div>

      <!-- Gráfico de uso de recursos del sistema -->
      <div 
        :class="[
          'p-6 rounded-xl',
          'bg-muted/30 border-2 border-border/50',
          'shadow-lg'
        ]"
      >
        <div 
          :class="[
            'w-full mx-auto'
          ]"
          style="height: 400px;"
        >
          <canvas ref="resourcesChartEl" />
        </div>
        
        <!-- Información de uso de recursos -->
        <div 
          v-if="systemInfo"
          :class="[
            'mt-6 pt-6 border-t-2 border-border/50',
            'grid grid-cols-1 md:grid-cols-3 gap-4'
          ]"
        >
          <!-- CPU -->
          <div 
            :class="[
              'p-4 rounded-lg',
              'bg-background/50 border-2 border-border/30',
              'flex flex-col items-center justify-center'
            ]"
          >
            <div 
              :class="[
                'text-sm font-medium text-muted-foreground mb-2'
              ]"
            >
              {{ t('admin.charts.CPU') }}
            </div>
            <div 
              :class="[
                'text-3xl font-bold',
                systemInfo.cpuUsage > 80 ? 'text-red-500' : 
                systemInfo.cpuUsage > 60 ? 'text-yellow-500' : 
                'text-green-500'
              ]"
            >
              {{ systemInfo.cpuUsage?.toFixed(1) || 0 }}%
            </div>
            <div class="text-xs text-muted-foreground mt-1">
              {{ t('admin.charts.Used') }}
            </div>
          </div>
          
          <!-- Memoria RAM -->
          <div 
            :class="[
              'p-4 rounded-lg',
              'bg-background/50 border-2 border-border/30',
              'flex flex-col items-center justify-center'
            ]"
          >
            <div 
              :class="[
                'text-sm font-medium text-muted-foreground mb-2'
              ]"
            >
              {{ t('admin.charts.Memory') }}
            </div>
            <div 
              :class="[
                'text-3xl font-bold',
                (resourceStats.usedMemory / resourceStats.totalMemory * 100) > 80 ? 'text-red-500' : 
                (resourceStats.usedMemory / resourceStats.totalMemory * 100) > 60 ? 'text-yellow-500' : 
                'text-green-500'
              ]"
            >
              {{ ((resourceStats.usedMemory / resourceStats.totalMemory) * 100).toFixed(1) }}%
            </div>
            <div class="text-xs text-muted-foreground mt-1">
              {{ (resourceStats.usedMemory / 1024 / 1024 / 1024).toFixed(2) }} GB / 
              {{ (resourceStats.totalMemory / 1024 / 1024 / 1024).toFixed(2) }} GB
            </div>
          </div>
          
          <!-- Disco -->
          <div 
            :class="[
              'p-4 rounded-lg',
              'bg-background/50 border-2 border-border/30',
              'flex flex-col items-center justify-center'
            ]"
          >
            <div 
              :class="[
                'text-sm font-medium text-muted-foreground mb-2'
              ]"
            >
              {{ t('admin.charts.Disk') }}
            </div>
            <div 
              :class="[
                'text-3xl font-bold',
                (resourceStats.usedDisk / resourceStats.totalDisk * 100) > 80 ? 'text-red-500' : 
                (resourceStats.usedDisk / resourceStats.totalDisk * 100) > 60 ? 'text-yellow-500' : 
                'text-green-500'
              ]"
            >
              {{ ((resourceStats.usedDisk / resourceStats.totalDisk) * 100).toFixed(1) }}%
            </div>
            <div class="text-xs text-muted-foreground mt-1">
              {{ (resourceStats.usedDisk / 1024 / 1024 / 1024).toFixed(2) }} GB / 
              {{ (resourceStats.totalDisk / 1024 / 1024 / 1024).toFixed(2) }} GB
            </div>
          </div>
        </div>
      </div>

      <!-- Gráfico de tráfico de red -->
      <div 
        :class="[
          'p-6 rounded-xl',
          'bg-muted/30 border-2 border-border/50',
          'shadow-lg'
        ]"
      >
        <div 
          :class="[
            'w-full mx-auto'
          ]"
          style="height: 400px;"
        >
          <canvas ref="networkChartEl" />
        </div>
        
        <!-- Información de tráfico de red -->
        <div 
          v-if="networkStats.history.length > 0"
          :class="[
            'mt-6 pt-6 border-t-2 border-border/50',
            'grid grid-cols-1 md:grid-cols-2 gap-4'
          ]"
        >
          <!-- Subida -->
          <div 
            :class="[
              'p-4 rounded-lg',
              'bg-background/50 border-2 border-blue-500/30',
              'flex flex-col items-center justify-center'
            ]"
          >
            <div 
              :class="[
                'text-sm font-medium text-muted-foreground mb-2 flex items-center gap-2'
              ]"
            >
              <icon name="hi-arrow-up" class="text-blue-500" />
              {{ t('admin.charts.NetworkSent') }}
            </div>
            <div 
              :class="[
                'text-3xl font-bold text-blue-500'
              ]"
            >
              {{ networkStats.history.length > 0 ? networkStats.history[networkStats.history.length - 1].sent.toFixed(2) : '0.00' }}
            </div>
            <div class="text-xs text-muted-foreground mt-1">
              MB/s (últimos 5 segundos)
            </div>
          </div>
          
          <!-- Bajada -->
          <div 
            :class="[
              'p-4 rounded-lg',
              'bg-background/50 border-2 border-green-500/30',
              'flex flex-col items-center justify-center'
            ]"
          >
            <div 
              :class="[
                'text-sm font-medium text-muted-foreground mb-2 flex items-center gap-2'
              ]"
            >
              <icon name="hi-arrow-down" class="text-green-500" />
              {{ t('admin.charts.NetworkRecv') }}
            </div>
            <div 
              :class="[
                'text-3xl font-bold text-green-500'
              ]"
            >
              {{ networkStats.history.length > 0 ? networkStats.history[networkStats.history.length - 1].recv.toFixed(2) : '0.00' }}
            </div>
            <div class="text-xs text-muted-foreground mt-1">
              MB/s (últimos 5 segundos)
            </div>
          </div>
        </div>
      </div>
      
      <!-- Gestión de Servidores -->
      <div 
        :class="[
          'p-6 rounded-xl',
          'bg-muted/30 border-2 border-border/50',
          'shadow-lg'
        ]"
      >
        <h2 
          :class="[
            'text-2xl font-bold text-foreground mb-4',
            'pb-2 border-b-2 border-border/50'
          ]"
        >
          {{ t('servers.Servers') }}
        </h2>
        <div class="flex flex-wrap gap-4">
          <router-link
            :to="{ name: 'Admin.ServerList' }"
            :class="[
              'px-6 py-3 rounded-lg',
              'bg-muted text-foreground',
              'hover:bg-muted/80 transition-colors',
              'flex items-center gap-2',
              'font-medium',
              'border-2 border-border/50'
            ]"
          >
            <icon name="hi-server" />
            {{ t('servers.Servers') }}
          </router-link>
        </div>
      </div>
      
      <!-- Acciones rápidas -->
      <div 
        :class="[
          'p-6 rounded-xl',
          'bg-muted/30 border-2 border-border/50',
          'shadow-lg'
        ]"
      >
        <h2 
          :class="[
            'text-xl font-bold text-foreground mb-4'
          ]"
        >
          {{ t('admin.QuickActions') }}
        </h2>
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <btn
            variant="outline"
            :class="['w-full']"
            @click="$router.push({ name: 'Admin.UserList' })"
          >
            <icon name="hi-users" />
            {{ t('users.Users') }}
          </btn>
          <btn
            variant="outline"
            :class="['w-full']"
            @click="$router.push({ name: 'Admin.NodeList' })"
          >
            <icon name="hi-server" />
            {{ t('nodes.Nodes') }}
          </btn>
          <btn
            variant="outline"
            :class="['w-full']"
            @click="$router.push({ name: 'Admin.TemplateList' })"
          >
            <icon name="hi-document" />
            {{ t('templates.Templates') }}
          </btn>
          <btn
            variant="outline"
            :class="['w-full']"
            @click="$router.push({ name: 'Admin.Settings' })"
          >
            <icon name="hi-cog" />
            {{ t('settings.Settings') }}
          </btn>
        </div>
      </div>

      <!-- Información del sistema -->
      <div 
        :class="[
          'p-6 rounded-xl',
          'bg-muted/30 border-2 border-border/50',
          'shadow-lg'
        ]"
      >
        <h2 
          :class="[
            'text-xl font-bold text-foreground mb-4'
          ]"
        >
          {{ t('admin.SystemInfo') }}
        </h2>
        <div class="space-y-3">
          <div class="flex items-center justify-between py-2 border-b border-border/30">
            <span class="text-muted-foreground">{{ t('admin.PanelVersion') }}</span>
            <span class="text-foreground font-semibold">SkyPanel</span>
          </div>
          <div class="flex items-center justify-between py-2 border-b border-border/30">
            <span class="text-muted-foreground">{{ t('admin.TotalServers') }}</span>
            <span class="text-foreground font-semibold">{{ stats.totalServers }}</span>
          </div>
          <div class="flex items-center justify-between py-2 border-b border-border/30">
            <span class="text-muted-foreground">{{ t('admin.TotalUsers') }}</span>
            <span class="text-foreground font-semibold">{{ stats.totalUsers }}</span>
          </div>
          <div class="flex items-center justify-between py-2">
            <span class="text-muted-foreground">{{ t('admin.TotalNodes') }}</span>
            <span class="text-foreground font-semibold">{{ stats.totalNodes }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.admin-view {
  min-height: calc(100vh - 3rem);
  padding-top: 1.5rem;
  padding-bottom: 1.5rem;
}
</style>

