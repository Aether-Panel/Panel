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
  <div class="sftp-container">
    <!-- Header con título y descripción -->
    <div class="sftp-header">
      <div class="sftp-header-content">
        <div class="sftp-header-icon">
          <icon name="sftp" />
        </div>
        <div>
          <h2 class="sftp-title" v-text="t('servers.SFTPInfo')" />
          <p class="sftp-subtitle" v-text="t('servers.SFTPInfoDescription') || 'Información de conexión SFTP para este servidor'" />
        </div>
      </div>
    </div>
    
    <!-- Información de conexión -->
    <div class="sftp-card">
      <div class="sftp-info">
        <!-- Host/Port -->
        <div class="sftp-field">
          <label class="sftp-label">
            <icon name="server" class="label-icon" />
            {{ t('common.Host') }}/{{ t('common.Port') }}
          </label>
          <div class="sftp-value-wrapper">
            <code class="sftp-value">{{ host }}</code>
              <button
              class="sftp-copy-btn"
              :class="{ 'copied': hostCopied }"
                :title="t('common.Copy')"
                @click="copyHost()"
              >
              <icon :name="hostCopied ? 'copy-check' : 'copy'" />
              </button>
            </div>
            <input ref="hostField" :value="host" class="sr-only" />
          </div>
          
        <!-- Username -->
        <div class="sftp-field">
          <label class="sftp-label">
            <icon name="user" class="label-icon" />
            {{ t('users.Username') }}
          </label>
          <div class="sftp-value-wrapper">
            <code class="sftp-value">{{ user }}</code>
              <button
              class="sftp-copy-btn"
              :class="{ 'copied': userCopied }"
                :title="t('common.Copy')"
                @click="copyUser()"
              >
              <icon :name="userCopied ? 'copy-check' : 'copy'" />
              </button>
            </div>
            <input ref="userField" :value="user" class="sr-only" />
          </div>
          
        <!-- Password -->
        <div class="sftp-field">
          <label class="sftp-label">
            <icon name="lock" class="label-icon" />
            {{ t('users.Password') }}
          </label>
          <span class="sftp-password-hint">{{ t('users.AccountPassword') }}</span>
          </div>
          
        <!-- Botón de conexión -->
        <div class="sftp-connect">
          <a :href="`sftp://${userEncoded}@${host}`" class="sftp-connect-link">
            <btn color="primary" variant="outline" class="sftp-connect-btn">
                <icon name="sftp" />
                {{ t('servers.SftpConnection') }}
              </btn>
            </a>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.sftp-container {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  padding: 2rem;
  max-width: 1400px;
  margin: 0 auto;
}

/* Header */
.sftp-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 1.5rem;
  border-bottom: 2px solid #475569;
}

.sftp-header-content {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.sftp-header-icon {
  width: 2.5rem;
  height: 2.5rem;
  padding: 0.5rem;
  background: #3b82f6;
  border-radius: 0.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.sftp-header-icon :deep(svg),
.sftp-header-icon :deep(svg path),
.sftp-header-icon :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
  width: 1.5rem;
  height: 1.5rem;
}

.sftp-title {
  font-size: 1.875rem;
  font-weight: 700;
  color: #f1f5f9;
  margin: 0;
  line-height: 1.2;
}

.sftp-subtitle {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0.25rem 0 0;
}

/* Card */
.sftp-card {
  background: #1e293b;
  border: 2px solid #475569;
  border-radius: 1rem;
  padding: 2rem;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.sftp-card:hover {
  border-color: #3b82f6;
}

.sftp-info {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.sftp-field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.sftp-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: #cbd5e1;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.sftp-label :deep(svg),
.sftp-label :deep(svg path),
.sftp-label :deep(svg *),
.label-icon {
  width: 1rem;
  height: 1rem;
  color: #94a3b8 !important;
  fill: #94a3b8 !important;
  stroke: #94a3b8 !important;
}

.sftp-value-wrapper {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem;
  background: #0f172a;
  border: 2px solid #475569;
  border-radius: 0.5rem;
  transition: all 0.2s;
}

.sftp-value-wrapper:hover {
  border-color: #3b82f6;
}

.sftp-value {
  flex: 1;
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 0.875rem;
  color: #f1f5f9;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Botón de copiar */
.sftp-copy-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  padding: 0;
  background: #334155;
  border: none;
  border-radius: 0.375rem;
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
  color: #e2e8f0;
}

.sftp-copy-btn icon {
  display: block;
  width: 1.125rem;
  height: 1.125rem;
}

.sftp-copy-btn :deep(svg) {
  width: 1.125rem !important;
  height: 1.125rem !important;
  display: block !important;
}

.sftp-copy-btn :deep(svg path),
.sftp-copy-btn :deep(svg *) {
  color: #e2e8f0 !important;
  fill: #e2e8f0 !important;
  stroke: #e2e8f0 !important;
}

.sftp-copy-btn:hover {
  background: #3b82f6;
  color: #ffffff;
  transform: scale(1.05);
}

.sftp-copy-btn:hover :deep(svg),
.sftp-copy-btn:hover :deep(svg path),
.sftp-copy-btn:hover :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
}

.sftp-copy-btn.copied {
  background: #10b981;
  color: #ffffff;
}

.sftp-copy-btn.copied :deep(svg),
.sftp-copy-btn.copied :deep(svg path),
.sftp-copy-btn.copied :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
}

.sftp-copy-btn:active {
  transform: scale(0.95);
}

.sftp-password-hint {
  font-size: 0.875rem;
  color: #cbd5e1;
  font-style: italic;
  padding: 0.5rem 0.75rem;
  background: #0f172a;
  border: 2px solid #475569;
  border-radius: 0.5rem;
}

.sftp-connect {
  padding-top: 0.5rem;
  border-top: 1px solid #334155;
}

.sftp-connect-link {
  text-decoration: none;
  display: inline-block;
}

.sftp-connect-btn {
  width: 100%;
  justify-content: center;
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

/* Responsive */
@media (max-width: 768px) {
  .sftp-container {
    padding: 1rem;
  }
  
  .sftp-title {
    font-size: 1.5rem;
  }
}
</style>
