<script setup lang="ts">
import type { Game } from '@/types/design'
defineProps<{ game: Game; small?: boolean }>()
</script>
<template>
  <div class="d-cover" :class="[game.theme, { small }]" aria-hidden="true">
    <span class="d-cover-orbit"></span><span class="d-cover-motif">{{ game.motif }}</span>
    <strong>{{ game.english }}</strong><span class="d-cover-line"></span>
  </div>
</template>
