<script setup>
import { computed } from 'vue'

const props = defineProps({
  disabled: { type: Boolean, default: () => false },
  variant: { type: String, default: () => 'raised' },
  color: { type: String, default: () => 'neutral' },
  tooltip: { type: String, default: () => undefined }
})

// Clases base de Tailwind para botones
const buttonClasses = computed(() => {
  const classes = []
  
  // Clases base
  classes.push(
    'inline-flex items-center justify-center gap-2',
    'font-standard text-sm font-medium',
    'h-11 px-5 py-2.5 my-2',
    'border-none rounded-lg',
    'cursor-pointer touch-manipulation',
    'text-center whitespace-nowrap',
    'transition-all duration-200 ease-in-out',
    'focus:outline-none focus:ring-2 focus:ring-offset-2',
    'shadow-sm hover:shadow-md active:scale-95',
    'transform-gpu'
  )
  
  // Variante raised (por defecto)
  if (props.variant === 'raised') {
    classes.push('w-full shadow-md')
  }
  
  // Variante text
  if (props.variant === 'text') {
    classes.push('bg-transparent hover:bg-primary/15')
    // Modo alto contraste
    classes.push('mode-dark-highcontrast:hover:underline mode-dark-highcontrast:hover:decoration-dashed')
  }
  
  // Variante icon
  if (props.variant === 'icon') {
    classes.push('min-w-12 w-12 h-12 p-2 m-0 rounded-full bg-transparent text-2xl')
    if (!props.disabled) {
      classes.push('hover:bg-primary/15')
    }
  }
  
  // Colores
  if (props.color === 'primary') {
    classes.push('bg-primary text-primary-foreground')
    if (props.variant !== 'text' && props.variant !== 'icon') {
      classes.push('hover:brightness-90')
    }
    classes.push('focus:ring-primary')
    // Modo alto contraste
    classes.push('mode-dark-highcontrast:bg-background mode-dark-highcontrast:border-[3px] mode-dark-highcontrast:border-primary')
    classes.push('mode-dark-highcontrast:text-primary mode-dark-highcontrast:font-bold')
    classes.push('mode-dark-highcontrast:hover:underline mode-dark-highcontrast:hover:decoration-dashed')
  } else if (props.color === 'error') {
    classes.push('bg-error text-error-foreground')
    if (props.variant !== 'text' && props.variant !== 'icon') {
      classes.push('hover:brightness-90')
    }
    classes.push('focus:ring-error')
    // Modo alto contraste
    classes.push('mode-dark-highcontrast:bg-background mode-dark-highcontrast:border-[3px] mode-dark-highcontrast:border-error')
    classes.push('mode-dark-highcontrast:text-error mode-dark-highcontrast:font-bold')
    classes.push('mode-dark-highcontrast:hover:underline mode-dark-highcontrast:hover:decoration-dashed')
  } else {
    // neutral (por defecto)
    classes.push('bg-muted text-foreground')
    classes.push('focus:ring-muted-foreground')
  }
  
  // Estado disabled
  if (props.disabled) {
    classes.push('opacity-40 cursor-not-allowed')
    if (props.variant !== 'icon') {
      classes.push('bg-black/10 text-muted-foreground shadow-none')
    }
    // Modo alto contraste disabled
    if (props.variant !== 'icon') {
      classes.push('mode-dark-highcontrast:border-[3px] mode-dark-highcontrast:border-muted-foreground')
    }
  }
  
  // Clases para iconos dentro del botón
  if (props.variant !== 'icon') {
    classes.push('[&_.icon]:text-2xl [&_.icon]:px-1')
  }
  
  return classes.join(' ')
})
</script>

<template>
  <button
    :disabled="props.disabled"
    :class="buttonClasses"
    :data-tooltip="props.tooltip"
    type="button"
  >
    <slot />
  </button>
</template>
