<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps({
  server: { type: Object, required: true }
})

const { t } = useI18n()

const data = ref({})

let task = null
onMounted(async () => {
  if (await props.server.canQuery()) {
    task = setInterval(async () => {
      data.value = await props.server.getQuery()
    }, 30000)
    data.value = await props.server.getQuery()
  }
})

onUnmounted(() => {
  if (task) clearInterval(task)
})
</script>

<template>
  <div class="query-container">
    <div v-if="data.minecraft" class="query-card">
      <div class="query-header">
        <h3 class="query-title">{{ t('servers.NumPlayersOnline', {current: data.minecraft.numPlayers, max: data.minecraft.maxPlayers}) }}</h3>
      </div>
      <div class="query-body">
      <progress
          class="query-progress"
        :value="data.minecraft.numPlayers"
        :max="data.minecraft.maxPlayers"
      />
        <div v-if="(data.minecraft.players || []).length > 0" class="query-players">
          <div v-for="player in data.minecraft.players || []" :key="player" class="query-player-badge" v-text="player" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.query-container {
  padding: 0;
}

.query-card {
  background: #1e293b;
  border: 2px solid #475569;
  border-radius: 1rem;
  padding: 2rem;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.query-card:hover {
  border-color: #3b82f6;
}

.query-header {
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 2px solid #334155;
}

.query-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #f1f5f9;
  margin: 0;
}

.query-body {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.query-progress {
  width: 100%;
  height: 1rem;
  border-radius: 0.5rem;
  overflow: hidden;
  background: #0f172a;
  border: 2px solid #334155;
}

.query-progress::-webkit-progress-bar {
  background: #0f172a;
}

.query-progress::-webkit-progress-value {
  background: #3b82f6;
  transition: width 0.3s ease;
}

.query-progress::-moz-progress-bar {
  background: #3b82f6;
}

.query-players {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.query-player-badge {
  padding: 0.5rem 1rem;
  background: #3b82f6;
  color: #ffffff;
  border-radius: 0.5rem;
  font-size: 0.875rem;
  font-weight: 500;
  border: 2px solid #2563eb;
}
</style>