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
  <div class="server-tab-content">
    <div class="server-tab-section">
      <h2 class="server-tab-title" v-text="t('users.Users')" />
    </div>
    
    <div v-if="users.length > 0" class="server-tab-section">
      <div class="server-users-list">
        <div
          v-for="user in users"
          :key="user.email"
          class="server-user-card"
        >
          <div
            class="server-user-header"
            @click="user.open = !user.open"
          >
            <div class="server-user-info">
              <h3 class="server-user-name" v-text="user.username" />
              <span class="server-user-email" v-text="user.email" />
            </div>
            <icon
              :name="user.open ? 'chevron-down' : 'chevron-right'"
              class="server-user-chevron"
            />
          </div>
          <div v-if="user.open" class="server-user-permissions">
            <div class="server-permissions-grid">
              <toggle
                v-for="perm in perms"
                :key="perm.name"
                v-model="user.scopes[perm.name]"
                :disabled="permissionDisabled(perm.name)"
                :label="perm.label"
                :hint="perm.hint"
                class="server-permission-item"
                @update:modelValue="updatePerms(user)"
              />
            </div>
            <div class="server-user-actions">
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
        </div>
      </div>
    </div>
    
    <div v-else class="server-tab-empty-state">
      <p class="server-tab-empty-text" v-text="t('servers.NoUsers')" />
    </div>
    
    <div v-if="server.hasScope('server.users.create')" class="server-tab-section">
      <h3 class="server-tab-section-title">{{ t('servers.InviteUser') }}</h3>
      <div class="server-tab-card">
        <div class="server-invite-form">
          <text-field
            v-model="newEmail"
            type="email"
            icon="email"
            :label="t('users.Email')"
            class="server-invite-email"
          />
          <btn color="primary" @click="sendInvite()">
            <icon name="plus" />
            {{ t('servers.InviteUser') }}
          </btn>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.server-tab-content {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  padding: 1.5rem;
  max-width: 100%;
}

.server-tab-title {
  font-size: 1.5rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  margin: 0;
  padding-bottom: 1rem;
  border-bottom: 2px solid rgb(var(--color-border) / 0.5);
}

.server-tab-section {
  width: 100%;
}

.server-tab-section-title {
  font-size: 1.125rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  margin: 0 0 1rem 0;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid rgb(var(--color-border) / 0.3);
}

.server-users-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.server-user-card {
  background: rgb(var(--color-background));
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
  padding: 1rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  transition: all 0.2s ease-in-out;
}

.server-user-card:hover {
  border-color: rgb(var(--color-border));
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.1);
}

.server-user-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
  padding: 0.5rem 0;
}

.server-user-info {
  flex: 1;
  min-width: 0;
}

.server-user-name {
  font-size: 1rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  margin: 0 0 0.25rem 0;
}

.server-user-email {
  display: block;
  font-size: 0.875rem;
  color: rgb(var(--color-muted-foreground));
}

.server-user-chevron {
  width: 1.25rem;
  height: 1.25rem;
  color: rgb(var(--color-muted-foreground));
  transition: transform 0.2s ease-in-out;
  flex-shrink: 0;
}

.server-user-header:hover .server-user-chevron {
  color: rgb(var(--color-primary));
}

.server-user-permissions {
  padding-top: 1rem;
  margin-top: 1rem;
  border-top: 1px solid rgb(var(--color-border) / 0.3);
}

.server-permissions-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.server-permission-item {
  padding: 0.5rem;
  border-radius: 0.5rem;
  transition: background 0.2s ease-in-out;
}

.server-permission-item:hover {
  background: rgb(var(--color-muted) / 0.3);
}

.server-user-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 1rem;
  border-top: 1px solid rgb(var(--color-border) / 0.2);
}

.server-tab-card {
  background: rgb(var(--color-background));
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
  padding: 1.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.server-invite-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.server-invite-email {
  flex: 1;
}

.server-tab-empty-state {
  padding: 3rem 1.5rem;
  text-align: center;
  background: rgb(var(--color-muted) / 0.2);
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
}

.server-tab-empty-text {
  color: rgb(var(--color-muted-foreground));
  margin: 0;
  font-size: 0.875rem;
}

@media (max-width: 768px) {
  .server-permissions-grid {
    grid-template-columns: 1fr;
  }
}
</style>
