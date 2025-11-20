<script setup>
</script>

<template>
  <ul 
    :class="[
      'list-none p-0 m-0',
      'flex flex-col gap-2'
    ]"
  >
    <slot />
  </ul>
</template>
