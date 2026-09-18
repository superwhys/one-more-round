<script setup lang="ts">
import { RouterView } from 'vue-router'
</script>

<template>
  <RouterView v-slot="{ Component }">
    <component :is="Component" v-if="Component" />
    <p v-else role="status">正在打开对局日记…</p>
  </RouterView>
</template>
