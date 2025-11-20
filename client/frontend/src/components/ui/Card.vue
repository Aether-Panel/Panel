<script setup>
const props = defineProps({
  variant: { type: String, default: () => 'default' }, // default, outlined, elevated
  padding: { type: String, default: () => 'normal' }, // none, small, normal, large
  hover: { type: Boolean, default: () => false }
})

const paddingClasses = {
  none: 'p-0',
  small: 'p-2',
  normal: 'p-4',
  large: 'p-6'
}

const variantClasses = {
  default: 'bg-background border border-border/50 shadow-sm hover:shadow-md transition-shadow duration-200',
  outlined: 'bg-background border-2 border-border/70 hover:border-primary/30 transition-colors duration-200',
  elevated: 'bg-background border border-border/50 shadow-lg hover:shadow-xl transition-shadow duration-200'
}
</script>

<template>
  <div 
    :class="[
      'rounded-lg transition-all duration-200',
      variantClasses[variant],
      paddingClasses[padding],
      hover && 'hover:shadow-md hover:border-primary/50 cursor-pointer'
    ]"
  >
    <slot />
  </div>
</template>

