<script setup>
import { ref, inject, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import Overlay from '@/components/ui/Overlay.vue'
import TextField from '@/components/ui/TextField.vue'

const { t } = useI18n()
const api = inject('api')
const events = inject('events')
const templatesLoaded = ref(false)
const templatesByRepo = ref([])
const firstEntry = ref(null)
const addingRepo = ref(false)
const currentRepo = ref({name: '', url: '', branch: ''})

onMounted(async () => {
  loadTemplates()
})

async function loadTemplates() {
  templatesLoaded.value = false
  const templates = await api.template.listAllTemplates()
  templatesByRepo.value = templates.sort((a, b) => a.id > b.id)
  templatesLoaded.value = true
}

function setFirstEntry(ref) {
  if (!firstEntry.value) firstEntry.value = ref
}

function focusList() {
  firstEntry.value.$el.focus()
}

function removeRepo(repo) {
  events.emit(
    'confirm',
    t('templates.ConfirmDeleteRepo', { name: repo.name }),
    {
      text: t('templates.DeleteRepo'),
      icon: 'remove',
      color: 'error',
      action: async () => {
        await api.template.deleteRepo(repo.id)
        await loadTemplates()
      }
    },
    {
      color: 'primary'
    }
  )
}

async function addRepo() {
  await api.template.addRepo({...currentRepo.value, isLocal: false, id: 2})
  resetAddRepo()
  await loadTemplates()
}

function resetAddRepo() {
  currentRepo.value = {name: '', url: '', branch: ''}
  addingRepo.value = false
}

function canAddRepo() {
  if (!currentRepo.value.name || currentRepo.value.name === '') return false
  if (!currentRepo.value.url || currentRepo.value.url === '') return false
  return true
}
</script>

<template>
  <div 
    :class="[
      'templatelist',
      'w-full max-w-7xl mx-auto',
      'space-y-6'
    ]"
  >
    <h1 
      :class="[
        'text-3xl font-bold text-foreground mb-6',
        'pb-3 border-b-2 border-border/50'
      ]"
      v-text="t('templates.Templates')" 
    />
    <div 
      v-hotkey="'l'" 
      :class="['space-y-6']"
      @hotkey="focusList()"
    >
      <div 
        v-for="repo in templatesByRepo" 
        :key="repo.id" 
        :class="['mb-8']"
      >
        <h2 
          :class="[
            'list-header',
            'template-repo-header',
            'flex items-center justify-between px-4 py-3 mb-4',
            'bg-muted/30 border-2 border-border/50 rounded-xl',
            'shadow-sm'
          ]"
        >
          <span 
            :class="[
              'name',
              'text-xl font-bold text-foreground'
            ]"
          >
            {{repo.name}}
          </span>
          <btn 
            v-if="!repo.isLocal && $api.auth.hasScope('templates.repo.delete')" 
            class="remove" 
            variant="icon" 
            color="error"
            @click="removeRepo(repo)"
          >
            <icon name="remove" />
          </btn>
        </h2>
        <!-- Mensaje de error del repositorio -->
        <div 
          v-if="repo.error" 
          :class="[
            'template-repo-error',
            'alert',
            'error',
            'mb-4 p-4 rounded-xl',
            'bg-error/10 border-2 border-error/30',
            'text-error font-medium',
            'shadow-sm'
          ]"
        >
          {{(repo.error.code === 'ErrGeneric' && repo.error.msg) ? t(repo.error.msg) : t('errors.' + repo.error.code)}}
        </div>
        
        <!-- Grid de templates en formato tabla 4 columnas -->
        <div 
          v-if="!repo.error && (repo.templates && repo.templates.length > 0 || (repo.isLocal && $api.auth.hasScope('templates.local.edit')))"
          class="template-grid-wrapper"
        >
          <div class="template-grid-4cols">
            <template v-for="template in repo.templates" :key="template.name">
              <router-link 
                :ref="setFirstEntry" 
                :to="{ name: 'TemplateView', params: { repo: repo.id, id: template.name } }"
                class="template-card-item"
              >
                <div class="template-card-wrapper">
                  <div class="template-card-top">
                    <icon name="chevron-right" class="template-card-arrow" />
                    <h3 class="template-card-title-text">
                      {{template.display || template.name}}
                    </h3>
                  </div>
                  <div class="template-card-bottom">
                    <span class="template-card-type-badge">
                      {{template.type}}
                    </span>
                  </div>
                </div>
              </router-link>
            </template>
            
            <!-- Botón de crear nuevo template (solo para repositorios locales) -->
            <router-link 
              v-if="repo.isLocal && $api.auth.hasScope('templates.local.edit')" 
              v-hotkey="'c'" 
              :to="{ name: 'TemplateCreate' }"
              class="template-card-item template-card-add"
            >
              <div class="template-card-wrapper template-card-add-wrapper">
                <div class="template-card-top template-card-add-top">
                  <icon name="plus" class="template-card-add-icon" />
                </div>
                <div class="template-card-bottom">
                  <span class="template-card-type-badge template-card-add-badge">
                    {{ t('templates.New') }}
                  </span>
                </div>
              </div>
            </router-link>
          </div>
        </div>
        
        <!-- Mensaje cuando no hay templates (solo para repositorios no locales) -->
        <div 
          v-else-if="!repo.error && (!repo.templates || repo.templates.length === 0) && (!repo.isLocal || !$api.auth.hasScope('templates.local.edit'))"
          :class="[
            'p-8 text-center rounded-xl',
            'bg-muted/20 border-2 border-border/30',
            'text-muted-foreground'
          ]"
        >
          <p>{{ t('templates.NoTemplates') || 'No hay templates disponibles en este repositorio' }}</p>
        </div>
      </div>
      <div v-if="templatesLoaded">
        <a 
          v-if="$api.auth.hasScope('templates.repo.create')" 
          :class="[
            'repo',
            'createLink',
            'inline-flex items-center gap-2 px-4 py-3',
            'bg-primary/10 border-2 border-primary/30 rounded-xl',
            'text-primary font-semibold',
            'hover:bg-primary/20 hover:border-primary/50',
            'transition-all duration-200',
            'shadow-sm hover:shadow-md',
            'cursor-pointer'
          ]"
          @click="addingRepo = true"
        >
          <icon name="plus" />
          <span>{{t('templates.AddRepo')}}</span>
        </a>
      </div>
      <div 
        v-else 
        :class="['list-item']"
      >
        <loader small />
      </div>
    </div>
    <overlay 
      v-model="addingRepo" 
      :title="t('templates.AddRepo')" 
      closable 
      class="server-name" 
      @close="resetAddRepo()"
    >
      <div 
        :class="[
          'actions',
          'space-y-4'
        ]"
      >
        <text-field v-model="currentRepo.name" :label="t('templates.RepoName')" />
        <text-field v-model="currentRepo.url" :label="t('templates.RepoUrl')" />
        <text-field v-model="currentRepo.branch" :label="t('templates.RepoBranch')" />
        <div 
          :class="[
            'flex gap-4 justify-end mt-6'
          ]"
        >
          <btn 
            v-hotkey="'Escape'" 
            color="error" 
            @click="resetAddRepo()"
          >
            <icon name="close" />
            {{ t('common.Cancel') }}
          </btn>
          <btn 
            :disabled="!canAddRepo()" 
            color="primary" 
            @click="addRepo()"
          >
            <icon name="save" />
            {{ t('templates.AddRepo') }}
          </btn>
        </div>
      </div>
    </overlay>
  </div>
</template>

<style scoped>
/* Grid de templates en formato tabla 4x4 - NOMBRES NUEVOS para evitar conflictos */
.template-grid-wrapper {
  display: block !important;
  width: 100% !important;
}

.template-grid-4cols {
  display: grid !important;
  grid-template-columns: repeat(4, 1fr) !important;
  gap: 1rem !important;
  width: 100% !important;
  padding: 0 !important;
  margin: 0 !important;
  list-style: none !important;
}

/* Responsive */
@media (max-width: 1279px) {
  .template-grid-4cols {
    grid-template-columns: repeat(3, 1fr) !important;
  }
}

@media (max-width: 1023px) {
  .template-grid-4cols {
    grid-template-columns: repeat(2, 1fr) !important;
  }
}

@media (max-width: 639px) {
  .template-grid-4cols {
    grid-template-columns: 1fr !important;
  }
}

/* Tarjetas de templates */
.template-card-item {
  display: block !important;
  width: 100% !important;
  text-decoration: none !important;
  padding: 0 !important;
  margin: 0 !important;
  border: none !important;
  background: none !important;
  box-shadow: none !important;
  list-style: none !important;
  position: relative !important;
}

.template-card-item::before,
.template-card-item::after,
.template-card-item::marker {
  display: none !important;
  content: '' !important;
}

.template-card-wrapper {
  display: flex !important;
  flex-direction: column !important;
  width: 100% !important;
  min-height: 140px !important;
  height: 100% !important;
  background: rgb(var(--color-background)) !important;
  border: 2px solid rgb(var(--color-border) / 0.5) !important;
  border-radius: 0.75rem !important;
  padding: 1rem !important;
  box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px -1px rgba(0, 0, 0, 0.1) !important;
  transition: all 0.2s ease-in-out !important;
  cursor: pointer !important;
}

.template-card-item:hover .template-card-wrapper {
  border-color: rgb(var(--color-primary) / 0.5) !important;
  background: rgb(var(--color-primary) / 0.05) !important;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -2px rgba(0, 0, 0, 0.1) !important;
  transform: translateY(-2px) !important;
}

.template-card-top {
  display: flex !important;
  align-items: flex-start !important;
  justify-content: flex-end !important;
  gap: 0.5rem !important;
  margin-bottom: 0.75rem !important;
  flex-grow: 1 !important;
  text-align: right !important;
}

.template-card-title-text {
  display: block !important;
  font-size: 1.125rem !important;
  font-weight: 600 !important;
  color: rgb(var(--color-foreground)) !important;
  line-height: 1.5rem !important;
  margin: 0 !important;
  padding: 1px !important;
  overflow: hidden !important;
  text-overflow: ellipsis !important;
  display: -webkit-box !important;
  -webkit-line-clamp: 2 !important;
  -webkit-box-orient: vertical !important;
  transition: color 0.2s ease-in-out !important;
  text-align: right !important;
  margin-left: auto !important;
}

.template-card-item:hover .template-card-title-text {
  color: rgb(var(--color-primary)) !important;
}

.template-card-arrow {
  flex-shrink: 0 !important;
  margin-top: 0.25rem !important;
  color: rgb(var(--color-muted-foreground)) !important;
  transition: color 0.2s ease-in-out, transform 0.2s ease-in-out !important;
  width: 1.25rem !important;
  height: 1.25rem !important;
}

.template-card-item:hover .template-card-arrow {
  color: rgb(var(--color-primary)) !important;
  transform: translateX(4px) !important;
}

.template-card-bottom {
  display: flex !important;
  align-items: center !important;
  justify-content: flex-end !important;
  margin-top: auto !important;
  padding-top: 0.75rem !important;
  border-top: 1px solid rgb(var(--color-border) / 0.3) !important;
}

.template-card-type-badge {
  display: inline-flex !important;
  align-items: center !important;
  justify-content: center !important;
  position: relative !important;
  padding: 0.5rem 1rem 0.5rem 1.25rem !important;
  font-size: 0.75rem !important;
  font-weight: 700 !important;
  border-radius: 0.25rem 0.5rem 0.5rem 0.25rem !important; /* Tag shape - esquina izquierda más redondeada */
  background: rgb(var(--color-primary) / 0.15) !important;
  color: rgb(var(--color-primary)) !important;
  border: 1.5px solid rgb(var(--color-primary) / 0.3) !important;
  border-left-width: 3px !important; /* Borde izquierdo más grueso */
  text-transform: uppercase !important;
  letter-spacing: 0.05em !important;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1) !important;
  transition: all 0.2s ease-in-out !important;
  white-space: nowrap !important;
  min-width: fit-content !important;
}

/* Efecto de etiqueta con corte en la esquina izquierda */
.template-card-type-badge::before {
  content: '' !important;
  position: absolute !important;
  left: 0 !important;
  top: 50% !important;
  transform: translateY(-50%) !important;
  width: 0 !important;
  height: 0 !important;
  border-style: solid !important;
  border-width: 0.4rem 0.4rem 0.4rem 0 !important;
  border-color: transparent rgb(var(--color-primary) / 0.3) transparent transparent !important;
}

.template-card-type-badge::after {
  content: '' !important;
  position: absolute !important;
  left: 3px !important;
  top: 50% !important;
  transform: translateY(-50%) !important;
  width: 0 !important;
  height: 0 !important;
  border-style: solid !important;
  border-width: 0.35rem 0.35rem 0.35rem 0 !important;
  border-color: transparent rgb(var(--color-primary) / 0.15) transparent transparent !important;
}

.template-card-item:hover .template-card-type-badge {
  background: rgb(var(--color-primary) / 0.25) !important;
  border-color: rgb(var(--color-primary) / 0.5) !important;
  border-left-color: rgb(var(--color-primary) / 0.7) !important;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.15) !important;
  transform: translateY(-1px) !important;
}

.template-card-item:hover .template-card-type-badge::before {
  border-color: transparent rgb(var(--color-primary) / 0.5) transparent transparent !important;
}

.template-card-item:hover .template-card-type-badge::after {
  border-color: transparent rgb(var(--color-primary) / 0.25) transparent transparent !important;
}

/* Botón agregar nuevo */
.template-card-add-wrapper {
  border-style: dashed !important;
  background: rgb(var(--color-primary) / 0.05) !important;
  border-color: rgb(var(--color-primary) / 0.3) !important;
  align-items: center !important;
  justify-content: center !important;
}

.template-card-item:hover .template-card-add-wrapper {
  background: rgb(var(--color-primary) / 0.1) !important;
  border-color: rgb(var(--color-primary) / 0.5) !important;
}

.template-card-add-top {
  justify-content: center !important;
  margin-bottom: 0 !important;
}

.template-card-add-icon {
  width: 2rem !important;
  height: 2rem !important;
  color: rgb(var(--color-primary)) !important;
  transition: transform 0.2s ease-in-out !important;
}

.template-card-item:hover .template-card-add-icon {
  transform: scale(1.15) !important;
}

.template-card-add-badge {
  background: rgb(var(--color-primary) / 0.15) !important;
  color: rgb(var(--color-primary)) !important;
  width: 100% !important;
  text-align: center !important;
  text-transform: none !important;
  font-size: 0.875rem !important;
}

/* Asegurar que NO haya estilos de lista */
.template-grid-wrapper *,
.template-grid-4cols *,
.template-card-item * {
  list-style: none !important;
  padding-left: 0 !important;
}

.template-grid-wrapper ul,
.template-grid-4cols ul {
  list-style: none !important;
  padding: 0 !important;
  margin: 0 !important;
}

.template-grid-wrapper li,
.template-grid-4cols li {
  list-style: none !important;
  padding: 0 !important;
  margin: 0 !important;
}
</style>
