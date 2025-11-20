<script setup>
import { ref, inject, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Dropdown from '@/components/ui/Dropdown.vue'
import Icon from '@/components/ui/Icon.vue'
import TextField from '@/components/ui/TextField.vue'
import ThemeSetting from '@/components/ui/ThemeSetting.vue'
import Toggle from '@/components/ui/Toggle.vue'

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
const theme = ref('Default')
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

onMounted(async () => {
  masterUrl.value = await api.settings.get('panel.settings.masterUrl')
  panelTitle.value = await api.settings.get('panel.settings.companyName')
  const regEnabled = await api.settings.get('panel.registrationEnabled')
  registrationEnabled.value = (regEnabled === "true" || regEnabled === true)
  theme.value = await api.settings.get('panel.settings.defaultTheme')
  discordWebhook.value = await api.settings.get('panel.notifications.discordWebhook')
  discordWebhookSystem.value = await api.settings.get('panel.notifications.discordWebhookSystem')
  discordWebhookNode.value = await api.settings.get('panel.notifications.discordWebhookNode')
  emailProvider.value = await api.settings.get('panel.email.provider')
  emailProviderChanged(emailProvider.value)
  Object.keys(email.value).map(async key => {
    email.value[key] = await api.settings.get('panel.email.' + key)
  })
  await themeChanged()
  themeSettings.value = themeApi.deserializeThemeSettings(
    themeSettings.value,
    await api.settings.get('panel.settings.themeSettings')
  )
})

function updateThemeSetting(name, newSetting) {
  themeSettings.value[name] = newSetting
}
</script>

<template>
  <div 
    :class="[
      'settings',
      'space-y-8'
    ]"
  >
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
  </div>
</template>
