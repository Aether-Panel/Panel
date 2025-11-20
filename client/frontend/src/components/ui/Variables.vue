<script setup>
import { computed, inject } from 'vue'
import { useI18n } from 'vue-i18n'
import SettingInput from '@/components/ui/SettingInput.vue'

const props = defineProps({
  modelValue: { type: Object, required: true },
  disabled: { type: Boolean, default: () => false }
})

const emit = defineEmits(['update:modelValue'])

const { t } = useI18n()
const conditions = inject('conditions')

// ensure groups are sorted correctly
if (Array.isArray(props.modelValue.groups)) {
  const isSorted = !!props.modelValue.groups.reduce((acc, curr) => {
    if (acc === false || acc.order > curr.order) return false
    return curr
  })
  if (!isSorted) {
    const v = { ...props.modelValue, groups: [ ...props.modelValue.groups ] }
    v.groups.sort((a, b) => a.order > b.order ? 1 : -1)
    emit('update:modelValue', v)
  }
}

const grouplessVars = computed(() => {
  if (Array.isArray(props.modelValue.groups)) {
    return Object.keys(props.modelValue.data).filter(varname => {
      return props.modelValue.groups.map(g => g.variables).flat().indexOf(varname) === -1
    })
  } else {
    return Object.keys(props.modelValue.data)
  }
})

function updateValue(name, event) {
  const v = { ...props.modelValue }
  v.data[name].value = event.value
  emit('update:modelValue', v)
}

function visibleGroups() {
  const data = {}
  Object.keys(props.modelValue.data).map(name => {
    data[name] = props.modelValue.data[name].value
  })
  return props.modelValue.groups.filter(group => {
    if (group.if) {
      return conditions(group.if, data)
    }
    return true
  })
}

function filtered(group) {
  return group.variables.filter(v => {
    return props.modelValue.data[v] && !props.modelValue.data[v].internal
  })
}

function grouplessVarsFiltered() {
  return grouplessVars.value.filter(v => {
    return props.modelValue.data[v] && !props.modelValue.data[v].internal
  })
}
</script>

<template>
  <div :class="['space-y-6']">
    <div v-if="modelValue.groups && modelValue.groups.length > 0">
      <div 
        v-for="group in visibleGroups()" 
        :key="group.order"
        :class="['space-y-4']"
      >
        <div 
          :class="[
            'group-header',
            'border-b-2 border-border/50 pb-3 mb-6',
            'shadow-sm'
          ]"
        >
          <div 
            :class="[
              'title',
              'space-y-2'
            ]"
          >
            <h3 
              :class="[
                'text-2xl font-bold text-foreground m-0',
                'pb-1'
              ]"
              v-text="group.display" 
            />
            <div 
              v-if="group.description"
              :class="[
                'hint',
                'text-sm text-muted-foreground',
                'leading-relaxed'
              ]"
              v-text="group.description" 
            />
          </div>
        </div>
        <div 
          v-for="name in filtered(group)" 
          :key="name"
          :class="['mb-4']"
        >
          <setting-input 
            :model-value="modelValue.data[name]" 
            :disabled="disabled" 
            @update:modelValue="updateValue(name, $event)" 
          />
        </div>
      </div>
      <div 
        v-if="grouplessVarsFiltered().length > 0"
        :class="['space-y-4']"
      >
        <div 
          :class="[
            'group-header',
            'border-b-2 border-border/50 pb-3 mb-6',
            'shadow-sm'
          ]"
        >
          <h3 
            :class="[
              'title',
              'text-2xl font-bold text-foreground m-0',
              'pb-1'
            ]"
            v-text="t('templates.NoGroup')" 
          />
        </div>
        <div 
          v-for="name in grouplessVarsFiltered()" 
          :key="name"
          :class="['mb-4']"
        >
          <setting-input 
            :model-value="modelValue.data[name]" 
            :disabled="disabled" 
            @update:modelValue="updateValue(name, $event)" 
          />
        </div>
      </div>
    </div>
    <div v-else :class="['space-y-4']">
      <div 
        v-for="(_, name) in modelValue.data" 
        :key="name"
        :class="['mb-4']"
      >
        <setting-input 
          :model-value="modelValue.data[name]" 
          :disabled="disabled" 
          @update:modelValue="updateValue(name, $event)" 
        />
      </div>
    </div>
  </div>
</template>