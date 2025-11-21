<script setup>
import { ref, inject, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Dropdown from '@/components/ui/Dropdown.vue'
import Icon from '@/components/ui/Icon.vue'
import TextField from '@/components/ui/TextField.vue'
import ThemeSetting from '@/components/ui/ThemeSetting.vue'
import Toggle from '@/components/ui/Toggle.vue'
import Loader from '@/components/ui/Loader.vue'

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

const masterUrl = ref('')
const panelTitle = ref('')
const registrationEnabled = ref(true)
const theme = ref('PufferPanel')
const themeSettings = ref([])
const discordWebhook = ref('')
const discordWebhookSystem = ref('')
const discordWebhookNode = ref('')

const emailProviders = []
Object.keys(emailProviderConfigs).map(provider => {
  emailProviders.push({
    label: t('settings.emailProviders.' + provider),
    value: provider
  })
})
if (import.meta.env.DEV) {
  emailProviderConfigs.debug = []
  emailProviders.push({ label: 'Debug', value: 'debug' })
}
const emailProvider = ref('none')
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

function autofillMasterUrl() {
  masterUrl.value = window.location.origin
}

async function themeChanged() {
  themeSettings.value = await themeApi.getThemeSettings(theme.value)
}

function emailProviderChanged(provider) {
  emailFields.value = emailProviderConfigs[provider]
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
    config.branding.name = panelTitle.value
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
  loading.value = true
  error.value = null
  
  try {
    // Verificar si el usuario está autenticado
    if (!api.auth.isLoggedIn()) {
      error.value = 'Usuario no autenticado'
      loading.value = false
      return
    }

    // Cargar todas las configuraciones con manejo de errores individual
    masterUrl.value = await loadSetting('panel.settings.masterUrl', '')
    panelTitle.value = await loadSetting('panel.settings.companyName', '')
    const regEnabled = await loadSetting('panel.registrationEnabled', 'false')
    registrationEnabled.value = (regEnabled === "true" || regEnabled === true)
    theme.value = await loadSetting('panel.settings.defaultTheme', 'PufferPanel')
    discordWebhook.value = await loadSetting('panel.notifications.discordWebhook', '')
    discordWebhookSystem.value = await loadSetting('panel.notifications.discordWebhookSystem', '')
    discordWebhookNode.value = await loadSetting('panel.notifications.discordWebhookNode', '')
    emailProvider.value = await loadSetting('panel.email.provider', 'none')
    emailProviderChanged(emailProvider.value)
    
    // Cargar campos de email correctamente esperando las promesas
    const emailPromises = Object.keys(email.value).map(async key => {
      email.value[key] = await loadSetting('panel.email.' + key, '')
    })
    await Promise.all(emailPromises)
    
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
  } catch (err) {
    console.error('Fatal error loading settings:', err)
    error.value = err.message || String(err)
  } finally {
    loading.value = false
  }
})

function updateThemeSetting(name, newSetting) {
  themeSettings.value[name] = newSetting
}
</script>

<template>
  <div 
    :class="[
      'settings',
      'w-full max-w-4xl mx-auto',
      'space-y-8',
      'p-4'
    ]"
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
    <div 
      :class="[
        'panel',
        'space-y-6'
      ]"
    >
      <h1 
        :class="[
          'text-3xl font-bold text-foreground mb-6',
          'pb-3 border-b-2 border-border/50'
        ]"
        v-text="t('settings.PanelSettings')" 
      />
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
      <dropdown v-model="theme" :options="$theme.getThemes()" :label="t('settings.DefaultTheme')" @change="themeChanged()" />
      <text-field v-model="discordWebhook" :label="t('settings.DiscordWebhook')" :hint="t('settings.DiscordWebhookHint')" />
      <text-field v-model="discordWebhookSystem" :label="t('settings.DiscordWebhookSystem')" :hint="t('settings.DiscordWebhookSystemHint')" />
      <text-field v-model="discordWebhookNode" :label="t('settings.DiscordWebhookNode')" :hint="t('settings.DiscordWebhookNodeHint')" />
      <btn color="primary" @click="testDiscordWebhook()" :disabled="!discordWebhook"><icon name="test" />{{ t('settings.TestDiscord') }}</btn>
      <theme-setting v-for="(setting, name) in themeSettings" :key="name" :model-value="setting" @update:modelValue="updateThemeSetting(name, $event)" />
      <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
        <btn color="primary" @click="savePanelSettings()"><icon name="save" />{{ t('settings.SavePanelSettings') }}</btn>
      </div>
    </div>
    <div 
      :class="[
        'email',
        'space-y-6'
      ]"
    >
      <h1 
        :class="[
          'text-3xl font-bold text-foreground mb-6',
          'pb-3 border-b-2 border-border/50'
        ]"
        v-text="t('settings.EmailSettings')" 
      />
      <dropdown v-model="emailProvider" :options="emailProviders" :label="t('settings.EmailProvider')" @change="emailProviderChanged" />
      <text-field v-for="elem in emailFields" :key="elem.key" v-model="email[elem.key]" :type="elem.type" :label="t('settings.email.' + elem.key)" />
      <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
        <btn color="primary" @click="saveEmailSettings()"><icon name="save" />{{ t('settings.SaveEmailSettings') }}</btn>
        <btn color="primary" @click="testEmailSettings()"><icon name="test" />{{ t('settings.TestEmail' )}}</btn>
      </div>
    </div>
    </template>
  </div>
</template>
