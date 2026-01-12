<script setup>
import { ref, computed, inject } from 'vue'
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

  // Filtrar rutas para el sidebar normal
  const routes = computed(() => {
    // Rutas que NO deben aparecer en el sidebar de cliente (solo en admin)
    const excludedRoutes = ['Settings', 'TemplateList', 'UserList', 'NodeList']
    
    // Filtrar rutas normales
    let filteredRoutes = router.getRoutes().filter(e => {
      // No mostrar rutas de admin (se agregarán manualmente si el usuario es admin)
      if (e.path.startsWith('/admin')) return false
      
      // Excluir rutas administrativas (Settings, Templates, Users, Nodes)
      if (excludedRoutes.includes(e.name)) return false
      
      // Solo mostrar rutas con meta definida
      if (!e.meta) return false
      
      // Si permission es true, mostrar para cualquier usuario autenticado
      if (e.meta.permission === true) {
        return api.auth.isLoggedIn()
      }
      
      // Si no tiene permission definido, no mostrar (rutas sin meta.permission no son del sidebar)
      if (!e.meta.permission) return false
      
      // Para otros permisos, verificar si el usuario lo tiene
      const hasPermission = api.auth.isLoggedIn() && api.auth.hasScope(e.meta.permission)
      
      // Debug para uptime
      if (e.name === 'Uptime') {
        console.log('[SIDEBAR] Uptime route check:', {
          name: e.name,
          permission: e.meta.permission,
          isLoggedIn: api.auth.isLoggedIn(),
          hasScope: api.auth.hasScope(e.meta.permission),
          hasPermission
        })
      }
      
      return hasPermission
    })

  // Agregar ruta de Admin al final si el usuario tiene permisos de admin
  if (api.auth.isLoggedIn() && api.auth.hasScope('admin')) {
    // Buscar la ruta de admin (puede ser la ruta padre o el child)
    const adminRouteParent = router.getRoutes().find(e => e.path === '/admin')
    const adminRouteChild = adminRouteParent?.children?.find(c => c.name === 'Admin')
    
    if (adminRouteChild && adminRouteChild.meta) {
      // Crear una entrada de ruta para Admin usando el child
      filteredRoutes = [...filteredRoutes, {
        ...adminRouteChild,
        path: '/admin' // Asegurar que el path sea /admin
      }]
    } else {
      // Si no se encuentra el child, crear una entrada manual
      filteredRoutes = [...filteredRoutes, {
        name: 'Admin',
        path: '/admin',
        meta: {
          tkey: 'admin.Admin',
          permission: 'admin',
          icon: 'hi-shield-check',
          hotkey: 'g a'
        }
      }]
    }
  }

  return filteredRoutes
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
  const currentRouteName = route.name
  
  // Si la ruta actual coincide exactamente, está activa
  if (currentRouteName === routeName) {
    return true
  }
  
  // Si estamos en una ruta de admin (Admin.XXX), solo marcar Admin como activa
  if (currentRouteName?.startsWith('Admin.')) {
    // Solo marcar Admin como activa cuando estamos en rutas hijas de Admin
    return routeName === 'Admin'
  }
  
  // Si la ruta actual es una ruta hija de la ruta especificada (ej: Admin.NodeList es hija de Admin)
  if (currentRouteName?.startsWith(routeName + '.')) {
    return true
  }
  
  return false
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
      'fixed top-12 z-[5]',
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
      'h-[calc(var(--inner-height)-env(safe-area-inset-bottom)-3rem)]',
      'group',
      'will-change-[width]'
    ]"
    :style="{
      transition: 'width 0.3s cubic-bezier(0.4, 0, 0.2, 1)'
    }"
  >
    <!-- Contenido superior (navegación) -->
    <div 
      :class="[
        'flex-1 overflow-y-auto overflow-x-hidden',
        'py-4',
        'space-y-1',
        'transition-all duration-300 ease-in-out',
        mini ? 'px-2' : 'px-2'
      ]"
    >
      <router-link
        v-for="routeItem in routes"
        :key="routeItem.name"
        v-hotkey="routeItem.meta?.hotkey"
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
            'transition-all duration-300 ease-in-out',
            'flex-1',
            'overflow-hidden'
          ]"
          style="opacity: 1; max-width: 100%;"
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
        'my-2 h-px',
        'bg-border/30',
        mini ? 'mx-2 w-12' : 'mx-2 w-full',
        'transition-all duration-300 ease-in-out'
      ]"
    />
    
    <!-- Contenido inferior (acciones) -->
    <div 
      :class="[
        'py-2',
        'px-2',
        'space-y-1',
        'border-t border-border/20',
        'transition-all duration-300 ease-in-out'
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
            'transition-all duration-300 ease-in-out',
            'overflow-hidden'
          ]"
          style="opacity: 1; max-width: 100%;"
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
          'focus:outline-none focus:ring-2 focus:ring-error/30 focus:ring-offset-2 focus:ring-offset-sidebar',
          'group/item'
        ]"
        :title="mini ? t('users.Logout') : undefined"
        @click="logout()"
      >
        <!-- Icono de logout - SVG inline para garantizar que siempre se muestre -->
        <svg 
          xmlns="http://www.w3.org/2000/svg" 
          fill="none" 
          viewBox="0 0 24 24" 
          stroke-width="1.5" 
          stroke="currentColor"
          :class="[
            'flex-shrink-0',
            'w-6 h-6',
            'transition-colors duration-200',
            'text-sidebar-foreground group-hover/item:text-error'
          ]"
        >
          <path 
            stroke-linecap="round" 
            stroke-linejoin="round" 
            d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15M12 9l-3 3m0 0l3 3m-3-3h12.75" 
        />
        </svg>
        <span 
          v-if="!mini"
          :class="[
            'font-semibold text-sm',
            'whitespace-nowrap',
            'transition-all duration-300 ease-in-out',
            'overflow-hidden'
          ]"
          style="opacity: 1; max-width: 100%;"
        >
          {{ t('users.Logout') }}
        </span>
      </button>
    </div>
  </nav>
</template>
