<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Overlay from '@/components/ui/Overlay.vue'
import TextField from '@/components/ui/TextField.vue'
import Status from './Status.vue'

const props = defineProps({
  server: { type: Object, required: true }
})

const { t } = useI18n()
const edit = ref(false)
const name = ref(props.server.name)

async function updateName() {
  await props.server.updateName(name.value)
  edit.value = false
}
</script>

<template>
  <div class="server-header-wrapper">
    <div class="server-header">
      <div class="server-header-left">
        <Status :server="server" />
        <h1 class="server-header-title">
          {{ server.name }}
        </h1>
        <btn 
          v-if="server.hasScope('server.name.edit')" 
          variant="icon" 
          size="sm"
          :tooltip="t('servers.EditName')" 
          class="server-edit-btn"
          @click="edit = !edit"
        >
          <icon name="edit" />
        </btn>
      </div>
      <div class="server-header-right">
        <slot name="actions" />
      </div>
    </div>
  </div>
  <overlay v-model="edit" :title="t('servers.EditName')" closable class="server-name">
    <text-field v-model="name" />
    <btn color="primary" @click="updateName()"><icon name="save" />{{ t('common.Save') }}</btn>
  </overlay>
</template>

<style scoped>
.server-header-wrapper {
  margin-bottom: 1rem;
}

.server-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.75rem 1rem;
  background: rgb(var(--color-background));
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.5rem;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
}

.server-header-left {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex: 1;
  min-width: 0;
}

.server-header-right {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.server-header-title {
  flex: 1;
  font-size: 1.25rem;
  font-weight: 600;
  color: rgb(var(--color-foreground));
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.server-edit-btn {
  color: rgb(var(--color-muted-foreground));
  padding: 0.25rem;
}

.server-edit-btn:hover {
  color: rgb(var(--color-primary));
  background: rgb(var(--color-primary) / 0.1);
}
</style>
