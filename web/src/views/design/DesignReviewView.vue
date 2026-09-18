<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { games, players, rounds, dateLabel, gameRounds } from '@/stores/designPreview'
import { useDesignPreview } from '@/composables/useDesignPreview'
import Icon from '@/components/common/AppIcon.vue'
import Avatar from '@/components/design/PlayerAvatar.vue'
import GameCover from '@/components/design/GameCover.vue'
import RoundCard from '@/components/design/RoundCard.vue'
const filterOpen = ref(false)
const gameFilter = ref('')
const playerFilter = ref('')
const from = ref('')
const to = ref('')
const activeFilters = computed(() => [gameFilter.value, playerFilter.value, from.value, to.value].filter(Boolean).length)
const filteredRounds = computed(() => rounds.filter((r) => (!gameFilter.value || r.game === gameFilter.value) && (!playerFilter.value || r.players.includes(playerFilter.value)) && (!from.value || r.date >= from.value) && (!to.value || r.date <= to.value)).sort((a, b) => b.date.localeCompare(a.date)))
const dateGroups = computed(() => [...new Set(filteredRounds.value.map((r) => r.date))].map((date) => ({ date, rounds: filteredRounds.value.filter((r) => r.date === date) })))
const commonGames = computed(() => [...games].sort((a, b) => gameRounds(b.id).length - gameRounds(a.id).length).slice(0, 3))
function clearFilters() { gameFilter.value = ''; playerFilter.value = ''; from.value = ''; to.value = '' }
const { modal } = useDesignPreview()
</script>

<template>
<header class="d-page-heading"><div><p class="d-eyebrow">GOOD TIMES, ONE ROUND AT A TIME</p><h1>一起玩过的日子<span class="d-title-dot">。</span></h1><p>有输有赢，有你们就很开心。</p></div><RouterLink to="/design/new" class="d-button d-heading-record"><Icon name="plus" :size="18" />记一局</RouterLink></header>
          <div class="d-overview"><div><strong>{{ filteredRounds.length }}<small>局</small></strong><span>{{ activeFilters ? '筛选范围内的对局' : '一起度过的好时光' }}</span></div><div><strong>{{ new Set(filteredRounds.map((r) => r.game)).size }}<small>款</small></strong><span>玩过的桌游</span></div><div><strong>{{ new Set(filteredRounds.flatMap((r) => r.players)).size }}<small>位</small></strong><span>同桌的朋友</span></div><div class="d-overview-note"><Icon name="heart" :size="22" /><span>最好的下一局，<br />是和你们一起。</span></div></div>
          <div class="d-review-columns"><section><div class="d-list-heading"><h2>对局回忆<small>{{ activeFilters ? '已筛选' : '全部记录' }}</small></h2><button class="d-filter-button" :class="{ selected: filterOpen || activeFilters }" @click="filterOpen = !filterOpen"><Icon name="filter" :size="16" />筛选{{ activeFilters ? ` · ${activeFilters}` : '' }}</button></div>
            <div v-if="filterOpen" class="d-filters"><label>桌游<select v-model="gameFilter"><option value="">全部桌游</option><option v-for="game in games" :key="game.id" :value="game.id">{{ game.name }}</option></select></label><label>玩家<select v-model="playerFilter"><option value="">全部玩家</option><option v-for="player in players" :key="player.id" :value="player.id">{{ player.name }}</option></select></label><label>开始日期<input v-model="from" type="date" /></label><label>结束日期<input v-model="to" type="date" /></label><button class="d-text-link" @click="clearFilters">清空筛选</button></div>
            <div v-for="group in dateGroups" :key="group.date" class="d-day-group"><div class="d-date-label"><span>{{ dateLabel(group.date) }}</span><span>{{ group.rounds.length }} 局小确幸</span></div><RoundCard v-for="round in group.rounds" :key="round.id" :round="round" /></div>
            <div v-if="!filteredRounds.length" class="d-empty"><Icon name="review" :size="34" /><h3>{{ activeFilters ? '这个范围还没有对局' : '第一次相聚，从这一局开始' }}</h3><p>留一点位置，给下一次好时光。</p><button v-if="activeFilters" class="d-button secondary" @click="clearFilters">清空筛选</button><RouterLink v-else to="/design/new" class="d-button">记第一局</RouterLink></div><p v-else class="d-end-note">— 回忆先到这里，下一局待续 —</p>
          </section><aside class="d-companion"><section class="d-side-panel"><div class="d-list-heading"><h2>这张桌子上的朋友</h2><RouterLink to="/design/group" aria-label="查看全部朋友"><Icon name="arrow" :size="16" /></RouterLink></div><div class="d-friend-grid"><RouterLink v-for="player in players.slice(0, 6)" :key="player.id" :to="`/design/players/${player.id}`"><Avatar :id="player.id" /><span>{{ player.name }}</span></RouterLink></div><button class="d-invite-button" @click="modal = 'invite'"><Icon name="plus" :size="17" />邀请朋友来坐坐</button></section><section class="d-side-panel"><div class="d-list-heading"><h2>最近常上桌</h2><RouterLink to="/design/games" aria-label="查看全部桌游"><Icon name="arrow" :size="16" /></RouterLink></div><RouterLink v-for="game in commonGames" :key="game.id" :to="`/design/games/${game.id}`" class="d-common-game"><GameCover :game="game" small /><span><strong>{{ game.name }}</strong><small>一起玩过 {{ gameRounds(game.id).length }} 局</small></span></RouterLink></section><div class="d-paper-note"><span>给下一次见面的我们</span><p>“要不，<br />再来一局？”</p><Icon name="heart" :size="22" /></div></aside></div>
</template>
