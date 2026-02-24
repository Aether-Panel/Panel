<script setup>
import { ref } from 'vue'
import Icon from './Icon.vue'

defineProps({
  title: { type: String, default: () => '' }
})

const open = ref(false)
</script>

<template>
  <div 
    :class="[
      'relative mt-3 p-4',
      'transition-all duration-200 ease-in-out',
      'rounded-xl',
      open 
        ? 'bg-background border-2 border-border shadow-md hover:shadow-lg'
        : 'hover:bg-muted/30 border border-transparent'
    ]"
  >
    <!-- Título clickeable -->
    <div 
      :class="[
        'text-xl font-bold font-headline cursor-pointer',
        'transition-all duration-200',
        'relative flex items-center justify-between w-full',
        'text-foreground hover:text-primary',
        'group',
        open && 'pb-3 mb-3 border-b-2 border-border/50'
      ]"
      @click="open = !open"
    >
      <span class="flex-grow">{{ title }}</span>
      <!-- Icono chevron -->
      <icon 
        :name="'chevron-down'"
        :class="[
          'text-2xl transition-transform duration-200',
          'text-muted-foreground group-hover:text-primary',
          open && 'rotate-180'
        ]"
      />
    </div>
    
    <!-- Contenido colapsable -->
    <div 
      :class="[
        'overflow-hidden transition-all duration-100 ease-in-out',
        open 
          ? 'max-h-[9999px] opacity-100' 
          : 'max-h-0 opacity-0'
      ]"
    >
      <div v-if="open">
        <slot />
      </div>
    </div>
  </div>
</template>
