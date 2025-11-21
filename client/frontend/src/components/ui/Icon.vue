<script setup>
import { computed } from 'vue'
import { OhVueIcon, addIcons } from 'oh-vue-icons'
import * as HiIcons from 'oh-vue-icons/icons/hi'
import * as MdIcons from 'oh-vue-icons/icons/md'
import * as BiIcons from 'oh-vue-icons/icons/bi'
import * as RiIcons from 'oh-vue-icons/icons/ri'

// Agregar todos los iconos de Heroicons, Material Design, Bootstrap y Remix
addIcons(...Object.values(HiIcons), ...Object.values(MdIcons), ...Object.values(BiIcons), ...Object.values(RiIcons))

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
  'chevron-left': 'hi-chevron-left',
  'chevron-right': 'hi-chevron-right',
  'chevron-up': 'hi-chevron-up',
  'chevron-down': 'hi-chevron-down',
  'nav-menu': 'hi-bars-3',
  
  // Acciones
  'save': 'hi-document-arrow-down',
  'remove': 'hi-x-circle',
  'delete': 'hi-trash',
  'close': 'hi-x-mark',
  'plus': 'hi-plus',
  'check': 'hi-check',
  'apply': 'hi-check-circle',
  
  // Archivos
  'folder': 'hi-folder',
  'file': 'hi-document',
  'file-upload': 'hi-cloud-arrow-up',
  'folder-upload': 'hi-folder-open',
  'search': 'hi-magnifying-glass',
  
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

// Función para obtener el nombre del icono
const iconName = computed(() => {
  // Si el nombre ya está en formato oh-vue-icons, usarlo directamente
  if (props.name && props.name.includes('-') && (props.name.startsWith('hi-') || props.name.startsWith('md-') || props.name.startsWith('bi-') || props.name.startsWith('ri-'))) {
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
  <OhVueIcon
    :name="iconName"
    :class="iconClasses"
  />
</template>
