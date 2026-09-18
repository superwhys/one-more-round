<script setup lang="ts">
import { players, playerName } from '@/stores/designPreview'
defineProps<{ id: string; small?: boolean }>()
</script>
<template><span class="d-avatar" :class="{ small }" :style="{ background: players.find((p) => p.id === id)?.color }" :title="playerName(id)">{{ playerName(id).slice(-1) }}</span></template>
