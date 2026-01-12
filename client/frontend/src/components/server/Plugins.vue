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
  <div class="server-tab-content">
    <div class="server-tab-section">
      <h2 class="server-tab-title" v-text="t('plugins.Plugins')" />
    </div>
    
    <!-- Installed Plugins -->
    <div class="server-tab-section">
      <div class="server-tab-section-header">
        <h3 class="server-tab-section-title" v-text="t('plugins.InstalledPlugins')" />
        <btn
          variant="icon"
          size="sm"
          :tooltip="t('common.Refresh')"
          @click="loadPlugins()"
        >
          <icon name="reload" />
        </btn>
      </div>
      <div class="server-plugins-installed">
        <loader v-if="loading" />
        <div v-else-if="!installedPlugins || installedPlugins.length === 0" class="server-tab-empty-state">
          <icon name="file" class="server-empty-icon" />
          <p class="server-tab-empty-text" v-text="t('plugins.NoPluginsInstalled')" />
        </div>
        <div v-else class="server-plugins-list">
          <div
            v-for="plugin in installedPlugins"
            :key="plugin.name"
            class="server-plugin-item"
          >
            <div class="server-plugin-icon">
              <icon name="file" />
            </div>
            <div class="server-plugin-info">
              <div class="server-plugin-name">{{ plugin.name.replace(/\.jar$/i, '') }}</div>
              <div class="server-plugin-meta">
                <span v-if="plugin.version">v{{ plugin.version }}</span>
                <span v-if="plugin.size">{{ formatFileSize(plugin.size) }}</span>
              </div>
            </div>
            <div class="server-plugin-actions">
              <btn
                v-if="server.hasScope('server.files.edit')"
                variant="icon"
                color="error"
                :tooltip="t('plugins.Delete')"
                :disabled="loading || installing"
                @click="promptDelete(plugin)"
              >
                <icon name="remove" />
              </btn>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Search Plugins -->
    <div class="server-tab-section">
      <h3 class="server-tab-section-title" v-text="t('plugins.SearchPlugins')" />
      <div class="server-tab-card">
        <div class="server-plugin-search">
          <text-field
            v-model="searchQuery"
            :label="t('plugins.SearchPlaceholder') || 'Buscar plugins...'"
            class="server-plugin-search-input"
            @keyup.enter="search()"
          />
          <btn
            color="primary"
            variant="text"
            :disabled="searching || !searchQuery.trim()"
            @click="search()"
            class="server-plugin-search-btn"
          >
            <icon v-if="!searching" name="search" class="w-4 h-4" />
            <icon v-else name="restart" spin class="w-4 h-4" />
            {{ t('plugins.Search') }}
          </btn>
        </div>
      </div>
    </div>

    <!-- Search Results -->
    <div v-if="searchResults.length > 0" class="server-tab-section">
      <h3 class="server-tab-section-title" v-text="t('plugins.SearchResults') || 'Resultados de búsqueda'" />
      <div class="server-plugins-search-results">
        <div
          v-for="plugin in searchResults"
          :key="plugin.id"
          class="server-plugin-search-item"
        >
          <div class="server-plugin-search-header">
            <h4 class="server-plugin-search-name">{{ plugin.name }}</h4>
            <span
              v-if="isPluginInstalled(plugin.id)"
              class="server-plugin-badge-installed"
            >
              <icon name="check" />
              {{ t('plugins.Installed') }}
            </span>
          </div>
          <div class="server-plugin-search-meta">
            <span>{{ t('plugins.By') }} {{ plugin.author }}</span>
            <span v-if="plugin.downloads">{{ t('plugins.Downloads') }}: {{ plugin.downloads.toLocaleString() }}</span>
            <span v-if="plugin.version">{{ t('plugins.Version') }}: {{ plugin.version }}</span>
          </div>
          <p v-if="plugin.description" class="server-plugin-search-description">
            {{ plugin.description }}
          </p>
          <div class="server-plugin-search-actions">
            <btn
              v-if="server.hasScope('server.files.edit') && !isPluginInstalled(plugin.id)"
              color="primary"
              :disabled="installing"
              @click="installPlugin(plugin)"
            >
              <icon v-if="!installing" name="plus" />
              <icon v-else name="restart" spin />
              {{ t('plugins.Install') }}
            </btn>
            <btn
              v-else-if="isPluginInstalled(plugin.id)"
              variant="outline"
              disabled
            >
              <icon name="check" />
              {{ t('plugins.AlreadyInstalled') }}
            </btn>
          </div>
        </div>
      </div>
    </div>
    
    <div v-else-if="searchQuery && !searching" class="server-tab-empty-state">
      <p class="server-tab-empty-text" v-text="t('plugins.NoResults')" />
    </div>
  </div>
</template>

<style scoped>
.server-tab-content {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  padding: 1.5rem;
  max-width: 100%;
}

.server-tab-title {
  font-size: 1.5rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  margin: 0;
  padding-bottom: 1rem;
  border-bottom: 2px solid rgb(var(--color-border) / 0.5);
}

.server-tab-section {
  width: 100%;
}

.server-tab-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}

.server-tab-section-title {
  font-size: 1.125rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  margin: 0;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid rgb(var(--color-border) / 0.3);
  flex: 1;
}

.server-tab-card {
  background: rgb(var(--color-background));
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
  padding: 1.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.server-plugins-installed {
  width: 100%;
}

.server-plugins-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.server-plugin-item {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1rem;
  background: rgb(var(--color-background));
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  transition: all 0.2s ease-in-out;
}

.server-plugin-item:hover {
  border-color: rgb(var(--color-border));
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.1);
}

.server-plugin-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3rem;
  height: 3rem;
  background: rgb(var(--color-primary) / 0.1);
  border-radius: 0.5rem;
  color: rgb(var(--color-primary));
  flex-shrink: 0;
}

.server-plugin-icon icon {
  width: 1.5rem;
  height: 1.5rem;
}

.server-plugin-info {
  flex: 1;
  min-width: 0;
}

.server-plugin-name {
  font-size: 1rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  margin-bottom: 0.25rem;
}

.server-plugin-meta {
  display: flex;
  gap: 0.75rem;
  font-size: 0.875rem;
  color: rgb(var(--color-muted-foreground));
}

.server-plugin-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.server-plugin-search {
  display: flex;
  gap: 1rem;
  align-items: center;
}

.server-plugin-search-input {
  flex: 1;
  min-width: 0;
}

.server-plugin-search-btn {
  flex-shrink: 0 !important;
  min-width: auto !important;
  width: auto !important;
  max-width: fit-content !important;
  padding: 0.625rem 1.25rem !important;
  height: 2.5rem !important;
  font-size: 0.875rem !important;
  white-space: nowrap;
}

.server-plugins-search-results {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.server-plugin-search-item {
  padding: 1.5rem;
  background: rgb(var(--color-background));
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  transition: all 0.2s ease-in-out;
}

.server-plugin-search-item:hover {
  border-color: rgb(var(--color-border));
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.1);
}

.server-plugin-search-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.75rem;
}

.server-plugin-search-name {
  font-size: 1.125rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  margin: 0;
}

.server-plugin-badge-installed {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.375rem 0.75rem;
  background: rgb(var(--color-success) / 0.1);
  color: rgb(var(--color-success));
  border-radius: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  flex-shrink: 0;
}

.server-plugin-search-meta {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
  font-size: 0.875rem;
  color: rgb(var(--color-muted-foreground));
  margin-bottom: 0.75rem;
}

.server-plugin-search-description {
  font-size: 0.875rem;
  color: rgb(var(--color-foreground));
  line-height: 1.5;
  margin: 0.75rem 0;
}

.server-plugin-search-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 1rem;
  border-top: 1px solid rgb(var(--color-border) / 0.2);
}

.server-tab-empty-state {
  padding: 3rem 1.5rem;
  text-align: center;
  background: rgb(var(--color-muted) / 0.2);
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
}

.server-empty-icon {
  width: 3rem;
  height: 3rem;
  opacity: 0.5;
  color: rgb(var(--color-muted-foreground));
}

.server-tab-empty-text {
  color: rgb(var(--color-muted-foreground));
  margin: 0;
  font-size: 0.875rem;
}
</style>

