<script setup>
</script>

<template>
  <div 
    :class="[
      'list-item-content',
      'flex-grow',
      'min-w-0',
      '[&_.text]:overflow-hidden [&_.text]:whitespace-nowrap [&_.text]:text-ellipsis [&_.text]:w-full [&_.text]:block [&_.text]:font-medium',
      '[&_.title]:overflow-hidden [&_.title]:whitespace-nowrap [&_.title]:text-ellipsis [&_.title]:w-full [&_.title]:block [&_.title]:text-lg [&_.title]:font-semibold [&_.title]:text-foreground',
      '[&_.subline]:overflow-hidden [&_.subline]:whitespace-nowrap [&_.subline]:text-ellipsis [&_.subline]:w-full [&_.subline]:block [&_.subline]:text-sm [&_.subline]:text-muted-foreground [&_.subline]:mt-1',
      '[&_.type]:inline-block [&_.type]:px-2 [&_.type]:py-1 [&_.type]:text-xs [&_.type]:font-semibold [&_.type]:rounded-md [&_.type]:bg-primary/10 [&_.type]:text-primary [&_.type]:ml-2'
    ]"
  >
    <slot />
  </div>
</template>
