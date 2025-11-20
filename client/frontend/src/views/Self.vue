<script setup>
import { ref, inject, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { updateLocale, locales } from '@/plugins/i18n'
import Overlay from '@/components/ui/Overlay.vue'
import TextField from '@/components/ui/TextField.vue'
import OtpInput from '@/components/ui/OtpInput.vue'
import Dropdown from '@/components/ui/Dropdown.vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import OAuth from '@/components/ui/OAuth.vue'
import Tab from '@/components/ui/Tab.vue'
import Tabs from '@/components/ui/Tabs.vue'
import ThemeSetting from '@/components/ui/ThemeSetting.vue'

const { t, locale } = useI18n()
const api = inject('api')
const toast = inject('toast')
const themeApi = inject('theme')
const theme = ref(themeApi.getActiveTheme())
const themeSettings = ref({})
const user = ref(undefined)
const acc = ref(undefined)
const newPass = ref({ old: '', new: '', confirm: ''})
const otpEnabled = ref(false)
const otpEnrolling = ref(false)
const otpQrCode = ref(false)
const otpSecret = ref(false)
const otpDisabling = ref(false)
const otpRecovery = ref(false)
const recoveryCodes = ref([])
const regeneratingRecoveryCodes = ref(false)
const token = ref('')
const selectedLocale = ref(locale.value)

onMounted(async () => {
  themeSettings.value = await themeApi.getThemeSettings()
  const data = await api.self.get()
  acc.value = { username: data.username, email: data.email, password: '' }
  user.value = data
  otpEnabled.value = await api.self.isOtpEnabled()
})

async function themeChanged() {
  themeSettings.value = await themeApi.getThemeSettings(theme.value)
}

function savePreferences() {
  if (locale.value !== selectedLocale.value) {
    updateLocale(selectedLocale.value)
  }

  const settings = {}
  Object.keys(themeSettings.value).map(key => {
    settings[key] = themeSettings.value[key].current
  })

  themeApi.setTheme(theme.value, settings)
  toast.success(t('users.PreferencesUpdated'))
}

async function startOtpEnroll() {
  const data = await api.self.startOtpEnroll()
  otpEnrolling.value = true
  otpQrCode.value = data.img
  otpSecret.value = data.secret
}

function resetOtpEnroll() {
  otpEnrolling.value = false
  otpQrCode.value = false
  otpSecret.value = false
  token.value = ''
}

async function confirmOtpEnroll() {
  const res = await api.self.validateOtpEnroll(token.value)
  resetOtpEnroll()
  recoveryCodes.value = res.recoveryCodes
  otpEnabled.value = await api.self.isOtpEnabled()
  toast.success(t('users.UpdateSuccess'))
}

function startRegenerateRecoveryCodes() {
  regeneratingRecoveryCodes.value = true
}

function resetRegenerateRecoveryCodes() {
  regeneratingRecoveryCodes.value = false
  token.value = ''
  otpRecovery.value = false
}

async function confirmRegenerateRecoveryCodes() {
  const res = await api.self.regenerateRecoveryCodes(token.value)
  resetRegenerateRecoveryCodes()
  recoveryCodes.value = res.recoveryCodes
  toast.success(t('users.UpdateSuccess'))
}

function saveRecoveryCodes() {
  const el = document.createElement('a')
  el.setAttribute('href', 'data:text/plain;charset=utf-8,' + encodeURIComponent(recoveryCodes.value.join('\n')))
  el.setAttribute('download', 'recovery-codes.txt')
  el.click()
}

function startOtpDeactivation() {
  otpDisabling.value = true
}

function resetOtpDeactivation() {
  otpDisabling.value = false
  token.value = ''
  otpRecovery.value = false
}

async function confirmOtpDeactivation() {
  await api.self.disableOtp(token.value)
  resetOtpDeactivation()
  otpEnabled.value = await api.self.isOtpEnabled()
  toast.success(t('users.UpdateSuccess'))
}

function isValidUsername(u) {
  return u.length >= 5
}

function isValidEmail(e) {
  return e.match(/.+@.+\..{2,}/)
}

function isValidPassword(p) {
  return p.length >= 8
}

function canSubmitDetailsChange() {
  return isValidUsername(acc.value.username) && isValidEmail(acc.value.email) && isValidPassword(acc.value.password)
}

function canSubmitPasswordChange() {
  return isValidPassword(newPass.value.old) && isValidPassword(newPass.value.new) && newPass.value.new === newPass.value.confirm
}

async function submitDetailsChange() {
  if (!canSubmitDetailsChange()) return
  await api.self.updateDetails(acc.value.username, acc.value.email, acc.value.password)
  toast.success(t('users.InfoChanged'))
}

async function submitPasswordChange() {
  if (!canSubmitPasswordChange()) return
  await api.self.changePassword(newPass.value.old, newPass.value.new)
  toast.success(t('users.PasswordChanged'))
}

function updateThemeSetting(name, newSetting) {
  themeSettings.value[name] = newSetting
}
</script>

<template>
  <div 
    v-if="!user" 
    :class="[
      'self',
      'loading',
      'flex items-center justify-center min-h-screen'
    ]"
  >
    <div :class="['loader']"><loader /></div>
  </div>
  <div 
    v-else 
    :class="[
      'self',
      'space-y-6'
    ]"
  >
    <tabs anchors>
      <tab id="preferences" :title="t('users.Preferences')" icon="settings" hotkey="t s">
        <div 
          :class="[
            'preferences',
            'space-y-6'
          ]"
        >
          <h1 
            :class="[
              'text-2xl font-bold text-foreground mb-6',
              'pb-3 border-b-2 border-border/50'
            ]"
            v-text="t('users.Preferences')" 
          />
          <form :class="['space-y-5']">
            <dropdown v-model="selectedLocale" class="locale-select" :options="locales" :label="t('common.Language')" :hint="`[${t('common.HelpTranslate')}](https://translate.SkyPanel.com)`">
              <template #singlelabel="{ value }">
                <div class="multiselect-single-label">
                  <span :data-locale="value.value" /> {{ value.label }}
                </div>
              </template>

              <template #option="{ option }">
                <span :data-locale="option.value" /> {{ option.label }}
              </template>
            </dropdown>
            <dropdown v-model="theme" :options="$theme.getThemes()" :label="t('common.theme.Theme')" @change="themeChanged()" />
            <theme-setting v-for="(setting, name) in themeSettings" :key="name" :model-value="setting" @update:modelValue="updateThemeSetting(name, $event)" />
            <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
              <btn color="primary" @click="savePreferences()"><icon name="save" />{{ t('users.SavePreferences') }}</btn>
            </div>
          </form>
        </div>
      </tab>
      <tab v-if="api.auth.hasScope('self.edit')" id="account" :title="t('users.ChangeInfo')" icon="account" hotkey="t a">
        <div 
          :class="[
            'accountdetails',
            'space-y-6'
          ]"
        >
          <h1 
            :class="[
              'text-2xl font-bold text-foreground mb-6',
              'pb-3 border-b-2 border-border/50'
            ]"
            v-text="t('users.ChangeInfo')" 
          />
          <form :class="['space-y-5']">
            <text-field v-model="acc.username" icon="account" :label="t('users.Username')" />
            <text-field v-model="acc.email" icon="email" type="email" :label="t('users.Email')" />
            <text-field v-model="acc.password" icon="lock" type="password" :label="t('users.ConfirmPassword')" />
            <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
              <btn :disabled="!canSubmitDetailsChange()" color="primary" @click="submitDetailsChange()"><icon name="save" />{{ t('users.ChangeInfo') }} </btn>
            </div>
          </form>
        </div>
      </tab>
      <tab v-if="api.auth.hasScope('self.edit')" id="changepassword" :title="t('users.ChangePassword')" icon="lock" hotkey="t p">
        <div 
          :class="[
            'changepassword',
            'space-y-6'
          ]"
        >
          <h1 
            :class="[
              'text-2xl font-bold text-foreground mb-6',
              'pb-3 border-b-2 border-border/50'
            ]"
            v-text="t('users.ChangePassword')" 
          />
          <form :class="['space-y-5']">
            <text-field v-model="newPass.old" icon="lock" type="password" :label="t('users.OldPassword')" />
            <text-field v-model="newPass.new" icon="lock" type="password" :label="t('users.NewPassword')" />
            <text-field v-model="newPass.confirm" icon="lock" type="password" :label="t('users.ConfirmPassword')" />
            <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
              <btn :disabled="!canSubmitPasswordChange()" color="primary" @click="submitPasswordChange()"><icon name="save" />{{ t('users.ChangePassword') }}</btn>
            </div>
          </form>
        </div>
      </tab>
      <tab v-if="api.auth.hasScope('self.edit')" id="otp" :title="t('users.Otp')" icon="2fa" hotkey="t 2">
        <div 
          :class="[
            'mfa',
            'space-y-6'
          ]"
        >
          <h1 
            :class="[
              'text-2xl font-bold text-foreground mb-6',
              'pb-3 border-b-2 border-border/50'
            ]"
            v-text="t('users.Otp')" 
          />
          <p 
            :class="[
              'description',
              'text-muted-foreground mb-4'
            ]"
          >
            {{ t('users.OtpHint') }}
          </p>
          <div :class="['flex gap-4 flex-wrap']">
            <btn v-if="otpEnabled" class="otp-regenerate-recovery-codes" variant="text" @click="startRegenerateRecoveryCodes()"><icon name="refresh" />{{ t('users.RegenerateRecoveryCodes') }}</btn>
            <btn v-if="otpEnabled" class="otp-toggle" color="error" @click="startOtpDeactivation()"><icon name="lock-off" />{{ t('users.OtpDisable') }}</btn>
            <btn v-else class="otp-toggle" color="primary" @click="startOtpEnroll()"><icon name="lock" />{{ t('users.OtpEnable') }}</btn>
          </div>
          <overlay v-model="otpEnrolling" :class="['otp-enroll']" :title="t('users.OtpEnable')" closable @close="resetOtpEnroll()">
            <div 
              :class="[
                'otp-enroll-content',
                'space-y-6'
              ]"
            >
              <div 
                :class="[
                  'otp-enroll-qr',
                  'flex justify-center'
                ]"
              >
                <img :src="otpQrCode" :class="['rounded-xl border-2 border-border/50 shadow-lg']" />
              </div>
              <div 
                :class="[
                  'otp-enroll-info',
                  'space-y-4'
                ]"
              >
                <h2 
                  :class="[
                    'text-xl font-bold text-foreground'
                  ]"
                  v-text="t('users.OtpSetupHint')" 
                />
                <div :class="['space-y-2']">
                  <b 
                    :class="[
                      'otp-enroll-secret',
                      'block font-semibold text-foreground'
                    ]"
                    v-text="t('users.OtpSecret')" 
                  />
                  <span 
                    :class="[
                      'otp-enroll-secret',
                      'font-mono text-sm p-3 rounded-lg',
                      'bg-muted/50 border-2 border-border/50',
                      'text-foreground break-all'
                    ]"
                    v-text="otpSecret" 
                  />
                </div>
                <h3 
                  :class="[
                    'otp-enroll-confirm',
                    'text-lg font-semibold text-foreground'
                  ]"
                  v-text="t('users.OtpConfirm')" 
                />
                <otp-input @update:modelValue="token = $event" @complete="token = $event; confirmOtpEnroll()" />
              </div>
            </div>
            <div :class="['otp-enroll-actions', 'flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
              <btn color="error" @click="resetOtpEnroll()" v-text="t('common.Cancel')" />
              <btn color="primary" @click="confirmOtpEnroll()" v-text="t('users.OtpEnable')" />
            </div>
          </overlay>
          <overlay v-model="regeneratingRecoveryCodes" :class="['recovery-code-regeneration']" :title="t('users.RegenerateRecoveryCodes')" closable @close="resetRegenerateRecoveryCodes()">
            <div 
              :class="[
                'recovery-code-regeneration-content',
                'space-y-5'
              ]"
            >
              <div :class="['text-foreground font-semibold']" v-text="t('users.OtpConfirm')" />
              <div :class="['text-muted-foreground']" v-text="t('users.RegenerateRecoveryCodesHint')" />
              <otp-input v-if="!otpRecovery" @update:modelValue="token = $event" @complete="token = $event; confirmRegenerateRecoveryCodes()" />
              <text-field v-else v-model="token" autofocus />
              <btn variant="text" @click="otpRecovery = !otpRecovery; token = ''" v-text="otpRecovery ? t('users.OtpUseAuthenticator') : t('users.OtpUseRecovery')" />
            </div>
            <div :class="['recovery-code-regeneration-actions', 'flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
              <btn color="error" @click="resetRegenerateRecoveryCodes()" v-text="t('common.Cancel')" />
              <btn color="primary" @click="confirmRegenerateRecoveryCodes()" v-text="t('users.RegenerateRecoveryCodes')" />
            </div>
          </overlay>
          <overlay v-model="otpDisabling" :class="['otp-deactivation']" :title="t('users.OtpDisable')" closable @close="resetOtpDeactivation()">
            <div 
              :class="[
                'otp-deactivation-content',
                'space-y-5'
              ]"
            >
              <span :class="['text-foreground font-semibold']" v-text="t('users.OtpConfirm')" />
              <otp-input v-if="!otpRecovery" @update:modelValue="token = $event" @complete="token = $event; confirmOtpDeactivation()" />
              <text-field v-else v-model="token" autofocus />
              <btn variant="text" @click="otpRecovery = !otpRecovery; token = ''" v-text="otpRecovery ? t('users.OtpUseAuthenticator') : t('users.OtpUseRecovery')" />
            </div>
            <div :class="['otp-deactivation-actions', 'flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
              <btn color="error" @click="resetOtpDeactivation()" v-text="t('common.Cancel')" />
              <btn color="primary" @click="confirmOtpDeactivation()" v-text="t('users.OtpDisable')" />
            </div>
          </overlay>
          <overlay :model-value="recoveryCodes.length > 0" :class="['recovery-codes']" :title="t('users.RecoveryCodes')" closable @close="recoveryCodes = []">
            <div :class="['text-muted-foreground mb-4']" v-text="t('users.RecoveryCodesHint')" />
            <div 
              :class="[
                'codes',
                'grid grid-cols-2 gap-2 p-4 rounded-xl',
                'bg-muted/30 border-2 border-border/50',
                'mb-4'
              ]"
            >
              <span 
                v-for="code in recoveryCodes" 
                :key="code" 
                :class="[
                  'code',
                  'font-mono text-sm p-2 rounded-lg',
                  'bg-background border border-border/50',
                  'text-center'
                ]"
              >
                <code :class="['text-foreground']" v-text="code" />
              </span>
            </div>
            <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
              <btn variant="text" @click="saveRecoveryCodes()"><icon name="download" />{{ t('users.SaveRecoveryCodes') }}</btn>
            </div>
          </overlay>
        </div>
      </tab>
      <tab v-if="api.auth.hasScope('self.clients')" id="oauth" :title="t('oauth.Clients')" icon="api" hotkey="t o">
        <div 
          :class="[
            'oauth',
            'space-y-6'
          ]"
        >
          <h1 
            :class="[
              'text-2xl font-bold text-foreground mb-6',
              'pb-3 border-b-2 border-border/50'
            ]"
            v-text="t('oauth.Clients')" 
          />
          <o-auth />
        </div>
      </tab>
    </tabs>
  </div>
</template>
