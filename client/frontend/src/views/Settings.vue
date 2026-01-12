<script setup>
import { ref, computed, inject, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Dropdown from '@/components/ui/Dropdown.vue'
import Icon from '@/components/ui/Icon.vue'
import TextField from '@/components/ui/TextField.vue'
import ThemeSetting from '@/components/ui/ThemeSetting.vue'
import Toggle from '@/components/ui/Toggle.vue'
import Loader from '@/components/ui/Loader.vue'
import Tabs from '@/components/ui/Tabs.vue'
import Tab from '@/components/ui/Tab.vue'

const emailProviderConfigs = {
  none: [],
  smtp: [
    { key: 'from', type: 'text' },
    { key: 'host', type: 'text' },
    { key: 'username', type: 'text' },
    { key: 'password', type: 'password' }
  ],
  mailgun: [
    { key: 'domain', type: 'text' },
    { key: 'from', type: 'text' },
    { key: 'key', type: 'password' }
  ],
  mailjet: [
    { key: 'domain', type: 'text' },
    { key: 'from', type: 'text' },
    { key: 'key', type: 'password' }
  ]
}

const { t } = useI18n()
const api = inject('api')
const toast = inject('toast')
const themeApi = inject('theme')
const config = inject('config')

// Logs de diagnóstico al inicio
console.log('🔍 [SETTINGS] Componente Settings inicializado')
console.log('   - api:', api)
console.log('   - api.settings:', api?.settings)
// COMENTADO: Referencias a licencias deshabilitadas
// if (api?.settings) {
//   console.log('   - api.settings.activateLicense:', typeof api?.settings?.activateLicense)
//   console.log('   - Métodos en api.settings:', Object.keys(api.settings))
// }

const masterUrl = ref('')
const panelTitle = ref('')
const registrationEnabled = ref(true)
const theme = ref('SkyPanel')
const themeSettings = ref([])
const discordWebhook = ref('')
const discordWebhookSystem = ref('')
const discordWebhookNode = ref('')

const emailProvider = ref('none')

// Crear emailProviders como ref reactivo que se actualiza cuando cambian las traducciones
const emailProviders = ref([])

// Función para obtener el objeto del proveedor por su valor
function getProviderObject(value) {
  if (!value) return null
  return emailProviders.value.find(p => p.value === value) || null
}

// Función para actualizar las opciones de proveedores
function updateEmailProviders() {
  const providers = []
  const seen = new Set() // Para evitar duplicados
  
  Object.keys(emailProviderConfigs).forEach(provider => {
    if (!seen.has(provider)) {
      seen.add(provider)
      const label = t('settings.emailProviders.' + provider)
      providers.push({
        label: label || provider.charAt(0).toUpperCase() + provider.slice(1),
    value: provider
  })
    }
})
  
  if (import.meta.env.DEV && !seen.has('debug')) {
    providers.push({ label: 'Debug', value: 'debug' })
  }
  
  // Asegurar que sea un array plano
  emailProviders.value = [...providers]
  console.log('📧 [EMAIL] Proveedores actualizados:', emailProviders.value)
  console.log('   - Cantidad:', emailProviders.value.length)
  console.log('   - Opciones:', providers.map(p => `${p.label} (${p.value})`).join(', '))
}

// Computed para obtener el objeto del proveedor seleccionado (para el dropdown con object mode)
const selectedEmailProvider = computed({
  get() {
    const provider = getProviderObject(emailProvider.value)
    return provider || null
  },
  set(newValue) {
    // Manejar cuando Multiselect actualiza el valor
    let providerValue = 'none'
    if (newValue && typeof newValue === 'object' && newValue !== null && 'value' in newValue) {
      providerValue = newValue.value
    } else if (typeof newValue === 'string') {
      providerValue = newValue
    }
    
    // Actualizar el valor sin llamar a emailProviderChanged para evitar loops
    emailProvider.value = providerValue
    // Actualizar campos directamente
    if (providerValue && emailProviderConfigs[providerValue]) {
      emailFields.value = emailProviderConfigs[providerValue]
    } else {
      emailFields.value = []
    }
  }
})

// Inicializar las opciones inmediatamente
updateEmailProviders()

// También inicializar en onMounted para asegurar que las traducciones estén disponibles
const emailFields = ref([])
const email = ref({
  from: '',
  domain: '',
  key: '',
  host: '',
  username: '',
  password: ''
})

const loading = ref(true)
const error = ref(null)

// Opciones de temas para el dropdown - inicializar con valor por defecto
const themeOptions = ref([{ label: 'Default', value: 'Default' }])

console.log('✅ themeOptions inicializado:', themeOptions.value)

// Variables de licencia - COMENTADO: Funcionalidad de licencias deshabilitada
// const licenseKey = ref('')
// const licenseStatus = ref('free') // 'free', 'pro', 'enterprise'
// const licenseActivating = ref(false)
// const licenseError = ref('')

function autofillMasterUrl() {
  masterUrl.value = window.location.origin
}

async function themeChanged() {
  themeSettings.value = await themeApi.getThemeSettings(theme.value)
}

function emailProviderChanged(provider) {
  console.log('📧 [EMAIL] Proveedor cambiado:', provider)
  console.log('   - Tipo:', typeof provider)
  console.log('   - Valor:', provider)
  
  // El provider puede venir como string directamente o como objeto con value
  let providerValue
  if (typeof provider === 'object' && provider !== null && 'value' in provider) {
    providerValue = provider.value
  } else if (typeof provider === 'string') {
    providerValue = provider
  } else {
    providerValue = 'none'
  }
  
  if (providerValue && emailProviderConfigs[providerValue]) {
    // No actualizar emailProvider.value aquí para evitar loops, solo actualizar campos
    emailFields.value = emailProviderConfigs[providerValue]
    console.log('✅ [EMAIL] Campos actualizados:', emailFields.value)
    console.log('   - Proveedor seleccionado:', providerValue)
  } else if (providerValue === 'none' || providerValue === '') {
    emailFields.value = []
    console.log('✅ [EMAIL] Proveedor establecido a "none"')
  } else {
    emailFields.value = []
    console.warn('⚠️ [EMAIL] Proveedor no encontrado:', providerValue)
  }
}

async function savePanelSettings() {
  await api.settings.set({
    'panel.settings.masterUrl': masterUrl.value,
    'panel.settings.companyName': panelTitle.value,
    'panel.settings.defaultTheme': theme.value,
    'panel.settings.themeSettings': themeApi.serializeThemeSettings(themeSettings.value),
    'panel.registrationEnabled': registrationEnabled.value,
    'panel.notifications.discordWebhook': discordWebhook.value,
    'panel.notifications.discordWebhookSystem': discordWebhookSystem.value,
    'panel.notifications.discordWebhookNode': discordWebhookNode.value
  })
  // Actualizar el config reactivo para que se refleje inmediatamente en el nav
  if (config && config.branding) {
    console.log('🔄 Actualizando nombre de empresa en navbar:', panelTitle.value)
    config.branding.name = panelTitle.value
    // Forzar actualización del título del documento
    document.title = panelTitle.value
  }
  toast.success(t('settings.Saved'))
}

async function saveEmailSettings() {
  const data = { 'panel.email.provider': emailProvider.value }
  emailFields.value.map(elem => {
    data['panel.email.' + elem.key] = email.value[elem.key]
  })
  await api.settings.set(data)
  toast.success(t('settings.Saved'))
}

async function testEmailSettings() {
  await api.settings.sendTestEmail()
  toast.success(t('settings.TestEmailSent'))
}

async function testDiscordWebhook() {
  try {
    await api.settings.sendTestDiscord()
    toast.success(t('settings.TestDiscordSent'))
  } catch (err) {
    toast.error(t('settings.TestDiscordFailed'))
  }
}

// Función auxiliar para cargar settings con manejo de errores
async function loadSetting(key, defaultValue = '') {
  try {
    const value = await api.settings.get(key)
    return value || defaultValue
  } catch (error) {
    console.warn(`Error loading setting ${key}:`, error)
    return defaultValue
  }
}

onMounted(async () => {
  console.group('⚙️ [SETTINGS] Inicializando componente Settings')
  console.log('🔍 Verificando objetos inyectados...')
  console.log('   - api:', api)
  console.log('   - api.settings:', api?.settings)
  console.log('   - api.auth:', api?.auth)
  console.log('   - api.auth.isLoggedIn():', api?.auth?.isLoggedIn?.())
  
  loading.value = true
  error.value = null
  
  try {
    // Verificar si el usuario está autenticado
    if (!api.auth.isLoggedIn()) {
      console.error('❌ Usuario no autenticado')
      error.value = 'Usuario no autenticado'
      loading.value = false
      console.groupEnd()
      return
    }
    
    console.log('✅ Usuario autenticado')
    
    // Verificar métodos disponibles en api.settings
    if (api.settings) {
      console.log('📋 Métodos disponibles en api.settings:')
      console.log('   - Keys:', Object.keys(api.settings))
      Object.keys(api.settings).forEach(key => {
        console.log(`   - ${key}:`, typeof api.settings[key])
      })
      
      // COMENTADO: Verificación de activateLicense deshabilitada
      // if (api.settings.activateLicense) {
      //   console.log('✅ activateLicense encontrado:', typeof api.settings.activateLicense)
      // } else {
      //   console.warn('⚠️ activateLicense NO encontrado en api.settings')
      // }
    } else {
      console.error('❌ api.settings no está disponible')
    }

    // Cargar todas las configuraciones con manejo de errores individual
    masterUrl.value = await loadSetting('panel.settings.masterUrl', '')
    panelTitle.value = await loadSetting('panel.settings.companyName', '')
    const regEnabled = await loadSetting('panel.registrationEnabled', 'false')
    registrationEnabled.value = (regEnabled === "true" || regEnabled === true)
    theme.value = await loadSetting('panel.settings.defaultTheme', 'SkyPanel')
    discordWebhook.value = await loadSetting('panel.notifications.discordWebhook', '')
    discordWebhookSystem.value = await loadSetting('panel.notifications.discordWebhookSystem', '')
    discordWebhookNode.value = await loadSetting('panel.notifications.discordWebhookNode', '')
    emailProvider.value = await loadSetting('panel.email.provider', 'none')
    updateEmailProviders() // Asegurar que las opciones estén actualizadas
    emailProviderChanged(emailProvider.value)
    
    // Cargar campos de email correctamente esperando las promesas
    const emailPromises = Object.keys(email.value).map(async key => {
      email.value[key] = await loadSetting('panel.email.' + key, '')
    })
    await Promise.all(emailPromises)
    
    // Cargar opciones de temas
    try {
      if (themeApi && typeof themeApi.getThemes === 'function') {
        const themes = themeApi.getThemes()
        if (Array.isArray(themes) && themes.length > 0) {
          themeOptions.value = themes.map(t => ({ label: t, value: t }))
        } else {
          themeOptions.value = []
        }
      } else {
        themeOptions.value = []
      }
    } catch (e) {
      console.warn('Error loading theme options:', e)
      themeOptions.value = []
    }
    
    // Cargar configuración del tema
    try {
      await themeChanged()
    } catch (e) {
      console.warn('Error loading theme settings:', e)
    }
    
    const themeSettingsValue = await loadSetting('panel.settings.themeSettings', '{}')
    try {
      themeSettings.value = themeApi.deserializeThemeSettings(
        themeSettings.value,
        themeSettingsValue
      )
    } catch (e) {
      console.warn('Error deserializing theme settings:', e)
      themeSettings.value = {}
    }
    
    // Cargar información de licencia - COMENTADO: Funcionalidad de licencias deshabilitada
    // licenseKey.value = await loadSetting('panel.license.key', '')
    // licenseStatus.value = await loadSetting('panel.license.status', 'free')
    
    console.log('✅ Configuración cargada exitosamente')
    // console.log('   - Licencia actual:', licenseKey.value || '(ninguna)')
    // console.log('   - Estado de licencia:', licenseStatus.value)
    
    // Verificar métodos disponibles en api.settings al final
    if (api?.settings) {
      console.log('📋 Métodos disponibles en api.settings:')
      const methods = Object.keys(api.settings)
      console.log('   Métodos:', methods)
      methods.forEach(key => {
        console.log(`   - ${key}:`, typeof api.settings[key])
      })
      
      // COMENTADO: Verificación de activateLicense deshabilitada
      // if (api.settings.activateLicense) {
      //   console.log('✅ activateLicense está disponible:', typeof api.settings.activateLicense)
      // } else {
      //   console.error('❌ activateLicense NO está disponible en api.settings')
      //   console.log('   Métodos disponibles:', methods.join(', '))
      // }
    } else {
      console.error('❌ api.settings no está disponible')
    }
    
    console.log('✅ [SETTINGS] Inicialización completada')
  } catch (err) {
    console.error('❌ Fatal error loading settings:', err)
    console.error('   - Error type:', err.constructor?.name)
    console.error('   - Error message:', err.message)
    console.error('   - Error stack:', err.stack)
    error.value = err.message || String(err)
  } finally {
    loading.value = false
    console.groupEnd()
  }
})

function updateThemeSetting(name, newSetting) {
  themeSettings.value[name] = newSetting
}

// ============================================================================
// FUNCIONALIDAD DE LICENCIAS - COMENTADA: Deshabilitada para no mostrar en la web
// ============================================================================

// Función para formatear la licencia automáticamente (Xxxx-xxxx-xxxx-xxxx)
// function formatLicenseKey(value) {
//   // Eliminar todos los caracteres que no sean alfanuméricos
//   const cleaned = value.replace(/[^A-Za-z0-9]/g, '').toUpperCase()
//   
//   // Limitar a 16 caracteres
//   const limited = cleaned.slice(0, 16)
//   
//   // Formatear en grupos de 4
//   const formatted = limited.match(/.{1,4}/g)?.join('-') || limited
//   
//   return formatted
// }

// Manejar el input de la licencia
// function onLicenseInput(event) {
//   const inputValue = event.target.value
//   const formatted = formatLicenseKey(inputValue)
//   licenseKey.value = formatted
//   licenseError.value = ''
//   
//   // Forzar actualización del input para mantener el formato
//   if (event.target.value !== formatted) {
//     event.target.value = formatted
//   }
// }

// Función para validar formato de licencia
// function isValidLicenseFormat(license) {
//   // Debe tener el formato Xxxx-xxxx-xxxx-xxxx (16 caracteres alfanuméricos)
//   const pattern = /^[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{4}$/
//   return pattern.test(license)
// }

// Función para activar la licencia
// async function activateLicense() {
//   console.group('🔐 [LICENCIA] Iniciando activación de licencia')
//   console.log('📋 Licencia ingresada:', licenseKey.value)
//   
//   if (!licenseKey.value || !isValidLicenseFormat(licenseKey.value)) {
//     console.error('❌ Formato de licencia inválido:', licenseKey.value)
//     licenseError.value = t('settings.LicenseInvalidFormat')
//     toast.error(t('settings.LicenseInvalidFormat'))
//     console.groupEnd()
//     return
//   }
//   
//   console.log('✅ Formato de licencia válido')
//   licenseActivating.value = true
//   licenseError.value = ''
//   
//   try {
//     // Verificar que el objeto API esté disponible
//     console.log('🔍 Verificando objeto API...')
//     console.log('   - api:', api)
//     console.log('   - api.settings:', api.settings)
//     
//     if (!api) {
//       console.error('❌ api no está disponible')
//       throw new Error('El objeto API no está disponible')
//     }
//     
//     if (!api.settings) {
//       console.error('❌ api.settings no está disponible')
//       console.log('   - Tipos disponibles en api:', Object.keys(api))
//       throw new Error('El objeto api.settings no está disponible. Tipos disponibles: ' + Object.keys(api).join(', '))
//     }
//     
//     // Verificar que el método esté disponible
//     console.log('🔍 Verificando método activateLicense...')
//     console.log('   - Tipo de api.settings.activateLicense:', typeof api.settings.activateLicense)
//     console.log('   - Es función?', typeof api.settings.activateLicense === 'function')
//     
//     if (typeof api.settings.activateLicense !== 'function') {
//       console.error('❌ activateLicense no es una función')
//       console.log('   - Métodos disponibles en api.settings:', Object.keys(api.settings))
//       throw new Error('El método activateLicense no está disponible. Métodos disponibles: ' + Object.keys(api.settings).join(', '))
//     }
//     
//     console.log('✅ Método activateLicense encontrado, llamando API...')
//     console.log('   - Endpoint: POST /api/settings/license/activate')
//     console.log('   - Body:', { key: licenseKey.value })
//     
//     // Llamar a la API para activar la licencia
//     const result = await api.settings.activateLicense(licenseKey.value)
//     
//     console.log('📥 Respuesta del servidor recibida:')
//     console.log('   - Resultado completo:', result)
//     console.log('   - Success:', result?.success)
//     console.log('   - Type:', result?.type)
//     console.log('   - Message:', result?.message)
//     console.log('   - Permissions:', result?.permissions)
//     
//     if (result && result.success) {
//       console.log('✅ Licencia activada exitosamente')
//       console.log('   - Tipo recibido del servidor:', result.type)
//       
//       // Asegurar que el tipo de licencia sea válido
//       const validTypes = ['free', 'pro', 'enterprise']
//       const receivedType = result.type?.toLowerCase()
//       
//       if (receivedType && validTypes.includes(receivedType)) {
//         licenseStatus.value = receivedType
//       } else {
//         // Si no viene tipo o es inválido, usar 'free' por defecto
//         licenseStatus.value = 'free'
//         console.warn('⚠️ Tipo de licencia no válido o no especificado, usando "free" por defecto')
//       }
//       
//       console.log('   - Tipo de licencia establecido:', licenseStatus.value)
//       
//       console.log('💾 Guardando configuración...')
//       await api.settings.set({
//         'panel.license.key': licenseKey.value,
//         'panel.license.status': licenseStatus.value
//       })
//       console.log('✅ Configuración guardada')
//       
//       // Mostrar mensaje según el tipo de licencia
//       let licenseTypeName
//       if (licenseStatus.value === 'enterprise') {
//         licenseTypeName = t('settings.LicenseEnterprise')
//       } else if (licenseStatus.value === 'pro') {
//         licenseTypeName = t('settings.LicensePro')
//       } else {
//         licenseTypeName = t('settings.LicenseFree')
//       }
//       
//       toast.success(`${t('settings.LicenseActivated')} - ${licenseTypeName}`)
//       console.log('🎉 Activación completada:', licenseTypeName)
//     } else {
//       console.error('❌ La respuesta indica fallo:', result)
//       licenseError.value = result?.message || t('settings.LicenseActivationFailed')
//       toast.error(licenseError.value)
//     }
//   } catch (err) {
//     console.error('❌ Error capturado en activateLicense:')
//     console.error('   - Tipo:', err.constructor.name)
//     console.error('   - Mensaje:', err.message)
//     console.error('   - Stack:', err.stack)
//     console.error('   - Error completo:', err)
//     
//     // Si es un error de red, mostrar más detalles
//     if (err.response) {
//       console.error('   - Status:', err.response.status)
//       console.error('   - Status Text:', err.response.statusText)
//       console.error('   - Data:', err.response.data)
//       console.error('   - Headers:', err.response.headers)
//     }
//     
//     licenseError.value = err.message || t('settings.LicenseActivationFailed')
//     toast.error(licenseError.value)
//   } finally {
//     licenseActivating.value = false
//     console.groupEnd()
//   }
// }
</script>

<template>
  <div 
    :class="[
      'settings',
      'w-full max-w-5xl ml-auto mr-0',
      'space-y-8'
    ]"
    style="margin-left: auto; margin-right: 0; padding-left: 16rem;"
  >
    <!-- Estado de carga -->
    <div 
      v-if="loading"
      :class="[
        'flex items-center justify-center',
        'min-h-[400px]'
      ]"
    >
      <loader />
    </div>

    <!-- Estado de error -->
    <div 
      v-else-if="error"
      :class="[
        'p-6',
        'bg-error/10 border-2 border-error/30 rounded-xl',
        'text-error-foreground'
      ]"
    >
      <h2 class="text-xl font-bold mb-2">{{ t('errors.ErrUnknownError') }}</h2>
      <p class="text-sm">{{ error }}</p>
      <p class="text-xs mt-2 opacity-75">
        Verifica la consola del navegador (F12 → Console) para más detalles.
      </p>
    </div>

    <!-- Contenido principal -->
    <template v-else>
      <!-- Título principal -->
      <div class="mb-8">
        <h1 
          :class="[
            'text-4xl font-bold text-foreground mb-2',
            'flex items-center gap-3'
          ]"
        >
          <icon name="hi-cog" class="text-primary" />
          {{ t('settings.PanelSettings') }}
        </h1>
            <p class="text-muted-foreground">
              {{ t('settings.SettingsDescription') }}
            </p>
      </div>

      <!-- Tabs para organizar las secciones -->
      <tabs anchors>
        <!-- Tab: Configuración General -->
        <tab id="general" :title="t('settings.PanelSettings')" icon="hi-cog">
    <div 
      :class="[
              'bg-background border-2 border-border/50 rounded-xl',
              'p-6 shadow-lg',
        'space-y-6'
      ]"
    >
            <div class="flex items-center gap-3 mb-4">
              <div 
                :class="[
                  'p-2 rounded-lg',
                  'bg-primary/10 text-primary'
                ]"
              >
                <icon name="hi-cog" class="text-2xl" />
              </div>
              <h2 
        :class="[
                  'text-2xl font-bold text-foreground m-0'
        ]"
              >
                {{ t('settings.GeneralSettings') }}
              </h2>
            </div>
            
            <div class="space-y-5">
      <div 
        :class="[
          'master-url',
          'flex items-end gap-2'
        ]"
      >
        <div class="flex-1">
          <text-field v-model="masterUrl" :label="t('settings.MasterUrl')" :hint="t('settings.MasterUrlHint')" />
        </div>
        <btn 
          variant="icon" 
          :class="['mb-5']"
          @click="autofillMasterUrl()"
        >
          <icon name="auto-fix" />
        </btn>
      </div>
      <text-field v-model="panelTitle" :label="t('settings.CompanyName')" />
      <toggle v-model="registrationEnabled" :label="t('settings.RegistrationEnabled')" :hint="t('settings.RegistrationEnabledHint')" />
      <dropdown v-model="theme" :options="themeOptions || []" :label="t('settings.DefaultTheme')" @change="themeChanged()" />
              <theme-setting v-for="(setting, name) in themeSettings" :key="name" :model-value="setting" @update:modelValue="updateThemeSetting(name, $event)" />
            </div>
            
            <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
              <btn color="primary" @click="savePanelSettings()">
                <icon name="save" />
                {{ t('settings.SavePanelSettings') }}
              </btn>
            </div>
          </div>
        </tab>

        <!-- Tab: Notificaciones Discord -->
        <tab id="discord" :title="t('settings.DiscordNotifications')" icon="hi-bell">
          <div 
            :class="[
              'bg-background border-2 border-border/50 rounded-xl',
              'p-6 shadow-lg',
              'space-y-6'
            ]"
          >
            <div class="flex items-center gap-3 mb-4">
              <div 
                :class="[
                  'p-2 rounded-lg',
                  'bg-purple-500/10 text-purple-500'
                ]"
              >
                <icon name="hi-bell" class="text-2xl" />
              </div>
              <h2 
                :class="[
                  'text-2xl font-bold text-foreground m-0'
                ]"
              >
                {{ t('settings.DiscordNotifications') }}
              </h2>
            </div>
            
            <div class="space-y-5">
      <text-field v-model="discordWebhook" :label="t('settings.DiscordWebhook')" :hint="t('settings.DiscordWebhookHint')" />
      <text-field v-model="discordWebhookSystem" :label="t('settings.DiscordWebhookSystem')" :hint="t('settings.DiscordWebhookSystemHint')" />
      <text-field v-model="discordWebhookNode" :label="t('settings.DiscordWebhookNode')" :hint="t('settings.DiscordWebhookNodeHint')" />
            </div>
            
      <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
              <btn color="primary" :disabled="!discordWebhook" @click="testDiscordWebhook()">
                <icon name="test" />
                {{ t('settings.TestDiscord') }}
              </btn>
              <btn color="primary" @click="savePanelSettings()">
                <icon name="save" />
                {{ t('settings.SavePanelSettings') }}
              </btn>
      </div>
    </div>
        </tab>

        <!-- Tab: Configuración de Email -->
        <tab id="email" :title="t('settings.EmailSettings')" icon="hi-mail">
    <div 
      :class="[
              'bg-background border-2 border-border/50 rounded-xl',
              'p-6 shadow-lg',
        'space-y-6'
      ]"
    >
            <div class="flex items-center gap-3 mb-4">
              <div 
                :class="[
                  'p-2 rounded-lg',
                  'bg-blue-500/10 text-blue-500'
                ]"
              >
                <icon name="hi-mail" class="text-2xl" />
              </div>
              <h2 
        :class="[
                  'text-2xl font-bold text-foreground m-0'
        ]"
              >
                {{ t('settings.EmailSettings') }}
              </h2>
            </div>
            
            <div class="space-y-5">
              <div>
                <dropdown 
                  v-model="selectedEmailProvider" 
                  :options="emailProviders" 
                  :label="t('settings.EmailProvider')"
                  :hint="t('settings.EmailProviderSelect')"
                  :placeholder="t('settings.EmailProviderPlaceholder')"
                  :searchable="false"
                  :object="true"
                />
                <!-- Debug info temporal -->
                <div v-if="emailProviders.length === 0" class="mt-2 p-2 bg-warning/10 border border-warning/30 rounded text-xs text-warning">
                  ⚠️ {{ t('settings.NoEmailProvidersAvailable') }}
      </div>
    </div>
              <div 
                v-if="emailFields.length > 0"
                :class="[
                  'p-4 rounded-xl border-2 border-border/50',
                  'bg-muted/30',
                  'space-y-4'
                ]"
              >
                <p class="text-sm font-medium text-foreground mb-3">
                  {{ t('settings.EmailConfigFor') }} <strong>{{ emailProviders.find(p => p.value === emailProvider)?.label || emailProvider }}</strong>
                </p>
                <text-field 
                  v-for="elem in emailFields" 
                  :key="elem.key" 
                  v-model="email[elem.key]" 
                  :type="elem.type" 
                  :label="t('settings.email.' + elem.key)" 
                />
              </div>
              <div 
                v-else
                :class="[
                  'p-4 rounded-xl border-2 border-border/50',
                  'bg-muted/20',
                  'text-center text-muted-foreground'
                ]"
              >
                <icon name="hi-information-circle" class="text-2xl mb-2 mx-auto" />
                <p class="text-sm">
                  {{ t('settings.EmailProviderSelectHint') }}
                </p>
              </div>
            </div>
            
            <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
              <btn color="primary" @click="testEmailSettings()">
                <icon name="test" />
                {{ t('settings.TestEmail') }}
              </btn>
              <btn color="primary" @click="saveEmailSettings()">
                <icon name="save" />
                {{ t('settings.SaveEmailSettings') }}
              </btn>
            </div>
          </div>
        </tab>

        <!-- Tab: Licencia - COMENTADO: Funcionalidad de licencias deshabilitada -->
        <!--
        <tab id="license" :title="t('settings.LicenseSettings')" icon="hi-key">
    <div 
      :class="[
              'bg-background border-2 border-border/50 rounded-xl',
              'p-6 shadow-lg',
        'space-y-6'
      ]"
    >
            <div class="flex items-center gap-3 mb-4">
              <div 
                :class="[
                  'p-2 rounded-lg',
                  licenseStatus === 'free' 
                    ? 'bg-muted/50 text-muted-foreground'
                    : licenseStatus === 'pro'
                    ? 'bg-primary/10 text-primary'
                    : 'bg-purple-500/10 text-purple-500'
                ]"
              >
                <icon name="hi-key" class="text-2xl" />
              </div>
              <h2 
        :class="[
                  'text-2xl font-bold text-foreground m-0'
        ]"
              >
                {{ t('settings.LicenseSettings') }}
              </h2>
            </div>
      
            <div class="space-y-5">
      <div 
        :class="[
          'p-4 rounded-xl border-2',
          licenseStatus === 'free' 
            ? 'bg-muted/50 border-border/50' 
            : licenseStatus === 'pro'
            ? 'bg-primary/10 border-primary/30'
            : 'bg-purple-500/10 border-purple-500/30'
        ]"
      >
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-muted-foreground mb-1">{{ t('settings.LicenseCurrentStatus') }}</p>
            <p :class="['text-lg font-semibold', licenseStatus === 'free' ? 'text-foreground' : licenseStatus === 'pro' ? 'text-primary' : 'text-purple-500']">
              {{ licenseStatus === 'free' ? t('settings.LicenseFree') : licenseStatus === 'pro' ? t('settings.LicensePro') : t('settings.LicenseEnterprise') }}
            </p>
          </div>
          <div 
            v-if="licenseStatus !== 'free'"
            :class="[
              'px-3 py-1 rounded-full text-xs font-bold uppercase',
              licenseStatus === 'pro' ? 'bg-primary/20 text-primary' : 'bg-purple-500/20 text-purple-500'
            ]"
          >
                    {{ t('settings.LicenseActive') }}
          </div>
        </div>
      </div>
      
      <div>
        <label 
          :class="[
            'block text-sm font-medium text-foreground mb-2'
          ]"
        >
          {{ t('settings.LicenseKey') }}
        </label>
        <div class="relative">
          <input
            v-model="licenseKey"
            type="text"
            :placeholder="t('settings.LicenseKeyPlaceholder')"
            maxlength="19"
            :class="[
              'w-full px-4 py-3',
              'bg-background border-2 rounded-xl',
              'font-mono text-lg tracking-widest uppercase',
              'transition-all duration-200',
              'shadow-sm hover:shadow-md focus:shadow-lg',
              'focus:outline-none focus:ring-2',
              licenseError
                ? 'border-error focus:border-error focus:ring-error/20'
                : 'border-input/50 focus:border-ring focus:ring-ring/20',
              licenseActivating && 'opacity-60 cursor-not-allowed'
            ]"
            :disabled="licenseActivating"
            @input="onLicenseInput($event)"
            @keyup.enter="activateLicense"
          />
          <icon 
            v-if="licenseKey && isValidLicenseFormat(licenseKey) && !licenseActivating"
            name="check-circle" 
            class="absolute right-3 top-1/2 -translate-y-1/2 text-primary"
          />
        </div>
        <p 
          v-if="licenseError"
          class="mt-2 text-sm text-error"
        >
          {{ licenseError }}
        </p>
        <p 
          v-else
          class="mt-2 text-xs text-muted-foreground"
        >
          {{ t('settings.LicenseKeyHint') }}
        </p>
              </div>
      </div>
      
      <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
        <btn 
          color="primary" 
          :disabled="!licenseKey || !isValidLicenseFormat(licenseKey) || licenseActivating"
          @click="activateLicense()"
        >
          <icon v-if="licenseActivating" name="loader" class="animate-spin" />
          <icon v-else name="check" />
          {{ licenseActivating ? t('settings.LicenseActivating') : t('settings.LicenseActivate') }}
        </btn>
      </div>
    </div>
        </tab>
        -->
      </tabs>
    </template>
  </div>
</template>
