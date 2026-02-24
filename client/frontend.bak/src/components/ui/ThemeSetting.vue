<script setup>
import { useI18n } from 'vue-i18n'
import Dropdown from './Dropdown.vue'

const props = defineProps({
  modelValue: { type: Object, required: true }
})

const emit = defineEmits(['update:modelValue'])

const { t, locale, fallbackLocale } = useI18n()

function onInput(event) {
  emit('update:modelValue', { ...props.modelValue, current: event })
}

function onNativeInput(event) {
  emit('update:modelValue', { ...props.modelValue, current: event.target.value })
}

function getSettingLabel(setting) {
  const fallback = setting.label || undefined
  if (setting.tkey) {
    return t(setting.tkey, fallback)
  } else if (setting.tlabels) {
    return setting.tlabels[locale.value] || setting.tlabels[fallbackLocale.value] || fallback
  } else {
    return fallback
  }
}

function withNormalizedLabels(options) {
  return options.map(option => {
    return { ...option, label: getSettingLabel(option) }
  })
}
</script>

<template>
  <div 
    :class="[
      'theme-setting-wrapper',
      'w-full mb-4'
    ]"
  >
    <dropdown 
      v-if="modelValue.type === 'class' || modelValue.type === 'snippet'" 
      :model-value="modelValue.current" 
      :options="withNormalizedLabels(modelValue.options)" 
      :label="getSettingLabel(modelValue)" 
      @update:modelValue="onInput($event)" 
    />
    <label 
      v-if="modelValue.type === 'color'" 
      :class="[
        'color-input',
        'flex items-center gap-4 p-3 rounded-xl',
        'bg-muted/20 border-2 border-border/50',
        'hover:border-primary/30 transition-all duration-200',
        'shadow-sm hover:shadow-md cursor-pointer'
      ]"
    >
      <span 
        :class="[
          'label',
          'text-foreground font-semibold text-lg',
          'flex-grow'
        ]"
      >
        <span v-text="getSettingLabel(modelValue)" />
      </span>
      <input 
        type="color" 
        :value="modelValue.current" 
        :class="[
          'w-12 h-12 rounded-lg border-2 border-border cursor-pointer',
          'bg-transparent shadow-sm hover:shadow-md',
          'transition-shadow duration-200'
        ]"
        @input="onNativeInput" 
      />
    </label>
  </div>
</template>
