<script setup>
import {ref, inject, onMounted, computed} from 'vue'
import { useI18n } from 'vue-i18n'
const events = inject('events')
import Loader from '@/components/ui/Loader.vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import TextField from '@/components/ui/TextField.vue'

const { t, locale } = useI18n()
const toast = inject('toast')

const props = defineProps({
  server: { type: Object, required: true }
})

const backups = ref(null)
const backupName = ref("")
const backupRunning = ref(false)
const loading = ref(false)
const sortedBackups = computed(() => backups.value.slice().sort((a, b) => b.createdAt.localeCompare(a.createdAt)));

onMounted(async () => {
  await loadBackups()
})

async function loadBackups() {
  backups.value = await props.server.getBackups()
}

function isBackingUp() {
  return backupRunning.value
}

function isLoading() {
  return !Array.isArray(backups.value) || loading.value
}

async function save() {
  try {
    backupRunning.value = true
    await props.server.createBackup(backupName.value)
    toast.success(t('backup.BackupStarted'))
    await loadBackups()
  }
  finally {
    backupRunning.value = false
  }
}

/*
const numFormat = new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 })
function formatFileSize(size) {
  if (!size) return '0 B'
  if (size < Math.pow(2, 10)) return numFormat.format(size) + ' B'
  if (size < Math.pow(2, 20)) return numFormat.format(size / Math.pow(2, 10)) + ' KiB'
  if (size < Math.pow(2, 30)) return numFormat.format(size / Math.pow(2, 20)) + ' MiB'
  if (size < Math.pow(2, 40)) return numFormat.format(size / Math.pow(2, 30)) + ' GiB'
  return numFormat.format(size / Math.pow(2, 40)) + ' TiB'
}
*/

function promptRestore(file){
  events.emit(
      'confirm',
      {
        title: t('backup.RestorePrompt'),
        body: t('backup.RestorePromptBody'),
      },
      {
        text: t('backup.Restore'),
        icon: 'remove',
        action: () => {
          restore(file)
        }
      },
      {
        color: 'neutral'
      }
    )
}

async function restore(file) {
  try {
    loading.value = true
    await props.server.restoreBackup(file.id);
    toast.success(t('backup.RestoreStarted'))
    await loadBackups()
  }
  finally {
    loading.value = false
  }
}

function promptDelete(file){
  events.emit(
      'confirm',
      {
        title: t('backup.DeletePrompt'),
        body: t('backup.DeletePromptBody'),
      },
      {
        text: t('backup.Delete'),
        icon: 'restore',
        color: 'error',
        action: () => {
          deleteBackup(file)
        }
      },
      {
        color: 'primary'
      }
    )
}

async function deleteBackup(file) {
  try {
    loading.value = true
    await props.server.deleteBackup(file.id);
    toast.success(t('backup.Deleted'))
    await loadBackups()
  }
  finally {
    loading.value = false
  }
}

const intl = new Intl.DateTimeFormat(
  [locale.value.replace('_', '-'), 'en'],
  { day: '2-digit', month: '2-digit', year: 'numeric', hour: 'numeric', minute: 'numeric', second: 'numeric' }
)

</script>

<template>
  <div class="server-tab-content">
    <div class="server-tab-section">
      <h2 class="server-tab-title" v-text="t('backup.Backup')" />
    </div>
    
    <div v-if="server.hasScope('server.backup.create')" class="server-tab-section">
      <h3 class="server-tab-section-title">{{ t('backup.Create') }}</h3>
      <div class="server-tab-card">
        <div class="server-backup-create">
          <text-field
            v-model="backupName"
            :label="t('backup.Name')"
            :placeholder="t('backup.NamePlaceholder') || 'Nombre del backup'"
            class="server-backup-name-input"
          />
          <btn
            color="primary"
            :disabled="isBackingUp() || isLoading()"
            @click="save()"
          >
            <icon v-if="!isBackingUp()" name="plus" />
            <icon v-else name="restart" spin />
            {{ t('backup.Create') }}
          </btn>
        </div>
      </div>
    </div>

    <div class="server-tab-section">
      <h3 class="server-tab-section-title" v-text="t('backup.BackupsHeader')" />
      <div class="server-backups-list">
        <loader v-if="isLoading()" />
        <div v-else-if="sortedBackups.length === 0" class="server-tab-empty-state">
          <p class="server-tab-empty-text" v-text="t('backup.NoBackups') || 'No hay backups disponibles'" />
        </div>
        <div
          v-for="backup in sortedBackups"
          :key="backup.id"
          class="server-backup-item"
        >
          <div class="server-backup-icon">
            <icon name="backup" />
          </div>
          <div class="server-backup-info">
            <div class="server-backup-name">{{ backup.name }}</div>
            <div class="server-backup-date">{{ intl.format(new Date(backup.createdAt)) }}</div>
          </div>
          <div class="server-backup-actions">
            <btn
              v-if="server.hasScope('server.backup.restore')"
              variant="icon"
              :tooltip="t('backup.Restore')"
              :disabled="isBackingUp()"
              @click="promptRestore(backup)"
            >
              <icon name="restart" />
            </btn>
            <a
              :href="props.server.getBackupUrl(backup.id)"
              target="_blank"
              rel="noopener"
            >
              <btn variant="icon" :tooltip="t('backup.Download')">
                <icon name="download" />
              </btn>
            </a>
            <btn
              v-if="server.hasScope('server.backup.delete')"
              variant="icon"
              color="error"
              :tooltip="t('backup.Delete')"
              :disabled="isBackingUp()"
              @click="promptDelete(backup)"
            >
              <icon name="remove" />
            </btn>
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

.server-tab-card {
  background: rgb(var(--color-background));
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
  padding: 1.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.server-backup-create {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.server-backup-name-input {
  flex: 1;
}

.server-backups-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.server-backup-item {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1rem;
  background: rgb(var(--color-background));
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.75rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  transition: all 0.2s ease-in-out;
}

.server-backup-item:hover {
  border-color: rgb(var(--color-border));
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.1);
}

.server-backup-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3rem;
  height: 3rem;
  background: rgb(var(--color-primary) / 0.1);
  border-radius: 0.5rem;
  color: rgb(var(--color-primary));
  flex-shrink: 0;
}

.server-backup-icon icon {
  width: 1.5rem;
  height: 1.5rem;
}

.server-backup-info {
  flex: 1;
  min-width: 0;
}

.server-backup-name {
  font-size: 1rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  margin-bottom: 0.25rem;
}

.server-backup-date {
  font-size: 0.875rem;
  color: rgb(var(--color-muted-foreground));
}

.server-backup-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
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
</style>
