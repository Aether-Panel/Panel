<script setup>
import Btn from './Btn.vue'
import Icon from './Icon.vue'

defineProps({
  title: { type: String, required: false, default: () => undefined },
  closable: { type: Boolean, required: false },
  modelValue: { type: Boolean, required: true }
})

defineEmits(['update:modelValue', 'close'])
</script>

<template>
  <div 
    v-if="modelValue" 
    :class="[
      'fixed inset-0 w-full h-full z-50',
      'bg-black/60 backdrop-blur-md',
      'flex justify-center items-start p-4',
      'mode-dark-highcontrast:bg-black/90',
      'overflow-y-auto',
      'animate-in fade-in duration-200'
    ]"
    @click.self="$emit('update:modelValue', false); $emit('close')"
  >
    <div 
      :class="[
        'relative bg-background text-foreground',
        'rounded-xl shadow-2xl border border-border/50',
        'mt-[10vh] mb-[10vh]',
        'max-h-[80vh] max-w-4xl w-full',
        'overflow-y-auto',
        'transform transition-all duration-200',
        'animate-in zoom-in-95 slide-in-from-bottom-2',
        'mode-dark-highcontrast:border-[3px] mode-dark-highcontrast:border-foreground',
        modelValue ? 'scale-100 opacity-100' : 'scale-95 opacity-0'
      ]"
    >
      <!-- Header del overlay -->
      <div 
        v-if="title" 
        :class="[
          'flex flex-row items-center',
          'p-4 border-b border-border',
          'sticky top-0 bg-background z-10'
        ]"
      >
        <h1 
          :class="[
            'flex-grow text-2xl font-bold text-foreground'
          ]"
          v-text="title" 
        />
        <btn 
          v-if="closable" 
          v-hotkey="'Escape'" 
          variant="icon" 
          color="neutral"
          :class="['w-auto mt-0 mr-2 font-normal']"
          @click="$emit('update:modelValue', false); $emit('close')"
        >
          <icon name="close" />
        </btn>
      </div>
      
      <!-- Contenido -->
      <div :class="['p-4']">
        <slot />
      </div>
    </div>
  </div>
</template>
