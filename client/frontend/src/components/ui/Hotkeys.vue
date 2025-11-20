<script setup>
import { ref, inject, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

const getHotkeys = inject('hotkeys')
const { t } = useI18n()
const route = useRoute()
const ctxOverrides = {
  'TemplateView': 'Template',
  'TemplateCreate': 'Template'
}
const hotkeys = ref(currentHotkeys())

function getContext() {
  return ctxOverrides[route.name] || route.name
}

function currentHotkeys() {
  const h = getHotkeys()
  const res = {
    global: h.root.flat().filter(e => e !== '?' && e !== 'Shift+?' && e !== 'Escape'),
    contextual: (h[route.name] || [])
      .flat()
      .filter(e => e !== 'Escape')
      .filter(e => e !== 'Control+a')
      .filter(e => !/^. \d \d$/.test(e))
      .sort(),
    context: getContext()
  }

  return res
}

watch(
  () => route.name,
  async () => {
    setTimeout(() => {
      hotkeys.value = currentHotkeys()
    }, 500)
  }
)

onMounted(() => {
  hotkeys.value = currentHotkeys()
})
</script>

<template>
  <div 
    :class="[
      'hotkey-list',
      'grid grid-cols-1 md:grid-cols-2 gap-6',
      'max-w-none'
    ]"
  >
    <div 
      v-if="hotkeys.contextual.length > 0" 
      :class="[
        'contextual',
        'md:order-2'
      ]"
    >
      <h3 
        :class="[
          'text-2xl font-bold mb-4 mt-0 text-foreground',
          'pb-2 border-b-2 border-border/50'
        ]"
        v-text="t(`hotkeys.${hotkeys.context}.Title`)" 
      />
      <div 
        v-for="keys in hotkeys.contextual" 
        :key="keys" 
        :class="[
          'hotkey',
          'flex items-center gap-3 mb-2'
        ]"
      >
        <span 
          :class="[
            'keys',
            'flex items-center gap-1 flex-shrink-0'
          ]"
        >
          <span 
            v-for="key in keys.split(' ')" 
            :key="key" 
            :class="[
              'key',
              'border-2 border-border/70 rounded-lg px-2.5 py-1.5',
              'min-w-[1.75rem] h-7',
              'flex items-center justify-center',
              'text-xs font-mono font-semibold',
              'bg-muted/70 text-foreground',
              'shadow-sm hover:shadow-md transition-all duration-200',
              'hover:border-primary/50'
            ]"
            v-text="key" 
          />
        </span>
        <span 
          :class="[
            'description',
            'text-foreground flex-grow'
          ]"
          v-text="t(`hotkeys.${hotkeys.context}.${keys}`)" 
        />
      </div>
    </div>
    <div 
      :class="[
        'global',
        'md:order-1'
      ]"
    >
      <h3 
        :class="[
          'text-2xl font-bold mb-4 mt-0 text-foreground',
          'pb-2 border-b-2 border-border/50'
        ]"
        v-text="t('hotkeys.Global.Title')" 
      />
      <div 
        v-for="keys in hotkeys.global" 
        :key="keys" 
        :class="[
          'hotkey',
          'flex items-center gap-3 mb-2'
        ]"
      >
        <span 
          :class="[
            'keys',
            'flex items-center gap-1 flex-shrink-0'
          ]"
        >
          <span 
            v-for="key in keys.split(' ')" 
            :key="key" 
            :class="[
              'key',
              'border-2 border-border/70 rounded-lg px-2.5 py-1.5',
              'min-w-[1.75rem] h-7',
              'flex items-center justify-center',
              'text-xs font-mono font-semibold',
              'bg-muted/70 text-foreground',
              'shadow-sm hover:shadow-md transition-all duration-200',
              'hover:border-primary/50'
            ]"
            v-text="key" 
          />
        </span>
        <span 
          :class="[
            'description',
            'text-foreground flex-grow'
          ]"
          v-text="t(`hotkeys.Global.${keys}`)" 
        />
      </div>
    </div>
  </div>
</template>
