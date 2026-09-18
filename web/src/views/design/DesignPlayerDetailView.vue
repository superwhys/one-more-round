<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { games, players, rounds, demoSession, modeNames, performance } from '@/stores/designPreview'
import Avatar from '@/components/design/PlayerAvatar.vue'
import GameCover from '@/components/design/GameCover.vue'
import RoundCard from '@/components/design/RoundCard.vue'
import type { Mode } from '@/types/design'
const route = useRoute()
const itemId = computed(() => String(route.params.id))
const currentPlayer = computed(() => players.find((p) => p.id === itemId.value))
const playerHistory = computed(() => rounds.filter((r) => r.players.includes(itemId.value)).sort((a, b) => b.date.localeCompare(a.date)))
const statMode = ref<Mode>('individual')
const modes: Mode[] = ['individual', 'team', 'coop']
</script>

<template>
<template v-if="currentPlayer">
          <div class="d-player-heading"><Avatar :id="currentPlayer.id" /><div><p class="d-eyebrow">A FRIEND AT OUR TABLE</p><h1>{{ currentPlayer.name }}<span v-if="currentPlayer.id === 'lin'">（我）</span></h1><p>{{ currentPlayer.linked ? '已关联账号' : '昵称玩家' }} · 在「{{ demoSession.group }}」的对局日记</p></div></div>
          <div class="d-overview"><div><strong>{{ playerHistory.length }}<small>局</small></strong><span>参与的对局</span></div><div><strong>{{ new Set(playerHistory.map((r) => r.game)).size }}<small>款</small></strong><span>一起玩过的桌游</span></div><div><strong>{{ new Set(playerHistory.map((r) => r.date)).size }}<small>天</small></strong><span>留下回忆的日子</span></div></div>
          <section class="d-surface d-stat-section"><div class="d-list-heading"><h2>每一款，都有自己的故事</h2></div><div class="d-segment small"><button v-for="mode in modes" :key="mode" :class="{ active: statMode === mode }" @click="statMode = mode">{{ modeNames[mode] }}</button></div><template v-for="game in games" :key="game.id"><div v-if="playerHistory.some((r) => r.game === game.id && r.mode === statMode)" class="d-player-stat"><GameCover :game="game" small /><RouterLink :to="`/design/games/${game.id}`">{{ game.name }}</RouterLink><span v-if="performance(playerHistory.filter((r) => r.game === game.id && r.mode === statMode), currentPlayer.id).total"><strong>{{ performance(playerHistory.filter((r) => r.game === game.id && r.mode === statMode), currentPlayer.id).rate }}%</strong><small>{{ performance(playerHistory.filter((r) => r.game === game.id && r.mode === statMode), currentPlayer.id).wins }} {{ statMode === 'coop' ? '成功' : '胜' }} / {{ performance(playerHistory.filter((r) => r.game === game.id && r.mode === statMode), currentPlayer.id).total }} 局</small></span><span v-else>暂无数据</span></div></template><p v-if="!playerHistory.some((r) => r.mode === statMode)" class="d-note">这个模式还没有记录，暂无数据。</p><p class="d-note">平局计入有效局数；未记结果不计入比例。</p></section>
          <div class="d-list-heading"><h2>最近一起玩过</h2></div><div class="d-history-grid"><RoundCard v-for="round in playerHistory" :key="round.id" :round="round" /></div>
</template><div v-else class="d-empty">这页回忆不存在。</div>
</template>
