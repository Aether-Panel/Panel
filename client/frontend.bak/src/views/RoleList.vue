<script setup>
import { ref, inject, onMounted } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const { t } = useI18n()
const route = useRoute()

const roles = ref([])
const rolesLoaded = ref(false)
const firstEntry = ref(null)

onMounted(async () => {
  try {
    roles.value = await api.role.list()
    rolesLoaded.value = true
  } catch (error) {
    console.error('Error loading roles:', error)
    rolesLoaded.value = true
  }
})

function setFirstEntry(ref) {
  if (!firstEntry.value) firstEntry.value = ref
}

function focusList() {
  if (firstEntry.value) {
    firstEntry.value.$el.focus()
  }
}
</script>

<template>
  <div 
    :class="[
      'rolelist',
      'w-full max-w-5xl ml-auto mr-0',
      'space-y-6'
    ]"
    style="padding-left: 2rem;"
  >
    <div 
      :class="[
        'flex items-center justify-between',
        'pb-3 border-b-2 border-border/50'
      ]"
    >
      <h1 
        :class="[
          'text-3xl font-bold text-foreground'
        ]"
        v-text="t('roles.Roles')" 
      />
      <router-link
        v-if="api.auth.hasScope('admin')"
        v-hotkey="'c'"
        :to="{ name: route.path.startsWith('/admin') ? 'Admin.RoleCreate' : 'RoleCreate' }"
        :class="[
          'btn btn-primary',
          'flex items-center gap-2',
          'px-4 py-2 rounded-lg',
          'bg-primary text-primary-foreground',
          'hover:bg-primary/90 transition-colors'
        ]"
      >
        <icon name="plus" />
        <span v-text="t('roles.Create')" />
      </router-link>
    </div>
    
    <div 
      v-hotkey="'l'" 
      :class="['list']"
      @hotkey="focusList()"
    >
      <div 
        v-if="!rolesLoaded"
        :class="['list-item']"
      >
        <loader />
      </div>
      
      <div 
        v-else-if="roles.length === 0"
        :class="[
          'list-item',
          'text-center py-12',
          'text-muted-foreground'
        ]"
      >
        <icon name="hi-shield" :class="['text-4xl mb-4 opacity-50']" />
        <p v-text="t('roles.NoRoles')" />
      </div>
      
      <div 
        v-for="role in roles" 
        :key="role.id" 
        :class="['list-item']"
      >
        <router-link 
          :ref="setFirstEntry" 
          :to="{ name: route.path.startsWith('/admin') ? 'Admin.RoleView' : 'RoleView', params: { id: role.id } }"
          :class="['block']"
        >
          <div 
            :class="[
              'role',
              'w-full flex items-center gap-4',
              'p-4 rounded-lg',
              'border-2 border-border/50',
              'bg-muted/30',
              'hover:bg-muted/50 hover:border-primary/30',
              'transition-all duration-200'
            ]"
          >
            <div 
              :class="[
                'flex items-center justify-center',
                'w-12 h-12 rounded-full',
                'bg-primary/20 text-primary',
                'flex-shrink-0'
              ]"
            >
              <icon name="hi-shield" :class="['text-xl']" />
            </div>
            <div :class="['flex-grow min-w-0']">
              <span 
                :class="[
                  'title',
                  'block text-lg font-semibold text-foreground'
                ]"
              >
                {{ role.name }}
              </span>
              <span 
                :class="[
                  'subline',
                  'block text-sm text-muted-foreground mt-1'
                ]"
              >
                {{ role.description || t('roles.NoDescription') }}
              </span>
              <div 
                v-if="role.scopes && role.scopes.length > 0"
                :class="[
                  'flex flex-wrap gap-1 mt-2'
                ]"
              >
                <span 
                  v-for="(scope, index) in role.scopes.slice(0, 5)"
                  :key="index"
                  :class="[
                    'px-2 py-0.5 text-xs rounded',
                    'bg-primary/10 text-primary',
                    'border border-primary/20'
                  ]"
                >
                  {{ scope }}
                </span>
                <span 
                  v-if="role.scopes.length > 5"
                  :class="[
                    'px-2 py-0.5 text-xs rounded',
                    'bg-muted text-muted-foreground'
                  ]"
                >
                  +{{ role.scopes.length - 5 }} {{ t('roles.More') }}
                </span>
              </div>
            </div>
            <icon 
              name="chevron-right" 
              :class="[
                'text-muted-foreground',
                'flex-shrink-0'
              ]"
            />
          </div>
        </router-link>
      </div>
    </div>
  </div>
</template>

