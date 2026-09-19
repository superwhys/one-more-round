<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import Icon from '@/components/common/AppIcon.vue'
import { modeNames, resultLabel } from '@/utils/round'
import { photoURL } from '@/api/photo'
import type { Snapshot, Page } from '@/types/journal'
import type { RoundFilters } from '@/types/round'
const props = defineProps<{ snapshot: Snapshot; page: Page; loading: boolean; error: string; owner: boolean; fixedGame?: boolean; fixedPlayer?: boolean }>()
const emit = defineEmits<{ filter: [filters: RoundFilters]; more: []; invite: [] }>()
const filterOpen = ref(false)
const from = ref(''); const to = ref(''); const game = ref(''); const player = ref('')
const dateGroups = computed(() => [...new Set(props.page.items.map(round => round.date))].map(date => ({ date, rounds: props.page.items.filter(round => round.date === date) })))
const playerName = (id: string) => props.snapshot.players.find(player => player.id === id)?.name ?? '玩家'
const gameName = (id: string) => props.snapshot.games.find(game => game.id === id)?.name ?? '桌游'
function apply() { emit('filter', { from: from.value, to: to.value, game: game.value, player: player.value }) }
function clear() { from.value = ''; to.value = ''; game.value = ''; player.value = ''; apply() }
</script>

<template>
<div class="d-review-columns"><section><div class="d-list-heading"><h2>对局回忆<small>按日期倒序</small></h2><button class="d-filter-button" :class="{ selected: filterOpen }" :aria-expanded="filterOpen" aria-controls="round-filters" @click="filterOpen = !filterOpen"><Icon name="filter" />筛选</button></div><form v-if="filterOpen" id="round-filters" class="d-filters" @submit.prevent="apply"><label v-if="!fixedGame">桌游<select v-model="game"><option value="">全部桌游</option><option v-for="g in snapshot.games" :key="g.id" :value="g.id">{{ g.name }}</option></select></label><label v-if="!fixedPlayer">玩家<select v-model="player"><option value="">全部玩家</option><option v-for="p in snapshot.players" :key="p.id" :value="p.id">{{ p.name }}</option></select></label><label>开始日期<input v-model="from" type="date" /></label><label>结束日期<input v-model="to" type="date" /></label><div class="j-filter-actions"><button type="button" class="d-text-link" @click="clear">清空筛选</button><button class="d-button secondary">应用筛选</button></div></form><div v-for="day in dateGroups" :key="day.date" class="d-day-group"><div class="d-date-label">{{ day.date }}</div><RouterLink v-for="r in day.rounds" :key="r.id" :to="`/rounds/${r.id}`" class="d-round-card"><div class="d-round-top"><div class="j-mini-cover">✳</div><div class="d-round-title"><h3>{{ gameName(r.game_id) }}</h3><span>{{ modeNames[r.mode] }} · {{ r.players.length }} 人一起玩</span></div><Icon name="arrow" /></div><div class="d-result" :class="r.outcome">{{ resultLabel(r, snapshot.players) }}</div><p v-if="r.memory" class="d-memory">{{ r.memory }}</p><div v-if="r.photos.length" class="d-memory-photo"><img :src="photoURL(snapshot.group.id, r.photos[0] ?? '', true)" alt="本局桌边回忆" loading="lazy" /><span>{{ r.photos.length }} 张回忆</span></div><div class="d-round-bottom"><span>{{ r.players.map(playerName).join('、') }}</span><span v-if="r.minutes">{{ r.minutes }} 分钟</span></div></RouterLink></div><div v-if="!page.items.length && !loading && !error" class="d-empty"><Icon name="review" :size="34" /><h3>{{ from || to || game || player ? '这个范围还没有对局' : '第一次相聚，从这一局开始' }}</h3><RouterLink to="/rounds/new" class="d-button">记一局</RouterLink></div><button v-if="page.items.length < page.total" class="d-button secondary full" :disabled="loading" @click="$emit('more')">加载更多</button><p v-else-if="page.total" class="d-end-note">— 回忆先到这里，下一局待续 —</p></section><aside class="d-companion"><section class="d-side-panel"><div class="d-list-heading"><h2>这张桌子上的朋友</h2><RouterLink to="/group" aria-label="查看小组"><Icon name="arrow" /></RouterLink></div><div class="d-friend-grid"><RouterLink v-for="p in snapshot.players.slice(0, 6)" :key="p.id" :to="`/players/${p.id}`"><span class="d-avatar">{{ Array.from(p.name)[0]?.toUpperCase() }}</span><span>{{ p.name }}</span></RouterLink></div><button v-if="owner" class="d-invite-button" @click="$emit('invite')">＋ 邀请朋友来坐坐</button></section><div class="d-paper-note"><span>给下一次见面的我们</span><p>“要不，<br />再来一局？”</p><Icon name="heart" /></div></aside></div>
</template>
