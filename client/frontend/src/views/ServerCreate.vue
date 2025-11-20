<script setup>
import { ref, inject } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import SelectTemplate from '@/components/ui/serverCreateSteps/SelectTemplate.vue'
import Environment from '@/components/ui/serverCreateSteps/Environment.vue'
import Settings from '@/components/ui/serverCreateSteps/Settings.vue'

const router = useRouter()
const { t } = useI18n()
const api = inject('api')
const step = ref('environment')
const environment = ref({})
const users = ref([])
const template = ref({})

function envConfirmed(name, nodeId, nodeOs, nodeArch, env, u) {
  users.value = u
  environment.value = { name, nodeId, nodeOs, nodeArch, env }
  step.value = 'template'
}

function templateBack() {
  environment.value = {}
  users.value = []
  step.value = 'environment'
}

function templateSelected(selected) {
  if (!Array.isArray(selected.supportedEnvironments)) selected.supportedEnvironments = [selected.environment]
  template.value = selected
  step.value = 'settings'
}

function settingsBack() {
  template.value = {}
  step.value = 'template'
}

async function settingsConfirmed(settings, envSettings) {
  // last step confirmed, create server
  const request = template.value
  request.name = environment.value.name
  request.node = environment.value.nodeId
  request.environment = envSettings
  request.users = users.value
  request.data = {}
  for (const setting in settings) {
    request.data[setting] = settings[setting]

    // fix value types
    if (request.data[setting].type === 'boolean') {
      request.data[setting].value =
        request.data[setting].value !== 'false' &&
        request.data[setting].value !== false
    }

    if (request.data[setting].type === 'integer') {
      request.data[setting].value = Number(request.data[setting].value)
    }
  }

  const id = await api.server.create(request)
  router.push({ name: 'ServerView', params: { id }, query: { created: true } })
}
</script>

<template>
  <div 
    :class="[
      'servercreate',
      'space-y-6',
      'max-w-4xl mx-auto'
    ]"
  >
    <h1 
      :class="[
        'text-3xl font-bold text-foreground mb-6',
        'pb-3 border-b-2 border-border/50'
      ]"
      v-text="t('servers.Create')" 
    />
    
    <div 
      v-if="$api.auth.hasScope('nodes.view') && $api.auth.hasScope('templates.view')"
      :class="['space-y-6']"
    >
      <!-- Indicador de progreso -->
      <div 
        :class="[
          'progress',
          'relative flex items-center justify-between mb-8',
          'before:absolute before:top-1/2 before:left-0 before:right-0',
          'before:h-1 before:bg-border/50 before:rounded-full',
          'before:-translate-y-1/2',
          'before:z-0'
        ]"
      >
        <div 
          :class="[
            'step',
            'step-environment',
            'relative z-10 flex flex-col items-center gap-2',
            'flex-1'
          ]"
        >
          <div 
            :class="[
              'w-12 h-12 rounded-full',
              'flex items-center justify-center',
              'font-bold text-lg',
              'border-2 transition-all duration-200',
              step === 'environment' 
                ? 'bg-primary border-primary text-primary-foreground shadow-lg' 
                : step === 'template' || step === 'settings'
                  ? 'bg-success border-success text-success-foreground shadow-md'
                  : 'bg-muted border-border text-muted-foreground'
            ]"
          >
            <icon 
              v-if="step === 'template' || step === 'settings'" 
              name="check" 
            />
            <span v-else>1</span>
          </div>
          <span 
            :class="[
              'text-sm font-medium',
              step === 'environment' ? 'text-primary' : 'text-muted-foreground'
            ]"
            v-text="t('servers.Environment')"
          />
        </div>
        
        <div 
          :class="[
            'step',
            'step-template',
            'relative z-10 flex flex-col items-center gap-2',
            'flex-1'
          ]"
        >
          <div 
            :class="[
              'w-12 h-12 rounded-full',
              'flex items-center justify-center',
              'font-bold text-lg',
              'border-2 transition-all duration-200',
              step === 'template' 
                ? 'bg-primary border-primary text-primary-foreground shadow-lg' 
                : step === 'settings'
                  ? 'bg-success border-success text-success-foreground shadow-md'
                  : 'bg-muted border-border text-muted-foreground'
            ]"
          >
            <icon 
              v-if="step === 'settings'" 
              name="check" 
            />
            <span v-else>2</span>
          </div>
          <span 
            :class="[
              'text-sm font-medium',
              step === 'template' ? 'text-primary' : step === 'settings' ? 'text-success' : 'text-muted-foreground'
            ]"
            v-text="t('servers.SelectTemplate')"
          />
        </div>
        
        <div 
          :class="[
            'step',
            'step-settings',
            'relative z-10 flex flex-col items-center gap-2',
            'flex-1'
          ]"
        >
          <div 
            :class="[
              'w-12 h-12 rounded-full',
              'flex items-center justify-center',
              'font-bold text-lg',
              'border-2 transition-all duration-200',
              step === 'settings' 
                ? 'bg-primary border-primary text-primary-foreground shadow-lg' 
                : 'bg-muted border-border text-muted-foreground'
            ]"
          >
            3
          </div>
          <span 
            :class="[
              'text-sm font-medium',
              step === 'settings' ? 'text-primary' : 'text-muted-foreground'
            ]"
            v-text="t('servers.Settings')"
          />
        </div>
      </div>
      
      <!-- Contenido de los pasos -->
      <div 
        :class="[
          'step-content',
          'bg-background rounded-xl border-2 border-border/50 shadow-lg',
          'p-6 lg:p-8'
        ]"
      >
        <Environment 
          v-if="step === 'environment'" 
          :nouser="!$api.auth.hasScope('users.info.search')" 
          @confirm="envConfirmed" 
        />
        
        <select-template
          v-if="step === 'template'"
          :env="environment.env"
          :os="environment.nodeOs"
          :arch="environment.nodeArch"
          @back="templateBack()"
          @selected="templateSelected"
        />
        
        <settings
          v-if="step === 'settings'"
          :data="template.data"
          :groups="template.groups"
          :env="template.supportedEnvironments.filter(e => e.type === environment.env)[0]"
          @back="settingsBack()"
          @confirm="settingsConfirmed"
        />
      </div>
    </div>
    
    <div 
      v-else 
      :class="[
        'p-6 rounded-xl',
        'bg-error/10 border-2 border-error/30',
        'text-error font-medium',
        'shadow-sm text-center'
      ]"
      v-text="t('servers.CreateMissingPermissions')" 
    />
  </div>
</template>
