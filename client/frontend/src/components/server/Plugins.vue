<script setup>
import {ref, inject, onMounted} from 'vue'
import { useI18n } from 'vue-i18n'
const events = inject('events')
import Loader from '@/components/ui/Loader.vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import TextField from '@/components/ui/TextField.vue'

const { t } = useI18n()
const toast = inject('toast')

const props = defineProps({
  server: { type: Object, required: true }
})

const installedPlugins = ref(null)
const searchResults = ref([])
const searchQuery = ref("")
const searching = ref(false)
const installing = ref(false)
const loading = ref(false)

const numFormat = new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 })
function formatFileSize(size) {
  if (!size) return '0 B'
  if (size < Math.pow(2, 10)) return numFormat.format(size) + ' B'
  if (size < Math.pow(2, 20)) return numFormat.format(size / Math.pow(2, 10)) + ' KiB'
  if (size < Math.pow(2, 30)) return numFormat.format(size / Math.pow(2, 20)) + ' MiB'
  if (size < Math.pow(2, 40)) return numFormat.format(size / Math.pow(2, 30)) + ' GiB'
  return numFormat.format(size / Math.pow(2, 40)) + ' TiB'
}

onMounted(async () => {
  await loadPlugins()
})

async function loadPlugins() {
  try {
    loading.value = true
    installedPlugins.value = await props.server.getPlugins()
  } catch (err) {
    toast.error(t('plugins.LoadError'))
  } finally {
    loading.value = false
  }
}

async function search() {
  if (!searchQuery.value.trim()) {
    searchResults.value = []
    return
  }

  try {
    searching.value = true
    searchResults.value = await props.server.searchPlugins(searchQuery.value)
  } catch (err) {
    toast.error(t('plugins.SearchError'))
    searchResults.value = []
  } finally {
    searching.value = false
  }
}

async function installPlugin(plugin) {
  try {
    installing.value = true
    await props.server.installPlugin(plugin.id)
    toast.success(t('plugins.InstallSuccess', { name: plugin.name }))
    await loadPlugins()
  } catch (err) {
    toast.error(t('plugins.InstallError'))
  } finally {
    installing.value = false
  }
}

function promptDelete(plugin){
  events.emit(
      'confirm',
      {
        title: t('plugins.DeletePrompt'),
        body: t('plugins.DeletePromptBody', { name: plugin.name }),
      },
      {
        text: t('plugins.Delete'),
        icon: 'remove',
        color: 'error',
        action: () => {
          deletePlugin(plugin)
        }
      },
      {
        color: 'primary'
      }
    )
}

async function deletePlugin(plugin) {
  try {
    loading.value = true
    // El nombre ya viene completo del backend (incluye .jar)
    await props.server.deletePlugin(plugin.name)
    toast.success(t('plugins.DeleteSuccess'))
    await loadPlugins()
  } catch (err) {
    toast.error(t('plugins.DeleteError'))
    console.error('Error deleting plugin:', err)
  } finally {
    loading.value = false
  }
}

function isPluginInstalled(pluginId) {
  if (!installedPlugins.value) return false
  // Buscar por nombre aproximado (los nombres pueden variar)
  return installedPlugins.value.some(p => p.name && p.name.toLowerCase().includes(pluginId.toString().toLowerCase()))
}
</script>

<template>
  <div class="space-y-6 p-4">
    <h2 class="text-2xl font-bold text-foreground" v-text="t('plugins.Plugins')" />
    
    <!-- Installed Plugins -->
    <div class="space-y-4">
      <div class="flex items-center justify-between gap-4 pb-3 border-b-2 border-border/50">
        <div class="flex items-center gap-2">
          <h3 class="text-xl font-semibold text-foreground m-0" v-text="t('plugins.InstalledPlugins')" />
          <btn variant="icon" :tooltip="t('common.Refresh')" @click="loadPlugins()">
            <icon name="reload" />
          </btn>
        </div>
      </div>
      <div class="space-y-2">
        <loader v-if="loading" />
        <div v-else-if="!installedPlugins || installedPlugins.length === 0" class="flex flex-col items-center justify-center py-12 gap-4 text-muted-foreground">
          <icon name="file" class="text-5xl opacity-50" />
          <span v-text="t('plugins.NoPluginsInstalled')" />
        </div>
        <div v-else class="space-y-2">
          <!-- eslint-disable-next-line vue/no-template-shadow -->
          <div v-for="plugin in installedPlugins" :key="plugin.name" tabindex="0" class="list-item flex items-center gap-4 cursor-pointer hover:bg-tertiary hover:border-primary transition-all duration-200">
            <icon class="text-2xl text-muted-foreground" name="file" />
            <div class="flex-1 min-w-0">
              <div class="font-medium text-foreground">{{ plugin.name.replace(/\.jar$/i, '') }}</div>
              <div class="flex gap-3 text-sm text-muted-foreground mt-1">
                <span v-if="plugin.version">v{{ plugin.version }}</span>
                <span v-if="plugin.size">{{ formatFileSize(plugin.size) }}</span>
              </div>
            </div>
            <btn
              v-if="server.hasScope('server.files.edit')"
              tabindex="-1"
              variant="icon"
              :tooltip="t('plugins.Delete')"
              :disabled="loading || installing"
              @click.stop="promptDelete(plugin)"
            >
              <icon name="remove" />
            </btn>
          </div>
        </div>
      </div>
    </div>

    <!-- Search Plugins -->
    <div class="space-y-4 mt-8">
      <div class="flex items-center justify-between gap-4 pb-3 border-b-2 border-border/50">
        <h3 class="text-xl font-semibold text-foreground m-0" v-text="t('plugins.SearchPlugins')" />
      </div>
      <div class="flex gap-3 items-center mb-4">
        <div class="flex-1 max-w-3xl">
          <text-field 
            v-model="searchQuery" 
            :label="t('plugins.SearchPlaceholder')" 
            @keyup.enter="search()"
          />
        </div>
        <btn 
          color="primary" 
          :disabled="searching || !searchQuery.trim()" 
          @click="search()"
          class="h-12 min-w-[150px] shrink-0"
        >
          <icon v-if="!searching" name="search" />
          <icon v-else name="loading" spin /> 
          {{ t('plugins.Search') }}
        </btn>
      </div>
    </div>

    <!-- Search Results -->
    <div v-if="searchResults.length > 0" class="space-y-2">
      <!-- eslint-disable-next-line vue/no-template-shadow -->
      <div v-for="plugin in searchResults" :key="plugin.id" class="list-item flex flex-col gap-4 p-4">
        <div class="flex-1 min-w-0">
          <div class="flex justify-between items-center mb-2">
            <h4 class="text-lg font-semibold text-foreground m-0">{{ plugin.name }}</h4>
            <span v-if="isPluginInstalled(plugin.id)" class="flex items-center gap-1 px-2 py-1 bg-success text-white rounded text-sm font-medium">
              <icon name="check" />
              {{ t('plugins.Installed') }}
            </span>
          </div>
          <div class="flex gap-4 text-sm text-muted-foreground mb-2">
            <span>{{ t('plugins.By') }} {{ plugin.author }}</span>
            <span v-if="plugin.downloads">{{ t('plugins.Downloads') }}: {{ plugin.downloads.toLocaleString() }}</span>
            <span v-if="plugin.version">{{ t('plugins.Version') }}: {{ plugin.version }}</span>
          </div>
          <p v-if="plugin.description" class="mt-2 text-sm text-muted-foreground leading-relaxed">{{ plugin.description }}</p>
        </div>
        <div class="flex justify-end">
          <btn
            v-if="server.hasScope('server.files.edit') && !isPluginInstalled(plugin.id)"
            color="primary"
            :disabled="installing"
            @click="installPlugin(plugin)"
          >
            <icon v-if="!installing" name="plus" />
            <icon v-else name="loading" spin />
            {{ t('plugins.Install') }}
          </btn>
          <btn
            v-else-if="isPluginInstalled(plugin.id)"
            variant="icon"
            :tooltip="t('plugins.AlreadyInstalled')"
            disabled
          >
            <icon name="check" />
          </btn>
        </div>
      </div>
    </div>
    <div v-else-if="searchQuery && !searching" class="flex flex-col items-center justify-center py-12 gap-4 text-muted-foreground">
      <span v-text="t('plugins.NoResults')" />
    </div>
  </div>
</template>

