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
      'flex items-center flex-row px-4 lg:px-6',
      'bg-primary/95 backdrop-blur-md text-primary-foreground',
      'border-b border-primary-foreground/10',
      'shadow-lg',
      'mode-dark-highcontrast:bg-background mode-dark-highcontrast:border-b-[3px] mode-dark-highcontrast:border-foreground mode-dark-highcontrast:shadow-none',
      'mode-dark-highcontrast:text-foreground'
    ]"
  >
    <!-- Toggle del sidebar (solo en móviles cuando existe sidebar) -->
    <icon 
      :class="[
        'hidden',
        'mode-dark-highcontrast:text-foreground',
        'md:hidden',
        'cursor-pointer text-3xl p-3 -mr-3',
        'hover:opacity-80 transition-opacity'
      ]"
      name="nav-menu" 
      @click="$emit('toggleSidebar')" 
    />
    
    <!-- Nombre de la empresa -->
    <div 
      :class="[
        name !== 'SkyPanel' 
          ? 'flex-grow font-headline mx-4 h-16 leading-[4rem] text-4xl font-black text-primary-foreground whitespace-nowrap text-ellipsis overflow-hidden'
          : 'h-8 mx-4 my-4 text-transparent bg-primary-foreground [mask:url(/img/logo.svg)_no-repeat_left_center] select-none',
        'mode-dark-highcontrast:text-foreground'
      ]"
      :data-name="name"
    >
      {{ name !== 'SkyPanel' ? name : '' }}
    </div>
    
    <!-- Búsqueda global -->
    <panel-search v-if="props.user" />
    
    <!-- Avatar del usuario -->
    <router-link 
      v-if="props.user" 
      v-hotkey="'g a'" 
      :to="{ name: 'Self' }"
      class="focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-foreground rounded"
    >
      <img 
        :src="getAvatarLink()" 
        :class="[
          'm-3 border-2 border-primary-foreground rounded-full transition-all duration-100',
          'hover:brightness-75',
          'mode-dark-highcontrast:border-foreground'
        ]"
        :alt="props.user.email"
      />
    </router-link>
  </header>
</template>
