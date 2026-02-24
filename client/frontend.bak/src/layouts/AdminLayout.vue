<script setup>
import { ref, inject, watch, onMounted, onUnmounted, onUpdated } from 'vue'
import AdminSidebar from '@/components/admin/AdminSidebar.vue'
import Topbar from '@/components/ui/Topbar.vue'

const api = inject('api')
const sidebarClosedBelow = inject('sidebarClosedBelow')
const ltr = inject('ltr')
const sidebarClosed = ref(window.innerWidth < sidebarClosedBelow.value)
const sidebarMini = ref(false)
const sidebarRef = ref(null)
const user = ref(undefined)
let lastWidth = window.innerWidth

onMounted(async () => {
  window.addEventListener('resize', onResize)
  document.documentElement.style.setProperty('--inner-height', `${window.innerHeight}px`)
  sidebarMini.value = localStorage.getItem('admin-sidebar.mini') === 'true'
  
  if (api.auth.isLoggedIn()) {
    user.value = await api.self.get()
  }
})

onUpdated(async () => {
  if (api.auth.isLoggedIn() && user.value == undefined) {
    user.value = await api.self.get()
  }
  if (!api.auth.isLoggedIn() && user.value) {
    user.value = undefined
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', onResize)
})

watch(() => sidebarRef.value?.mini, (newValue) => {
  if (newValue !== undefined) {
    sidebarMini.value = newValue
  }
}, { immediate: true, flush: 'post' })

function onResize() {
  if (lastWidth < sidebarClosedBelow.value && window.innerWidth >= sidebarClosedBelow.value) {
    sidebarClosed.value = false
  } else if (lastWidth >= sidebarClosedBelow.value && window.innerWidth < sidebarClosedBelow.value) {
    sidebarClosed.value = true
  }

  lastWidth = window.innerWidth
  document.documentElement.style.setProperty('--inner-height', `${window.innerHeight}px`)
}

function maybeCloseSidebar() {
  if (window.innerWidth < sidebarClosedBelow.value) {
    sidebarClosed.value = true
  }
}
</script>

<template>
  <div class="admin-layout">
    <topbar 
      :class="'sidebar-exists'" 
      :user="user"
      @toggleSidebar="sidebarClosed = !sidebarClosed" 
    />
    <admin-sidebar 
      ref="sidebarRef"
      :closed="sidebarClosed" 
      :right="!ltr" 
    />
    <main 
      :class="[
        'pt-12 pb-6',
        'min-h-screen bg-background',
        'flex flex-col items-center',
        // Padding fijo siempre presente para el sidebar (máximo cuando está abierto)
        // Esto evita que el contenido se mueva cuando el sidebar cambia
        !ltr ? 'md:pl-[16rem] lg:pl-[16rem]' : 'md:pr-[16rem] lg:pr-[16rem]',
        // Padding adicional para el contenido
        !ltr ? 'pr-4 lg:pr-6' : 'pl-4 lg:pl-6'
      ]"
      style="
        -webkit-font-smoothing: antialiased;
        -moz-osx-font-smoothing: grayscale;
        text-rendering: optimizeLegibility;
        isolation: isolate;
      "
      @click="maybeCloseSidebar()"
    >
      <div class="w-full max-w-7xl px-4 lg:px-6">
        <router-view />
      </div>
    </main>
  </div>
</template>

<style scoped>
.admin-layout {
  width: 100%;
  min-height: 100vh;
}
</style>

