<script setup>
import { inject, computed } from 'vue'
import { RouterLink } from 'vue-router'
import md5 from 'js-md5'
import Icon from './Icon.vue'
import PanelSearch from './PanelSearch.vue'

const props = defineProps({
  user: { type: Object, default: () => undefined }
})

defineEmits(['toggleSidebar'])

const config = inject('config')
const name = computed(() => config.branding.name)

function getAvatarLink() {
  return 'https://www.gravatar.com/avatar/' + md5(props.user.email.trim().toLowerCase()) + '?d=mp'
}
</script>

<template>
  <header 
    :class="[
      'fixed top-0 left-0 right-0 h-16 z-40',
      'flex items-center justify-between flex-row px-4 lg:px-6',
      'bg-primary/95 backdrop-blur-md text-primary-foreground',
      'border-b border-primary-foreground/10',
      'shadow-lg',
      'mode-dark-highcontrast:bg-background mode-dark-highcontrast:border-b-[3px] mode-dark-highcontrast:border-foreground mode-dark-highcontrast:shadow-none',
      'mode-dark-highcontrast:text-foreground'
    ]"
  >
    <!-- Lado izquierdo: Toggle del sidebar y nombre de la empresa -->
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
      
      <!-- Nombre de la empresa -->
      <div 
        :class="[
          name !== 'SkyPanel' 
            ? 'font-headline h-16 leading-[4rem] text-2xl lg:text-4xl font-black text-primary-foreground whitespace-nowrap text-ellipsis overflow-hidden'
            : 'h-8 my-4 text-transparent bg-primary-foreground [mask:url(/img/logo.svg)_no-repeat_left_center] select-none flex-shrink-0',
          'mode-dark-highcontrast:text-foreground'
        ]"
        :data-name="name"
      >
        {{ name !== 'SkyPanel' ? name : '' }}
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
