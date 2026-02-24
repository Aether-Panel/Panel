<script setup>
import { ref, inject, onMounted } from 'vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import TextField from '@/components/ui/TextField.vue'

const toast = inject('toast')
const api = inject('api')

const databases = ref([])
const databaseHosts = ref([])
const newDatabaseName = ref('')
const selectedHost = ref(null)
const isLoading = ref(false)
const copiedId = ref(null)

const props = defineProps({
  server: { type: Object, required: true }
})

async function createDatabase() {
  if (!newDatabaseName.value || !selectedHost.value) {
    toast.error('Por favor completa todos los campos')
    return
  }

  isLoading.value = true
  try {
    await api.database.create(props.server.id, {
      database_name: newDatabaseName.value,
      database_host_id: parseInt(selectedHost.value)
    })
    toast.success('Base de datos creada exitosamente')
    newDatabaseName.value = ''
    selectedHost.value = null
    await loadDatabases()
  } catch (error) {
    console.error('Error creating database:', error)
    toast.error('Error al crear la base de datos: ' + (error.msg || error.message))
  } finally {
    isLoading.value = false
  }
}

async function deleteDatabase(db) {
  if (!confirm(`¿Estás seguro de que deseas eliminar la base de datos "${db.database_name}"? Esto también eliminará el usuario y todos los datos.`)) {
    return
  }

  try {
    await api.database.delete(props.server.id, db.id)
    toast.success('Base de datos eliminada exitosamente')
    await loadDatabases()
  } catch (error) {
    console.error('Error deleting database:', error)
    toast.error('Error al eliminar la base de datos: ' + (error.msg || error.message))
  }
}

async function loadDatabases() {
  try {
    databases.value = await api.database.list(props.server.id)
  } catch (error) {
    console.error('Error loading databases:', error)
    databases.value = []
  }
}

async function loadDatabaseHosts() {
  try {
    databaseHosts.value = await api.databaseHost.list()
    if (databaseHosts.value.length > 0) {
      selectedHost.value = databaseHosts.value[0].id.toString()
    }
  } catch (error) {
    console.error('Error loading database hosts:', error)
    databaseHosts.value = []
  }
}

function copyToClipboard(text, id) {
  navigator.clipboard.writeText(text).then(() => {
    copiedId.value = id
    setTimeout(() => {
      copiedId.value = null
    }, 2000)
  }).catch(err => {
    console.error('Error al copiar:', err)
    toast.error('Error al copiar al portapapeles')
  })
}

function toggleDatabase(db) {
  db.open = !db.open
}

onMounted(async () => {
  await Promise.all([loadDatabases(), loadDatabaseHosts()])
})
</script>

<template>
  <div class="databases-container">
    <!-- Header con título y contador -->
    <div class="databases-header">
      <div class="databases-header-content">
        <div class="databases-header-icon">
          <icon name="database" />
        </div>
        <div>
          <h2 class="databases-title">Bases de Datos</h2>
          <p class="databases-subtitle">
            {{ databases.length }} {{ databases.length === 1 ? 'base de datos' : 'bases de datos' }}
          </p>
        </div>
      </div>
    </div>
    
    <!-- Lista de bases de datos existentes -->
    <div v-if="databases.length > 0" class="databases-grid">
      <div
        v-for="db in databases"
        :key="db.id"
        class="database-card"
        :class="{ 'expanded': db.open }"
      >
        <div class="database-card-header" @click="toggleDatabase(db)">
          <div class="database-card-title">
            <div class="database-icon-wrapper">
              <icon name="database" class="database-icon" />
            </div>
            <div class="database-card-info">
              <h3 class="database-name">{{ db.database_name }}</h3>
              <span class="database-host-badge">
                <icon name="server" class="badge-icon" />
                {{ db.host_name || 'Database Host' }}
              </span>
            </div>
          </div>
          <icon
            :name="db.open ? 'chevron-down' : 'chevron-right'"
            class="expand-icon"
          />
        </div>
        
        <transition name="expand">
          <div v-if="db.open" class="database-card-body">
            <!-- Información de conexión en grid -->
            <div class="connection-grid">
              <!-- Host -->
              <div class="connection-item">
                <label class="connection-label">
                  <icon name="server" class="label-icon" />
                  Host
                </label>
                <div class="connection-value-wrapper">
                  <code class="connection-value">{{ db.host }}:{{ db.port }}</code>
                  <button
                    class="copy-button"
                    :class="{ 'copied': copiedId === `host-${db.id}` }"
                    @click="copyToClipboard(`${db.host}:${db.port}`, `host-${db.id}`)"
                    title="Copiar"
                  >
                    <icon :name="copiedId === `host-${db.id}` ? 'copy-check' : 'copy'" />
                  </button>
                </div>
              </div>
              
              <!-- Usuario -->
              <div class="connection-item">
                <label class="connection-label">
                  <icon name="user" class="label-icon" />
                  Usuario
                </label>
                <div class="connection-value-wrapper">
                  <code class="connection-value">{{ db.username }}</code>
                  <button
                    class="copy-button"
                    :class="{ 'copied': copiedId === `user-${db.id}` }"
                    @click="copyToClipboard(db.username, `user-${db.id}`)"
                    title="Copiar"
                  >
                    <icon :name="copiedId === `user-${db.id}` ? 'copy-check' : 'copy'" />
                  </button>
                </div>
              </div>
              
              <!-- Contraseña -->
              <div class="connection-item">
                <label class="connection-label">
                  <icon name="lock" class="label-icon" />
                  Contraseña
                </label>
                <div class="connection-value-wrapper">
                  <code class="connection-value password-value">{{ db.password }}</code>
                  <button
                    class="copy-button"
                    :class="{ 'copied': copiedId === `pass-${db.id}` }"
                    @click="copyToClipboard(db.password, `pass-${db.id}`)"
                    title="Copiar"
                  >
                    <icon :name="copiedId === `pass-${db.id}` ? 'copy-check' : 'copy'" />
                  </button>
                </div>
              </div>
              
              <!-- Base de Datos -->
              <div class="connection-item">
                <label class="connection-label">
                  <icon name="database" class="label-icon" />
                  Base de Datos
                </label>
                <div class="connection-value-wrapper">
                  <code class="connection-value">{{ db.database_name }}</code>
                  <button
                    class="copy-button"
                    :class="{ 'copied': copiedId === `db-${db.id}` }"
                    @click="copyToClipboard(db.database_name, `db-${db.id}`)"
                    title="Copiar"
                  >
                    <icon :name="copiedId === `db-${db.id}` ? 'copy-check' : 'copy'" />
                  </button>
                </div>
              </div>
            </div>
            
            <!-- Cadena de conexión completa -->
            <div class="connection-string-section">
              <label class="connection-label">
                <icon name="link" class="label-icon" />
                Cadena de Conexión
              </label>
              <div class="connection-string-wrapper">
                <code class="connection-string">
                  mysql://{{ db.username }}:{{ db.password }}@{{ db.host }}:{{ db.port }}/{{ db.database_name }}
                </code>
                <button
                  class="copy-button"
                  :class="{ 'copied': copiedId === `conn-${db.id}` }"
                  @click="copyToClipboard(`mysql://${db.username}:${db.password}@${db.host}:${db.port}/${db.database_name}`, `conn-${db.id}`)"
                  title="Copiar cadena completa"
                >
                  <icon :name="copiedId === `conn-${db.id}` ? 'copy-check' : 'copy'" />
                </button>
              </div>
            </div>
            
            <!-- Botón de eliminar -->
            <div class="database-actions">
              <btn
                color="error"
                variant="outline"
                @click="deleteDatabase(db)"
              >
                <icon name="remove" />
                Eliminar Base de Datos
              </btn>
            </div>
          </div>
        </transition>
      </div>
    </div>
    
    <!-- Estado vacío -->
    <div v-else class="empty-state">
      <div class="empty-state-icon">
        <icon name="database" />
      </div>
      <h3 class="empty-state-title">No hay bases de datos</h3>
      <p class="empty-state-text">Crea tu primera base de datos para comenzar</p>
    </div>
    
    <!-- Formulario de creación -->
    <div v-if="databaseHosts.length > 0" class="create-section">
      <div class="create-card">
        <div class="create-header">
          <icon name="plus" class="create-icon" />
          <h3 class="create-title">Crear Nueva Base de Datos</h3>
        </div>
        
        <div class="create-form">
          <div class="form-grid">
            <text-field
              v-model="newDatabaseName"
              icon="database"
              label="Nombre de la Base de Datos"
              placeholder="mi_base_datos"
            />
            
            <div class="select-field">
              <label class="select-label">
                <icon name="server" class="label-icon" />
                Database Host
              </label>
              <select v-model="selectedHost" class="select-input">
                <option
                  v-for="host in databaseHosts"
                  :key="host.id"
                  :value="host.id.toString()"
                >
                  {{ host.name }}
                </option>
              </select>
            </div>
          </div>
          
          <btn
            color="primary"
            size="lg"
            :disabled="isLoading || !newDatabaseName || !selectedHost"
            @click="createDatabase()"
            class="create-button"
          >
            <icon :name="isLoading ? 'loading' : 'plus'" :class="{ 'spinning': isLoading }" />
            {{ isLoading ? 'Creando...' : 'Crear Base de Datos' }}
          </btn>
        </div>
      </div>
    </div>
    
    <!-- Advertencia si no hay hosts -->
    <div v-else class="warning-card">
      <icon name="warning" class="warning-icon" />
      <div>
        <h3 class="warning-title">Sin Database Hosts Configurados</h3>
        <p class="warning-text">
          No hay Database Hosts configurados. Contacta al administrador para configurar uno antes de crear bases de datos.
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.databases-container {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  padding: 2rem;
  max-width: 1400px;
  margin: 0 auto;
}

/* Header */
.databases-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 1.5rem;
  border-bottom: 2px solid #475569;
}

.databases-header-content {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.databases-header-icon {
  width: 2.5rem;
  height: 2.5rem;
  padding: 0.5rem;
  background: #3b82f6;
  border-radius: 0.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.databases-header-icon :deep(svg),
.databases-header-icon :deep(svg path),
.databases-header-icon :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
  width: 1.5rem;
  height: 1.5rem;
}

.databases-title {
  font-size: 1.875rem;
  font-weight: 700;
  color: #f1f5f9;
  margin: 0;
  line-height: 1.2;
}

.databases-subtitle {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0.25rem 0 0;
}

/* Grid de bases de datos */
.databases-grid {
  display: grid;
  gap: 1.5rem;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
}

@media (max-width: 768px) {
  .databases-grid {
    grid-template-columns: 1fr;
  }
}

/* Tarjeta de base de datos */
.database-card {
  background: #1e293b;
  border: 2px solid #475569;
  border-radius: 1rem;
  overflow: hidden;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.database-card:hover {
  border-color: #3b82f6;
  transform: translateY(-2px);
}

.database-card.expanded {
  border-color: #3b82f6;
}

/* Header de la tarjeta */
.database-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1.25rem 1.5rem;
  cursor: pointer;
  user-select: none;
  background: #1e293b;
  transition: background 0.2s;
}

.database-card-header:hover {
  background: #2d3e52;
}

.database-card-title {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex: 1;
}

.database-icon-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3rem;
  height: 3rem;
  background: #3b82f6;
  border-radius: 0.75rem;
}

.database-icon-wrapper :deep(svg),
.database-icon-wrapper :deep(svg path),
.database-icon-wrapper :deep(svg *),
.database-icon {
  width: 1.5rem;
  height: 1.5rem;
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
}

.database-card-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.database-name {
  font-size: 1.125rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0;
}

.database-host-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.75rem;
  color: #cbd5e1;
  padding: 0.25rem 0.625rem;
  background: #334155;
  border-radius: 0.375rem;
  width: fit-content;
}

.database-host-badge :deep(svg),
.database-host-badge :deep(svg path),
.database-host-badge :deep(svg *),
.badge-icon {
  width: 0.875rem;
  height: 0.875rem;
  color: #cbd5e1 !important;
  fill: #cbd5e1 !important;
  stroke: #cbd5e1 !important;
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

.database-card.expanded .expand-icon {
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
.database-card-body {
  padding: 1.5rem;
  background: #0f172a;
  border-top: 2px solid #334155;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

/* Grid de conexión */
.connection-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
}

.connection-item {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.connection-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: #cbd5e1;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.connection-label :deep(svg),
.connection-label :deep(svg path),
.connection-label :deep(svg *),
.label-icon {
  width: 1rem;
  height: 1rem;
  color: #94a3b8 !important;
  fill: #94a3b8 !important;
  stroke: #94a3b8 !important;
}

.connection-value-wrapper {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem;
  background: #1e293b;
  border: 2px solid #475569;
  border-radius: 0.5rem;
  transition: all 0.2s;
}

.connection-value-wrapper:hover {
  border-color: var(--primary);
}

.connection-value {
  flex: 1;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 0.875rem;
  color: #f1f5f9;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.password-value {
  letter-spacing: 0.1em;
}

/* Botón de copiar */
.copy-button {
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

.copy-button icon {
  display: block;
  width: 1.125rem;
  height: 1.125rem;
}

.copy-button :deep(svg) {
  width: 1.125rem !important;
  height: 1.125rem !important;
  display: block !important;
}

.copy-button :deep(svg path),
.copy-button :deep(svg *) {
  color: #e2e8f0 !important;
  fill: #e2e8f0 !important;
  stroke: #e2e8f0 !important;
}

.copy-button:hover {
  background: #3b82f6;
  color: #ffffff;
  transform: scale(1.05);
}

.copy-button:hover :deep(svg),
.copy-button:hover :deep(svg path),
.copy-button:hover :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
}

.copy-button.copied {
  background: #10b981;
  color: #ffffff;
}

.copy-button.copied :deep(svg),
.copy-button.copied :deep(svg path),
.copy-button.copied :deep(svg *) {
  color: #ffffff !important;
  fill: #ffffff !important;
  stroke: #ffffff !important;
}

.copy-button:active {
  transform: scale(0.95);
}

/* Sección de cadena de conexión */
.connection-string-section {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.connection-string-wrapper {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 1rem;
  background: #1e293b;
  border: 2px solid #475569;
  border-radius: 0.5rem;
  transition: all 0.2s;
}

.connection-string-wrapper:hover {
  border-color: var(--primary);
}

.connection-string {
  flex: 1;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 0.875rem;
  color: #f1f5f9;
  overflow-x: auto;
  white-space: nowrap;
  scrollbar-width: thin;
  scrollbar-color: #475569 transparent;
}

.connection-string::-webkit-scrollbar {
  height: 4px;
}

.connection-string::-webkit-scrollbar-track {
  background: transparent;
}

.connection-string::-webkit-scrollbar-thumb {
  background: #475569;
  border-radius: 2px;
}

/* Acciones */
.database-actions {
  display: flex;
  gap: 0.75rem;
  padding-top: 0.5rem;
  border-top: 1px solid var(--border);
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

/* Sección de creación */
.create-section {
  margin-top: 1rem;
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

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 1rem;
}

.select-field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.select-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: #f1f5f9;
}

.select-label :deep(svg),
.select-label :deep(svg path),
.select-label :deep(svg *) {
  color: #e2e8f0 !important;
  fill: #e2e8f0 !important;
  stroke: #e2e8f0 !important;
  width: 1rem;
  height: 1rem;
}

.select-input {
  padding: 0.75rem 1rem;
  background: #0f172a;
  border: 2px solid #475569;
  border-radius: 0.5rem;
  color: #f1f5f9;
  font-size: 1rem;
  cursor: pointer;
  transition: all 0.2s;
}

.select-input option {
  background: #1e293b;
  color: #f1f5f9;
}

.select-input:hover {
  border-color: var(--primary);
}

.select-input:focus {
  outline: none;
  border-color: var(--primary);
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

/* Tarjeta de advertencia */
.warning-card {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  padding: 1.5rem;
  background: #1e293b;
  border: 2px solid #f59e0b;
  border-radius: 1rem;
}

.warning-icon {
  width: 1.5rem;
  height: 1.5rem;
  flex-shrink: 0;
  margin-top: 0.25rem;
}

.warning-icon :deep(svg),
.warning-icon :deep(svg path),
.warning-icon :deep(svg *) {
  color: #f59e0b !important;
  fill: #f59e0b !important;
  stroke: #f59e0b !important;
  width: 1.5rem;
  height: 1.5rem;
}

.warning-title {
  font-size: 1rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0 0 0.5rem;
}

.warning-text {
  font-size: 0.875rem;
  color: #cbd5e1;
  margin: 0;
  line-height: 1.5;
}

/* Responsive */
@media (max-width: 768px) {
  .databases-container {
    padding: 1rem;
  }
  
  .databases-title {
    font-size: 1.5rem;
  }
  
  .form-grid {
    grid-template-columns: 1fr;
  }
  
  .connection-grid {
    grid-template-columns: 1fr;
  }
}
</style>
