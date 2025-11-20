<script setup>
import { computed } from 'vue'

const props = defineProps({
  name: { type: String, required: true },
  spin: { type: Boolean, required: false }
})

const iconClasses = computed(() => {
  return [
    'icon',
    'icon-' + props.name,
    'inline-block',
    'align-middle',
    props.spin && 'animate-spin'
  ].filter(Boolean)
})
</script>

<template>
  <span :class="iconClasses" />
</template>
