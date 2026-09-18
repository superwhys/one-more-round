<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { games, players, rounds, modeNames, performance } from '@/stores/designPreview'
import Icon from '@/components/common/AppIcon.vue'
import Avatar from '@/components/design/PlayerAvatar.vue'
import GameCover from '@/components/design/GameCover.vue'
import RoundCard from '@/components/design/RoundCard.vue'
import type { Mode } from '@/types/design'
const route = useRoute()
const itemId = computed(() => String(route.params.id))
const currentGame = computed(() => games.find((g) => g.id === itemId.value))
const gameHistory = computed(() => rounds.filter((r) => r.game === itemId.value).sort((a, b) => b.date.localeCompare(a.date)))
const statMode = ref<Mode>('individual')
const visiblePlayers = computed(() => players.filter((p) => gameHistory.value.some((r) => r.mode === statMode.value && r.players.includes(p.id))))
const modes: Mode[] = ['individual', 'team', 'coop']
</script>

<template>
<template v-if="currentGame">
          <div class="d-game-hero"><GameCover :game="currentGame" /><div><p class="d-eyebrow">ON OUR TABLE</p><h1>{{ currentGame.name }}</h1><span class="d-english-name">{{ currentGame.english }}</span><p>{{ currentGame.caption }}</p><div class="d-game-meta">{{ gameHistory.length }} 次相聚<span>·</span>{{ new Set(gameHistory.flatMap((r) => r.players)).size }} 位朋友一起玩过</div><RouterLink :to="`/design/new?game=${currentGame.id}`" class="d-button"><Icon name="plus" :size="17" />再玩一局</RouterLink></div></div>
          <section class="d-surface d-stat-section"><div class="d-list-heading"><h2>这款游戏里的我们</h2><span>只比较同一种模式</span></div><div class="d-segment small"><button v-for="mode in modes" :key="mode" :class="{ active: statMode === mode }" @click="statMode = mode">{{ modeNames[mode] }}</button></div><div v-for="player in visiblePlayers" :key="player.id" class="d-stat-row"><RouterLink :to="`/design/players/${player.id}`"><Avatar :id="player.id" small />{{ player.name }}</RouterLink><div class="d-stat-track"><span :style="{ width: `${performance(gameHistory.filter((r) => r.mode === statMode), player.id).rate ?? 0}%` }"></span></div><span v-if="performance(gameHistory.filter((r) => r.mode === statMode), player.id).total"><strong>{{ performance(gameHistory.filter((r) => r.mode === statMode), player.id).rate }}%</strong><small>{{ performance(gameHistory.filter((r) => r.mode === statMode), player.id).wins }} {{ statMode === 'coop' ? '成功' : '胜' }} / {{ performance(gameHistory.filter((r) => r.mode === statMode), player.id).total }} 局</small></span><span v-else>暂无数据</span></div><p v-if="!visiblePlayers.length" class="d-note">这个模式还没有对局，暂无数据。</p></section>
          <div class="d-list-heading"><h2>一起玩过的 {{ gameHistory.length }} 局</h2></div><div class="d-history-grid"><RoundCard v-for="round in gameHistory" :key="round.id" :round="round" /></div>
</template><div v-else class="d-empty">这页回忆不存在。</div>
</template>
