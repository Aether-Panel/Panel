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
        :class="['list', 'space-y-2']"
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
        <div 
          v-for="template in repo.templates" 
          :key="template.name" 
          :class="['list-item']"
        >
          <router-link 
            :ref="setFirstEntry" 
            :to="{ name: 'TemplateView', params: { repo: repo.id, id: template.name } }"
            :class="['block']"
          >
            <div 
              :class="[
                'template',
                'w-full'
              ]"
            >
              <span 
                :class="[
                  'title',
                  'block text-lg font-semibold text-foreground'
                ]"
              >
                {{template.display}}
              </span>
              <span 
                :class="[
                  'subline',
                  'block text-sm text-muted-foreground mt-1'
                ]"
              >
                {{template.type}}
              </span>
            </div>
          </router-link>
        </div>
        <div 
          v-if="repo.error" 
          :class="[
            'template-repo-error',
            'alert',
            'error',
            'p-4 rounded-xl',
            'bg-error/10 border-2 border-error/30',
            'text-error font-medium',
            'shadow-sm'
          ]"
        >
          {{(repo.error.code === 'ErrGeneric' && repo.error.msg) ? t(repo.error.msg) : t('errors.' + repo.error.code)}}
        </div>
        <div 
          v-if="repo.isLocal && $api.auth.hasScope('templates.local.edit')" 
          :class="['list-item']"
        >
          <router-link 
            v-hotkey="'c'" 
            :to="{ name: 'TemplateCreate' }"
            :class="['block']"
          >
            <div 
              :class="[
                'createLink',
                'flex items-center gap-2 px-4 py-3',
                'bg-primary/10 border-2 border-primary/30 rounded-xl',
                'text-primary font-semibold',
                'hover:bg-primary/20 hover:border-primary/50',
                'transition-all duration-200',
                'shadow-sm hover:shadow-md',
                'cursor-pointer'
              ]"
            >
              <icon name="plus" />
              <span>{{ t('templates.New') }}</span>
            </div>
          </router-link>
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
