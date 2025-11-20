<script setup>
import { ref, inject, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Overlay from '@/components/ui/Overlay.vue'
import markdown from '@/utils/markdown.js'

const { t } = useI18n()
const api = inject('api')
const emit = defineEmits(['selected', 'back'])
const templatesByRepo = ref([])
const incompatibleTemplates = ref([])
const showing = ref(false)
const currentTemplate = ref({})

const props = defineProps({
  arch: { type: String, required: true },
  env: { type: String, required: true },
  os: { type: String, required: true }
})

function templateEnvMatches(template) {
  if (!Array.isArray(template.supportedEnvironments)) {
    if (!template.environment) return false // neither supported nor default env defined, ignore
    template.supportedEnvironments = [template.environment]
  }
  if (template.supportedEnvironments.filter(e => e.type === props.env).length > 0) return true
  return false
}

function templateOsMatches(template) {
  if (!template.requirements || !template.requirements.os) return true
  return template.requirements.os === props.os
}

function templateArchMatches(template) {
  if (!template.requirements || !template.requirements.arch) return true
  return template.requirements.arch === props.arch
}

async function load() {
  const repos = await api.template.listAllTemplates()
  const compatible = []
  const incompatible = []
  Object.keys(repos).sort((a, b) => repos[a].id > repos[b].id).map(repo => {
    if (repos[repo].templates.length === 0) return
    const templates = repos[repo].templates.filter(template => {
      return templateEnvMatches(template) &&
        templateOsMatches(template) &&
        templateArchMatches(template)
    })
    if (templates.length !== 0) compatible.push({ ...repos[repo], templates })
    if (templates.length !== repos[repo].templates.length) {
      incompatible.push({
        ...repos[repo],
        templates: undefined,
        arch: repos[repo].templates.filter(t => !templateArchMatches(t)),
        os: repos[repo].templates.filter(t => !templateOsMatches(t)),
        env: repos[repo].templates.filter(t => !templateEnvMatches(t))
      })
    }
  })
  templatesByRepo.value = compatible
  incompatibleTemplates.value = incompatible
}

onMounted(async () => {
  load()
})

async function show(repo, template) {
  currentTemplate.value = await api.template.get(repo, template)
  if (currentTemplate.value.readme) {
    showing.value = true
  } else {
    // no readme, skip readme popup
    emit('selected', currentTemplate.value)
  }
}

function choice(confirm) {
  showing.value = false
  if (confirm) emit('selected', currentTemplate.value)
}
</script>

<template>
  <div 
    :class="[
      'select-template',
      'space-y-6'
    ]"
  >
    <h2 
      :class="[
        'text-2xl font-bold text-foreground mb-4',
        'pb-2 border-b-2 border-border/50'
      ]"
      v-text="t('servers.SelectTemplate')" 
    />
    
    <div 
      :class="[
        'space-y-6'
      ]"
    >
      <div 
        v-for="repo in templatesByRepo" 
        :key="repo.id" 
        :class="['list', 'space-y-2']"
      >
        <h3 
          :class="[
            'list-header',
            'flex items-center justify-between px-4 py-3 mb-4',
            'bg-muted/30 border-2 border-border/50 rounded-xl',
            'shadow-sm',
            'text-xl font-bold text-foreground'
          ]"
          v-text="repo.name" 
        />
        <div 
          v-for="template in repo.templates" 
          :key="template.name" 
          :class="[
            'list-item',
            'template',
            'cursor-pointer'
          ]"
          @click="show(repo.id, template.name)"
        >
          <span 
            :class="[
              'title',
              'block text-lg font-semibold text-foreground'
            ]"
            v-text="template.display" 
          />
        </div>
      </div>
    </div>
    
    <div 
      v-if="incompatibleTemplates.length > 0" 
      :class="[
        'space-y-4 pt-6 border-t-2 border-border/50'
      ]"
    >
      <h2 
        :class="[
          'incompatible-title',
          'text-2xl font-bold text-foreground mb-2'
        ]"
        v-text="t('servers.IncompatibleTemplates')" 
      />
      <div 
        :class="[
          'incompatible-desc',
          'text-muted-foreground mb-4'
        ]"
        v-text="t('servers.IncompatibleTemplatesDescription')" 
      />
      
      <div 
        v-for="repo in incompatibleTemplates" 
        :key="repo.id" 
        :class="[
          'list',
          'incompatible-list',
          'space-y-2',
          'opacity-60'
        ]"
      >
        <h3 
          :class="[
            'list-header',
            'flex items-center justify-between px-4 py-3 mb-4',
            'bg-muted/20 border-2 border-border/30 rounded-xl',
            'text-xl font-bold text-foreground'
          ]"
          v-text="repo.name" 
        />
        
        <div v-if="repo.arch.length > 0" :class="['list', 'space-y-2', 'ml-4']">
          <h4 
            :class="[
              'list-header',
              'px-3 py-2 mb-2',
              'bg-muted/20 border border-border/30 rounded-lg',
              'text-lg font-semibold text-foreground'
            ]"
            v-text="t('servers.IncompatibleArch', {arch})" 
          />
          <div 
            v-for="template in repo.arch" 
            :key="template.name" 
            :class="[
              'list-item',
              'template',
              'disabled',
              'opacity-50 cursor-not-allowed'
            ]"
          >
            <span 
              :class="[
                'title',
                'block text-base font-medium text-muted-foreground'
              ]"
              v-text="template.display" 
            />
          </div>
        </div>
        
        <div v-if="repo.os.length > 0" :class="['list', 'space-y-2', 'ml-4']">
          <h4 
            :class="[
              'list-header',
              'px-3 py-2 mb-2',
              'bg-muted/20 border border-border/30 rounded-lg',
              'text-lg font-semibold text-foreground'
            ]"
            v-text="t('servers.IncompatibleOs', {os})" 
          />
          <div 
            v-for="template in repo.os" 
            :key="template.name" 
            :class="[
              'list-item',
              'template',
              'disabled',
              'opacity-50 cursor-not-allowed'
            ]"
          >
            <span 
              :class="[
                'title',
                'block text-base font-medium text-muted-foreground'
              ]"
              v-text="template.display" 
            />
          </div>
        </div>
        
        <div v-if="repo.env.length > 0" :class="['list', 'space-y-2', 'ml-4']">
          <h4 
            :class="[
              'list-header',
              'px-3 py-2 mb-2',
              'bg-muted/20 border border-border/30 rounded-lg',
              'text-lg font-semibold text-foreground'
            ]"
            v-text="t('servers.IncompatibleEnv', {env})" 
          />
          <div 
            v-for="template in repo.env" 
            :key="template.name" 
            :class="[
              'list-item',
              'template',
              'disabled',
              'opacity-50 cursor-not-allowed'
            ]"
          >
            <span 
              :class="[
                'title',
                'block text-base font-medium text-muted-foreground'
              ]"
              v-text="template.display" 
            />
          </div>
        </div>
      </div>
    </div>
    
    <div 
      :class="[
        'flex justify-between pt-4 border-t border-border/50'
      ]"
    >
      <btn 
        color="error" 
        @click="emit('back')"
      >
        <icon name="back" />
        {{ t('common.Back') }}
      </btn>
    </div>

    <overlay 
      v-model="showing" 
      :title="currentTemplate.display" 
      closable
    >
      <!-- eslint-disable-next-line vue/no-v-html -->
      <div 
        dir="ltr" 
        :class="[
          'readme',
          'prose prose-sm max-w-none',
          'text-foreground'
        ]"
        v-html="markdown(currentTemplate.readme)" 
      />
      <div 
        :class="[
          'actions',
          'flex justify-end gap-4 pt-6 mt-6 border-t border-border/50'
        ]"
      >
        <btn 
          color="error" 
          @click="choice(false)"
        >
          <icon name="close" />
          {{ t('common.Cancel') }}
        </btn>
        <btn 
          color="primary" 
          @click="choice(true)"
        >
          <icon name="check" />
          {{ t('servers.SelectThisTemplate') }}
        </btn>
      </div>
    </overlay>
  </div>
</template>
