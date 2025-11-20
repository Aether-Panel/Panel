<script setup>
import { onMounted } from 'vue'

onMounted(() => {
  // Redirigir directamente a /uptime/ para que el proxy del backend maneje la redirección a Gatus
  window.location.href = '/uptime/'
})
</script>

<template>
  <div 
    :class="[
      'flex items-center justify-center min-h-screen',
      'p-8 text-center'
    ]"
  >
    <p 
      :class="[
        'text-lg text-muted-foreground'
      ]"
    >
      Redirigiendo a skyUpTime...
    </p>
  </div>
</template>

