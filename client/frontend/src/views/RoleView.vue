<script setup>
import { ref, inject, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import TextField from '@/components/ui/TextField.vue'
import Toggle from '@/components/ui/Toggle.vue'

const { t, te, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const api = inject('api')
const toast = inject('toast')
const events = inject('events')

const name = ref('')
const description = ref('')
const isNew = computed(() => !route.params.id || route.params.id === 'new')

const nameError = ref('')

// Todos los scopes disponibles organizados por categoría
const scopes = {
  general: [
    'admin',
    'login',
    'self.edit',
    'self.clients',
    'settings.edit',
    'uptime.view'
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

function canSubmit() {
  return name.value && name.value.length > 0
}

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
    templates: 'templates.Templates',
    general: 'roles.General'
  }
  return t(map[category] || category)
}

function permissionDisabled(scope) {
  if (!api.auth.hasScope('admin')) return true
  if (scope === 'admin' && api.auth.hasScope('admin')) return false
  if (scope === 'admin') return true
  return !api.auth.hasScope(scope)
}

function togglePermission(scope) {
  if (permissions.value.indexOf(scope) === -1) {
    permissions.value = [...permissions.value, scope]
  } else {
    permissions.value = permissions.value.filter(e => e !== scope)
  }
}

async function submit() {
  if (!canSubmit()) return
  
  try {
    const roleData = {
      name: name.value,
      description: description.value,
      scopes: permissions.value
    }
    
    if (isNew.value) {
      const id = await api.role.create(roleData)
      toast.success(t('roles.Created'))
      const routeName = route.path.startsWith('/admin') ? 'Admin.RoleView' : 'RoleView'
      router.push({ name: routeName, params: { id } })
    } else {
      await api.role.update(route.params.id, roleData)
      toast.success(t('roles.Updated'))
    }
  } catch (error) {
    console.error('Error saving role:', error)
    toast.error(t('roles.ErrorSaving'))
  }
}

async function deleteRole() {
  events.emit(
    'confirm',
    t('roles.ConfirmDelete', { name: name.value }),
    {
      text: t('roles.Delete'),
      icon: 'remove',
      color: 'error',
      action: async () => {
        try {
          await api.role.delete(route.params.id)
          toast.success(t('roles.Deleted'))
          router.push({ name: route.path.startsWith('/admin') ? 'Admin.RoleList' : 'RoleList' })
        } catch (error) {
          console.error('Error deleting role:', error)
          toast.error(t('roles.ErrorDeleting'))
        }
      }
    },
    {
      color: 'primary'
    }
  )
}

onMounted(async () => {
  if (!isNew.value) {
    try {
      const role = await api.role.get(route.params.id)
      name.value = role.name
      description.value = role.description || ''
      permissions.value = role.scopes || []
    } catch (error) {
      console.error('Error loading role:', error)
      toast.error(t('roles.ErrorLoading'))
    }
  }
})
</script>

<template>
  <div 
    :class="[
      'roleview',
      'w-full max-w-5xl ml-auto mr-0 space-y-6'
    ]"
    style="
      padding-left: 2rem;
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
      v-text="isNew ? t('roles.Create') : t('roles.Edit')" 
    />
    
    <form 
      :class="['space-y-6']"
      @submit.prevent="submit()"
    >
      <!-- Información básica del rol -->
      <div 
        :class="[
          'p-6 rounded-xl border-2 border-border/50',
          'bg-muted/30 space-y-5'
        ]"
      >
        <h2 
          :class="[
            'text-xl font-semibold text-foreground mb-4'
          ]"
          v-text="t('roles.BasicInfo')" 
        />
        
        <text-field
          v-model="name"
          autofocus
          :label="t('roles.Name')"
          icon="hi-shield"
          :error="nameError"
          @blur="nameError = name.length > 0 ? '' : t('roles.NameRequired')"
        />
        
        <text-field
          v-model="description"
          :label="t('roles.Description')"
          icon="document-text"
          :hint="t('roles.DescriptionHint')"
        />
      </div>

      <!-- Permisos -->
      <div 
        :class="[
          'p-6 rounded-xl border-2 border-border/50',
          'bg-muted/30 space-y-6'
        ]"
      >
        <h2 
          :class="[
            'text-xl font-semibold text-foreground mb-4'
          ]"
          v-text="t('roles.Permissions')" 
        />
        
        <div 
          v-for="(scopeCat, catName) in scopes" 
          :key="catName"
          :class="['space-y-4 mb-6']"
        >
          <h3 
            :class="[
              'text-lg font-semibold text-foreground mb-3',
              'pb-2 border-b border-border/30'
            ]"
            v-text="permissionCategoryHeading(catName)" 
          />
          <div :class="['space-y-3']">
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

      <!-- Botones de acción -->
      <div :class="['flex gap-4 justify-end mt-6 pt-4 border-t-2 border-border/50']">
        <btn 
          color="primary" 
          :disabled="!canSubmit()" 
          @click="submit()"
        >
          <icon name="save" />
          {{ isNew ? t('roles.Create') : t('roles.Update') }}
        </btn>
        <btn 
          v-if="!isNew"
          color="error" 
          @click="deleteRole()"
        >
          <icon name="remove" />
          {{ t('roles.Delete') }}
        </btn>
      </div>
    </form>
  </div>
</template>

