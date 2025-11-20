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
  <div class="space-y-4 p-4">
    <div v-if="data.minecraft" class="rounded-xl border-2 border-border/50 bg-background p-4 space-y-4">
      <div class="text-lg font-semibold text-foreground">
        {{ t('servers.NumPlayersOnline', {current: data.minecraft.numPlayers, max: data.minecraft.maxPlayers}) }}
      </div>
      <progress
        class="w-full h-4 rounded-full overflow-hidden bg-muted"
        :value="data.minecraft.numPlayers"
        :max="data.minecraft.maxPlayers"
      />
      <div v-if="(data.minecraft.players || []).length > 0" class="flex flex-wrap gap-2">
        <div v-for="player in data.minecraft.players || []" :key="player" class="px-3 py-1 rounded-lg bg-primary/10 text-primary-foreground text-sm" v-text="player" />
      </div>
    </div>
  </div>
</template>