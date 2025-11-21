<script>
import { ref, nextTick } from 'vue'
import Multiselect from '@vueform/multiselect'
import Icon from '@/components/ui/Icon.vue'
import markdown from '@/utils/markdown.js'

export default {
  components: {
    Icon,
    Multiselect
  },
  props: {
    // default to random id as labels need target ids to exist exactly once
    id: { type: String, default: () => (Math.random() + 1).toString(36).substring(2) },
    label: { type: String, default: () => undefined },
    labelProp: { type: String, default: () => 'label' },
    error: { type: String, default: () => undefined },
    hint: { type: String, default: () => undefined },
    object: { type: Boolean, default: () => false},
    options: { type: Array, default: () => [] },
    type: { type: String, default: () => 'text' },
    icon: { type: String, default: () => undefined },
    modelValue: { type: [String, Number, Object, Array], default: () => '' },
    mode: { type: String, default: () => 'single' }, // 'single' or 'tags'
    searchFunction: { type: Function, default: () => undefined },
    minChars: { type: Number, default: () => 0 },
    resolveOnLoad: { type: Boolean, default: () => true },
    delay: { type: Number, default: () => 0 },
    disabled: { type: Boolean, default: () => false },
    placeholder: { type: String, default: () => undefined },
    closeOnSelect: { type: Boolean, default: () => true },
    canClear: { type: Boolean, default: () => false },
    canDeselect: { type: Boolean, default: () => false },
    filterResults: { type: Boolean, default: () => true }
  },
  emits: ['update:modelValue', 'change'],
  setup() {
    const ms = ref(null)
    const isOpen = ref(false)

    function select(item) {
      nextTick(() => ms.value.select(item))
    }

    function open() {
      if (ms.value) ms.value.open()
    }

    return { isOpen, ms, select, open, markdown }
  }
}
</script>

<template>
  <div class="w-full mb-5">
    <div 
      :class="[
        'relative inline-block w-full',
        'group',
        error && 'border-error'
      ]"
    >
      <!-- Icono izquierdo -->
      <icon 
        v-if="icon" 
        :name="icon" 
        :class="[
          'absolute left-3 top-1/2 -translate-y-1/2',
          'text-muted-foreground pointer-events-none z-10',
          'text-2xl'
        ]"
      />
      
      <!-- Multiselect component -->
      <div 
        :class="[
          icon && 'pl-10',
          'w-full'
        ]"
      >
        <multiselect 
          :id="id" 
          ref="ms" 
          :model-value="modelValue" 
          :label="labelProp"  
          :mode="mode" 
          :can-deselect="canDeselect" 
          :can-clear="canClear" 
          :options="searchFunction || options" 
          :object="object" 
          :placeholder="placeholder || label"
          :close-on-select="closeOnSelect"
          :filter-results="filterResults"
          :min-chars="minChars"
          :resolve-on-load="resolveOnLoad"
          :delay="delay"
          :searchable="!!searchFunction || searchFunction === undefined"
          :disabled="disabled"
          class="multiselect-dropdown"
          @input="$emit('update:modelValue', $event); $emit('change', $event)" 
          @open="$nextTick(() => isOpen = true)" 
          @close="isOpen = false"
        >
          <template v-for="(index, name) in $slots" #[name]="data">
            <slot :name="name" v-bind="data"></slot>
          </template>
        </multiselect>
      </div>
      
      <!-- Label flotante -->
      <label 
        v-if="label" 
        :for="id"
        :class="[
          'absolute left-3 pointer-events-none',
          'transition-all duration-200 ease-in-out',
          'origin-top-left',
          isOpen 
            ? 'top-0 -translate-y-1/2 bg-background px-2 text-primary scale-90 font-semibold' 
            : (modelValue === null || modelValue === undefined || (Array.isArray(modelValue) && modelValue.length === 0))
              ? 'top-[1.05rem] scale-100 text-muted-foreground' 
              : 'top-0 -translate-y-1/2 bg-background px-2 text-foreground scale-90 font-semibold',
          icon && 'left-12'
        ]"
        @click="ms.open()"
      >
        {{ label }}
      </label>
    </div>
    
    <!-- Mensaje de error -->
    <span 
      v-if="error" 
      :class="[
        'block mt-2 text-sm',
        'text-error font-medium'
      ]"
      v-text="error" 
    />
    
    <!-- Hint -->
    <!-- eslint-disable-next-line vue/no-v-html -->
    <span 
      v-if="hint && !error" 
      :class="[
        'block mt-2 text-sm',
        'text-muted-foreground'
      ]"
      v-html="markdown(hint)" 
    />
  </div>
</template>
