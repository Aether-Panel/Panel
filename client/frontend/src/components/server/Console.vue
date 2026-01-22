<script setup>
import { ref, inject, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/ui/Icon.vue'
import TextField from '@/components/ui/TextField.vue'

import ConsoleWorker from '@/utils/consoleWorker.js?worker&inline'
const worker = new ConsoleWorker()
let lastElem = null

const { t } = useI18n()
const config = inject('config')
const panelName = config.branding.name

const command = ref('')
const console = ref(null)
let lastMessageTime = 0
let autoScroll = true // Control de auto-scroll

const props = defineProps({
  server: { type: Object, required: true }
})

let unbindEvent = null
let task = null
onMounted(async () => {
  worker.addEventListener("message", onWorkerMessage)
  unbindEvent = props.server.on('console', onMessage)

  onMessage(await props.server.getConsole())
  task = props.server.startTask(async () => {
    if (props.server.needsPolling() && props.server.hasScope('server.console')) {
      onMessage(await props.server.getConsole(lastMessageTime))
    }
  }, 5000)
  
  // Scroll inicial al final
  await nextTick()
  scrollToBottom()
})

onUnmounted(() => {
  if (unbindEvent) unbindEvent()
  if (task) props.server.stopTask(task)
  clearConsole()
})

function onMessage(e) {
  if ('epoch' in e) {
    lastMessageTime = e.epoch
  } else {
    lastMessageTime = Date.now()
  }
  worker.postMessage({ ...e, panelName })
}

function scrollToBottom() {
  if (console.value) {
    // Verificar si el usuario está cerca del final antes de hacer scroll automático
    const isNearBottom = console.value.scrollHeight - console.value.scrollTop <= console.value.clientHeight + 100
    if (isNearBottom || autoScroll) {
      nextTick(() => {
        console.value.scrollTop = console.value.scrollHeight
      })
    }
  }
}

function onWorkerMessage(e) {
  const newElems = []
  e.data.map(update => {
    if (update.op === 'update' && lastElem) {
      lastElem.innerHTML = update.content
    } else {
      const el = document.createElement('div')
      el.innerHTML = update.content
      newElems.push(el)
      lastElem = el
    }
  })
  if (newElems.length + console.value.children.length > 1200) {
    let elems = console.value.children.concat(newElems)
    elems = elems.slice(elems.length - 1000, elems.length)
    console.value.replaceChildren(elems)
  } else {
    console.value.append(...newElems)
  }
  
  // Auto-scroll al final después de agregar nuevos elementos
  scrollToBottom()
}

function clearConsole() {
  if (console.value) {
    console.value.replaceChildren([])
    nextTick(() => {
      scrollToBottom()
    })
  }
}

// Detectar cuando el usuario hace scroll manual para desactivar auto-scroll temporalmente
function onScroll() {
  if (console.value) {
    const isAtBottom = console.value.scrollHeight - console.value.scrollTop <= console.value.clientHeight + 50
    autoScroll = isAtBottom
  }
}

const history = ref([])
const historyIndex = ref(-1)
const temporaryCommand = ref('')

function sendCommand() {
  if (historyIndex.value !== -1) {
    history.value.splice(historyIndex.value, 1)
  }

  if (history.value.length === 0 || history.value[history.value.length - 1] !== command.value) {
    history.value.push(command.value)
  }

  historyIndex.value = -1
  temporaryCommand.value = ''

  if (history.value.length > 100) {
    history.value.splice(0, 1)
  }

  props.server.sendCommand(command.value)
  command.value = ''
  
  // Forzar auto-scroll después de enviar comando
  autoScroll = true
  nextTick(() => {
    scrollToBottom()
  })
}

function previousCommand() {
  if (historyIndex.value === -1 && history.value.length > 0) {
    historyIndex.value = history.value.length - 1
    temporaryCommand.value = command.value
  } else if (historyIndex.value > 0) {
    historyIndex.value--
  } else {
    return
  }

  command.value = history.value[historyIndex.value]
}

function nextCommand() {
  if (historyIndex.value === -1) {
    return
  }

  historyIndex.value++

  if (historyIndex.value >= history.value.length) {
    historyIndex.value = -1
    command.value = temporaryCommand.value
  } else {
    command.value = history.value[historyIndex.value]
  }
}
</script>

<style scoped>
.console-container {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  padding: 2rem;
  max-width: 1400px;
  margin: 0 auto;
  height: calc(100vh - 12rem);
  min-height: 600px;
  max-height: 900px;
}

/* Header */
.console-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 1.5rem;
  border-bottom: 2px solid #475569;
  flex-shrink: 0;
}

.console-header-content {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.console-header-icon {
  width: 2.5rem;
  height: 2.5rem;
  padding: 0.5rem;
  background: #3b82f6;
  border-radius: 0.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.console-header-icon :deep(svg),
.console-header-icon :deep(svg path),
.console-header-icon :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
  width: 1.5rem;
  height: 1.5rem;
}

.console-title {
  font-size: 1.875rem;
  font-weight: 700;
  color: #f1f5f9;
  margin: 0;
  line-height: 1.2;
}

.console-subtitle {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0.25rem 0 0;
}

.console-clear-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.5rem;
  height: 2.5rem;
  padding: 0;
  border: 2px solid #475569;
  background: #1e293b;
  color: #cbd5e1;
  cursor: pointer;
  border-radius: 0.5rem;
  transition: all 0.2s ease-in-out;
}

.console-clear-btn :deep(svg),
.console-clear-btn :deep(svg path),
.console-clear-btn :deep(svg *) {
  color: #cbd5e1 !important;
  fill: #cbd5e1 !important;
  stroke: #cbd5e1 !important;
  width: 1.25rem;
  height: 1.25rem;
}

.console-clear-btn:hover {
  background: #334155;
  border-color: #64748b;
  color: #f1f5f9;
}

.console-clear-btn:hover :deep(svg),
.console-clear-btn:hover :deep(svg path),
.console-clear-btn:hover :deep(svg *) {
  color: #f1f5f9 !important;
  fill: #f1f5f9 !important;
  stroke: #f1f5f9 !important;
}

/* Console Card */
.console-card {
  display: flex;
  flex-direction: column;
  flex: 1;
  background: #1e293b;
  border: 2px solid #475569;
  border-radius: 1rem;
  overflow: hidden;
  min-height: 0;
}

.console-output {
  flex: 1;
  overflow: hidden;
  background: #0a0f1a;
  min-height: 0;
  border: 2px solid #1e293b;
  border-radius: 0.5rem;
  margin: 1rem;
}

.console-content {
  height: 100%;
  padding: 1.25rem;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 0.875rem;
  line-height: 1.6;
  color: #cbd5e1;
  overflow-y: auto;
  overflow-x: auto;
  scrollbar-width: thin;
  scrollbar-color: #475569 #0a0f1a;
  background: #0a0f1a;
  background-image: 
    linear-gradient(rgba(59, 130, 246, 0.05) 1px, transparent 1px),
    linear-gradient(90deg, rgba(59, 130, 246, 0.05) 1px, transparent 1px);
  background-size: 20px 20px;
  position: relative;
  border-radius: 0.375rem;
}

.console-content::-webkit-scrollbar {
  width: 10px;
  height: 10px;
}

.console-content::-webkit-scrollbar-track {
  background: #0a0f1a;
}

.console-content::-webkit-scrollbar-thumb {
  background: #475569;
  border-radius: 5px;
}

.console-content::-webkit-scrollbar-thumb:hover {
  background: #64748b;
}

.console-input-wrapper {
  flex-shrink: 0;
  padding: 1.5rem;
  background: #1e293b;
  border-top: 2px solid #334155;
}

.console-input-container {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.console-input-field {
  flex: 1;
}

.console-send-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 3.5rem;
  height: 3rem;
  padding: 0 1rem;
  border: none;
  background: #3b82f6;
  color: #ffffff;
  cursor: pointer;
  border-radius: 0.5rem;
  transition: all 0.2s ease-in-out;
  flex-shrink: 0;
  font-weight: 600;
  font-size: 0.875rem;
}

.console-send-btn :deep(svg),
.console-send-btn :deep(svg path),
.console-send-btn :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
  width: 1.25rem;
  height: 1.25rem;
}

.console-send-btn:hover {
  background: #2563eb;
  transform: translateX(2px);
}

.console-send-btn:active {
  transform: translateX(4px);
}

/* Estilos para el contenido de la consola */
.console-content :deep(div) {
  margin: 0;
  padding: 0.125rem 0;
  word-break: break-all;
  color: #cbd5e1;
}

.console-content :deep(span) {
  display: inline;
  color: #cbd5e1;
}

/* Asegurar que los inputs sean visibles */
.console-input-wrapper :deep(input),
.console-input-wrapper :deep(label) {
  color: #f1f5f9 !important;
}

.console-input-wrapper :deep(input) {
  background: #0f172a !important;
  border-color: #475569 !important;
}

/* Asegurar que el TextField y el botón tengan la misma altura */
.console-input-wrapper :deep(.text-field) {
  margin-bottom: 0 !important;
}

.console-input-wrapper :deep(.text-field > div) {
  min-height: 3rem;
}

.console-input-wrapper :deep(.text-field input) {
  min-height: 3rem;
  box-sizing: border-box;
  padding-top: 0.75rem;
  padding-bottom: 0.75rem;
}

/* Responsive */
@media (max-width: 768px) {
  .console-container {
    padding: 1rem;
    height: calc(100vh - 10rem);
    min-height: 500px;
  }
  
  .console-title {
    font-size: 1.5rem;
  }
  
  .console-content {
    padding: 1rem;
    font-size: 0.75rem;
  }
  
  .console-input-wrapper {
    padding: 1rem;
  }
}
</style>

<template>
  <div class="console-container">
    <!-- Header -->
    <div class="console-header">
      <div class="console-header-content">
        <div class="console-header-icon">
          <icon name="codepen" />
        </div>
        <div>
          <h1 class="console-title">{{ t('servers.Console') }}</h1>
          <p class="console-subtitle">{{ t('servers.ConsoleDescription') }}</p>
        </div>
      </div>
      <button
        v-if="server.hasScope('server.console')"
        v-hotkey="'c x'"
        class="console-clear-btn"
        :title="t('servers.ClearConsole')"
        @click="clearConsole()"
      >
        <icon name="clear" />
      </button>
    </div>
    
    <!-- Console Card -->
    <div class="console-card">
    <!-- Área de consola -->
    <div 
      v-if="server.hasScope('server.console')" 
      dir="ltr" 
      class="console-output"
    >
        <div ref="console" class="console-content" @scroll="onScroll" />
    </div>
    
    <!-- Input de comandos -->
    <div 
      v-if="server.hasScope('server.console.send')" 
      dir="ltr" 
      class="console-input-wrapper"
    >
      <div class="console-input-container">
        <text-field
          v-model="command"
          v-hotkey="'c c'"
          :label="t('servers.Command')"
          :placeholder="t('servers.EnterCommand')"
          class="console-input-field"
          @keyup.enter="sendCommand()"
          @keydown.up.prevent="previousCommand()"
          @keydown.down.prevent="nextCommand()"
        />
        <button
          class="console-send-btn"
          :title="t('servers.SendCommand')"
          @click="sendCommand()"
        >
          <icon name="chevron-right" />
        </button>
        </div>
      </div>
    </div>
  </div>
</template>
