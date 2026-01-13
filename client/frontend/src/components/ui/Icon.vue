<script setup>
import { computed } from 'vue'
import { OhVueIcon, addIcons } from 'oh-vue-icons'
import * as HiIcons from 'oh-vue-icons/icons/hi'
import * as MdIcons from 'oh-vue-icons/icons/md'
import * as BiIcons from 'oh-vue-icons/icons/bi'
import * as RiIcons from 'oh-vue-icons/icons/ri'
import { Gi3DHammer } from 'oh-vue-icons/icons/gi'

// Agregar todos los iconos de Heroicons, Material Design, Bootstrap y Remix, y especificos de Game Icons
addIcons(...Object.values(HiIcons), ...Object.values(MdIcons), ...Object.values(BiIcons), ...Object.values(RiIcons), Gi3DHammer)

const props = defineProps({
  name: { type: String, required: true },
  spin: { type: Boolean, required: false }
})

// Mapeo de nombres antiguos a iconos de oh-vue-icons (Heroicons principalmente)
const iconMap = {
  // Navegación
  'server': 'hi-server',
  'node': 'hi-cube',
  'users': 'hi-users',
  'template': 'hi-document-text',
  'settings': 'hi-cog',
  'uptime': 'hi-chart-bar',
  'logout': 'hi-arrow-right-on-rectangle',
  'sign-out': 'hi-arrow-right-on-rectangle',
  'log-out': 'hi-arrow-right-on-rectangle',
  'chevron-left': 'hi-chevron-left',
  'chevron-right': 'hi-chevron-right',
  'chevron-up': 'hi-chevron-up',
  'chevron-down': 'hi-chevron-down',
  'nav-menu': 'hi-bars-3',
  
  // Acciones
  'save': 'hi-document-arrow-down',
  'remove': 'hi-x-circle',
  'delete': 'hi-trash',
  'close': 'hi-x-circle',
  'plus': 'hi-plus',
  'check': 'hi-check',
  'apply': 'hi-check-circle',
  
  // Archivos
  'folder': 'hi-folder',
  'file': 'hi-document',
  'file-upload': 'hi-arrow-up',
  'folder-upload': 'hi-folder-open',
  'archive': 'hi-folder',
  'extract': 'hi-arrow-up',
  'search': 'hi-magnifying-glass',
  'refresh': 'hi-arrow-up',
  
  // Servidor
  'play': 'hi-play',
  'stop': 'hi-stop',
  'restart': 'hi-arrow-path',
  'kill': 'hi-x-circle',
  'install': 'hi-arrow-down-tray',
  'console': 'hi-command-line',
  'stats': 'hi-chart-bar',
  'files': 'hi-folder',
  'backup': 'hi-server-stack',
  'sftp': 'hi-server',
  'admin': 'hi-shield-check',
  'tasks': 'hi-clipboard-document-list',
  
  // UI
  'email': 'hi-envelope',
  'lock': 'hi-lock-closed',
  'eye': 'hi-eye',
  'eye-off': 'hi-eye-slash',
  'test': 'hi-bolt',
  'auto-fix': 'hi-sparkles',
  'loading': 'hi-arrow-path'
}

// Verificar si es un icono de Devicon
const isDevicon = computed(() => props.name && props.name.startsWith('devicon:'))
const deviconName = computed(() => props.name ? props.name.replace('devicon:', '') : '')
const deviconUrl = computed(() => {
  if (isDevicon.value) {
    return `https://cdn.jsdelivr.net/gh/devicons/devicon@latest/icons/${deviconName.value}/${deviconName.value}-original.svg`
  }
  return null
})

// Función para obtener el nombre del icono
const iconName = computed(() => {
  // Si es un icono de Devicon, no usar oh-vue-icons
  if (isDevicon.value) {
    return null
  }
  // Si el nombre ya está en formato oh-vue-icons, usarlo directamente
  if (props.name && props.name.includes('-') && (props.name.startsWith('hi-') || props.name.startsWith('md-') || props.name.startsWith('bi-') || props.name.startsWith('ri-') || props.name.startsWith('gi-'))) {
    return props.name
  }
  // Buscar en el mapeo
  if (props.name && iconMap[props.name]) {
    return iconMap[props.name]
  }
  // Intentar con el nombre directamente agregando el prefijo hi-
  // Convertir a kebab-case si es necesario
  if (!props.name) {
    return 'hi-question-mark-circle'
  }
  const kebabName = props.name.replace(/([a-z])([A-Z])/g, '$1-$2').toLowerCase()
  return `hi-${kebabName}`
})

const iconClasses = computed(() => {
  return [
    'inline-flex items-center justify-center',
    'align-middle',
    props.spin && 'animate-spin'
  ].filter(Boolean)
})
</script>

<template>
  <img 
    v-if="isDevicon" 
    :src="deviconUrl" 
    :alt="deviconName"
    :class="iconClasses"
    style="width: 1em; height: 1em; display: inline-flex; align-items: center; justify-content: center; object-fit: contain;"
  />
  <OhVueIcon
    v-else
    :name="iconName"
    :class="iconClasses"
    style="font-size: 1.2em;"
  />
</template>

<style scoped>
img {
  /* Asegurar que las imágenes de Devicon se vean bien */
  max-width: 100%;
  max-height: 100%;
}
</style>
