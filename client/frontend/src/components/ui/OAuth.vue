<script setup>
import { ref, inject, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Overlay from '@/components/ui/Overlay.vue'
import TextField from '@/components/ui/TextField.vue'

const { t } = useI18n()
const api = inject('api')
const events = inject('events')

defineProps({
  allowCreate: { type: Boolean, default: () => true },
  allowDelete: { type: Boolean, default: () => true }
})

const docsUrl = location.origin + '/swagger/index.html'

const clients = ref([])
const creating = ref(false)
const newName = ref('')
const newDescription = ref('')
const created = ref(false)
const createdData = ref(null)

onMounted(() => {
  refresh()
})

async function refresh() {
  clients.value = await api.self.getOAuthClients()
}

function startCreate() {
  newName.value = ''
  newDescription.value = ''
  creating.value = true
}

async function create() {
  createdData.value = await api.self.createOAuthClient(newName.value, newDescription.value)

  created.value = true
  creating.value = false
  refresh()
}

async function deleteClient(clientId, clientName) {
  events.emit(
    'confirm',
    t('oauth.ConfirmDelete', { name: clientName }),
    {
      text: t('oauth.Delete'),
      icon: 'remove',
      color: 'error',
      action: async () => {
        clients.value = await api.self.deleteOAuthClient(clientId)
        refresh()
      }
    },
    {
      color: 'primary'
    }
  )
}
</script>

<template>
  <div 
    :class="[
      'oauth',
      'space-y-4'
    ]"
  >
    <!-- Información -->
    <div 
      :class="[
        'info',
        'p-5 rounded-xl bg-muted/50 border border-border/50',
        'shadow-sm space-y-2'
      ]"
    >
      <div 
        :class="[
          'text-foreground'
        ]"
        v-text="t('oauth.AccountDescription')" 
      />
      <div>
        <a 
          target="_blank" 
          :href="docsUrl" 
          :class="[
            'text-primary hover:underline',
            'transition-colors duration-100'
          ]"
          v-text="t('oauth.Docs')" 
        />
      </div>
    </div>
    
    <!-- Lista de clientes OAuth -->
    <div 
      v-for="(client, index) in clients" 
      :key="client.client_id"
      :class="['space-y-4']"
    >
      <div 
        :class="[
          'oauth-client',
          'flex items-center justify-between',
          'p-5 rounded-xl bg-background border-2 border-border/50 shadow-md',
          'hover:shadow-lg hover:border-primary/30 transition-all duration-200'
        ]"
      >
        <div 
          :class="[
            'details',
            'flex-grow space-y-1'
          ]"
        >
          <div 
            :class="[
              'text-lg font-semibold text-foreground'
            ]"
            v-text="client.name || t('oauth.UnnamedClient')" 
          />
          <div 
            :class="[
              'text-sm font-mono text-muted-foreground'
            ]"
            v-text="client.client_id" 
          />
          <div 
            :title="client.description" 
            :class="[
              'text-sm text-muted-foreground',
              'truncate'
            ]"
            v-text="client.description" 
          />
        </div>
        <btn 
          v-if="allowDelete" 
          variant="icon" 
          color="error"
          @click="deleteClient(client.client_id, client.name || t('oauth.UnnamedClient'))"
        >
          <icon name="remove" />
        </btn>
      </div>
      <hr 
        v-if="index < clients.length - 1" 
        :class="[
          'my-4 border-t-2 border-border/50'
        ]"
      />
    </div>
    
    <!-- Botón crear -->
    <btn 
      v-if="allowCreate" 
      color="primary" 
      @click="startCreate()"
    >
      <icon name="plus" />{{ t('oauth.Create') }}
    </btn>
    
    <!-- Overlay para crear cliente -->
    <overlay v-model="creating" :title="t('oauth.Create')" closable>
      <div :class="['space-y-4']">
        <text-field v-model="newName" autofocus :label="t('common.Name')" />
        <text-field v-model="newDescription" :label="t('common.Description')" />
        <div 
          :class="[
            'flex gap-4 justify-end pt-6 mt-6 border-t-2 border-border/50'
          ]"
        >
          <btn color="error" @click="creating = false">
            <icon name="close" />
            {{ t('common.Cancel') }}
          </btn>
          <btn color="primary" @click="create()">
            <icon name="save" />
            {{ t('oauth.Create') }}
          </btn>
        </div>
      </div>
    </overlay>
    
    <!-- Overlay para mostrar credenciales -->
    <overlay v-model="created" :title="t('oauth.Credentials')" closable>
      <div :class="['space-y-4']">
        <!-- Advertencia -->
        <div 
          :class="[
            'warning',
            'p-4 rounded-xl',
            'bg-warning/10 border-2 border-warning/30',
            'text-warning-foreground font-medium',
            'shadow-sm'
          ]"
          v-text="t('oauth.NewClientWarning')" 
        />
        
        <!-- Client ID -->
        <div 
          :class="[
            'client-id',
            'p-5 rounded-xl bg-muted/50 border border-border/50',
            'space-y-2 shadow-sm'
          ]"
        >
          <span 
            :class="[
              'name',
              'block font-semibold text-foreground'
            ]"
            v-text="t('oauth.ClientId')+':'" 
          />
          <span 
            :class="[
              'value',
              'block font-mono text-sm text-muted-foreground break-all'
            ]"
            v-text="createdData?.client_id" 
          />
        </div>
        
        <!-- Client Secret -->
        <div 
          :class="[
            'client-secret',
            'p-5 rounded-xl bg-muted/50 border border-border/50',
            'space-y-2 shadow-sm'
          ]"
        >
          <span 
            :class="[
              'name',
              'block font-semibold text-foreground'
            ]"
            v-text="t('oauth.ClientSecret')+':'" 
          />
          <span 
            :class="[
              'value',
              'block font-mono text-sm text-muted-foreground break-all'
            ]"
            v-text="createdData?.client_secret" 
          />
        </div>
      </div>
    </overlay>
  </div>
</template>
