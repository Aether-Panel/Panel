<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from './Btn.vue'
import Icon from './Icon.vue'
import TextField from './TextField.vue'

const props = defineProps({
  label: { type: String, default: () => undefined },
  addLabel: { type: String, default: () => undefined },
  hint: { type: String, default: () => undefined },
  error: { type: String, default: () => undefined },
  allowSwap: { type: Boolean, default: () => false },
  modelValue: { type: Array, default: () => [] }
})

const emit = defineEmits(['update:modelValue'])

const { t } = useI18n()

const values = ref([ ...props.modelValue ])

function emitUpdate() {
  emit('update:modelValue', values.value)
}

function addEntry() {
  values.value.push('')
}

function onInput(i, event) {
  values.value[i] = event
  emitUpdate()
}

function swap(i1, i2) {
  const x = values.value[i1]
  values.value[i1] = values.value[i2]
  values.value[i2] = x
  emitUpdate()
}

function removeEntry(i) {
  values.value.splice(i, 1)
  emitUpdate()
}
</script>

<template>
  <div 
    :class="[
      'list-input',
      'space-y-3'
    ]"
  >
    <div 
      v-if="label" 
      :class="[
        'label',
        'text-2xl font-semibold text-foreground'
      ]"
      v-text="label" 
    />
    <div 
      v-if="error" 
      :class="[
        'error',
        'text-error text-sm'
      ]"
      v-text="error" 
    />
    <div 
      v-else-if="hint" 
      :class="[
        'hint',
        'text-muted-foreground text-sm'
      ]"
      v-text="hint" 
    />
    <div 
      v-for="(entry, index) in values" 
      :key="index" 
      :class="[
        'entry',
        'flex items-center gap-3',
        'p-4 rounded-xl bg-muted/50 border-2 border-border/50',
        'shadow-sm hover:shadow-md transition-all duration-200',
        'hover:border-primary/30',
        allowSwap && 'flex-wrap'
      ]"
    >
      <div 
        :class="[
          'flex-1 min-w-0',
          allowSwap ? 'md:w-[calc(100%-10rem)] w-full' : 'w-[calc(100%-4rem)]'
        ]"
      >
        <text-field 
          :model-value="entry" 
          @update:modelValue="onInput(index, $event)" 
        />
      </div>
      <div 
        v-if="allowSwap" 
        :class="[
          'flex gap-1'
        ]"
      >
        <btn 
          :disabled="index === 0" 
          variant="icon"
          @click="swap(index, index-1)"
        >
          <icon name="up" />
        </btn>
        <btn 
          :disabled="index === values.length-1" 
          variant="icon"
          @click="swap(index, index+1)"
        >
          <icon name="down" />
        </btn>
      </div>
      <btn 
        variant="icon" 
        color="error"
        @click="removeEntry(index)"
      >
        <icon name="remove" />
      </btn>
    </div>
    <btn 
      variant="text" 
      @click="addEntry()"
    >
      <icon name="plus" />{{ addLabel || t('common.Add') }}
    </btn>
  </div>
</template>
