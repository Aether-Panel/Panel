<script setup>
import { ref, inject, nextTick } from 'vue'
import { useRouter, RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n'
import Loader from './Loader.vue'
import TextField from './TextField.vue'

const api = inject('api')
const router = useRouter()
const { t } = useI18n()

let timeout = null

const active = ref(false)
const loading = ref(false)
const query = ref('')
const maxIndex = ref(0)
const currIndex = ref(0)
const currRef = ref(null)
const containerRef = ref(null)
const links = ref([])

const servers = ref([])
const templates = ref([])
const users = ref([])
const nodes = ref([])

function cancel() {
  if (timeout) {
    clearTimeout(timeout)
    timeout = null
  }
  setTimeout(() => {
    active.value = false
  }, 500)
}

function close() {
  active.value = false
  query.value = ''
  document.activeElement.blur()
}

function input() {
  if (timeout) {
    clearTimeout(timeout)
    timeout = null
  }
  timeout = setTimeout(search, 500)
}

async function search() {
  loading.value = true
  active.value = true
  const q = query.value.toLowerCase()
  reset()
  await Promise.all([
    findServers(q),
    api.auth.hasScope('users.info.search') ? findUsers(q) : Promise.resolve(),
    api.auth.hasScope('nodes.view') ? findNodes(q) : Promise.resolve(),
    api.auth.hasScope('templates.view') ? findTemplates(q) : Promise.resolve()
  ])
  let mi = servers.value.length + users.value.length + nodes.value.length
  templates.value.map(repo => mi += repo.templates.length)
  maxIndex.value = mi - 1
  loading.value = false
}

async function findServers(query) {
  servers.value = (await api.server.list(1, 5, query)).servers
}

async function findTemplates(query) {
  templates.value = (await api.template.listAllTemplates()).map(repo => {
    repo.templates = repo.templates.filter(template => {
      if (template.name.toLowerCase().indexOf(query) > -1) return true
      return template.display.toLowerCase().indexOf(query) > -1
    }).slice(0, 5)
    return repo
  }).filter(repo => {
    return repo.templates.length > 0
  })
}

async function findUsers(query) {
  const byName = await api.user.search(query, 5)
  const byEmail = (await api.user.searchEmail(query, 5)).filter(u => {
    return byName.filter(n => {
      return n.id === u.id
    }).length === 0
  })
  users.value = byName.concat(byEmail).slice(0, 5)
}

async function findNodes(query) {
  nodes.value = (await api.node.list()).filter(node => node.name.toLowerCase().indexOf(query) > -1).slice(0, 5)
}

function reset() {
  maxIndex.value = 0
  currIndex.value = 0
  currRef.value = null
  containerRef.value = null
  links.value = []
  servers.value = []
  templates.value = []
  users.value = []
  nodes.value = []
}

function getServerAddress(server) {
  let ip = server.node.publicHost
  if (server.ip && server.ip !== '0.0.0.0') {
    ip = server.ip
  }
  return ip + (server.port ? ':' + server.port : '')
}

function serverIndex(i) {
  return i
}

function userIndex(i) {
  return servers.value.length + i
}

function nodeIndex(i) {
  return servers.value.length + users.value.length + i
}

function templateIndex(repo, i) {
  let offset = servers.value.length + users.value.length + nodes.value.length
  let foundRepo = false
  templates.value.map(r => {
    if (repo.id === r.id) foundRepo = true
    if (!foundRepo) offset += r.templates.length
  })
  return offset + i
}

function setRef(i) {
  return ref => {
    if (currIndex.value === i)
      currRef.value = ref
  }
}

function scrollIfNeeded(complete) {
  nextTick(() => {
    const container = containerRef.value.getBoundingClientRect()
    const curr = currRef.value.getBoundingClientRect()
    if (container.top > curr.top) {
      complete ? containerRef.value.scrollTo({ behavior: 'smooth', top: 0 }) : containerRef.value.scrollBy({ behavior: 'smooth', top: curr.top - container.top })
    }
    if (container.bottom < curr.bottom) {
      complete ? containerRef.value.scrollTo({ behavior: 'smooth', top: 9999 }) : containerRef.value.scrollBy({ behavior: 'smooth', top: curr.bottom - container.bottom })
    }
  })
}

function up() {
  if (currIndex.value === 0) {
    currIndex.value = maxIndex.value
    scrollIfNeeded(true)
  } else {
    currIndex.value = currIndex.value - 1
    scrollIfNeeded(false)
  }
}

function down() {
  if (currIndex.value === maxIndex.value) {
    currIndex.value = 0
    scrollIfNeeded(true)
  } else {
    currIndex.value = currIndex.value + 1
    scrollIfNeeded(false)
  }
}

function go() {
  router.push(links[currIndex.value])
  close()
}

function link(index, to) {
  links[index] = to
  return to
}
</script>

<template>
  <span 
    :class="[
      'global-search',
      'relative inline-block',
      'w-full max-w-[180px] lg:max-w-[240px]'
    ]"
  >
    <div class="mb-0 [&>div]:mb-0 [&_input]:py-2 [&_input]:text-sm [&_input]:px-3 [&_input]:pl-8 [&>div>div]:rounded-lg [&>div>div]:h-9 [&_svg]:w-4 [&_svg]:h-4">
      <text-field 
        v-model="query" 
        v-hotkey="'/'" 
        icon="search" 
        @change="input()" 
        @blur="cancel()" 
        @keyup.up="up()" 
        @keyup.down="down()" 
        @keyup.enter="go()" 
        @keyup.esc="close()" 
      />
    </div>
    
    <!-- Resultados cargando -->
    <div 
      v-if="active && loading" 
      :class="[
        'results',
        'absolute top-full left-0 right-0 mt-2',
        'bg-background border-2 border-border/50 rounded-xl shadow-xl',
        'p-4 z-50',
        'max-h-[60vh] overflow-y-auto',
        'backdrop-blur-sm',
        'animate-in fade-in zoom-in-95 duration-200',
        'scrollbar-thin scrollbar-thumb-border scrollbar-track-transparent'
      ]"
    >
      <loader />
    </div>
    
    <!-- Resultados -->
    <div 
      v-if="active && !loading" 
      ref="containerRef" 
      :class="[
        'results',
        'absolute top-full left-0 right-0 mt-2',
        'bg-background/95 backdrop-blur-md border-2 border-border/50 rounded-xl shadow-2xl',
        'p-4 z-50',
        'max-h-[60vh] overflow-y-auto',
        'animate-in fade-in zoom-in-95 duration-200',
        'scrollbar-thin scrollbar-thumb-border scrollbar-track-transparent'
      ]"
    >
      <!-- Servidores -->
      <div v-if="servers.length > 0" :class="['server-results', 'mb-4']">
        <h3 
          :class="[
            'text-lg font-semibold mb-2 text-foreground'
          ]"
          v-text="t('servers.Servers')" 
        />
        <div 
          v-for="(server, i) in servers" 
          :key="server.id" 
          :ref="setRef(serverIndex(i))" 
          :class="[
            'result',
            'rounded-xl transition-all duration-200',
            'border border-transparent',
            currIndex === serverIndex(i) 
              ? 'selected bg-primary/15 border-primary/30 shadow-sm' 
              : 'hover:bg-primary/10 hover:border-primary/20 hover:shadow-sm'
          ]"
        >
          <router-link 
            :to="link(serverIndex(i), { name: 'ServerView', params: { id: server.id } })"
            :class="['block p-3']"
          >
            <div :class="['server', `server-${(server.icon || 'none')}`]">
              <div 
                :class="[
                  'title',
                  'font-semibold text-foreground truncate'
                ]"
              >
                {{ server.name }}
              </div>
              <div 
                :class="[
                  'subline',
                  'text-sm text-muted-foreground truncate'
                ]"
              >
                {{ getServerAddress(server) }} @ {{ server.node.name }}
              </div>
            </div>
          </router-link>
        </div>
      </div>
      
      <!-- Usuarios -->
      <div v-if="users.length > 0" :class="['user-results', 'mb-4']">
        <h3 
          :class="[
            'text-lg font-semibold mb-2 text-foreground'
          ]"
          v-text="t('users.Users')" 
        />
        <div 
          v-for="(user, i) in users" 
          :key="user.id" 
          :ref="setRef(userIndex(i))" 
          :class="[
            'result',
            'rounded-xl transition-all duration-200',
            'border border-transparent',
            currIndex === userIndex(i) 
              ? 'selected bg-primary/15 border-primary/30 shadow-sm' 
              : 'hover:bg-primary/10 hover:border-primary/20 hover:shadow-sm'
          ]"
        >
          <router-link 
            :to="link(userIndex(i), { name: 'UserView', params: { id: user.id } })"
            :class="['block p-3']"
          >
            <div 
              :class="[
                'title',
                'font-semibold text-foreground truncate'
              ]"
            >
              {{ user.username }}
            </div>
            <div 
              :class="[
                'subline',
                'text-sm text-muted-foreground truncate'
              ]"
            >
              {{ user.email }}
            </div>
          </router-link>
        </div>
      </div>
      
      <!-- Nodos -->
      <div v-if="nodes.length > 0" :class="['node-results', 'mb-4']">
        <h3 
          :class="[
            'text-lg font-semibold mb-2 text-foreground'
          ]"
          v-text="t('nodes.Nodes')" 
        />
        <div 
          v-for="(node, i) in nodes" 
          :key="node.id" 
          :ref="setRef(nodeIndex(i))" 
          :class="[
            'result',
            'rounded-xl transition-all duration-200',
            'border border-transparent',
            currIndex === nodeIndex(i) 
              ? 'selected bg-primary/15 border-primary/30 shadow-sm' 
              : 'hover:bg-primary/10 hover:border-primary/20 hover:shadow-sm'
          ]"
        >
          <router-link 
            :to="link(nodeIndex(i), { name: 'NodeView', params: { id: node.id } })"
            :class="['block p-3']"
          >
            <div 
              :class="[
                'title',
                'font-semibold text-foreground truncate'
              ]"
            >
              {{ node.name }}
            </div>
            <div 
              :class="[
                'subline',
                'text-sm text-muted-foreground truncate'
              ]"
            >
              {{ node.publicHost + ':' + node.publicPort }}
            </div>
          </router-link>
        </div>
      </div>
      
      <!-- Plantillas -->
      <div v-if="templates.length > 0" :class="['template-results', 'mb-4']">
        <h3 
          :class="[
            'text-lg font-semibold mb-2 text-foreground'
          ]"
          v-text="t('templates.Templates')" 
        />
        <div v-for="repo in templates" :key="repo.id" :class="['mb-3']">
          <h4 
            :class="[
              'text-base font-semibold mb-2 text-foreground'
            ]"
            v-text="repo.name" 
          />
          <div 
            v-for="(template, i) in repo.templates" 
            :key="template.name" 
            :ref="setRef(templateIndex(repo, i))" 
            :class="[
              'result',
              'rounded-xl transition-all duration-200',
              'border border-transparent',
              currIndex === templateIndex(repo, i) 
                ? 'selected bg-primary/15 border-primary/30 shadow-sm' 
                : 'hover:bg-primary/10 hover:border-primary/20 hover:shadow-sm'
            ]"
          >
            <router-link 
              :to="link(templateIndex(repo, i), { name: 'TemplateView', params: { repo: repo.id, id: template.name } })"
              :class="['block p-3']"
            >
              <div 
                :class="[
                  'title',
                  'font-semibold text-foreground truncate'
                ]"
              >
                {{ template.display }}
              </div>
              <div 
                :class="[
                  'subline',
                  'text-sm text-muted-foreground truncate'
                ]"
              >
                {{ template.name }}
              </div>
            </router-link>
          </div>
        </div>
      </div>
      
      <!-- Sin resultados -->
      <div 
        v-if="maxIndex === -1" 
        :class="[
          'no-results',
          'p-4 text-center text-muted-foreground'
        ]"
        v-text="t('common.NoResults')" 
      />
    </div>
  </span>
</template>
