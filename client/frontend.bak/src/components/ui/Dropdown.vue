<script>
import { ref, computed, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
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
    options: { type: [Array, Function], default: () => [] },
    type: { type: String, default: () => 'text' },
    icon: { type: String, default: () => undefined },
    modelValue: { type: [String, Number, Object, Array], default: () => '' },
    mode: { type: String, default: () => 'single' }, // 'single' or 'tags'
    searchFunction: { type: Function, default: undefined },
    minChars: { type: Number, default: () => 0 },
    resolveOnLoad: { type: Boolean, default: () => true },
    delay: { type: Number, default: () => 0 },
    disabled: { type: Boolean, default: () => false },
    placeholder: { type: String, default: () => undefined },
    closeOnSelect: { type: Boolean, default: () => true },
    canClear: { type: Boolean, default: () => false },
    canDeselect: { type: Boolean, default: () => false },
    filterResults: { type: Boolean, default: () => true },
    searchable: { type: Boolean, default: () => undefined }
  },
  emits: ['update:modelValue', 'change'],
  setup(props) {
    const { t } = useI18n()
    console.group('🔍 [DROPDOWN] Inicializando componente Dropdown')
    console.log('   - props.options:', props.options)
    console.log('   - Tipo de props.options:', typeof props.options)
    console.log('   - Es array?', Array.isArray(props.options))
    console.log('   - props.searchFunction:', props.searchFunction)
    
    const ms = ref(null)
    const isOpen = ref(false)

    function select(item) {
      nextTick(() => ms.value.select(item))
    }

    function open() {
      if (ms.value) ms.value.open()
    }

    // Computed para las opciones que siempre devuelve un valor válido
    const computedOptions = computed(() => {
      console.log('🔍 [DROPDOWN] computedOptions evaluado')
      console.log('   - props.options:', props.options)
      console.log('   - Tipo de props.options:', typeof props.options)
      console.log('   - Es array?', Array.isArray(props.options))
      console.log('   - props.searchFunction:', props.searchFunction)
      console.log('   - Tipo de searchFunction:', typeof props.searchFunction)
      
      try {
        // PRIORIDAD 1: Si hay searchFunction, usarla como options (para búsqueda dinámica)
        // Esto es necesario para que Multiselect muestre el dropdown al escribir
        if (props.searchFunction && typeof props.searchFunction === 'function') {
          const funcStr = props.searchFunction.toString().trim()
          console.log('   - searchFunction toString:', funcStr)
          
          // Si la función está vacía o es muy corta, no usarla
          if (funcStr === '()=>{}' || funcStr === 'function(){}' || funcStr.length < 15) {
            console.warn('⚠️ [DROPDOWN] searchFunction es vacía/inválida')
            // Continuar con la siguiente prioridad
          } else {
            console.log('✅ [DROPDOWN] PRIORIDAD SEARCHFUNCTION: Retornando searchFunction')
            return props.searchFunction
          }
        }
        
        // PRIORIDAD 2: Si options es un array (incluso vacío), retornarlo
        const isArrayValue = Array.isArray(props.options) || 
                            (props.options && 
                             typeof props.options === 'object' && 
                             'length' in props.options && 
                             typeof props.options.length === 'number' &&
                             !(props.options instanceof Date))
        
        if (isArrayValue) {
          const arrayValue = Array.isArray(props.options) 
            ? props.options 
            : Array.from(props.options || [])
          console.log('✅ [DROPDOWN] PRIORIDAD ARRAY: Retornando array con', arrayValue.length, 'items')
          return arrayValue
        }
        
        // PRIORIDAD 3: Si options es una función, devolverla
        if (typeof props.options === 'function') {
          console.log('✅ [DROPDOWN] Retornando función options')
          return props.options
        }
        
        // Si options es undefined o null, usar array vacío
        if (props.options === undefined || props.options === null) {
          console.warn('⚠️ [DROPDOWN] props.options es undefined/null')
          return []
        }
      } catch (e) {
        console.error('❌ [DROPDOWN] Error en computedOptions:', e)
      }
      // Por defecto, devolver array vacío
      console.warn('⚠️ [DROPDOWN] Retornando [] por defecto')
      return []
    })
    
    // Función wrapper para asegurar que siempre devuelva un valor válido
    const safeOptions = computed(() => {
      const opts = computedOptions.value
      console.log('🔍 [DROPDOWN] safeOptions evaluado:', opts)
      
      // Si es undefined o null, retornar array vacío
      if (opts === undefined || opts === null) {
        console.warn('⚠️ [DROPDOWN] opts es undefined/null, retornando []')
        return []
      }
      
      // Si es un array o tiene propiedades de array (Proxy), retornarlo directamente
      const isArrayLike = Array.isArray(opts) || 
                         (opts && typeof opts === 'object' && 'length' in opts && typeof opts.length === 'number' && !(opts instanceof Date))
      
      if (isArrayLike) {
        const arrayValue = Array.isArray(opts) ? opts : Array.from(opts || [])
        console.log('✅ [DROPDOWN] Retornando array:', arrayValue.length, 'items')
        return arrayValue
      }
      
      // Si es una función, verificar que no sea vacía antes de retornarla
      if (typeof opts === 'function') {
        const funcStr = opts.toString().trim()
        // Si es una función vacía como ()=>{}, retornar array vacío en su lugar
        if (funcStr === '()=>{}' || funcStr === 'function(){}' || funcStr.length < 15) {
          console.warn('⚠️ [DROPDOWN] Función vacía detectada, retornando [] en su lugar')
          return []
        }
        console.log('✅ [DROPDOWN] Retornando función válida')
        return opts
      }
      
      // Cualquier otro caso, retornar array vacío
      console.warn('⚠️ [DROPDOWN] Tipo inesperado:', typeof opts, 'retornando []')
      return []
    })
    
    // Verificar el valor inicial
    console.log('✅ [DROPDOWN] Computed options creado')
    console.log('   - computedOptions.value:', computedOptions.value)
    console.log('   - safeOptions.value:', safeOptions.value)
    console.groupEnd()

    return { isOpen, ms, select, open, markdown, safeOptions, t }
  }
}
</script>

<template>
  <div class="w-full mb-5">
    <!-- Label superior (siempre visible) -->
    <label 
      v-if="label" 
      :for="id"
      :class="[
        'block mb-2 text-sm font-medium',
        error ? 'text-error' : 'text-foreground'
      ]"
    >
      {{ label }}
    </label>
    
    <div 
      :class="[
        'relative inline-block w-full',
        'group',
        error && 'has-error'
      ]"
    >
      <!-- Icono izquierdo -->
      <icon 
        v-if="icon" 
        :name="icon" 
        :class="[
          'absolute left-3 top-1/2 -translate-y-1/2',
          'text-muted-foreground pointer-events-none z-10',
          'text-xl'
        ]"
      />
      
      <!-- Multiselect component -->
      <div 
        :class="[
          icon && 'pl-10',
          'w-full relative'
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
          :options="safeOptions" 
          :object="object" 
          :placeholder="placeholder || t('common.Select') || 'Seleccionar...'"
          :close-on-select="closeOnSelect"
          :filter-results="filterResults"
          :min-chars="minChars"
          :resolve-on-load="resolveOnLoad"
          :delay="delay"
          :searchable="searchable !== undefined ? searchable : (searchFunction ? true : undefined)"
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
