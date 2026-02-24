<script setup>
import Icon from '@/components/ui/Icon.vue'

defineProps({
  icon: { type: String, default: () => undefined }
})
</script>

<template>
  <div 
    :class="[
      'list-item-icon',
      'flex-shrink-0',
      'mr-3 ml-0',
      '[&_.icon]:text-2xl',
      '[&_.icon]:text-muted-foreground',
      'group-hover:[&_.icon]:text-primary',
      'transition-colors duration-200',
      '[dir=rtl]:mr-0',
      '[dir=rtl]:ml-3'
    ]"
  >
    <icon v-if="icon" :name="icon" />
    <slot v-else />
  </div>
</template>
