<script setup>
import { ref, inject } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/ui/Icon.vue'

const props = defineProps({
  right: { type: Boolean, default: () => false },
  closed: { type: Boolean, default: () => false }
})

const api = inject('api')
const events = inject('events')
const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const routes = router.getRoutes().filter(e => {
  // Solo mostrar rutas con meta definida
  if (!e.meta) return false
  
  // Si permission es true, mostrar para cualquier usuario autenticado
  if (e.meta.permission === true) {
    return api.auth.isLoggedIn()
  }
  
  // Si no tiene permission definido, no mostrar (rutas sin meta.permission no son del sidebar)
  if (!e.meta.permission) return false
  
  // Para otros permisos, verificar si el usuario lo tiene
  return api.auth.isLoggedIn() && api.auth.hasScope(e.meta.permission)
})

const mini = ref(localStorage.getItem('sidebar.mini') === 'true')

// Exponer el estado mini para que App.vue pueda acceder
defineExpose({
  mini
})

async function logout() {
  await api.auth.logout()
  router.push({ name: 'Login' })
  events.emit('logout')
}

function toggleMini() {
  mini.value = !mini.value
  localStorage.setItem('sidebar.mini', mini.value ? 'true' : 'false')
  // Emitir evento para notificar cambios
  events.emit('sidebar:toggle-mini', mini.value)
}

function isActiveRoute(routeName) {
  return route.name === routeName || route.name?.startsWith(routeName + '.')
}

function getRouteLabel(routeMeta) {
  if (routeMeta.tkey) {
    return t(routeMeta.tkey)
  }
  return t('common.navigation.' + routeMeta.name)
}
</script>

<template>
  <nav 
    :class="[
      'fixed top-16 z-[5]',
      'flex flex-col',
      'bg-sidebar/98 backdrop-blur-md text-sidebar-foreground',
      'border-r-2 border-border/30',
      'shadow-xl',
      'transition-all duration-300 ease-in-out',
      'scrollbar-thin scrollbar-thumb-border/50 scrollbar-track-transparent',
      props.right ? 'right-0 border-l-2 border-r-0' : 'left-0',
      closed 
        ? 'w-0 overflow-hidden' 
        : mini 
          ? 'w-16' 
          : 'w-64',
      'h-[calc(var(--inner-height)-env(safe-area-inset-bottom)-4rem)]',
      'group'
    ]"
  >
    <!-- Contenido superior (navegación) -->
    <div 
      :class="[
        'flex-1 overflow-y-auto overflow-x-hidden',
        'px-2 py-4',
        'space-y-1'
      ]"
    >
      <router-link
        v-for="routeItem in routes"
        :key="routeItem.name"
        v-hotkey="routeItem.meta.hotkey"
        :to="routeItem"
        :class="[
          'flex items-center gap-3',
          'px-3 py-2.5',
          'rounded-lg',
          'transition-all duration-200',
          'group/item',
          mini ? 'justify-center' : '',
          isActiveRoute(routeItem.name)
            ? 'bg-primary/20 text-primary border-2 border-primary/30 shadow-md'
            : 'text-sidebar-foreground hover:bg-primary/10 hover:text-primary border-2 border-transparent hover:border-primary/20'
        ]"
        :title="mini ? getRouteLabel(routeItem.meta) : undefined"
      >
        <!-- Icono -->
        <icon 
          v-if="routeItem.meta && routeItem.meta.icon" 
          :name="routeItem.meta.icon"
          :class="[
            'flex-shrink-0',
            'text-xl',
            'w-6 h-6',
            'transition-all duration-200',
            isActiveRoute(routeItem.name) 
              ? 'text-primary' 
              : 'text-sidebar-foreground group-hover/item:text-primary'
          ]"
        />
        
        <!-- Texto (solo visible cuando no está mini) -->
        <span 
          v-if="!mini"
          :class="[
            'font-semibold text-sm',
            'whitespace-nowrap',
            'transition-opacity duration-300',
            'flex-1'
          ]"
        >
          {{ getRouteLabel(routeItem.meta) }}
        </span>
        
        <!-- Indicador de ruta activa (solo cuando no está mini) -->
        <div
          v-if="!mini && isActiveRoute(routeItem.name)"
          :class="[
            'w-1.5 h-1.5 rounded-full',
            'bg-primary',
            'ml-auto'
          ]"
        />
      </router-link>
    </div>
    
    <!-- Separador -->
    <div 
      :class="[
        'mx-2 my-2 h-px',
        'bg-border/30',
        mini ? 'w-12' : 'w-full',
        'transition-all duration-300'
      ]"
    />
    
    <!-- Contenido inferior (acciones) -->
    <div 
      :class="[
        'px-2 py-2',
        'space-y-1',
        'border-t border-border/20'
      ]"
    >
      <!-- Botón para colapsar/expandir -->
      <button
        tabindex="0"
        :class="[
          'w-full flex items-center gap-3',
          'px-3 py-2.5',
          'rounded-lg',
          'text-sidebar-foreground',
          'transition-all duration-200',
          'hover:bg-primary/10 hover:text-primary',
          'border-2 border-transparent hover:border-primary/20',
          mini ? 'justify-center' : '',
          'focus:outline-none focus:ring-2 focus:ring-primary/30 focus:ring-offset-2 focus:ring-offset-sidebar'
        ]"
        :title="mini ? t('common.' + (mini ? 'Expand' : 'Collapse')) : undefined"
        @click="toggleMini()"
      >
        <icon 
          :name="mini ? 'chevron-right' : 'chevron-left'"
          :class="[
            'flex-shrink-0',
            'text-lg',
            'transition-transform duration-300',
            mini ? '' : ''
          ]"
        />
        <span 
          v-if="!mini"
          :class="[
            'font-semibold text-sm',
            'whitespace-nowrap',
            'transition-opacity duration-300'
          ]"
        >
          {{ t('common.' + (mini ? 'Expand' : 'Collapse')) }}
        </span>
      </button>
      
      <!-- Botón de logout -->
      <button
        tabindex="0"
        :class="[
          'w-full flex items-center gap-3',
          'px-3 py-2.5',
          'rounded-lg',
          'text-sidebar-foreground',
          'transition-all duration-200',
          'hover:bg-error/10 hover:text-error',
          'border-2 border-transparent hover:border-error/20',
          mini ? 'justify-center' : '',
          'focus:outline-none focus:ring-2 focus:ring-error/30 focus:ring-offset-2 focus:ring-offset-sidebar'
        ]"
        :title="mini ? t('users.Logout') : undefined"
        @click="logout()"
      >
        <icon 
          name="logout"
          :class="[
            'flex-shrink-0',
            'text-lg',
            'transition-colors duration-200',
            'group-hover/item:text-error'
          ]"
        />
        <span 
          v-if="!mini"
          :class="[
            'font-semibold text-sm',
            'whitespace-nowrap',
            'transition-opacity duration-300'
          ]"
        >
          {{ t('users.Logout') }}
        </span>
      </button>
    </div>
  </nav>
</template>
