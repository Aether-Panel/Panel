<script setup>
import { getCurrentInstance, onMounted, onUnmounted, ref, nextTick } from 'vue'
import Icon from '@/components/ui/Icon.vue'

const props = defineProps({
  actions: { type: Array, required: true },
  title: { type: String, required: true }
})

const source = ref(null)
const style = ref({})
const suggestedDirection = ref(null)
const menu = ref(null)

function canOpen() {
  return Array.isArray(props.actions) && props.actions.length !== 0
}

function onClick(event) {
  if (!canOpen()) return
  handleEvent(event, 'activator')
}

function onContext(event) {
  if (!canOpen()) return
  event.preventDefault()
  handleEvent(event)
}

function handleEvent(event, eventSource) {
  const bodyWidth = document.body.scrollWidth
  const bodyHeight = document.body.scrollHeight
  source.value = eventSource || event.pointerType
  nextTick(() => {
    const suggestX = (event.pageX + menu.value.clientWidth) > bodyWidth ? 'left' : 'right'
    const suggestY = (event.pageY + menu.value.clientHeight) > bodyHeight ? 'top' : 'bottom'
    suggestedDirection.value = `${suggestY} ${suggestX}`
    
    // Calcular posición dinámica
    let left = event.pageX
    let top = event.pageY
    
    if (suggestX === 'left') {
      left = event.pageX - menu.value.clientWidth
    }
    if (suggestY === 'top') {
      top = event.pageY - menu.value.clientHeight
    }
    
    style.value = {
      top: `${top}px`,
      left: `${left}px`,
      '--x': `${event.x}px`,
      '--y': `${event.y}px`,
      '--layer-x': `${event.layerX}px`,
      '--layer-y': `${event.layerY}px`,
      '--client-x': `${event.clientX}px`,
      '--client-y': `${event.clientY}px`,
      '--page-x': `${event.pageX}px`,
      '--page-y': `${event.pageY}px`,
      '--screen-x': `${event.screenX}px`,
      '--screen-y': `${event.screenY}px`,
      '--width': `${menu.value.clientWidth}px`,
      '--height': `${menu.value.clientHeight}px`
    }
  })
}

function click(action) {
  action.action()
  close()
}

function close() {
  source.value = null
}

onMounted(() => {
  getCurrentInstance().vnode.el.parentNode.addEventListener('contextmenu', onContext, true)
})

onUnmounted(() => {
  getCurrentInstance().vnode.el.parentNode.removeEventListener('contextmenu', onContext, true)
})
</script>

<template>
  <slot name="activator" :can-open="canOpen()" :on-click="onClick" />
  <div 
    v-if="source !== null" 
    :class="[
      'context-menu-wrapper',
      'fixed inset-0 z-40',
      'pointer-events-none'
    ]"
  >
    <ul 
      ref="menu" 
      v-click-outside.stop="close" 
      v-hotkey="'Escape'" 
      :class="[
        'context-menu',
        `source-${source}`,
        'absolute min-w-[220px]',
        'bg-background/95 backdrop-blur-md border-2 border-border/50 rounded-xl shadow-2xl',
        'list-none m-0 p-2',
        'transform z-50',
        'pointer-events-auto',
        'animate-in fade-in zoom-in-95 duration-200'
      ]" 
      :style="style"
      :data-suggested-direction="suggestedDirection" 
      @hotkey="close()"
    >
      <slot name="title">
        <li 
          :class="[
            'context-title',
            'font-bold text-lg mb-2 px-3 py-2',
            'text-foreground border-b-2 border-border/50'
          ]"
          v-text="title" 
        />
      </slot>
      <slot 
        v-for="action, index in actions" 
        :key="index" 
        name="action" 
        :action="{...action, action: () => click(action)}"
      >
        <li 
          :class="[
            'context-action',
            'px-2',
            action.class || ''
          ]"
        >
          <a 
            v-hotkey="action.hotkey" 
            tabindex="0" 
            :class="[
              'flex items-center gap-2',
              'px-3 py-2.5 rounded-lg',
              'cursor-pointer transition-all duration-200',
              'text-foreground hover:bg-primary/10 active:bg-primary/15',
              'focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:ring-offset-background',
              'mode-dark-highcontrast:hover:bg-transparent mode-dark-highcontrast:hover:underline mode-dark-highcontrast:hover:decoration-dashed'
            ]"
            @click.stop="click(action)" 
            @keydown.enter.stop="click(action)"
          >
            <icon 
              v-if="action.icon" 
              :name="action.icon"
              :class="[
                'text-xl',
                'flex-shrink-0'
              ]"
            />
            <span 
              :class="[
                'label',
                'flex-grow font-medium'
              ]"
              v-text="action.label" 
            />
          </a>
        </li>
      </slot>
    </ul>
  </div>
</template>
