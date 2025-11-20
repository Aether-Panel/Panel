<script setup>
import { useI18n } from 'vue-i18n'
import Icon from './Icon.vue'

defineProps({
  small: { type: Boolean, default: () => false },
  text: { type: String, default: () => undefined }
})

const { t } = useI18n()
</script>

<template>
  <div 
    :class="[
      'flex flex-col items-center justify-center gap-4',
      'py-12 px-4',
      small && 'py-6 gap-3'
    ]"
  >
    <icon 
      name="loading" 
      :class="[
        'animate-spin text-primary',
        'drop-shadow-sm',
        small ? 'text-2xl' : 'text-4xl'
      ]"
    />
    <span 
      :class="[
        'text-foreground font-medium',
        'text-center',
        small ? 'text-sm' : 'text-base'
      ]"
      v-text="text ? text : t('common.Loading')" 
    />
  </div>
</template>
