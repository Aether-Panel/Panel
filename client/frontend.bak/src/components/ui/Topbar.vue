<script setup>
import { inject, computed, ref, onMounted, onUnmounted, watch } from 'vue'
import { RouterLink } from 'vue-router'
import md5 from 'js-md5'
import Icon from './Icon.vue'
import PanelSearch from './PanelSearch.vue'

const props = defineProps({
  user: { type: Object, default: () => undefined }
})

defineEmits(['toggleSidebar'])

const config = inject('config')
const name = computed(() => config.branding?.name || 'SkyPanel')

// Debug: ver cambios en el nombre
watch(() => config.branding?.name, (newName, oldName) => {
  console.log('🏷️ [TOPBAR] Nombre actualizado:', { oldName, newName })
}, { immediate: true })

const isVisible = ref(true)
let lastScrollY = 0
let scrollTimeout = null

function handleScroll() {
  const currentScrollY = window.scrollY || window.pageYOffset
  
  // Si estamos en la parte superior, siempre mostrar
  if (currentScrollY < 10) {
    isVisible.value = true
    lastScrollY = currentScrollY
    return
  }
  
  // Si el scroll es muy pequeño, no hacer nada (evitar parpadeos)
  if (Math.abs(currentScrollY - lastScrollY) < 5) {
    return
  }
  
  // Ocultar al hacer scroll hacia abajo, mostrar al hacer scroll hacia arriba
  if (currentScrollY > lastScrollY && currentScrollY > 48) {
    // Scroll hacia abajo - ocultar
    isVisible.value = false
  } else if (currentScrollY < lastScrollY) {
    // Scroll hacia arriba - mostrar
    isVisible.value = true
  }
  
  lastScrollY = currentScrollY
  
  // Mostrar automáticamente después de un tiempo sin scroll
  clearTimeout(scrollTimeout)
  scrollTimeout = setTimeout(() => {
    if (currentScrollY > 48) {
      isVisible.value = true
    }
  }, 2000)
}

onMounted(() => {
  window.addEventListener('scroll', handleScroll, { passive: true })
})

onUnmounted(() => {
  window.removeEventListener('scroll', handleScroll)
  if (scrollTimeout) clearTimeout(scrollTimeout)
})

function getAvatarLink() {
  return 'https://www.gravatar.com/avatar/' + md5(props.user.email.trim().toLowerCase()) + '?d=mp'
}
</script>

<template>
  <header 
    :class="[
      'fixed top-0 left-0 right-0 h-12 z-40',
      'flex items-center justify-between flex-row px-4 lg:px-6',
      'text-primary-foreground',
      'border-b border-primary-foreground/10',
      'shadow-lg',
      'mode-dark-highcontrast:bg-background mode-dark-highcontrast:border-b-[3px] mode-dark-highcontrast:border-foreground mode-dark-highcontrast:shadow-none',
      'mode-dark-highcontrast:text-foreground'
    ]"
    :style="{
      transform: isVisible ? 'translateY(0)' : 'translateY(-100%)',
      opacity: isVisible ? 1 : 0,
      background: 'linear-gradient(500deg, rgba(0, 0, 0, 0.85), rgba(30, 30, 30, 0.85))',
      backdropFilter: 'blur(40px) saturate(180%)',
      WebkitBackdropFilter: 'blur(40px) saturate(180%)',
      transition: 'transform 0.4s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.4s cubic-bezier(0.4, 0, 0.2, 1)'
    }"
  >
    <!-- Lado izquierdo: Toggle del sidebar, logo y nombre de la empresa -->
    <div class="flex items-center flex-shrink-0 gap-2 lg:gap-4">
      <!-- Toggle del sidebar (solo en móviles cuando existe sidebar) -->
      <icon 
        :class="[
          'md:hidden',
          'mode-dark-highcontrast:text-foreground',
          'cursor-pointer text-3xl p-2',
          'hover:opacity-80 transition-opacity',
          'flex-shrink-0'
        ]"
        name="nav-menu" 
        @click="$emit('toggleSidebar')" 
      />
      
      <!-- Logo de Aether Panel -->
      <img 
        src="/img/resources/image.png" 
        alt="Aether Panel Logo"
        :class="[
          'h-16 w-16 object-contain',
          'flex-shrink-0',
          'drop-shadow-lg'
        ]"
      />
      
      <!-- Nombre de la empresa -->
      <div 
        :class="[
          'font-headline text-xl lg:text-2xl font-black text-primary-foreground whitespace-nowrap',
          'mode-dark-highcontrast:text-foreground',
          'flex-shrink-0'
        ]"
      >
        {{ name }}
      </div>
    </div>
    
    <!-- Lado derecho: Búsqueda global y avatar del usuario -->
    <div 
      v-if="props.user" 
      :class="[
        'flex items-center gap-2 lg:gap-3',
        'flex-shrink-0'
      ]"
    >
      <!-- Búsqueda global -->
      <panel-search />
      
      <!-- Avatar del usuario -->
      <router-link 
        v-hotkey="'g a'" 
        :to="{ name: 'Self' }"
        :class="[
          'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-foreground rounded',
          'flex-shrink-0'
        ]"
      >
        <img 
          :src="getAvatarLink()" 
          :class="[
            'w-9 h-9 lg:w-10 lg:h-10 border-2 border-primary-foreground rounded-full transition-all duration-100',
            'hover:brightness-75 hover:scale-105',
            'mode-dark-highcontrast:border-foreground'
          ]"
          :alt="props.user.email"
        />
      </router-link>
    </div>
  </header>
</template>
