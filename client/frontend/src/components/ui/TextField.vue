<script setup>
import { ref, onMounted, nextTick, computed } from 'vue'
import Icon from './Icon.vue'
import markdown from '@/utils/markdown.js'

const props = defineProps({
  // default to random id as labels need target ids to exist exactly once
  id: { type: String, default: () => (Math.random() + 1).toString(36).substring(2) },
  label: { type: String, default: () => undefined },
  hint: { type: String, default: () => undefined },
  error: { type: String, default: () => undefined },
  name: { type: String, default: () => undefined },
  type: { type: String, default: () => 'text' },
  icon: { type: String, default: () => undefined },
  autofocus: { type: Boolean, default: () => false },
  disabled: { type: Boolean, default: () => false },
  afterIcon: { type: String, default: () => undefined },
  afterHint: { type: String, default: () => undefined },
  modelValue: { type: [String, Number], default: () => '' }
})

const emit = defineEmits(['change', 'blur', 'focus', 'update:modelValue'])

const input = ref(null)
const showPassword = ref(false)
const isFocused = ref(false)

const labelFloating = computed(() => {
  return (props.modelValue && props.modelValue.toString().length > 0) || isFocused.value
})

onMounted(() => {
  if (props.autofocus) {
    nextTick(() => {
      input.value.focus()
    })
  }
})

function onInput(e) {
  emit('update:modelValue', e.target.value)
  emit('change', e)
}

function onBlur(e) {
  isFocused.value = false
  emit('blur', e)
}

function onFocus(e) {
  isFocused.value = true
  emit('focus', e)
}
</script>

<template>
  <div class="w-full cursor-text mb-5 group" @click="input.focus()">
    <div 
      :class="[
        'relative flex items-center',
        'border-2 rounded-xl transition-all duration-200',
        'bg-background',
        'shadow-sm hover:shadow-md focus-within:shadow-lg',
        error 
          ? 'border-error focus-within:border-error focus-within:ring-2 focus-within:ring-error/20' 
          : 'border-input/50 focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/20',
        disabled && 'opacity-60 cursor-not-allowed bg-muted shadow-none hover:shadow-none'
      ]"
    >
      <!-- Icono izquierdo -->
      <icon 
        v-if="icon" 
        :name="icon" 
        class="absolute left-3 text-muted-foreground pointer-events-none"
      />
      
      <!-- Input -->
      <input 
        :id="id" 
        ref="input" 
        :value="modelValue" 
        :type="showPassword ? 'text' : type" 
        :placeholder="label" 
        :name="name" 
        :disabled="disabled"
        :class="[
          'w-full bg-transparent border-none outline-none',
          'font-standard text-foreground placeholder:text-muted-foreground',
          'px-4 py-3',
          icon && 'pl-10',
          (type === 'password' || afterIcon) && 'pr-10',
          disabled && 'cursor-not-allowed'
        ]"
        @input="onInput($event)" 
        @blur="onBlur($event)" 
        @focus="onFocus($event)" 
      />
      
      <!-- Icono derecho o toggle de contraseña -->
      <div class="absolute right-3 flex items-center gap-2">
        <icon 
          v-if="type === 'password'" 
          :name="showPassword ? 'eye-off' : 'eye'" 
          class="cursor-pointer text-muted-foreground hover:text-foreground transition-colors"
          @click.stop="showPassword = !showPassword" 
        />
        <template v-else>
          <icon 
            v-if="afterIcon" 
            :name="afterIcon" 
            class="text-muted-foreground pointer-events-none"
          />
          <span 
            v-if="afterHint" 
            class="post-tooltip cursor-help text-muted-foreground" 
            :data-tooltip="afterHint"
          />
        </template>
      </div>
      
      <!-- Label flotante -->
      <label 
        v-if="label" 
        :for="id"
        :class="[
          'absolute left-3 pointer-events-none transition-all duration-200',
          labelFloating
            ? 'top-0 -translate-y-1/2 bg-background px-2 text-xs text-primary' 
            : 'top-1/2 -translate-y-1/2 text-muted-foreground',
          icon && !labelFloating && 'left-12',
          icon && labelFloating && 'left-3'
        ]"
      >
        <span v-text="label" />
      </label>
    </div>
    
    <!-- Mensaje de error -->
    <span 
      v-if="error" 
      class="block mt-1 text-sm text-error" 
      v-text="error" 
    />
    
    <!-- Hint -->
    <!-- eslint-disable-next-line vue/no-v-html -->
    <span 
      v-if="hint && !error" 
      class="block mt-1 text-sm text-muted-foreground" 
      v-html="markdown(hint)" 
    />
  </div>
</template>
