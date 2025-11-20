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

<template>
  <div class="space-y-4 p-4">
    <div class="flex items-center justify-between gap-4">
      <h2 class="text-2xl font-bold text-foreground" v-text="t('servers.Console')" />
      <icon v-if="server.hasScope('server.console')" v-hotkey="'c x'" name="clear-console" class="cursor-pointer text-muted-foreground hover:text-foreground transition-colors" @click="clearConsole()" />
    </div>
    <div v-if="server.hasScope('server.console')" dir="ltr" class="rounded-xl border-2 border-border/50 bg-background p-4 font-mono text-sm overflow-auto max-h-[600px]">
      <div ref="console" class="console" />
    </div>
    <div v-if="server.hasScope('server.console.send')" dir="ltr" class="flex gap-2 items-end">
      <div class="flex-1">
        <text-field
          v-model="command"
          v-hotkey="'c c'"
          :label="t('servers.Command')"
          @keyup.enter="sendCommand()"
          @keydown.up.prevent="previousCommand()"
          @keydown.down.prevent="nextCommand()"
        />
      </div>
      <btn variant="icon" :tooltip="t('servers.SendCommand')" @click="sendCommand()">
        <icon name="send" />
      </btn>
    </div>
  </div>
</template>
