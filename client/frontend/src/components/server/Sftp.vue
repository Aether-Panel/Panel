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
  <div class="server-tab-content">
    <div class="server-tab-section">
      <h2 class="server-tab-title" v-text="t('servers.SFTPInfo')" />
      <p class="server-tab-subtitle" v-text="t('servers.SFTPInfoDescription') || 'Información de conexión SFTP para este servidor'" />
    </div>
    
    <div class="server-tab-section">
      <div class="server-tab-card">
        <div class="server-sftp-info">
          <div class="server-sftp-field">
            <label class="server-sftp-label">{{ t('common.Host') }}/{{ t('common.Port') }}:</label>
            <div class="server-sftp-value-group">
              <code class="server-sftp-value">{{ host }}</code>
              <button
                class="server-sftp-copy-btn"
                :title="t('common.Copy')"
                @click="copyHost()"
              >
                <icon name="copy" />
              </button>
              <span v-if="hostCopied" class="server-sftp-copied" v-text="t('common.Copied')" />
            </div>
            <input ref="hostField" :value="host" class="sr-only" />
          </div>
          
          <div class="server-sftp-field">
            <label class="server-sftp-label">{{ t('users.Username') }}:</label>
            <div class="server-sftp-value-group">
              <code class="server-sftp-value">{{ user }}</code>
              <button
                class="server-sftp-copy-btn"
                :title="t('common.Copy')"
                @click="copyUser()"
              >
                <icon name="copy" />
              </button>
              <span v-if="userCopied" class="server-sftp-copied" v-text="t('common.Copied')" />
            </div>
            <input ref="userField" :value="user" class="sr-only" />
          </div>
          
          <div class="server-sftp-field">
            <label class="server-sftp-label">{{ t('users.Password') }}:</label>
            <span class="server-sftp-password-hint">{{ t('users.AccountPassword') }}</span>
          </div>
          
          <div class="server-sftp-connect">
            <a :href="`sftp://${userEncoded}@${host}`">
              <btn color="primary" variant="outline">
                <icon name="sftp" />
                {{ t('servers.SftpConnection') }}
              </btn>
            </a>
          </div>
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
  margin: 0 0 0.5rem 0;
  padding-bottom: 1rem;
  border-bottom: 2px solid rgb(var(--color-border) / 0.5);
}

.server-tab-subtitle {
  font-size: 0.875rem;
  color: rgb(var(--color-muted-foreground));
  margin: 0;
  padding-top: 0.5rem;
}

.server-tab-section {
  width: 100%;
}

.server-tab-card {
  background: rgb(var(--color-background));
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
  padding: 1.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.server-sftp-info {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.server-sftp-field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.server-sftp-label {
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
}

.server-sftp-value-group {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.server-sftp-value {
  display: inline-block;
  padding: 0.5rem 0.75rem;
  background: rgb(var(--color-muted) / 0.5);
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.5rem;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 0.875rem;
  color: rgb(var(--color-foreground));
  flex: 1;
  min-width: 200px;
}

.server-sftp-copy-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  padding: 0;
  border: none;
  background: rgb(var(--color-muted) / 0.5);
  color: rgb(var(--color-muted-foreground));
  cursor: pointer;
  border-radius: 0.5rem;
  transition: all 0.2s ease-in-out;
  flex-shrink: 0;
}

.server-sftp-copy-btn:hover {
  background: rgb(var(--color-primary) / 0.1);
  color: rgb(var(--color-primary));
}

.server-sftp-copied {
  font-size: 0.75rem;
  color: rgb(var(--color-success));
  font-weight: 500;
}

.server-sftp-password-hint {
  font-size: 0.875rem;
  color: rgb(var(--color-muted-foreground));
  font-style: italic;
}

.server-sftp-connect {
  padding-top: 0.5rem;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border-width: 0;
}
</style>
