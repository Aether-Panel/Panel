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
  
  toast.success(t('servers.SettingsSaved'))
  
  // Emit event to update plugins tab visibility
  if (events) {
    events.emit('server:plugins-enabled-changed', pluginsEnabled.value)
  }
}

function getFlagHint(name) {
  if (te(`servers.flags.hint.${name}`, locale))
    return t(`servers.flags.hint.${name}`)
}
</script>

<template>
  <div class="space-y-6 p-4">
    <h2 class="text-2xl font-bold text-foreground" v-text="t('servers.Settings')" />
    <variables v-model="vars" :disabled="!server.hasScope('server.data.edit')" />
    <div class="space-y-4">
      <div class="flex items-center justify-between gap-4 pb-3 border-b-2 border-border/50">
        <h3 class="text-xl font-semibold text-foreground m-0" v-text="t('servers.FlagsHeader')" />
      </div>
      <div v-for="(_, name) in flags" :key="name" class="space-y-2">
        <toggle v-model="flags[name]" :disabled="!server.hasScope('server.flags.edit')" :label="t(`servers.flags.${name}`)" :hint="getFlagHint()" />
      </div>
    </div>
    <div v-if="isMinecraftJava" class="space-y-4 mt-8">
      <div class="flex items-center justify-between gap-4 pb-3 border-b-2 border-border/50">
        <h3 class="text-xl font-semibold text-foreground m-0" v-text="t('plugins.PluginsSettings')" />
      </div>
      <toggle 
        v-model="pluginsEnabled" 
        :disabled="!server.hasScope('server.data.edit')" 
        :label="t('plugins.EnablePluginsTab')" 
        :hint="t('plugins.EnablePluginsTabHint')" 
      />
    </div>
    <div v-if="!anyItems" class="text-muted-foreground" v-text="t('servers.NoSettings')" />
    <btn v-else color="primary" @click="save()"><icon name="save" />{{ t('servers.SaveSettings') }}</btn>
  </div>
</template>
