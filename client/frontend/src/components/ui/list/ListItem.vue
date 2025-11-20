<script setup>
import { RouterLink } from 'vue-router'

defineProps({
  // eslint-disable-next-line vue/require-prop-types
  to: { default: () => undefined }
})
</script>

<template>
  <router-link v-if="to" :to="to" class="block">
    <li 
      :class="[
        'list-item',
        'm-0 px-4 py-4',
        'flex items-center',
        'transition-all duration-200 ease-in-out',
        'text-foreground rounded-xl',
        'bg-background border-2 border-border/50',
        'shadow-sm hover:shadow-md',
        'hover:bg-primary/10 hover:border-primary/30',
        'active:bg-primary/15 active:shadow-lg',
        'cursor-pointer',
        'group'
      ]"
    >
      <slot />
    </li>
  </router-link>
  <li 
    v-else 
    :class="[
      'list-item',
      'm-0 px-4 py-4',
      'flex items-center',
      'transition-all duration-200 ease-in-out',
      'text-foreground rounded-xl',
      'bg-background border-2 border-border/50',
      'shadow-sm',
      $attrs.onClick 
        ? 'hover:bg-primary/10 hover:border-primary/30 hover:shadow-md cursor-pointer' 
        : ''
    ]"
  >
    <slot />
  </li>
</template>
