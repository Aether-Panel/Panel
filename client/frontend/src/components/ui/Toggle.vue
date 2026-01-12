<script setup>
import markdown from '@/utils/markdown.js'

const props = defineProps({
  label: { type: String, default: () => undefined },
  hint: { type: String, default: () => undefined },
  disabled: { type: Boolean, default: () => false },
  modelValue: { type: Boolean, required: true }
})

const emit = defineEmits(['update:modelValue'])

function onInput() {
  emit('update:modelValue', !props.modelValue)
}
</script>

<template>
  <label 
    :class="[
      'relative inline-block mb-2 cursor-pointer',
      disabled && 'opacity-50 cursor-not-allowed'
    ]"
  >
    <!-- Checkbox oculto -->
    <input 
      type="checkbox" 
      :disabled="disabled" 
      :checked="modelValue" 
      :class="['sr-only']"
      @input="onInput" 
    />
    
    <!-- Toggle switch -->
    <span 
      :class="[
        'relative inline-block mt-3 mb-3',
        'w-14 h-7 rounded-full',
        'transition-all duration-200',
        'shadow-sm',
        modelValue ? 'bg-primary shadow-primary/50' : 'bg-gray-400',
        'focus-within:outline focus-within:outline-2 focus-within:outline-primary focus-within:outline-offset-2',
        'mode-dark-highcontrast:bg-background',
        'mode-dark-highcontrast:border-[3px] mode-dark-highcontrast:border-foreground',
        'mode-dark-highcontrast:w-[calc(3.5rem-6px)] mode-dark-highcontrast:h-[calc(1.75rem-6px)]',
        modelValue && 'mode-dark-highcontrast:bg-background'
      ]"
    >
      <!-- Círculo del toggle -->
      <span
        :class="[
          'absolute top-[0.125rem] left-[0.125rem]',
          'w-6 h-6 rounded-full',
          'bg-primary-foreground shadow-lg',
          'transition-all duration-200',
          'transform',
          modelValue && 'translate-x-[1.375rem]',
          'mode-dark-highcontrast:shadow-none',
          'mode-dark-highcontrast:bg-foreground'
        ]"
      />
    </span>
    
    <!-- Label -->
    <div 
      v-if="label"
      :class="[
        'inline-block ml-4 mr-2 -mt-[0.5rem] relative',
        'leading-6',
        'text-foreground'
      ]"
      v-text="label" 
    />
    
    <!-- Hint -->
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div 
      v-if="hint" 
      :class="[
        'block ml-16 mr-2 mb-3',
        'text-muted-foreground'
      ]"
      v-html="markdown(hint)" 
    />
  </label>
</template>
