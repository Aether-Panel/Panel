<script setup>
import { ref, inject, onMounted, onUnmounted, nextTick } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import md5 from 'js-md5'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const { t } = useI18n()

const users = ref([])
let lastPage = 0
let loadingPage = false
const allUsersLoaded = ref(false)
const loaderRef = ref(null)
const firstEntry = ref(null)

function addUsers(newUsers) {
  newUsers.map(user => users.value.push(user))
}

function isLoaderVisible() {
  if (!loaderRef.value) return false
  const vw = window.innerWidth || document.documentElement.clientWidth
  const vh = window.innerHeight || document.documentElement.clientHeight
  const rect = loaderRef.value.$el.getBoundingClientRect()
  return rect.top >= 0 && rect.left >= 0 && rect.bottom <= vh && rect.right <= vw
}

async function loadPage(page = 1) {
  loadingPage = true
  const data = await api.user.list(page)
  addUsers(data.users)
  lastPage = data.paging.page
  allUsersLoaded.value = data.paging.page * data.paging.pageSize >= (data.paging.total || 0)
  nextTick(() => {
    loadingPage = false
    if (!allUsersLoaded.value && isLoaderVisible()) loadPage(lastPage + 1)
  })
}

function onScroll() {
  if (!loadingPage && isLoaderVisible()) loadPage(lastPage + 1)
}

onMounted(() => {
  nextTick(() => {
    loadPage()
    window.addEventListener('scroll', onScroll)
  })
})

onUnmounted(() => {
  window.removeEventListener('scroll', onScroll)
})

function setFirstEntry(ref) {
  if (!firstEntry.value) firstEntry.value = ref
}

function focusList() {
  firstEntry.value.$el.focus()
}
</script>

<template>
  <div 
    :class="[
      'userlist',
      'w-full max-w-7xl mx-auto',
      'space-y-6'
    ]"
  >
    <h1 
      :class="[
        'text-3xl font-bold text-foreground mb-6',
        'pb-3 border-b-2 border-border/50'
      ]"
      v-text="t('users.Users')" 
    />
    <div 
      v-hotkey="'l'" 
      :class="['list']"
      @hotkey="focusList()"
    >
      <div 
        v-for="user in users" 
        :key="user.id" 
        :class="['list-item']"
      >
        <router-link 
          :ref="setFirstEntry" 
          :to="{ name: 'UserView', params: { id: user.id } }"
          :class="['block']"
        >
          <div 
            :class="[
              'user',
              'w-full flex items-center gap-4'
            ]"
          >
            <img 
              :src="'https://www.gravatar.com/avatar/' + md5(user.email) + '?d=mp'" 
              :class="[
                'avatar',
                'w-12 h-12 rounded-full border-2 border-border',
                'flex-shrink-0 shadow-sm'
              ]"
              :alt="user.username"
            />
            <div :class="['flex-grow min-w-0']">
              <span 
                :class="[
                  'title',
                  'block text-lg font-semibold text-foreground'
                ]"
              >
                {{user.username}}{{ $api.auth.hasScope('users.perms.view') && user.otpActive ? ' (' + t('users.OtpAbreviated') + ')' : '' }}
              </span>
              <span 
                :class="[
                  'subline',
                  'block text-sm text-muted-foreground mt-1'
                ]"
              >
                {{user.email}}
              </span>
            </div>
          </div>
        </router-link>
      </div>
      <div 
        v-if="!allUsersLoaded" 
        ref="loaderRef" 
        :class="['list-item']"
      >
        <loader small />
      </div>
      <div 
        v-if="$api.auth.hasScope('users.info.edit')" 
        :class="['list-item']"
      >
        <router-link 
          v-hotkey="'c'" 
          :to="{ name: 'UserCreate' }"
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
            <span>{{ t('users.Add') }}</span>
          </div>
        </router-link>
      </div>
    </div>
  </div>
</template>
