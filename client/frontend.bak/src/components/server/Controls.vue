<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/ui/Icon.vue'

const { t } = useI18n()

const props = defineProps({
  server: { type: Object, required: true }
})

const dropdownOpen = ref(false)

const hotkeys = {
  'r r': () => props.server.restart(),
  'r s': () => props.server.stop(),
  'r k': () => props.server.kill()
}

function onHotkey(keys) {
  if (hotkeys[keys]) hotkeys[keys]()
}

function toggleDropdown() {
  dropdownOpen.value = !dropdownOpen.value
}

function closeDropdown() {
  dropdownOpen.value = false
}

const actions = [
  {
    id: 'start',
    label: t('servers.Start'),
    icon: 'play',
    scope: 'server.start',
    color: 'success',
    action: () => props.server.start()
  },
  {
    id: 'restart',
    label: t('servers.Restart'),
    icon: 'restart',
    scope: 'server.start',
    scope2: 'server.stop',
    color: 'primary',
    action: () => props.server.restart()
  },
  {
    id: 'stop',
    label: t('servers.Stop'),
    icon: 'stop',
    scope: 'server.stop',
    color: 'warning',
    action: () => props.server.stop()
  },
  {
    id: 'kill',
    label: t('servers.Kill'),
    icon: 'kill',
    scope: 'server.kill',
    color: 'error',
    action: () => props.server.kill()
  }
]

const visibleActions = actions.filter(action => {
  if (action.scope2) {
    return props.server.hasScope(action.scope) && props.server.hasScope(action.scope2)
  }
  return props.server.hasScope(action.scope)
})
</script>

<template>
  <div 
    v-hotkey="Object.keys(hotkeys)" 
    class="server-actions-panel"
    @hotkey="onHotkey"
  >
    <!-- Desktop: Botones verticales separados -->
    <div class="server-actions-desktop">
      <button
        v-for="action in visibleActions"
        :key="action.id"
        :class="['server-action-btn', `server-action-${action.id}`, `server-action-${action.color}`]"
        :title="action.label"
        @click="action.action()"
      >
        <icon :name="action.icon" class="server-action-icon" />
        <span class="server-action-text">{{ action.label }}</span>
      </button>
    </div>

    <!-- Mobile: Menú desplegable -->
    <div class="server-actions-mobile">
      <button
        class="server-actions-dropdown-trigger"
        @click="toggleDropdown()"
      >
        <span class="server-actions-dropdown-title">{{ t('servers.Actions') || 'Acciones' }}</span>
        <icon
          :name="dropdownOpen ? 'chevron-up' : 'chevron-down'"
          class="server-actions-dropdown-icon"
        />
      </button>
      <div
        v-if="dropdownOpen"
        class="server-actions-dropdown"
        @click.stop
      >
        <button
          v-for="action in visibleActions"
          :key="action.id"
          :class="['server-action-dropdown-btn', `server-action-${action.id}`, `server-action-${action.color}`]"
          @click="action.action(); closeDropdown()"
        >
          <icon :name="action.icon" class="server-action-icon" />
          <span class="server-action-text">{{ action.label }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.server-actions-panel {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 0.5rem;
  width: 100%;
  max-width: 200px;
}

/* Desktop: Botones verticales separados */
.server-actions-desktop {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  width: 100%;
  align-items: stretch;
}

.server-action-btn {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 0.375rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid rgb(var(--color-border) / 0.3);
  background: transparent;
  color: rgb(var(--color-foreground));
  font-size: 0.75rem;
  font-weight: 500;
  border-radius: 0.375rem;
  cursor: pointer;
  transition: all 0.2s ease-in-out;
  white-space: nowrap;
  width: 100%;
}

.server-action-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.server-action-btn:active {
  transform: translateY(0);
}

.server-action-icon {
  width: 0.875rem;
  height: 0.875rem;
  flex-shrink: 0;
}

.server-action-text {
  font-size: 0.75rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

/* Colores específicos para cada acción */
.server-action-success {
  color: rgb(var(--color-success));
  border-color: rgb(var(--color-success) / 0.3);
  background: rgb(var(--color-success) / 0.05);
}

.server-action-success:hover {
  background: rgb(var(--color-success) / 0.15);
  border-color: rgb(var(--color-success) / 0.5);
}

.server-action-primary {
  color: rgb(var(--color-primary));
  border-color: rgb(var(--color-primary) / 0.3);
  background: rgb(var(--color-primary) / 0.05);
}

.server-action-primary:hover {
  background: rgb(var(--color-primary) / 0.15);
  border-color: rgb(var(--color-primary) / 0.5);
}

.server-action-warning {
  color: rgb(var(--color-warning));
  border-color: rgb(var(--color-warning) / 0.3);
  background: rgb(var(--color-warning) / 0.05);
}

.server-action-warning:hover {
  background: rgb(var(--color-warning) / 0.15);
  border-color: rgb(var(--color-warning) / 0.5);
}

.server-action-error {
  color: rgb(var(--color-error));
  border-color: rgb(var(--color-error) / 0.3);
  background: rgb(var(--color-error) / 0.05);
}

.server-action-error:hover {
  background: rgb(var(--color-error) / 0.15);
  border-color: rgb(var(--color-error) / 0.5);
}

/* Mobile: Menú desplegable */
.server-actions-mobile {
  display: none;
}

.server-actions-dropdown-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 0.75rem 1rem;
  border: 1px solid rgb(var(--color-border) / 0.3);
  background: rgb(var(--color-background));
  color: rgb(var(--color-foreground));
  font-size: 0.875rem;
  font-weight: 500;
  border-radius: 0.5rem;
  cursor: pointer;
  transition: all 0.2s ease-in-out;
}

.server-actions-dropdown-trigger:hover {
  background: rgb(var(--color-muted) / 0.3);
  border-color: rgb(var(--color-border));
}

.server-actions-dropdown-title {
  flex: 1;
  text-align: left;
}

.server-actions-dropdown-icon {
  width: 1rem;
  height: 1rem;
  flex-shrink: 0;
  transition: transform 0.2s ease-in-out;
}

.server-actions-dropdown {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-top: 0.5rem;
  padding: 0.5rem;
  background: rgb(var(--color-background));
  border: 1px solid rgb(var(--color-border) / 0.3);
  border-radius: 0.5rem;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
}

.server-action-dropdown-btn {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  width: 100%;
  padding: 0.75rem 1rem;
  border: none;
  background: transparent;
  color: rgb(var(--color-foreground));
  font-size: 0.875rem;
  font-weight: 500;
  text-align: left;
  border-radius: 0.5rem;
  cursor: pointer;
  transition: all 0.2s ease-in-out;
}

.server-action-dropdown-btn:hover {
  background: rgb(var(--color-muted) / 0.5);
}

.server-action-dropdown-btn.server-action-success {
  color: rgb(var(--color-success));
}

.server-action-dropdown-btn.server-action-success:hover {
  background: rgb(var(--color-success) / 0.1);
}

.server-action-dropdown-btn.server-action-primary {
  color: rgb(var(--color-primary));
}

.server-action-dropdown-btn.server-action-primary:hover {
  background: rgb(var(--color-primary) / 0.1);
}

.server-action-dropdown-btn.server-action-warning {
  color: rgb(var(--color-warning));
}

.server-action-dropdown-btn.server-action-warning:hover {
  background: rgb(var(--color-warning) / 0.1);
}

.server-action-dropdown-btn.server-action-error {
  color: rgb(var(--color-error));
}

.server-action-dropdown-btn.server-action-error:hover {
  background: rgb(var(--color-error) / 0.1);
}

/* Responsive: Mostrar móvil en pantallas pequeñas */
@media (max-width: 768px) {
  .server-actions-desktop {
    display: none;
  }

  .server-actions-mobile {
    display: block;
  }
}

/* Ajustes para pantallas pequeñas */
@media (max-width: 640px) {
  .server-action-btn {
    padding: 0.5rem 0.75rem;
  }

  .server-action-icon {
    width: 1rem;
    height: 1rem;
  }
}
</style>
