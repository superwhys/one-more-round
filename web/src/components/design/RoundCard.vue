<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { gameById, modeNames, playerName, resultLabel } from '@/stores/designPreview'
import type { Round } from '@/types/design'
import GameCover from './GameCover.vue'
import PlayerAvatar from './PlayerAvatar.vue'
import Icon from '@/components/common/AppIcon.vue'
defineProps<{ round: Round }>()
</script>
<template>
  <RouterLink :to="`/design/rounds/${round.id}`" class="d-round-card">
    <div class="d-round-top"><GameCover :game="gameById(round.game)" small /><div class="d-round-title"><h3>{{ gameById(round.game).name }}</h3><span>{{ modeNames[round.mode] }}<i>·</i>{{ round.players.length }} 人一起玩</span></div><Icon name="arrow" :size="18" /></div>
    <div class="d-result" :class="round.outcome"><Icon :name="round.outcome === 'win' ? 'cup' : 'heart'" :size="16" />{{ resultLabel(round) }}</div>
    <p v-if="round.memory" class="d-memory">{{ round.memory }}</p>
    <div v-if="round.photos.length" class="d-memory-photo"><img :src="round.photos[0]" alt="桌边相聚的示意照片" loading="lazy" /><span><Icon name="photo" :size="14" />{{ round.photos.length }} 张回忆</span></div>
    <div class="d-round-bottom"><div class="d-people"><div class="d-avatar-stack"><PlayerAvatar v-for="id in round.players.slice(0, 4)" :key="id" :id="id" small /></div><span>{{ round.players.slice(0, 2).map(playerName).join('、') }}等 {{ round.players.length }} 人</span></div><span v-if="round.minutes" class="d-inline"><Icon name="clock" :size="14" />{{ round.minutes }} 分钟</span></div>
  </RouterLink>
</template>
