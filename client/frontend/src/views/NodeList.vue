<script setup>
import { ref, inject, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const { t } = useI18n()
const api = inject('api')
const nodesLoaded = ref(false)
const nodes = ref([])
const firstEntry = ref(null)

onMounted(async () => {
  nodes.value = await api.node.list()
  nodesLoaded.value = true
})

function setFirstEntry(ref) {
  if (!firstEntry.value) firstEntry.value = ref
}

function focusList() {
  firstEntry.value.$el.focus()
}
</script>

<template>
  <div 
    :class="[
      'nodelist',
      'w-full max-w-7xl mx-auto',
      'space-y-6'
    ]"
  >
    <h1 
      :class="[
        'text-3xl font-bold text-foreground mb-6',
        'pb-3 border-b-2 border-border/50'
      ]"
      v-text="t('nodes.Nodes')" 
    />
    <div 
      v-hotkey="'l'" 
      :class="['list']"
      @hotkey="focusList()"
    >
      <div 
        v-for="node in nodes" 
        :key="node.name" 
        :class="['list-item']"
      >
        <router-link 
          :ref="setFirstEntry" 
          :to="{ name: 'NodeView', params: { id: node.id } }"
          :class="['block']"
        >
          <div 
            :class="[
              'node',
              'w-full'
            ]"
          >
            <span 
              :class="[
                'title',
                'block text-lg font-semibold text-foreground'
              ]"
            >
              {{node.name}}
            </span>
            <span 
              :class="[
                'subline',
                'block text-sm text-muted-foreground mt-1'
              ]"
            >
              {{node.publicHost + ':' + node.publicPort}}
            </span>
          </div>
        </router-link>
      </div>
      <div 
        v-if="!nodesLoaded" 
        :class="['list-item']"
      >
        <loader small />
      </div>
      <div 
        v-if="$api.auth.hasScope('nodes.create')" 
        :class="['list-item']"
      >
        <router-link 
          v-hotkey="'c'" 
          :to="{ name: 'NodeCreate' }"
          :class="['block']"
        >
          <div 
            :class="[
              'createLink',
              'flex items-center gap-2 px-4 py-3',
              'bg-primary/10 border-2 border-primary/30 rounded-xl',
              'text-primary font-semibold',
              'hover:bg-primary/20 hover:border-primary/50',
              'transition-all duration-200',
              'shadow-sm hover:shadow-md',
              'cursor-pointer'
            ]"
          >
            <icon name="plus" />
            <span>{{ t('nodes.Add') }}</span>
          </div>
        </router-link>
      </div>
    </div>
  </div>
</template>
