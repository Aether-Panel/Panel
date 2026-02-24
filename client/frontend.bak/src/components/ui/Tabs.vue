<script>
import { ref, onMounted, onUnmounted, provide, nextTick } from 'vue'
import Icon from './Icon.vue'

export default {
  components: {
    Icon
  },
  props: {
    anchors: { type: Boolean, default: () => false }
  },
  emits: ['tabChanged'],
  setup(props, { slots, emit }) {
    const tabButtons = ref(null)
    const needsScroller = ref(false)

    const tabs = ref([])
    const activeKey = ref('')
    provide('activeKey', activeKey)

    function setActive(key) {
      activeKey.value = key

      if (props.anchors) {
        history.replaceState(history.state, '', '#' + key)
      }

      // deferring emit to next tick to ensure the tab content has changed
      nextTick(() => emit('tabChanged', key))
    }

    function onResize() {
      needsScroller.value = tabButtons.value.scrollWidth > tabButtons.value.offsetWidth
    }

    function scroll(dir) {
      const dist = (tabButtons.value.offsetWidth / 2)
      tabButtons.value.scrollTo({
        behavior: 'smooth',
        left: tabButtons.value.scrollLeft + (dir === 'right' ? dist : dist * -1)
      })
    }

    onMounted(() => {
      window.addEventListener('resize', onResize)
      nextTick(() => onResize())

      tabs.value = slots
        .default()
        .filter(e => e && e.props && e.props.title)
        .map(e => {
          return {
            key: e.props.id || e.props.title.toLowerCase().replace(/ /g, '-'),
            title: e.props.title,
            icon: e.props.icon,
            hotkey: e.props.hotkey
          }
        })

      if (props.anchors && tabs.value.length > 0 && location.hash) {
        const tab = tabs.value.find(e => e.key === location.hash.substring(1))
        if (tab) setActive(tab.key)
      }

      if (tabs.value.length > 0 && !activeKey.value) {
        setActive(tabs.value[0].key)
      }
    })

    onUnmounted(() => {
      window.removeEventListener('resize', onResize)
    })

    return { tabButtons, needsScroller, tabs, activeKey, setActive, scroll }
  }
}
</script>

<template>
  <div 
    :class="[
      'relative max-w-[calc(100vw-4rem)]'
    ]"
  >
    <!-- Botón de scroll izquierdo -->
    <div 
      v-if="needsScroller" 
      :class="[
        'absolute left-[-0.325rem] top-[0.625rem] z-20',
        'w-[1.125rem] h-[1.125rem] rounded-full',
        'bg-background shadow-md cursor-pointer',
        'flex items-center justify-center',
        'text-2xl',
        'hover:bg-primary/15 transition-colors',
        'hidden [@media(hover:hover)]:block'
      ]"
      @click="scroll('left')"
    >
      <icon name="chevron-left" />
    </div>
    
    <!-- Botón de scroll derecho -->
    <div 
      v-if="needsScroller" 
      :class="[
        'absolute right-[-0.325rem] top-[0.625rem] z-10',
        'w-[1.125rem] h-[1.125rem] rounded-full',
        'bg-background shadow-md cursor-pointer',
        'flex items-center justify-center',
        'text-2xl',
        'hover:bg-primary/15 transition-colors',
        'hidden [@media(hover:hover)]:block'
      ]"
      @click="scroll('right')"
    >
      <icon name="chevron-right" />
    </div>
    
    <!-- Botones de pestañas -->
    <div 
      ref="tabButtons"
      :class="[
        'flex mb-4 overflow-x-auto',
        'scrollbar-none',
        '[-ms-overflow-style:none] [scrollbar-width:none]',
        '[&::-webkit-scrollbar]:hidden'
      ]"
    >
      <button
        v-for="tab in tabs"
        :key="tab.key"
        v-hotkey="tab.hotkey"
        type="button"
        :class="[
          'flex-1 flex-shrink-0 cursor-pointer',
          'text-center py-3 px-4 text-lg font-medium',
          'border-b-[3px] whitespace-nowrap',
          'transition-all duration-200 ease-in-out',
          'relative',
          tab.key === activeKey
            ? 'border-primary text-primary font-semibold bg-primary/5'
            : 'border-transparent text-foreground/70 hover:text-foreground hover:bg-primary/5 hover:border-primary/30',
          tab.icon ? 'flex flex-row items-center justify-start gap-2' : 'flex items-center justify-center',
          'mode-dark-highcontrast:hover:bg-transparent',
          'mode-dark-highcontrast:hover:[&_.title]:underline',
          'mode-dark-highcontrast:hover:[&_.title]:decoration-dashed',
          'active:scale-[0.98]'
        ]"
        @click="setActive(tab.key)"
      >
        <img 
          v-if="tab.icon === 'console'"
          src="/img/resources/terminal.png"
          alt="Console"
          :class="['w-6 h-6 object-contain flex-shrink-0']"
        />
        <icon 
          v-else-if="tab.icon" 
          :name="tab.icon" 
          :class="['text-2xl flex-shrink-0']"
        />
        <span 
          :class="[
            'title',
            'text-base'
          ]"
          v-text="tab.title" 
        />
      </button>
    </div>
    
    <!-- Contenido de las pestañas -->
    <slot />
  </div>
</template>
