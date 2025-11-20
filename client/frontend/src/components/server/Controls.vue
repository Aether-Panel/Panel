<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'

const { t } = useI18n()

const menuOpen = ref(false)

function hideMenu() {
  if (!menuOpen.value) return
  menuOpen.value = false
}

function toggleMenu() {
  const state = menuOpen.value
  setTimeout(() => menuOpen.value = !state, 100)
}

const props = defineProps({
  server: { type: Object, required: true }
})

const hotkeys = {
  'r r': () => props.server.restart(),
  'r s': () => props.server.stop(),
  'r k': () => props.server.kill(),
  'r i': () => props.server.install()
}

function onHotkey(keys) {
  if (hotkeys[keys]) hotkeys[keys]()
}

const showMenu =
  props.server.hasScope('server.start') || 
  props.server.hasScope('server.stop') || 
  props.server.hasScope('server.kill') || 
  props.server.hasScope('server.install')
</script>

<template>
  <span v-hotkey="Object.keys(hotkeys)" class="inline-flex items-center gap-2 flex-wrap" @hotkey="onHotkey">
    <btn v-if="server.hasScope('server.start')" @click="server.start()">
      <icon name="play" />
      <span>{{ t('servers.Start') }}</span>
    </btn>
    <btn v-if="server.hasScope('server.start') && server.hasScope('server.stop')" @click="server.restart()">
      <icon name="restart" />
      <span>{{ t('servers.Restart') }}</span>
    </btn>
    <btn v-if="server.hasScope('server.stop')" @click="server.stop()">
      <icon name="stop" />
      <span>{{ t('servers.Stop') }}</span>
    </btn>
    <btn v-if="server.hasScope('server.kill')" @click="server.kill()">
      <icon name="kill" />
      <span>{{ t('servers.Kill') }}</span>
    </btn>
    <btn v-if="server.hasScope('server.install')" @click="server.install()">
      <icon name="install" />
      <span>{{ t('servers.Install') }}</span>
    </btn>
    <div v-if="showMenu" class="relative">
      <btn variant="icon" @click="toggleMenu()">
        <icon name="menu" />
      </btn>
      <transition
        enter-active-class="transition-all duration-200 ease-out"
        enter-from-class="opacity-0 scale-95 translate-y-[-10px]"
        enter-to-class="opacity-100 scale-100 translate-y-0"
        leave-active-class="transition-all duration-150 ease-in"
        leave-from-class="opacity-100 scale-100 translate-y-0"
        leave-to-class="opacity-0 scale-95 translate-y-[-10px]"
      >
        <div v-if="menuOpen" v-click-outside="hideMenu" class="absolute right-0 top-full mt-2 rounded-xl border-2 border-border/50 bg-background shadow-xl z-50 min-w-[200px] overflow-hidden">
          <div class="flex flex-col p-2 gap-1">
            <btn v-if="server.hasScope('server.start')" variant="text" class="justify-start w-full" @click="menuOpen = false; server.start()">
              <icon name="play" />
              <span>{{ t('servers.Start') }}</span>
            </btn>
            <btn v-if="server.hasScope('server.start') && server.hasScope('server.stop')" variant="text" class="justify-start w-full" @click="menuOpen = false; server.restart()">
              <icon name="restart" />
              <span>{{ t('servers.Restart') }}</span>
            </btn>
            <btn v-if="server.hasScope('server.stop')" variant="text" class="justify-start w-full" @click="menuOpen = false; server.stop()">
              <icon name="stop" />
              <span>{{ t('servers.Stop') }}</span>
            </btn>
            <btn v-if="server.hasScope('server.kill')" variant="text" class="justify-start w-full" @click="menuOpen = false; server.kill()">
              <icon name="kill" />
              <span>{{ t('servers.Kill') }}</span>
            </btn>
            <btn v-if="server.hasScope('server.install')" variant="text" class="justify-start w-full" @click="menuOpen = false; server.install()">
              <icon name="install" />
              <span>{{ t('servers.Install') }}</span>
            </btn>
          </div>
        </div>
      </transition>
    </div>
  </span>
</template>
