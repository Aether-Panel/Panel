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
  <div class="backup-container">
    <!-- Header con título y contador -->
    <div class="backup-header">
      <div class="backup-header-content">
        <div class="backup-header-icon">
          <icon name="backup" />
        </div>
        <div>
          <h2 class="backup-title" v-text="t('backup.Backup')" />
          <p class="backup-subtitle">
            {{ sortedBackups.length }} {{ sortedBackups.length === 1 ? 'backup' : 'backups' }}
          </p>
        </div>
      </div>
    </div>
    
    <!-- Formulario de creación -->
    <div v-if="server.hasScope('server.backup.create')" class="create-section">
      <div class="create-card">
        <div class="create-header">
          <icon name="plus" class="create-icon" />
          <h3 class="create-title">{{ t('backup.Create') }}</h3>
        </div>
        <div class="create-form">
          <text-field
            v-model="backupName"
            :label="t('backup.Name')"
            :placeholder="t('backup.NamePlaceholder') || 'Nombre del backup'"
            icon="backup"
          />
          <btn
            color="primary"
            size="lg"
            :disabled="isBackingUp() || isLoading()"
            @click="save()"
            class="create-button"
          >
            <icon v-if="!isBackingUp()" name="plus" />
            <icon v-else name="restart" :class="{ 'spinning': isBackingUp() }" />
            {{ t('backup.Create') }}
          </btn>
        </div>
      </div>
    </div>

    <!-- Lista de backups -->
    <div class="backups-section">
        <loader v-if="isLoading()" />
      <div v-else-if="sortedBackups.length === 0" class="empty-state">
        <div class="empty-state-icon">
          <icon name="backup" />
        </div>
        <h3 class="empty-state-title" v-text="t('backup.NoBackups') || 'No hay backups'" />
        <p class="empty-state-text">Crea tu primer backup para comenzar</p>
      </div>
      <div v-else class="backups-grid">
        <div
          v-for="backup in sortedBackups"
          :key="backup.id"
          class="backup-card"
        >
          <div class="backup-card-content">
            <div class="backup-icon-wrapper">
            <icon name="backup" />
          </div>
            <div class="backup-info">
              <h3 class="backup-name">{{ backup.name }}</h3>
              <p class="backup-date">{{ intl.format(new Date(backup.createdAt)) }}</p>
          </div>
            <div class="backup-actions">
            <btn
              v-if="server.hasScope('server.backup.restore')"
              variant="icon"
              :tooltip="t('backup.Restore')"
              :disabled="isBackingUp()"
              @click="promptRestore(backup)"
                class="backup-action-btn"
            >
              <icon name="restart" />
            </btn>
            <a
              :href="props.server.getBackupUrl(backup.id)"
              target="_blank"
              rel="noopener"
                class="backup-action-link"
            >
                <btn variant="icon" :tooltip="t('backup.Download')" class="backup-action-btn">
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
                class="backup-action-btn"
            >
              <icon name="remove" />
            </btn>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.backup-container {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  padding: 2rem;
  max-width: 1400px;
  margin: 0 auto;
}

/* Header */
.backup-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 1.5rem;
  border-bottom: 2px solid #475569;
}

.backup-header-content {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.backup-header-icon {
  width: 2.5rem;
  height: 2.5rem;
  padding: 0.5rem;
  background: #3b82f6;
  border-radius: 0.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.backup-header-icon :deep(svg),
.backup-header-icon :deep(svg path),
.backup-header-icon :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
  width: 1.5rem;
  height: 1.5rem;
}

.backup-title {
  font-size: 1.875rem;
  font-weight: 700;
  color: #f1f5f9;
  margin: 0;
  line-height: 1.2;
}

.backup-subtitle {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0.25rem 0 0;
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

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

.spinning {
  animation: spin 1s linear infinite;
}

/* Grid de backups */
.backups-section {
  margin-top: 0;
}

.backups-grid {
  display: grid;
  gap: 1.5rem;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
}

@media (max-width: 768px) {
  .backups-grid {
    grid-template-columns: 1fr;
  }
}

/* Tarjeta de backup */
.backup-card {
  background: #1e293b;
  border: 2px solid #475569;
  border-radius: 1rem;
  overflow: hidden;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.backup-card:hover {
  border-color: #3b82f6;
  transform: translateY(-2px);
}

.backup-card-content {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1.25rem 1.5rem;
}

.backup-icon-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3rem;
  height: 3rem;
  background: #3b82f6;
  border-radius: 0.75rem;
  flex-shrink: 0;
}

.backup-icon-wrapper :deep(svg),
.backup-icon-wrapper :deep(svg path),
.backup-icon-wrapper :deep(svg *) {
  width: 1.5rem;
  height: 1.5rem;
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
}

.backup-info {
  flex: 1;
  min-width: 0;
}

.backup-name {
  font-size: 1.125rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0 0 0.25rem;
}

.backup-date {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0;
}

.backup-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.backup-action-btn {
  width: 2.5rem;
  height: 2.5rem;
}

.backup-action-link {
  text-decoration: none;
  display: inline-flex;
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
  .backup-container {
    padding: 1rem;
  }
  
  .backup-title {
    font-size: 1.5rem;
  }
  
  .backup-card-content {
    flex-wrap: wrap;
  }
  
  .backup-actions {
    width: 100%;
    justify-content: flex-end;
    margin-top: 0.5rem;
  }
}
</style>
