<script setup>
import { ref, inject, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const { t } = useI18n()
const api = inject('api')
const hostsLoaded = ref(false)
const hosts = ref([])

onMounted(async () => {
  try {
    hosts.value = await api.databaseHost.list()
  } catch (error) {
    console.error('Error loading database hosts:', error)
    hosts.value = []
  }
  hostsLoaded.value = true
})

async function deleteHost(id) {
  if (!confirm(t('common.ConfirmDelete') || '¿Estás seguro de que deseas eliminar este host de base de datos?')) {
    return
  }
  
  try {
    await api.databaseHost.delete(id)
    hosts.value = hosts.value.filter(h => h.id !== id)
  } catch (error) {
    console.error('Error deleting database host:', error)
    alert(t('common.Error') || 'Error al eliminar el host de base de datos')
  }
}
</script>

<template>
  <div 
    :class="[
      'databasehostlist',
      'w-full max-w-5xl ml-auto mr-0',
      'space-y-6'
    ]"
    style="padding-left: 2rem;"
  >
    <h1 
      :class="[
        'text-3xl font-bold text-foreground mb-6',
        'pb-3 border-b-2 border-border/50'
      ]"
    >
      Database Hosts
    </h1>
    <div 
      :class="['list']"
    >
      <div 
        v-for="host in hosts" 
        :key="host.id" 
        :class="['list-item']"
      >
        <div 
          :class="[
            'host',
            'w-full',
            'flex items-center justify-between',
            'p-4 bg-card rounded-xl',
            'border-2 border-border/30',
            'hover:border-primary/50',
            'transition-all duration-200'
          ]"
        >
          <div :class="['flex-1']">
            <span 
              :class="[
                'title',
                'block text-lg font-semibold text-foreground'
              ]"
            >
              {{ host.name }}
            </span>
            <span 
              :class="[
                'subline',
                'block text-sm text-muted-foreground mt-1'
              ]"
            >
              {{ host.host }}:{{ host.port }} (Usuario: {{ host.username }})
            </span>
            <span 
              v-if="host.max_databases > 0"
              :class="[
                'subline',
                'block text-xs text-muted-foreground mt-1'
              ]"
            >
              Máx. bases de datos: {{ host.max_databases }}
            </span>
          </div>
          <div :class="['flex gap-2']">
            <router-link 
              :to="{ name: 'Admin.DatabaseHostEdit', params: { id: host.id } }"
              :class="[
                'px-3 py-1.5',
                'bg-primary/10 border border-primary/30 rounded-lg',
                'text-primary text-sm font-medium',
                'hover:bg-primary/20',
                'transition-all duration-200'
              ]"
            >
              Editar
            </router-link>
            <button
              :class="[
                'px-3 py-1.5',
                'bg-error/10 border border-error/30 rounded-lg',
                'text-error text-sm font-medium',
                'hover:bg-error/20',
                'transition-all duration-200'
              ]"
              @click="deleteHost(host.id)"
            >
              Eliminar
            </button>
          </div>
        </div>
      </div>
      <div 
        v-if="!hostsLoaded" 
        :class="['list-item']"
      >
        <loader small />
      </div>
      <div 
        :class="['list-item']"
      >
        <router-link 
          v-hotkey="'c'" 
          :to="{ name: 'Admin.DatabaseHostCreate' }"
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
            <span>Agregar Database Host</span>
          </div>
        </router-link>
      </div>
    </div>
  </div>
</template>
