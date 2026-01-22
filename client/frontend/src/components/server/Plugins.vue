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
  <div class="plugins-container">
    <!-- Header con título y contador -->
    <div class="plugins-header">
      <div class="plugins-header-content">
        <div class="plugins-header-icon">
          <icon name="plugins" />
        </div>
        <div>
          <h2 class="plugins-title" v-text="t('plugins.Plugins')" />
          <p class="plugins-subtitle">
            {{ installedPlugins?.length || 0 }} {{ (installedPlugins?.length || 0) === 1 ? 'plugin instalado' : 'plugins instalados' }}
          </p>
        </div>
      </div>
    </div>
    
    <!-- Plugins Instalados -->
    <div class="plugins-section">
      <div class="plugins-card">
        <div class="plugins-card-header">
          <icon name="file" class="plugins-card-icon" />
          <h3 class="plugins-card-title" v-text="t('plugins.InstalledPlugins')" />
        <btn
          variant="icon"
          size="sm"
          :tooltip="t('common.Refresh')"
          @click="loadPlugins()"
            class="plugins-refresh-btn"
        >
            <icon name="refresh" />
        </btn>
      </div>
        <div class="plugins-content">
        <loader v-if="loading" />
          <div v-else-if="!installedPlugins || installedPlugins.length === 0" class="empty-state">
            <div class="empty-state-icon">
              <icon name="file" />
            </div>
            <h3 class="empty-state-title" v-text="t('plugins.NoPluginsInstalled')" />
            <p class="empty-state-text">Busca e instala plugins desde el buscador</p>
        </div>
          <div v-else class="plugins-grid">
          <div
            v-for="plugin in installedPlugins"
            :key="plugin.name"
              class="plugin-card"
          >
              <div class="plugin-card-content">
                <div class="plugin-icon-wrapper">
              <icon name="file" />
            </div>
                <div class="plugin-info">
                  <h3 class="plugin-name">{{ plugin.name.replace(/\.jar$/i, '') }}</h3>
                  <div class="plugin-meta">
                <span v-if="plugin.version">v{{ plugin.version }}</span>
                <span v-if="plugin.size">{{ formatFileSize(plugin.size) }}</span>
              </div>
            </div>
                <div class="plugin-actions">
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
      </div>
    </div>

    <!-- Buscar Plugins -->
    <div class="plugins-section">
      <div class="plugins-card">
        <div class="plugins-card-header">
          <icon name="search" class="plugins-card-icon" />
          <h3 class="plugins-card-title" v-text="t('plugins.SearchPlugins')" />
        </div>
        <div class="plugins-search-form">
          <text-field
            v-model="searchQuery"
            :label="t('plugins.SearchPlaceholder') || 'Buscar plugins...'"
            icon="search"
            @keyup.enter="search()"
            class="plugins-search-input"
          />
          <btn
            color="primary"
            :disabled="searching || !searchQuery.trim()"
            @click="search()"
            class="plugins-search-btn"
          >
            <icon v-if="!searching" name="search" />
            <icon v-else name="restart" :class="{ 'spinning': searching }" />
            {{ t('plugins.Search') }}
          </btn>
        </div>
      </div>
    </div>

    <!-- Resultados de Búsqueda -->
    <div v-if="searchResults.length > 0" class="plugins-section">
      <div class="plugins-card">
        <div class="plugins-card-header">
          <icon name="search" class="plugins-card-icon" />
          <h3 class="plugins-card-title" v-text="t('plugins.SearchResults') || 'Resultados de búsqueda'" />
        </div>
        <div class="plugins-search-results">
        <div
          v-for="plugin in searchResults"
          :key="plugin.id"
            class="plugin-search-card"
        >
            <div class="plugin-search-header">
              <h4 class="plugin-search-name">{{ plugin.name }}</h4>
            <span
              v-if="isPluginInstalled(plugin.id)"
                class="plugin-badge-installed"
            >
              <icon name="check" />
              {{ t('plugins.Installed') }}
            </span>
          </div>
            <div class="plugin-search-meta">
            <span>{{ t('plugins.By') }} {{ plugin.author }}</span>
            <span v-if="plugin.downloads">{{ t('plugins.Downloads') }}: {{ plugin.downloads.toLocaleString() }}</span>
            <span v-if="plugin.version">{{ t('plugins.Version') }}: {{ plugin.version }}</span>
          </div>
            <p v-if="plugin.description" class="plugin-search-description">
            {{ plugin.description }}
          </p>
            <div class="plugin-search-actions">
            <btn
              v-if="server.hasScope('server.files.edit') && !isPluginInstalled(plugin.id)"
              color="primary"
              :disabled="installing"
              @click="installPlugin(plugin)"
            >
              <icon v-if="!installing" name="plus" />
                <icon v-else name="restart" :class="{ 'spinning': installing }" />
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
    </div>
    
    <div v-else-if="searchQuery && !searching" class="empty-state">
      <div class="empty-state-icon">
        <icon name="search" />
      </div>
      <h3 class="empty-state-title" v-text="t('plugins.NoResults')" />
      <p class="empty-state-text">Intenta con otros términos de búsqueda</p>
    </div>
  </div>
</template>

<style scoped>
.plugins-container {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  padding: 2rem;
  max-width: 1400px;
  margin: 0 auto;
}

/* Header */
.plugins-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 1.5rem;
  border-bottom: 2px solid #475569;
}

.plugins-header-content {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.plugins-header-icon {
  width: 2.5rem;
  height: 2.5rem;
  padding: 0.5rem;
  background: #3b82f6;
  border-radius: 0.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.plugins-header-icon :deep(svg),
.plugins-header-icon :deep(svg path),
.plugins-header-icon :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
  width: 1.5rem;
  height: 1.5rem;
}

.plugins-title {
  font-size: 1.875rem;
  font-weight: 700;
  color: #f1f5f9;
  margin: 0;
  line-height: 1.2;
}

.plugins-subtitle {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0.25rem 0 0;
}

/* Secciones */
.plugins-section {
  margin-top: 0;
}

.plugins-card {
  background: #1e293b;
  border: 2px solid #475569;
  border-radius: 1rem;
  padding: 2rem;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.plugins-card:hover {
  border-color: #3b82f6;
}

.plugins-card-header {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 2px solid #334155;
}

.plugins-card-icon {
  width: 1.5rem;
  height: 1.5rem;
  color: #3b82f6;
}

.plugins-card-icon :deep(svg),
.plugins-card-icon :deep(svg path),
.plugins-card-icon :deep(svg *) {
  color: #3b82f6 !important;
  fill: #3b82f6 !important;
  stroke: #3b82f6 !important;
  width: 1.5rem;
  height: 1.5rem;
}

.plugins-card-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0;
  flex: 1;
}

.plugins-refresh-btn {
  margin-left: auto;
}

.plugins-content {
  width: 100%;
}

/* Grid de plugins instalados */
.plugins-grid {
  display: grid;
  gap: 1.5rem;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
}

@media (max-width: 768px) {
  .plugins-grid {
    grid-template-columns: 1fr;
  }
}

/* Tarjeta de plugin */
.plugin-card {
  background: #0f172a;
  border: 2px solid #475569;
  border-radius: 1rem;
  overflow: hidden;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.plugin-card:hover {
  border-color: #3b82f6;
  transform: translateY(-2px);
}

.plugin-card-content {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1.25rem 1.5rem;
}

.plugin-icon-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3rem;
  height: 3rem;
  background: #3b82f6;
  border-radius: 0.75rem;
  flex-shrink: 0;
}

.plugin-icon-wrapper :deep(svg),
.plugin-icon-wrapper :deep(svg path),
.plugin-icon-wrapper :deep(svg *) {
  width: 1.5rem;
  height: 1.5rem;
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
}

.plugin-info {
  flex: 1;
  min-width: 0;
}

.plugin-name {
  font-size: 1.125rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0 0 0.25rem;
}

.plugin-meta {
  display: flex;
  gap: 0.75rem;
  font-size: 0.875rem;
  color: #cbd5e1;
}

.plugin-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

/* Formulario de búsqueda */
.plugins-search-form {
  display: flex;
  gap: 1rem;
  align-items: flex-end;
}

.plugins-search-input {
  flex: 1;
  min-width: 0;
}

.plugins-search-btn {
  flex-shrink: 0;
  min-width: 120px;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

.spinning {
  animation: spin 1s linear infinite;
}

/* Resultados de búsqueda */
.plugins-search-results {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.plugin-search-card {
  padding: 1.5rem;
  background: #0f172a;
  border: 2px solid #475569;
  border-radius: 0.75rem;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.plugin-search-card:hover {
  border-color: #3b82f6;
  transform: translateY(-2px);
}

.plugin-search-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.75rem;
}

.plugin-search-name {
  font-size: 1.125rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0;
}

.plugin-badge-installed {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.375rem 0.75rem;
  background: #10b981;
  color: #ffffff;
  border-radius: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  flex-shrink: 0;
}

.plugin-badge-installed :deep(svg) {
  width: 0.875rem;
  height: 0.875rem;
  color: #ffffff !important;
}

.plugin-search-meta {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
  font-size: 0.875rem;
  color: #cbd5e1;
  margin-bottom: 0.75rem;
}

.plugin-search-description {
  font-size: 0.875rem;
  color: #cbd5e1;
  line-height: 1.5;
  margin: 0.75rem 0;
}

.plugin-search-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 1rem;
  border-top: 1px solid #334155;
}

/* Estado vacío */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 4rem 2rem;
  text-align: center;
  background: #1e293b;
  border: 2px dashed #475569;
  border-radius: 1rem;
}

.empty-state-icon {
  width: 4rem;
  height: 4rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #334155;
  border-radius: 1rem;
  margin-bottom: 1.5rem;
}

.empty-state-icon :deep(svg),
.empty-state-icon :deep(svg path),
.empty-state-icon :deep(svg *) {
  width: 2rem;
  height: 2rem;
  color: #94a3b8 !important;
  fill: #94a3b8 !important;
  stroke: #94a3b8 !important;
}

.empty-state-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0 0 0.5rem;
}

.empty-state-text {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0;
}

/* Responsive */
@media (max-width: 768px) {
  .plugins-container {
    padding: 1rem;
  }
  
  .plugins-title {
    font-size: 1.5rem;
  }
  
  .plugins-search-form {
    flex-direction: column;
    align-items: stretch;
  }
  
  .plugins-search-btn {
    width: 100%;
  }
}
</style>

