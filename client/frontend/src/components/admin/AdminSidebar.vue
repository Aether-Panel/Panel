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

const mini = ref(localStorage.getItem('admin-sidebar.mini') === 'true')

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
  localStorage.setItem('admin-sidebar.mini', mini.value ? 'true' : 'false')
  events.emit('admin-sidebar:toggle-mini', mini.value)
}

function isActiveRoute(routeName) {
  const currentRouteName = route.name
  
  // Si la ruta actual coincide exactamente, está activa
  if (currentRouteName === routeName) {
    return true
  }
  
  // Si estamos en una ruta hija de Admin (Admin.XXX), solo marcar Admin como activa
  if (currentRouteName?.startsWith('Admin.')) {
    // Solo marcar Admin como activa cuando estamos en rutas hijas de Admin
    return routeName === 'Admin'
  }
  
  // Si la ruta actual es una ruta hija de la ruta especificada
  if (currentRouteName?.startsWith(routeName + '.')) {
    return true
  }
  
  return false
}

// Rutas administrativas - Dashboard, Settings, Plantillas, Usuarios, Nodos y Roles
const adminRoutes = [
  {
    name: 'Admin',
    path: '/admin',
    label: t('admin.Dashboard'),
    icon: 'hi-chart-pie',
    permission: true
  },
  {
    name: 'Admin.ServerList',
    path: '/admin/servers',
    label: t('servers.Servers'),
    icon: 'hi-server',
    permission: true
  },
  {
    name: 'Admin.Settings',
    path: '/admin/settings',
    label: t('settings.Settings'),
    icon: 'hi-cog',
    permission: true
  },
  {
    name: 'Admin.TemplateList',
    path: '/admin/templates',
    label: t('templates.Templates'),
    icon: 'hi-document',
    permission: 'templates.view'
  },
  {
    name: 'Admin.UserList',
    path: '/admin/users',
    label: t('users.Users'),
    icon: 'hi-users',
    permission: 'users.info.view'
  },
  {
    name: 'Admin.NodeList',
    path: '/admin/nodes',
    label: t('nodes.Nodes'),
    icon: 'hi-server',
    permission: 'nodes.view'
  },
  {
    name: 'Admin.RoleList',
    path: '/admin/roles',
    label: t('roles.Roles'),
    icon: 'gi-3d-hammer',
    permission: 'admin'
  }
].filter(route => {
  if (route.permission === true) {
    return api.auth.isLoggedIn()
  }
  return api.auth.isLoggedIn() && api.auth.hasScope(route.permission)
})
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
        v-for="routeItem in adminRoutes"
        :key="routeItem.name"
        :to="routeItem.path"
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
        :title="mini ? routeItem.label : undefined"
      >
        <!-- Icono -->
        <icon 
          :name="routeItem.icon"
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
          {{ routeItem.label }}
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
      <!-- Botón para volver al panel normal -->
      <router-link
        :to="{ name: 'ServerList' }"
        :class="[
          'w-full flex items-center gap-3',
          'px-3 py-2.5',
          'rounded-lg',
          'text-sidebar-foreground',
          'transition-all duration-200',
          'hover:bg-primary/10 hover:text-primary',
          'border-2 border-transparent hover:border-primary/20',
          mini ? 'justify-center' : '',
          'focus:outline-none focus:ring-2 focus:ring-primary/30 focus:ring-offset-2 focus:ring-offset-sidebar',
          'group/item'
        ]"
        :title="mini ? t('admin.BackToPanel') || 'Volver al panel' : undefined"
      >
        <icon 
          name="hi-arrow-left"
          :class="[
            'flex-shrink-0',
            'text-lg',
            'transition-colors duration-200',
            'text-sidebar-foreground group-hover/item:text-primary'
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
          {{ t('admin.BackToPanel') || 'Volver al panel' }}
        </span>
      </router-link>

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

