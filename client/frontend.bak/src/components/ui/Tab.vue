<script setup>
import { inject } from 'vue'

const props = defineProps({
  title: { type: String, required: true },
  icon: { type: String, default: () => undefined },
  id: { type: String, default: () => undefined }
})

const activeKey = inject('activeKey')
const key = props.id || props.title.toLowerCase().replace(/ /g, '-')
</script>

<template>
  <div 
    :id="id" 
    :class="[
      'tab-content',
      'transition-opacity duration-200',
      key === activeKey ? 'block opacity-100' : 'hidden opacity-0'
    ]"
  >
    <slot />
  </div>
</template>
