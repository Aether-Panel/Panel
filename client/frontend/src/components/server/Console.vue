<script setup>
import { ref, inject, onMounted, onUnmounted } from 'vue'
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
  if (newElems + console.value.children.length > 1200) {
    let elems = console.value.children.concat(newElems)
    elems = elems.slice(elems.length - 1000, elems.length)
    console.value.replaceChildren(elems)
  } else {
    console.value.append(...newElems)
  }
}

function clearConsole() {
  if (console.value) console.value.replaceChildren([])
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
  height: calc(100vh - 18rem);
  min-height: 500px;
  max-height: 800px;
  background: rgb(var(--color-background));
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
  overflow: hidden;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
}

.console-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid rgb(var(--color-border) / 0.3);
  background: rgb(var(--color-muted) / 0.3);
  flex-shrink: 0;
}

.console-title {
  font-size: 1.125rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  margin: 0;
}

.console-clear-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  padding: 0;
  border: none;
  background: transparent;
  color: rgb(var(--color-muted-foreground));
  cursor: pointer;
  border-radius: 0.375rem;
  transition: all 0.2s ease-in-out;
}

.console-clear-btn:hover {
  background: rgb(var(--color-error) / 0.1);
  color: rgb(var(--color-error));
}

.console-output {
  flex: 1;
  overflow: hidden;
  background: rgb(var(--color-background));
  border-bottom: 1px solid rgb(var(--color-border) / 0.3);
}

.console-content {
  height: 100%;
  padding: 1rem 1.25rem;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 0.875rem;
  line-height: 1.5;
  color: rgb(var(--color-foreground));
  overflow-y: auto;
  overflow-x: auto;
  scrollbar-width: thin;
  scrollbar-color: rgb(var(--color-border)) transparent;
}

.console-content::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.console-content::-webkit-scrollbar-track {
  background: transparent;
}

.console-content::-webkit-scrollbar-thumb {
  background: rgb(var(--color-border));
  border-radius: 4px;
}

.console-content::-webkit-scrollbar-thumb:hover {
  background: rgb(var(--color-primary) / 0.5);
}

.console-input-wrapper {
  flex-shrink: 0;
  padding: 1rem 1.25rem;
  background: rgb(var(--color-muted) / 0.2);
  border-top: 1px solid rgb(var(--color-border) / 0.3);
}

.console-input-container {
  display: flex;
  align-items: flex-end;
  gap: 0.75rem;
}

.console-input-field {
  flex: 1;
}

.console-send-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.5rem;
  height: 2.5rem;
  padding: 0;
  border: none;
  background: rgb(var(--color-primary));
  color: rgb(var(--color-primary-foreground));
  cursor: pointer;
  border-radius: 0.5rem;
  transition: all 0.2s ease-in-out;
  flex-shrink: 0;
}

.console-send-btn:hover {
  background: rgb(var(--color-primary) / 0.9);
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
}

.console-content :deep(span) {
  display: inline;
}
</style>

<template>
  <div class="console-container">
    <!-- Header minimalista -->
    <div class="console-header">
      <h2 class="console-title" v-text="t('servers.Console')" />
      <button
        v-if="server.hasScope('server.console')"
        v-hotkey="'c x'"
        class="console-clear-btn"
        :title="t('servers.ClearConsole')"
        @click="clearConsole()"
      >
        <icon name="close" />
      </button>
    </div>
    
    <!-- Área de consola -->
    <div 
      v-if="server.hasScope('server.console')" 
      dir="ltr" 
      class="console-output"
    >
      <div ref="console" class="console-content" />
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
</template>
