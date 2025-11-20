<script setup>
import { ref, inject } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { List, ListItem, ListItemContent, ListItemIcon } from '@/components/ui/list'

const props = defineProps({
  right: { type: Boolean, default: () => false },
  closed: { type: Boolean, default: () => false }
})

const api = inject('api')
const events = inject('events')
const { t } = useI18n()
const router = useRouter()
const routes = router.getRoutes().filter(e => {
  if (e.meta.permission === true) return true
  return e.meta.permission && api.auth.isLoggedIn() && api.auth.hasScope(e.meta.permission)
})

const mini = ref(localStorage.getItem('sidebar.mini') === 'true')

async function logout() {
  await api.auth.logout()
  router.push({ name: 'Login' })
  events.emit('logout')
}

function toggleMini() {
  mini.value = !mini.value
  localStorage.setItem('sidebar.mini', mini.value ? 'true' : 'false')
}
</script>

<template>
  <nav 
    :class="[
      'fixed top-16 z-[5]',
      'flex flex-col',
      'bg-sidebar/95 backdrop-blur-md text-sidebar-foreground',
      'border-r border-border/50',
      'shadow-lg lg:shadow-xl',
      'transition-all duration-300 ease-in-out',
      'text-xl font-bold',
      'scrollbar-thin scrollbar-thumb-border/50 scrollbar-track-transparent',
      props.right ? 'right-0 border-l border-r-0' : 'left-0',
      closed 
        ? 'w-0 overflow-hidden' 
        : mini 
          ? 'w-[52px] lg:w-[52px]' 
          : 'w-64',
      'h-[calc(var(--inner-height)-env(safe-area-inset-bottom)-4rem)]',
      'group'
    ]"
  >
    <!-- Contenido superior (navegación) -->
    <div 
      tabindex="-1" 
      :class="[
        'overflow-x-hidden overflow-y-auto',
        'mb-auto'
      ]"
    >
      <list>
        <list-item
          v-for="route in routes"
          :key="route.name"
          v-hotkey="route.meta.hotkey"
          :to="route"
        >
          <list-item-icon v-if="route.meta.icon" :icon="route.meta.icon" />
          <list-item-content 
            :class="[
              'transition-opacity duration-100',
              mini && 'lg:opacity-0 lg:group-hover:opacity-100'
            ]"
            v-text="t(route.meta.tkey ? route.meta.tkey : 'common.navigation.' + route.name)" 
          />
        </list-item>
      </list>
    </div>
    
    <!-- Contenido inferior (acciones) -->
    <div 
      tabindex="-1" 
      :class="[
        'overflow-x-hidden overflow-y-auto'
      ]"
    >
      <list>
        <list-item 
          tabindex="0" 
          class="cursor-pointer hover:bg-accent/50 transition-colors"
          @click="toggleMini()"
        >
          <list-item-icon :icon="mini ? 'chevron-right' : 'chevron-left'" />
          <list-item-content 
            :class="[
              'transition-opacity duration-100',
              mini && 'lg:opacity-0 lg:group-hover:opacity-100'
            ]"
            v-text="t('common.' + (mini ? 'Expand' : 'Collapse'))" 
          />
        </list-item>
        <list-item 
          tabindex="0" 
          class="cursor-pointer hover:bg-accent/50 transition-colors"
          @click="logout()"
        >
          <list-item-icon icon="logout" />
          <list-item-content 
            :class="[
              'transition-opacity duration-100',
              mini && 'lg:opacity-0 lg:group-hover:opacity-100'
            ]"
            v-text="t('users.Logout')" 
          />
        </list-item>
      </list>
    </div>
  </nav>
</template>
