<script setup>
import { ref, inject, onMounted, onUnmounted, nextTick } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const { t } = useI18n()

const servers = ref([])
let lastPage = 0
let loadingPage = false
const allServersLoaded = ref(false)
const loaderRef = ref(null)
const firstEntry = ref(null)
let interval = null

function addServers(newServers) {
  newServers.map(server => servers.value.push(server))
  refreshServerStatus()
}

async function refreshServerStatus() {
  servers.value.map(async s => {
    if (s.canGetStatus) {
      s.online = 'loading'
      try {
        s.online = await api.server.getStatus(s.id)
      } catch {
        s.online = undefined
      }
    }
  })
}

function isLoaderVisible() {
  if (!loaderRef.value) return false
  const vw = window.innerWidth || document.documentElement.clientWidth
  const vh = window.innerHeight || document.documentElement.clientHeight
  const rect = loaderRef.value.$el.getBoundingClientRect()
  return rect.top >= 0 && rect.left >= 0 && rect.bottom <= vh && rect.right <= vw
}

async function loadPage(page = 1) {
  loadingPage = true
  const data = await api.server.list(page)
  addServers(data.servers)
  lastPage = data.paging.page
  allServersLoaded.value = data.paging.page * data.paging.pageSize >= (data.paging.total || 0)
  nextTick(() => {
    loadingPage = false
    if (!allServersLoaded.value && isLoaderVisible()) loadPage(lastPage + 1)
  })
}

function onScroll() {
  if (!loadingPage && isLoaderVisible()) loadPage(lastPage + 1)
}

onMounted(() => {
  interval = setInterval(refreshServerStatus, 30 * 1000)
  nextTick(() => {
    loadPage()
    window.addEventListener('scroll', onScroll)
  })
})

onUnmounted(() => {
  clearInterval(interval)
  window.removeEventListener('scroll', onScroll)
})

function getServerAddress(server) {
  let ip = server.node.publicHost
  if (server.ip && server.ip !== '0.0.0.0') {
    ip = server.ip
  }
  return ip + (server.port ? ':' + server.port : '')
}

function setFirstEntry(ref) {
  if (!firstEntry.value) firstEntry.value = ref
}

function focusList() {
  firstEntry.value.$el.focus()
}
</script>

<template>
  <div 
    :class="[
      'serverlist',
      'w-full max-w-7xl mx-auto',
      'space-y-6'
    ]"
  >
    <h1 
      :class="[
        'text-3xl font-bold text-foreground mb-6',
        'pb-3 border-b-2 border-border/50'
      ]"
      v-text="t('servers.Servers')" 
    />
    <div 
      v-hotkey="'l'" 
      class="server-grid-wrapper"
      @hotkey="focusList()"
    >
      <div class="server-grid-4cols">
        <template v-for="server in servers" :key="server.id">
          <router-link 
            :ref="setFirstEntry" 
            :to="{ name: 'ServerView', params: { id: server.id } }"
            class="server-card-item"
            :class="[
              server.online === true ? 'server-online' : '',
              server.online === false ? 'server-offline' : '',
              server.online === 'loading' ? 'server-loading' : ''
            ]"
          >
            <div class="server-card-wrapper">
              <div class="server-card-top">
                <icon name="chevron-right" class="server-card-arrow" />
                <h3 class="server-card-title-text" :title="server.name">
                  {{server.name}}
                </h3>
              </div>
              <div class="server-card-bottom">
                <span class="server-card-type-badge">
                  {{server.type}}
                </span>
                <span class="server-card-address-text">
                  {{getServerAddress(server)}} @ {{server.node.name}}
                </span>
              </div>
            </div>
          </router-link>
        </template>
        
        <!-- Botón de crear nuevo servidor -->
        <router-link 
          v-if="$api.auth.hasScope('server.create')" 
          v-hotkey="'c'" 
          :to="{ name: 'ServerCreate' }"
          class="server-card-item server-card-add"
        >
          <div class="server-card-wrapper server-card-add-wrapper">
            <div class="server-card-top server-card-add-top">
              <icon name="plus" class="server-card-add-icon" />
            </div>
            <div class="server-card-bottom">
              <span class="server-card-type-badge server-card-add-badge">
                {{ t('servers.Add') }}
              </span>
            </div>
          </div>
        </router-link>
        
        <!-- Loader para scroll infinito -->
        <div 
          v-if="!allServersLoaded" 
          class="server-card-item server-card-loader"
        >
          <div class="server-card-wrapper">
            <loader ref="loaderRef" small />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Grid de servidores en formato tabla 4x4 */
.server-grid-wrapper {
  display: block !important;
  width: 100% !important;
}

.server-grid-4cols {
  display: grid !important;
  grid-template-columns: repeat(4, 1fr) !important;
  gap: 1rem !important;
  width: 100% !important;
  padding: 0 !important;
  margin: 0 !important;
  list-style: none !important;
}

/* Responsive */
@media (max-width: 1279px) {
  .server-grid-4cols {
    grid-template-columns: repeat(3, 1fr) !important;
  }
}

@media (max-width: 1023px) {
  .server-grid-4cols {
    grid-template-columns: repeat(2, 1fr) !important;
  }
}

@media (max-width: 639px) {
  .server-grid-4cols {
    grid-template-columns: 1fr !important;
  }
}

/* Tarjetas de servidores */
.server-card-item {
  display: block !important;
  width: 100% !important;
  text-decoration: none !important;
  padding: 0 !important;
  margin: 0 !important;
  border: none !important;
  background: none !important;
  box-shadow: none !important;
  list-style: none !important;
  position: relative !important;
}

.server-card-item::before,
.server-card-item::after,
.server-card-item::marker {
  display: none !important;
  content: '' !important;
}

.server-card-wrapper {
  display: flex !important;
  flex-direction: column !important;
  width: 100% !important;
  min-height: 140px !important;
  height: 100% !important;
  background: rgb(var(--color-background)) !important;
  border: 2px solid rgb(var(--color-border) / 0.5) !important;
  border-radius: 0.75rem !important;
  padding: 1rem !important;
  box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px -1px rgba(0, 0, 0, 0.1) !important;
  transition: all 0.2s ease-in-out !important;
  cursor: pointer !important;
}

.server-card-item:hover .server-card-wrapper {
  border-color: rgb(var(--color-primary) / 0.5) !important;
  background: rgb(var(--color-primary) / 0.05) !important;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -2px rgba(0, 0, 0, 0.1) !important;
  transform: translateY(-2px) !important;
}

/* Estados del servidor */
.server-online .server-card-wrapper {
  border-color: rgb(var(--color-success) / 0.5) !important;
  background: rgb(var(--color-success) / 0.05) !important;
}

.server-online:hover .server-card-wrapper {
  border-color: rgb(var(--color-success) / 0.7) !important;
  background: rgb(var(--color-success) / 0.1) !important;
}

.server-offline .server-card-wrapper {
  border-color: rgb(var(--color-error) / 0.5) !important;
  background: rgb(var(--color-error) / 0.05) !important;
}

.server-offline:hover .server-card-wrapper {
  border-color: rgb(var(--color-error) / 0.7) !important;
  background: rgb(var(--color-error) / 0.1) !important;
}

.server-loading .server-card-wrapper {
  border-color: rgb(var(--color-warning) / 0.5) !important;
  background: rgb(var(--color-warning) / 0.05) !important;
}

.server-loading:hover .server-card-wrapper {
  border-color: rgb(var(--color-warning) / 0.7) !important;
  background: rgb(var(--color-warning) / 0.1) !important;
}

.server-card-top {
  display: flex !important;
  align-items: flex-start !important;
  justify-content: flex-end !important;
  gap: 0.5rem !important;
  margin-bottom: 0.75rem !important;
  flex-grow: 1 !important;
  text-align: right !important;
}

.server-card-title-text {
  display: block !important;
  font-size: 1.125rem !important;
  font-weight: 600 !important;
  color: rgb(var(--color-foreground)) !important;
  line-height: 1.5rem !important;
  margin: 0 !important;
  padding: 1px !important;
  overflow: hidden !important;
  text-overflow: ellipsis !important;
  display: -webkit-box !important;
  -webkit-line-clamp: 2 !important;
  -webkit-box-orient: vertical !important;
  transition: color 0.2s ease-in-out !important;
  text-align: right !important;
  margin-left: auto !important;
}

.server-card-item:hover .server-card-title-text {
  color: rgb(var(--color-primary)) !important;
}

.server-card-arrow {
  flex-shrink: 0 !important;
  margin-top: 0.25rem !important;
  color: rgb(var(--color-muted-foreground)) !important;
  transition: color 0.2s ease-in-out, transform 0.2s ease-in-out !important;
  width: 1.25rem !important;
  height: 1.25rem !important;
}

.server-card-item:hover .server-card-arrow {
  color: rgb(var(--color-primary)) !important;
  transform: translateX(4px) !important;
}

.server-card-bottom {
  display: flex !important;
  flex-direction: column !important;
  align-items: flex-end !important;
  gap: 0.5rem !important;
  margin-top: auto !important;
  padding-top: 0.75rem !important;
  border-top: 1px solid rgb(var(--color-border) / 0.3) !important;
}

.server-card-type-badge {
  display: inline-flex !important;
  align-items: center !important;
  justify-content: center !important;
  position: relative !important;
  padding: 0.5rem 1rem 0.5rem 1.25rem !important;
  font-size: 0.75rem !important;
  font-weight: 700 !important;
  border-radius: 0.25rem 0.5rem 0.5rem 0.25rem !important;
  background: rgb(var(--color-primary) / 0.15) !important;
  color: rgb(var(--color-primary)) !important;
  border: 1.5px solid rgb(var(--color-primary) / 0.3) !important;
  border-left-width: 3px !important;
  text-transform: uppercase !important;
  letter-spacing: 0.05em !important;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1) !important;
  transition: all 0.2s ease-in-out !important;
  white-space: nowrap !important;
  min-width: fit-content !important;
}

.server-card-type-badge::before {
  content: '' !important;
  position: absolute !important;
  left: 0 !important;
  top: 50% !important;
  transform: translateY(-50%) !important;
  width: 0 !important;
  height: 0 !important;
  border-style: solid !important;
  border-width: 0.4rem 0.4rem 0.4rem 0 !important;
  border-color: transparent rgb(var(--color-primary) / 0.3) transparent transparent !important;
}

.server-card-type-badge::after {
  content: '' !important;
  position: absolute !important;
  left: 3px !important;
  top: 50% !important;
  transform: translateY(-50%) !important;
  width: 0 !important;
  height: 0 !important;
  border-style: solid !important;
  border-width: 0.35rem 0.35rem 0.35rem 0 !important;
  border-color: transparent rgb(var(--color-primary) / 0.15) transparent transparent !important;
}

.server-card-item:hover .server-card-type-badge {
  background: rgb(var(--color-primary) / 0.25) !important;
  border-color: rgb(var(--color-primary) / 0.5) !important;
  border-left-color: rgb(var(--color-primary) / 0.7) !important;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.15) !important;
  transform: translateY(-1px) !important;
}

.server-card-item:hover .server-card-type-badge::before {
  border-color: transparent rgb(var(--color-primary) / 0.5) transparent transparent !important;
}

.server-card-item:hover .server-card-type-badge::after {
  border-color: transparent rgb(var(--color-primary) / 0.25) transparent transparent !important;
}

.server-card-address-text {
  display: block !important;
  font-size: 0.75rem !important;
  color: rgb(var(--color-muted-foreground)) !important;
  text-align: right !important;
  overflow: hidden !important;
  text-overflow: ellipsis !important;
  white-space: nowrap !important;
  max-width: 100% !important;
}

/* Botón agregar nuevo */
.server-card-add-wrapper {
  border-style: dashed !important;
  background: rgb(var(--color-primary) / 0.05) !important;
  border-color: rgb(var(--color-primary) / 0.3) !important;
  align-items: center !important;
  justify-content: center !important;
}

.server-card-item:hover .server-card-add-wrapper {
  background: rgb(var(--color-primary) / 0.1) !important;
  border-color: rgb(var(--color-primary) / 0.5) !important;
}

.server-card-add-top {
  justify-content: center !important;
  margin-bottom: 0 !important;
}

.server-card-add-icon {
  width: 2rem !important;
  height: 2rem !important;
  color: rgb(var(--color-primary)) !important;
  transition: transform 0.2s ease-in-out !important;
}

.server-card-item:hover .server-card-add-icon {
  transform: scale(1.15) !important;
}

.server-card-add-badge {
  background: rgb(var(--color-primary) / 0.15) !important;
  color: rgb(var(--color-primary)) !important;
  width: 100% !important;
  text-align: center !important;
  text-transform: none !important;
  font-size: 0.875rem !important;
}

/* Loader en el grid */
.server-card-loader {
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
}

.server-card-loader .server-card-wrapper {
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
  min-height: 140px !important;
}

/* Asegurar que NO haya estilos de lista */
.server-grid-wrapper *,
.server-grid-4cols *,
.server-card-item * {
  list-style: none !important;
  padding-left: 0 !important;
}

.server-grid-wrapper ul,
.server-grid-4cols ul {
  list-style: none !important;
  padding: 0 !important;
  margin: 0 !important;
}

.server-grid-wrapper li,
.server-grid-4cols li {
  list-style: none !important;
  padding: 0 !important;
  margin: 0 !important;
}
</style>
