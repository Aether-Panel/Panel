<script setup>
import { ref, inject, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import TextField from '@/components/ui/TextField.vue'
import Dropdown from '@/components/ui/Dropdown.vue'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const api = inject('api')
const validate = inject('validate')

const username = ref('')
const email = ref('')
const password = ref('')
const selectedRoleId = ref(null)
const roles = ref([])
const rolesLoaded = ref(false)

const usernameError = ref('')
const emailError = ref('')
const passwordError = ref('')

function canSubmit() {
  return validate.username(username.value) &&
    validate.email(email.value) &&
    validate.password(password.value)
}

async function submit() {
  if (!canSubmit()) return false
  const roleId = selectedRoleId.value ? Number(selectedRoleId.value) : null
  const id = await api.user.create(username.value, email.value, password.value, roleId)
  const routeName = route.path.startsWith('/admin') ? 'Admin.UserView' : 'UserView'
  router.push({ name: routeName, params: { id } })
}

onMounted(async () => {
  try {
    roles.value = await api.role.list()
    rolesLoaded.value = true
  } catch (error) {
    console.error('Error loading roles:', error)
    rolesLoaded.value = true
  }
})
</script>

<template>
  <div 
    :class="[
      'usercreate',
      'max-w-md mx-auto space-y-6'
    ]"
    style="
      -webkit-font-smoothing: antialiased;
      -moz-osx-font-smoothing: grayscale;
      text-rendering: optimizeLegibility;
      transform: translateZ(0);
      will-change: auto;
    "
  >
    <h1 
      :class="[
        'text-3xl font-bold text-foreground mb-6',
        'pb-3 border-b-2 border-border/50'
      ]"
      v-text="t('users.Create')" 
    />
    <form 
      :class="['space-y-5']"
      @keydown.enter="submit()"
    >
      <text-field
        v-model="username"
        autofocus
        :label="t('users.Username')"
        icon="account"
        :error="usernameError"
        @blur="usernameError = validate.username(username) ? '' : t('errors.ErrUsernameRequirements')"
      />
      <text-field
        v-model="email"
        :label="t('users.Email')"
        type="email"
        icon="email"
        :error="emailError"
        @blur="emailError = validate.email(email) ? '' : t('errors.ErrEmailInvalid')"
      />
      <text-field
        v-model="password"
        :label="t('users.Password')"
        type="password"
        icon="lock"
        :error="passwordError"
        @blur="passwordError = validate.password(password) ? '' : t('errors.ErrPasswordRequirements')"
      />
      <dropdown
        v-if="rolesLoaded && roles.length > 0"
        v-model="selectedRoleId"
        :label="t('users.Role')"
        :options="[{ value: null, label: t('users.NoRole') }, ...roles.map(r => ({ value: r.id, label: r.name }))]"
        :hint="t('users.RoleHint')"
        icon="hi-shield"
        :can-clear="true"
      />
      <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
        <btn color="primary" :disabled="!canSubmit()" @click="submit()"><icon name="save" />{{ t('users.Create') }}</btn>
      </div>
    </form>
  </div>
</template>
