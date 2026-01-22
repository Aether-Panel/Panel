<script setup>
import { ref, inject, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import TextField from '@/components/ui/TextField.vue'
import Toggle from '@/components/ui/Toggle.vue'

const { t, te, locale } = useI18n()
const toast = inject('toast')

const users = ref([])
const newEmail = ref('')

const perms = [
  'server.view',
	'server.admin',
	'server.delete',
	'server.definition.view',
	'server.definition.edit',
	'server.data.view',
	'server.data.edit',
  'server.data.edit.admin',
	'server.flags.view',
	'server.flags.edit',
	'server.name.edit',
	'server.users.view',
	'server.users.create',
	'server.users.edit',
	'server.users.delete',
	//'server.tasks.view',
	//'server.tasks.run',
	//'server.tasks.create',
	//'server.tasks.delete',
	//'server.tasks.edit',
	'server.start',
	'server.stop',
	'server.kill',
	'server.install',
	'server.files.view',
	'server.files.edit',
	'server.sftp',
	'server.console',
	'server.console.send',
	'server.stats',
	'server.status',
	'server.backup.view',
	'server.backup.create',
	'server.backup.restore',
	'server.backup.delete',
].map(scope => {
  const res = {
    label: t('scopes.name.' + scope.replace(/\./g, '-')),
    name: scope
  }
  if (te('scopes.hint.' + scope.replace(/\./g, '-'), locale))
    res.hint = t('scopes.hint.' + scope.replace(/\./g, '-'))
  return res
})

const props = defineProps({
  server: { type: Object, required: true }
})

async function sendInvite() {
  const newUser = { email: newEmail.value }
  await props.server.updateUser(newUser)
  toast.success(t('users.UserInvited'))
  loadUsers()
}

async function updatePerms(user) {
  const scopes = Object.keys(user.scopes).filter(p => user.scopes[p])
  const update = { ...user, scopes }
  await props.server.updateUser(update)
  toast.success(t('users.UpdateSuccess'))
}

async function deleteUser(user) {
  await props.server.deleteUser(user.email)
  loadUsers()
}

async function loadUsers() {
  const u = await props.server.getUsers()
  users.value = u.map(user => {
    const scopes = {}
    perms.map(p => {
      scopes[p.name] = user.scopes.indexOf(p.name) > -1
    })
    user.scopes = scopes
    return user
  })
}

function permissionDisabled(scope) {
  // always deny changing any permission if user doesn't have edit user permission
  if (!props.server.hasScope('server.users.edit')) return true

  // only allow changing any permission the current user posseses themselves
  return !props.server.hasScope(scope)
}

onMounted(async () => {
  loadUsers()
})
</script>

<template>
  <div class="users-container">
    <!-- Header con título y contador -->
    <div class="users-header">
      <div class="users-header-content">
        <div class="users-header-icon">
          <icon name="users" />
        </div>
        <div>
          <h2 class="users-title" v-text="t('users.Users')" />
          <p class="users-subtitle">
            {{ users.length }} {{ users.length === 1 ? 'usuario' : 'usuarios' }}
          </p>
        </div>
      </div>
    </div>
    
    <!-- Lista de usuarios -->
    <div v-if="users.length > 0" class="users-section">
      <div class="users-grid">
        <div
          v-for="user in users"
          :key="user.email"
          class="user-card"
          :class="{ 'expanded': user.open }"
        >
          <div
            class="user-card-header"
            @click="user.open = !user.open"
          >
            <div class="user-card-title">
              <div class="user-icon-wrapper">
                <icon name="user" />
              </div>
              <div class="user-card-info">
                <h3 class="user-name" v-text="user.username" />
                <span class="user-email" v-text="user.email" />
              </div>
            </div>
            <icon
              :name="user.open ? 'chevron-down' : 'chevron-right'"
              class="expand-icon"
            />
          </div>
          <transition name="expand">
            <div v-if="user.open" class="user-card-body">
              <div class="permissions-grid">
              <toggle
                v-for="perm in perms"
                :key="perm.name"
                v-model="user.scopes[perm.name]"
                :disabled="permissionDisabled(perm.name)"
                :label="perm.label"
                :hint="perm.hint"
                  class="permission-item"
                @update:modelValue="updatePerms(user)"
              />
            </div>
              <div class="user-actions">
              <btn
                v-if="server.hasScope('server.users.delete')"
                color="error"
                variant="outline"
                @click="deleteUser(user)"
              >
                <icon name="remove" />
                {{ t('users.Delete') }}
              </btn>
            </div>
          </div>
          </transition>
        </div>
      </div>
    </div>
    
    <!-- Estado vacío -->
    <div v-else class="empty-state">
      <div class="empty-state-icon">
        <icon name="users" />
      </div>
      <h3 class="empty-state-title" v-text="t('servers.NoUsers')" />
      <p class="empty-state-text">Invita usuarios para compartir acceso al servidor</p>
    </div>
    
    <!-- Formulario de invitación -->
    <div v-if="server.hasScope('server.users.create')" class="create-section">
      <div class="create-card">
        <div class="create-header">
          <icon name="plus" class="create-icon" />
          <h3 class="create-title">{{ t('servers.InviteUser') }}</h3>
        </div>
        <div class="create-form">
          <text-field
            v-model="newEmail"
            type="email"
            icon="email"
            :label="t('users.Email')"
          />
          <btn color="primary" size="lg" @click="sendInvite()" class="create-button">
            <icon name="plus" />
            {{ t('servers.InviteUser') }}
          </btn>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.users-container {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  padding: 2rem;
  max-width: 1400px;
  margin: 0 auto;
}

/* Header */
.users-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 1.5rem;
  border-bottom: 2px solid #475569;
}

.users-header-content {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.users-header-icon {
  width: 2.5rem;
  height: 2.5rem;
  padding: 0.5rem;
  background: #3b82f6;
  border-radius: 0.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.users-header-icon :deep(svg),
.users-header-icon :deep(svg path),
.users-header-icon :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
  width: 1.5rem;
  height: 1.5rem;
}

.users-title {
  font-size: 1.875rem;
  font-weight: 700;
  color: #f1f5f9;
  margin: 0;
  line-height: 1.2;
}

.users-subtitle {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0.25rem 0 0;
}

/* Grid de usuarios */
.users-section {
  margin-top: 0;
}

.users-grid {
  display: grid;
  gap: 1.5rem;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
}

@media (max-width: 768px) {
  .users-grid {
    grid-template-columns: 1fr;
  }
}

/* Tarjeta de usuario */
.user-card {
  background: #1e293b;
  border: 2px solid #475569;
  border-radius: 1rem;
  overflow: hidden;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.user-card:hover {
  border-color: #3b82f6;
  transform: translateY(-2px);
}

.user-card.expanded {
  border-color: #3b82f6;
}

.user-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1.25rem 1.5rem;
  cursor: pointer;
  user-select: none;
  background: #1e293b;
  transition: background 0.2s;
}

.user-card-header:hover {
  background: #2d3e52;
}

.user-card-title {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex: 1;
}

.user-icon-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3rem;
  height: 3rem;
  background: #3b82f6;
  border-radius: 0.75rem;
}

.user-icon-wrapper :deep(svg),
.user-icon-wrapper :deep(svg path),
.user-icon-wrapper :deep(svg *) {
  width: 1.5rem;
  height: 1.5rem;
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
}

.user-card-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.user-name {
  font-size: 1.125rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0;
}

.user-email {
  font-size: 0.875rem;
  color: #cbd5e1;
}

.expand-icon {
  width: 1.25rem;
  height: 1.25rem;
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  flex-shrink: 0;
}

.expand-icon :deep(svg),
.expand-icon :deep(svg path),
.expand-icon :deep(svg *) {
  color: #e2e8f0 !important;
  fill: #e2e8f0 !important;
  stroke: #e2e8f0 !important;
  width: 1.25rem;
  height: 1.25rem;
}

.user-card.expanded .expand-icon {
  transform: rotate(180deg);
}

/* Animación de expansión */
.expand-enter-active,
.expand-leave-active {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}

.expand-enter-from,
.expand-leave-to {
  opacity: 0;
  max-height: 0;
}

.expand-enter-to,
.expand-leave-from {
  opacity: 1;
  max-height: 1000px;
}

/* Body de la tarjeta */
.user-card-body {
  padding: 1.5rem;
  background: #0f172a;
  border-top: 2px solid #334155;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.permissions-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 0.75rem;
}

.permission-item {
  padding: 0.75rem;
  background: #1e293b;
  border: 1px solid #334155;
  border-radius: 0.5rem;
  transition: all 0.2s;
}

.permission-item:hover {
  border-color: #475569;
  background: #2d3e52;
}

.user-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 0.5rem;
  border-top: 1px solid #334155;
}

/* Sección de creación */
.create-section {
  margin-top: 0;
}

.create-card {
  background: #1e293b;
  border: 2px solid #3b82f6;
  border-radius: 1rem;
  padding: 2rem;
}

.create-header {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.create-icon {
  width: 2rem;
  height: 2rem;
  padding: 0.5rem;
  background: #3b82f6;
  border-radius: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.create-icon :deep(svg),
.create-icon :deep(svg path),
.create-icon :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
  width: 1rem;
  height: 1rem;
}

.create-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0;
}

.create-form {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.create-button {
  width: 100%;
  justify-content: center;
}

/* Estado vacío */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 4rem 2rem;
  text-align: center;
  background: #1e293b;
  border: 2px dashed #475569;
  border-radius: 1rem;
}

.empty-state-icon {
  width: 4rem;
  height: 4rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #334155;
  border-radius: 1rem;
  margin-bottom: 1.5rem;
}

.empty-state-icon :deep(svg),
.empty-state-icon :deep(svg path),
.empty-state-icon :deep(svg *) {
  width: 2rem;
  height: 2rem;
  color: #94a3b8 !important;
  fill: #94a3b8 !important;
  stroke: #94a3b8 !important;
}

.empty-state-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0 0 0.5rem;
}

.empty-state-text {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0;
}

/* Responsive */
@media (max-width: 768px) {
  .users-container {
    padding: 1rem;
  }
  
  .users-title {
    font-size: 1.5rem;
  }
  
  .permissions-grid {
    grid-template-columns: 1fr;
  }
}
</style>
