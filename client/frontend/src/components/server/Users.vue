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
  <div class="space-y-6 p-4">
    <h2 class="text-2xl font-bold text-foreground" v-text="t('users.Users')" />
    <div v-for="user in users" :key="user.email" class="rounded-xl border-2 border-border/50 bg-background p-4 space-y-4">
      <h3 @click="user.open = !user.open" class="text-xl font-semibold text-foreground cursor-pointer hover:text-primary transition-colors" v-text="user.username" />
      <div v-if="user.open" class="space-y-3 pl-4 border-l-2 border-border/50">
        <toggle
          v-for="perm in perms"
          :key="perm.name"
          v-model="user.scopes[perm.name]"
          :disabled="permissionDisabled(perm.name)"
          :label="perm.label"
          :hint="perm.hint"
          @update:modelValue="updatePerms(user)"
        />
        <div class="pt-2">
          <btn v-if="server.hasScope('server.users.delete')" color="error" @click="deleteUser(user)" v-text="t('users.Delete')" />
        </div>
      </div>
    </div>
    <div v-if="users.length === 0" class="text-center py-8 text-muted-foreground" v-text="t('servers.NoUsers')" />
    <div v-if="server.hasScope('server.users.create')" class="space-y-4 p-4 rounded-xl border-2 border-border/50 bg-background">
      <text-field v-model="newEmail" type="email" icon="email" :label="t('users.Email')" />
      <btn color="primary" @click="sendInvite()"><icon name="plus" />{{ t('servers.InviteUser') }}</btn>
    </div>
  </div>
</template>
