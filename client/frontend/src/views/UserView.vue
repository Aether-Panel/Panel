<script setup>
import { ref, inject, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import TextField from '@/components/ui/TextField.vue'
import Toggle from '@/components/ui/Toggle.vue'
import Dropdown from '@/components/ui/Dropdown.vue'

const { t, te, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const api = inject('api')
const toast = inject('toast')
const validate = inject('validate')
const events = inject('events')

const username = ref('')
const email = ref('')
const password = ref('')
const otpActive = ref('')
const selectedRoleId = ref(null)
const roles = ref([])
const rolesLoaded = ref(false)

const usernameError = ref('')
const emailError = ref('')
const passwordError = ref('')

function canSubmitDetails() {
  return validate.username(username.value) &&
    validate.email(email.value) &&
    (validate.password(password.value) || password.value.length === 0)
}

async function submitDetails() {
  const updateData = {
    username: username.value,
    email: email.value,
    password: password.value || undefined
  }
  if (selectedRoleId.value !== null) {
    updateData.roleId = selectedRoleId.value ? Number(selectedRoleId.value) : null
  }
  await api.user.update(route.params.id, updateData)
  
  // Si se asignó un rol, aplicar los permisos del rol automáticamente
  if (selectedRoleId.value && api.auth.hasScope('users.perms.edit')) {
    try {
      await api.user.updatePermissions(route.params.id, { scopes: permissions.value })
    } catch (error) {
      console.error('Error updating permissions from role:', error)
    }
  }
  
  toast.success(t('users.UpdateSuccess'))
}

async function submitPermissions() {
  if (!canSubmitDetails()) return false
  // Si hay un rol asignado, los permisos se aplicarán automáticamente desde el rol
  if (selectedRoleId.value) {
    toast.info(t('users.PermissionsFromRoleInfo'))
    return
  }
  await api.user.updatePermissions(route.params.id, { scopes: permissions.value })
  toast.success(t('users.UpdateSuccess'))
}

async function deleteUser() {
  events.emit(
    'confirm',
    t('users.ConfirmDelete', { name: username.value }),
    {
      text: t('users.Delete'),
      icon: 'remove',
      color: 'error',
      action: async () => {
        await api.user.delete(route.params.id)
        toast.success(t('users.DeleteSuccess'))
        const routeName = route.path.startsWith('/admin') ? 'Admin.UserList' : 'UserList'
        router.push({ name: routeName })
      }
    },
    {
      color: 'primary'
    }
  )
}

const scopes = {
  general: [
    'admin',
    'login',
    'self.edit',
    'self.clients',
    'settings.edit'
  ],
  servers: [
    'server.create'
  ],
  nodes: [
    'nodes.view',
    'nodes.create',
    'nodes.edit',
    'nodes.deploy',
    'nodes.delete'
  ],
  users: [
    'users.info.search',
    'users.info.view',
    'users.info.edit',
    'users.perms.view',
    'users.perms.edit'
  ],
  templates: [
    'templates.view',
    'templates.local.edit',
    'templates.repo.view',
    'templates.repo.add',
    'templates.repo.remove'
  ]
}

const permissions = ref([])

async function loadRolePermissions(roleId) {
  if (!roleId) {
    return
  }
  try {
    const role = await api.role.get(roleId)
    if (role && role.scopes && role.scopes.length > 0) {
      permissions.value = [...role.scopes]
    }
  } catch (error) {
    console.error('Error loading role permissions:', error)
  }
}

onMounted(async () => {
  const user = await api.user.get(route.params.id)
  username.value = user.username
  email.value = user.email
  if (user.roleId) {
    selectedRoleId.value = user.roleId
  }
  if (api.auth.hasScope('users.perms.view')) {
    // Si el usuario tiene un rol, cargar permisos del rol primero
    if (user.roleId) {
      await loadRolePermissions(user.roleId)
    } else {
      // Si no tiene rol, cargar permisos del usuario
      permissions.value = await api.user.getPermissions(route.params.id)
    }
  }
  otpActive.value = user.otpActive !== undefined ? user.otpActive : false
  
  // Cargar roles si el usuario tiene permisos para editarlos
  if (api.auth.hasScope('users.info.edit') || api.auth.hasScope('admin')) {
    try {
      roles.value = await api.role.list()
      rolesLoaded.value = true
    } catch (error) {
      console.error('Error loading roles:', error)
      rolesLoaded.value = true
    }
  }
})

// Observar cambios en el rol seleccionado para aplicar permisos automáticamente
watch(selectedRoleId, async (newRoleId) => {
  if (api.auth.hasScope('users.perms.view') && newRoleId) {
    await loadRolePermissions(newRoleId)
  } else if (!newRoleId && api.auth.hasScope('users.perms.view')) {
    // Si se quita el rol, cargar permisos del usuario directamente
    try {
      permissions.value = await api.user.getPermissions(route.params.id)
    } catch (error) {
      console.error('Error loading user permissions:', error)
    }
  }
})

function scopeLabel(scope) {
  return t('scopes.name.' + scope.replace(/\./g, '-'))
}

function scopeHint(scope) {
  if (te('scopes.hint.' + scope.replace(/\./g, '-'), locale)) {
    return t('scopes.hint.' + scope.replace(/\./g, '-'))
  }
}

function permissionCategoryHeading(category) {
  const map = {
    servers: 'servers.Servers',
    nodes: 'nodes.Nodes',
    users: 'users.Users',
    templates: 'templates.Templates'
  }
  return t(map[category])
}

function permissionDisabled(scope) {
  if (!api.auth.hasScope('user.perms.edit')) return true
  if (scope === 'admin' && api.auth.hasScope('admin')) return false
  if (scope === 'admin') return true

  if (permissions.value.indexOf('admin') >= 0) return true
  return !api.auth.hasScope(scope)
}

function togglePermission(scope) {
  if (permissions.value.indexOf(scope) === -1) {
    permissions.value = [...permissions.value, scope]
  } else {
    permissions.value = permissions.value.filter(e => e !== scope)
  }
}
</script>

<template>
  <div 
    :class="[
      'userview',
      'w-full max-w-5xl ml-auto mr-0',
      'space-y-8'
    ]"
  >
    <!-- Sección de Detalles del Usuario -->
    <div 
      v-if="$api.auth.hasScope('users.info.view')" 
      :class="[
        'details',
        'bg-card rounded-2xl border-2 border-border/50',
        'p-6 lg:p-8',
        'shadow-lg'
      ]"
    >
      <div 
        :class="[
          'flex items-center justify-between mb-6',
          'pb-4 border-b-2 border-border/50'
        ]"
      >
        <h1 
          :class="[
            'text-3xl font-bold text-foreground',
            'flex items-center gap-3'
          ]"
        >
          <icon name="account" class="text-primary" />
          <span v-text="t('users.Details')" />
        </h1>
      </div>
      
      <form 
        :class="['space-y-6']"
        @submit.prevent="submitDetails()"
      >
        <!-- Campos en grid responsive -->
        <div :class="['grid grid-cols-1 md:grid-cols-2 gap-6']">
          <text-field
            v-model="username"
            :label="t('users.Username')"
            icon="account"
            :error="usernameError"
            :disabled="!$api.auth.hasScope('users.info.edit')"
            @blur="usernameError = validate.username(username) ? '' : t('errors.ErrUsernameRequirements')"
          />
          <text-field
            v-model="email"
            :label="t('users.Email')"
            type="email"
            icon="email"
            :error="emailError"
            :disabled="!$api.auth.hasScope('users.info.edit')"
            @blur="emailError = validate.email(email) ? '' : t('errors.ErrEmailInvalid')"
          />
        </div>

        <div :class="['grid grid-cols-1 md:grid-cols-2 gap-6']">
          <text-field
            v-if="$api.auth.hasScope('users.info.edit')"
            v-model="password"
            :label="t('users.Password')"
            type="password"
            icon="lock"
            :error="passwordError"
            @blur="passwordError = (validate.password(password) || password.length === 0) ? '' : t('error.PasswordInvalid')"
          />
          <dropdown
            v-if="rolesLoaded && roles.length > 0 && ($api.auth.hasScope('users.info.edit') || $api.auth.hasScope('admin'))"
            v-model="selectedRoleId"
            :label="t('users.Role')"
            :options="[{ value: null, label: t('users.NoRole') }, ...roles.map(r => ({ value: r.id, label: r.name }))]"
            :hint="t('users.RoleHint')"
            icon="hi-shield"
            :can-clear="true"
          />
        </div>

        <!-- Información de OTP -->
        <div 
          v-if="$api.auth.hasScope('users.perms.view')"
          :class="[
            'p-5 rounded-xl border-2',
            otpActive 
              ? 'bg-primary/10 border-primary/30' 
              : 'bg-muted/30 border-border/50'
          ]"
        >
          <div :class="['flex items-center gap-3']">
            <icon 
              :name="otpActive ? 'hi-shield-check' : 'hi-shield-exclamation'" 
              :class="[
                'text-2xl',
                otpActive ? 'text-primary' : 'text-muted-foreground'
              ]"
            />
            <div>
              <h3 :class="['text-lg font-semibold text-foreground']">
                {{ t('users.OtpEnabled') }}
              </h3>
              <p :class="['text-sm text-muted-foreground']">
                {{ otpActive ? t('common.Yes') : t('common.No') }}
              </p>
            </div>
          </div>
        </div>

        <!-- Botones de acción -->
        <div :class="['flex gap-4 justify-end pt-6 mt-6 border-t-2 border-border/50']">
          <btn 
            v-if="$api.auth.hasScope('users.info.edit')" 
            color="error" 
            @click="deleteUser()"
          >
            <icon name="remove" />
            {{ t('users.Delete') }}
          </btn>
          <btn 
            v-if="$api.auth.hasScope('users.info.edit')" 
            color="primary" 
            :disabled="!canSubmitDetails()" 
            @click="submitDetails()"
          >
            <icon name="save" />
            {{ t('users.UpdateDetails') }}
          </btn>
        </div>
      </form>
    </div>

    <!-- Sección de Permisos -->
    <div 
      v-if="$api.auth.hasScope('users.perms.view')" 
      :class="[
        'permissions',
        'bg-card rounded-2xl border-2 border-border/50',
        'p-6 lg:p-8',
        'shadow-lg'
      ]"
    >
      <div 
        :class="[
          'flex items-center justify-between mb-6',
          'pb-4 border-b-2 border-border/50'
        ]"
      >
        <h1 
          :class="[
            'text-3xl font-bold text-foreground',
            'flex items-center gap-3'
          ]"
        >
          <icon name="hi-shield" class="text-primary" />
          <span v-text="t('users.Permissions')" />
        </h1>
        <div 
          v-if="selectedRoleId"
          :class="[
            'flex items-center gap-2 px-4 py-2 rounded-lg',
            'bg-primary/20 text-primary border-2 border-primary/30',
            'text-sm font-semibold'
          ]"
        >
          <icon name="hi-shield" />
          <span v-text="t('users.PermissionsFromRole')" />
        </div>
      </div>

      <!-- Alerta de permisos desde rol -->
      <div 
        v-if="selectedRoleId"
        :class="[
          'mb-6 p-4 rounded-xl',
          'bg-primary/10 border-2 border-primary/30',
          'text-sm text-foreground'
        ]"
      >
        <div :class="['flex items-start gap-3']">
          <icon name="hi-information" :class="['text-primary text-xl flex-shrink-0 mt-0.5']" />
          <span v-text="t('users.PermissionsFromRoleHint')" />
        </div>
      </div>

      <!-- Categorías de permisos -->
      <div :class="['space-y-8']">
        <div 
          v-for="(scopeCat, catName) in scopes" 
          :key="scopeCat"
          :class="[
            'space-y-4',
            catName !== 'general' ? 'p-5 rounded-xl bg-muted/20 border border-border/30' : ''
          ]"
        >
          <h3 
            v-if="catName !== 'general'" 
            :class="[
              'text-xl font-bold text-foreground mb-4',
              'pb-3 border-b border-border/30',
              'flex items-center gap-2'
            ]"
          >
            <icon 
              :name="catName === 'servers' ? 'hi-server' : catName === 'nodes' ? 'hi-cube' : catName === 'users' ? 'account' : 'hi-document'"
              class="text-primary"
            />
            <span v-text="permissionCategoryHeading(catName)" />
          </h3>
          <div :class="['grid grid-cols-1 md:grid-cols-2 gap-3']">
            <toggle
              v-for="scope in scopeCat"
              :key="scope"
              :model-value="permissions.indexOf(scope) >= 0"
              :disabled="permissionDisabled(scope)"
              :label="scopeLabel(scope)"
              :hint="scopeHint(scope)"
              @update:modelValue="togglePermission(scope)"
            />
          </div>
        </div>
      </div>

      <!-- Botón de guardar permisos -->
      <div :class="['flex gap-4 justify-end pt-6 mt-6 border-t-2 border-border/50']">
        <btn 
          v-if="api.auth.hasScope('user.perms.edit')" 
          color="primary" 
          @click="submitPermissions()"
        >
          <icon name="save" />
          {{ t('users.UpdatePermissions') }}
        </btn>
      </div>
    </div>
  </div>
</template>
