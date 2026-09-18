<script setup lang="ts">
defineProps<{ title: string; description: string; symbol?: string }>()
</script>

<template>
  <section class="empty-state">
    <div class="empty-symbol" aria-hidden="true">{{ symbol ?? '✦' }}</div>
    <h2>{{ title }}</h2>
    <p>{{ description }}</p>
    <slot />
  </section>
</template>
