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
      :class="['list']"
      @hotkey="focusList()"
    >
      <div 
        v-for="server in servers" 
        :key="server.id" 
        :class="[
          'list-item',
          server.online === true ? 'border-success/50 bg-success/5 hover:border-success/70 hover:bg-success/10' : '',
          server.online === false ? 'border-error/50 bg-error/5 hover:border-error/70 hover:bg-error/10' : '',
          server.online === 'loading' ? 'border-warning/50 bg-warning/5 hover:border-warning/70 hover:bg-warning/10' : ''
        ]"
      >
        <router-link 
          :ref="setFirstEntry" 
          :to="{ name: 'ServerView', params: { id: server.id } }"
          :class="['block']"
        >
          <div
            :class="[
              'server',
              `server-${(server.icon || 'none')}`,
              'w-full'
            ]"
            :data-online="server.online"
          >
            <span 
              :class="[
                'title',
                'block text-lg font-semibold text-foreground'
              ]"
              :title="server.name"
            >
              {{server.name}}
            </span>
            <div :class="['flex items-center gap-2 mt-1']">
              <span 
                :class="[
                  'type',
                  'inline-block px-2.5 py-1 text-xs font-semibold rounded-md',
                  'bg-primary/10 text-primary'
                ]"
              >
                {{server.type}}
              </span>
              <span 
                :class="[
                  'subline',
                  'block text-sm text-muted-foreground'
                ]"
              >
                {{getServerAddress(server)}} @ {{server.node.name}}
              </span>
            </div>
          </div>
        </router-link>
      </div>
      <div 
        v-if="!allServersLoaded" 
        :class="['list-item']"
      >
        <loader ref="loaderRef" small />
      </div>
      <div 
        v-if="$api.auth.hasScope('server.create')" 
        :class="['list-item']"
      >
        <router-link 
          v-hotkey="'c'" 
          :to="{ name: 'ServerCreate' }"
          :class="['block']"
        >
          <div 
            :class="[
              'createLink',
              'flex items-center gap-2 px-4 py-3',
              'bg-primary/10 border-2 border-primary/30 rounded-xl',
              'text-primary font-semibold',
              'hover:bg-primary/20 hover:border-primary/50',
              'transition-all duration-200',
              'shadow-sm hover:shadow-md',
              'cursor-pointer'
            ]"
          >
            <icon name="plus" />
            <span>{{ t('servers.Add') }}</span>
          </div>
        </router-link>
      </div>
    </div>
  </div>
</template>
