<script setup>
import { ref, onMounted, inject } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'

const { t } = useI18n()
const api = inject('api')

const props = defineProps({
  server: { type: Object, required: true }
})

const host = ref('')
const user = ref('')
const hostField = ref(null)
const userField = ref(null)
const hostCopied = ref(false)
const userCopied = ref(false)
const userEncoded = ref('')

onMounted(async () => {
  host.value = (props.server.node.publicHost !== '127.0.0.1' && props.server.node.publicHost !== 'localhost') ? props.server.node.publicHost : window.location.hostname
  host.value = host.value + ':' + props.server.node.sftpPort
  const u = await api.self.get()
  user.value = `${u.email}#${props.server.id}`
  userEncoded.value = encodeURIComponent(user.value);
})

function copyHost() {
  hostField.value.select()
  document.execCommand('copy')
  userCopied.value = false
  hostCopied.value = true
  setTimeout(() => {
    hostCopied.value = false
  }, 6000)
}

function copyUser() {
  userField.value.select()
  document.execCommand('copy')
  hostCopied.value = false
  userCopied.value = true
  setTimeout(() => {
    userCopied.value = false
  }, 6000)
}
</script>

<template>
  <div class="space-y-4 p-4">
    <h2 class="text-2xl font-bold text-foreground" v-text="t('servers.SFTPInfo')" />
    <div class="space-y-4">
      <div class="flex items-center gap-2 flex-wrap">
        <span class="font-semibold text-foreground">{{t('common.Host')}}/{{t('common.Port')}}: </span>
        <span class="text-foreground font-mono">{{host}}</span>
        <btn variant="icon" :tooltip="t('common.Copy')" @click="copyHost()"><icon name="copy" /></btn>
        <span v-if="hostCopied" class="text-sm text-success" v-text="t('common.Copied')" />
      </div>
      <input ref="hostField" :value="host" class="fixed left-[-100vw] w-px h-px" />
      <div class="flex items-center gap-2 flex-wrap">
        <span class="font-semibold text-foreground">{{t('users.Username')}}: </span>
        <span class="text-foreground font-mono">{{user}}</span>
        <btn variant="icon" :tooltip="t('common.Copy')" @click="copyUser()"><icon name="copy" /></btn>
        <span v-if="userCopied" class="text-sm text-success" v-text="t('common.Copied')" />
      </div>
      <input ref="userField" :value="user" class="fixed left-[-100vw] w-px h-px" />
      <div class="flex items-center gap-2 flex-wrap">
        <span class="font-semibold text-foreground">{{t('users.Password')}}: </span>
        <span class="text-foreground">{{t('users.AccountPassword')}}</span>
      </div>
      <div class="pt-2">
        <a :href="`sftp://${userEncoded}@${host}`"><btn color="primary" v-text="t('servers.SftpConnection')" /></a>
      </div>
    </div>
  </div>
</template>
