<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import Icon from '@/components/common/AppIcon.vue'
import PlayerChip from '@/components/common/PlayerChip.vue'
import { modeNames, resultLabel } from '@/utils/round'
import { photoURL } from '@/api/photo'
import { today } from '@/utils/date'
import type { Snapshot, Page } from '@/types/journal'
import type { RoundFilters } from '@/types/round'

const props = defineProps<{
  snapshot: Snapshot
  page: Page
  loading: boolean
  error: string
  owner: boolean
  fixedGame?: boolean
  fixedPlayer?: boolean
}>()
const emit = defineEmits<{ filter: [filters: RoundFilters]; more: []; invite: [] }>()
const filterOpen = ref(false)
const filterMotion = ref(false)
const from = ref('')
const to = ref('')
const game = ref('')
const player = ref('')
const q = ref('')
const mode = ref('')
const outcome = ref('')
const hasPhotos = ref('')
const dateGroups = computed(() =>
  [...new Set(props.page.items.map(round => round.date))].map(date => ({
    date,
    rounds: props.page.items.filter(round => round.date === date),
  })),
)
const hasFilter = computed(() =>
  [from.value, to.value, game.value, player.value, q.value, mode.value, outcome.value, hasPhotos.value].some(Boolean),
)
const playerName = (id: string) => props.snapshot.players.find(player => player.id === id)?.name ?? '玩家'
const gameName = (id: string) => props.snapshot.games.find(game => game.id === id)?.name ?? '桌游'
function values(): RoundFilters {
  return {
    from: from.value,
    to: to.value,
    game: game.value,
    player: player.value,
    q: q.value,
    mode: mode.value,
    outcome: outcome.value,
    has_photos: hasPhotos.value,
  }
}
function apply() {
  emit('filter', values())
}
function toggleFilters(event: MouseEvent) {
  filterMotion.value = event.detail > 0
  filterOpen.value = !filterOpen.value
}
function clear() {
  from.value = ''
  to.value = ''
  game.value = ''
  player.value = ''
  q.value = ''
  mode.value = ''
  outcome.value = ''
  hasPhotos.value = ''
  apply()
}
function quick(range: 'month' | 'year') {
  const current = today()
  from.value = range === 'month' ? `${current.slice(0, 7)}-01` : `${current.slice(0, 4)}-01-01`
  to.value = current
  apply()
}
</script>

<template>
  <div class="d-review-columns journal-history">
    <section class="history-main">
      <div class="d-list-heading">
        <h2>对局回忆<small>按日期倒序</small></h2>
        <button
          class="d-filter-button"
          :class="{ selected: filterOpen || hasFilter }"
          :aria-expanded="filterOpen"
          aria-controls="round-filters"
          @click="toggleFilters"
        >
          <Icon name="filter" />筛选<span v-if="hasFilter">· 已应用</span>
        </button>
      </div>
      <Transition name="filter-reveal" :css="filterMotion">
        <form
          v-show="filterOpen"
          id="round-filters"
          class="d-filters j-advanced-filters"
          :inert="!filterOpen"
          @submit.prevent="apply"
        >
          <label class="j-filter-query"
            >搜索回忆<input v-model="q" type="search" placeholder="例如：第一次通关"
          /></label>
          <label v-if="!fixedGame"
            >桌游<select v-model="game">
              <option value="">全部桌游</option>
              <option v-for="g in snapshot.games" :key="g.id" :value="g.id">{{ g.name }}</option>
            </select></label
          >
          <label v-if="!fixedPlayer"
            >玩家<select v-model="player">
              <option value="">全部玩家</option>
              <option v-for="p in snapshot.players" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select></label
          >
          <label
            >模式<select v-model="mode">
              <option value="">全部模式</option>
              <option v-for="(label, value) in modeNames" :key="value" :value="value">{{ label }}</option>
            </select></label
          >
          <label
            >结果<select v-model="outcome">
              <option value="">全部结果</option>
              <option value="win">胜利 / 成功</option>
              <option value="loss">失败</option>
              <option value="draw">平局</option>
              <option value="unknown">未记结果</option>
            </select></label
          >
          <label
            >照片<select v-model="hasPhotos">
              <option value="">不限</option>
              <option value="true">有照片</option>
              <option value="false">无照片</option>
            </select></label
          >
          <label>开始日期<input v-model="from" type="date" /></label
          ><label>结束日期<input v-model="to" type="date" /></label>
          <div class="j-filter-quick">
            <button type="button" class="d-text-link" @click="quick('month')">本月</button
            ><button type="button" class="d-text-link" @click="quick('year')">今年</button>
          </div>
          <div class="j-filter-actions">
            <button type="button" class="d-text-link" @click="clear">清空筛选</button
            ><button class="d-button secondary">应用筛选</button>
          </div>
        </form>
      </Transition>
      <div v-for="day in dateGroups" :key="day.date" class="d-day-group">
        <div class="d-date-label">
          <time :datetime="day.date">{{ day.date }}</time
          ><span>一起坐在桌边的日子</span>
        </div>
        <RouterLink v-for="r in day.rounds" :key="r.id" :to="`/rounds/${r.id}`" class="d-round-card"
          ><div class="d-round-top">
            <div class="j-mini-cover" :class="r.mode" aria-hidden="true">
              <Icon :name="r.mode === 'coop' ? 'heart' : r.mode === 'team' ? 'group' : 'game'" :size="27" />
            </div>
            <div class="d-round-title">
              <h3>{{ gameName(r.game_id) }}</h3>
              <span>{{ modeNames[r.mode] }} · {{ r.players.length }} 人一起玩</span>
            </div>
            <Icon name="arrow" />
          </div>
          <div class="d-result" :class="r.outcome">{{ resultLabel(r, snapshot.players) }}</div>
          <p v-if="r.memory" class="d-memory">{{ r.memory }}</p>
          <div v-if="r.photos.length" class="d-memory-photo">
            <img :src="photoURL(snapshot.group.id, r.photos[0] ?? '')" alt="本局桌边回忆" loading="lazy" /><span
              >{{ r.photos.length }} 张回忆</span
            >
          </div>
          <div class="d-round-bottom">
            <span class="history-players"
              ><Icon name="group" :size="14" />{{ r.players.map(playerName).join('、') }}</span
            ><span v-if="r.minutes" class="history-duration"><Icon name="clock" :size="13" />{{ r.minutes }} 分钟</span>
          </div></RouterLink
        >
      </div>
      <div v-if="!page.items.length && !loading && !error" class="d-empty">
        <Icon name="review" :size="34" />
        <h3>{{ hasFilter ? '这个范围还没有对局' : '第一次相聚，从这一局开始' }}</h3>
        <p>{{ hasFilter ? '换个条件找找，或者看看全部回忆。' : '选一款桌游，记下今天和谁一起玩。' }}</p>
        <button v-if="hasFilter" class="d-button secondary" @click="clear">清空筛选</button>
        <RouterLink v-else to="/rounds/new" class="d-button">记一局</RouterLink>
      </div>
      <button
        v-if="page.items.length < page.total"
        class="d-button secondary full"
        :disabled="loading"
        @click="$emit('more')"
      >
        加载更多
      </button>
      <p v-else-if="page.total" class="d-end-note">— 回忆先到这里，下一局待续 —</p>
    </section>
    <aside class="d-companion">
      <section class="d-side-panel">
        <div class="d-list-heading">
          <h2>这张桌子上的朋友</h2>
          <RouterLink to="/group" aria-label="查看小组"><Icon name="arrow" /></RouterLink>
        </div>
        <div class="d-friend-grid">
          <RouterLink v-for="p in snapshot.players.slice(0, 6)" :key="p.id" :to="`/players/${p.id}`"
            ><PlayerChip :name="p.name" stacked
          /></RouterLink>
        </div>
        <button v-if="owner" class="d-invite-button" @click="$emit('invite')">＋ 邀请朋友来坐坐</button>
      </section>
      <div class="d-paper-note">
        <span>给下一次见面的我们</span>
        <p>“要不，<br />再来一局？”</p>
        <Icon name="heart" />
      </div>
    </aside>
  </div>
</template>

<style scoped>
.history-main {
  min-width: 0;
}
.journal-history .d-list-heading h2 {
  font-size: 16px;
}
.journal-history .d-list-heading h2 small {
  font-size: 10px;
}
.journal-history .d-date-label {
  position: relative;
  gap: 10px;
  margin-bottom: 15px;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}
.journal-history .d-date-label::before {
  width: 7px;
  height: 7px;
  box-shadow: 0 0 0 4px var(--d-bg);
  background: #bd8164;
}
.journal-history .d-date-label > span {
  font-size: 9px;
  letter-spacing: 0.4px;
}
.d-day-group {
  position: relative;
  padding-left: 22px;
}
.d-day-group::before {
  content: '';
  position: absolute;
  left: 3px;
  top: 9px;
  bottom: -28px;
  border-left: 1px solid #e2e4d7;
}
.d-day-group:last-of-type::before {
  bottom: 0;
}
.d-day-group .d-date-label {
  margin-left: -22px;
}
.journal-history .d-round-card {
  position: relative;
  padding: 22px;
  border-radius: 14px;
  box-shadow: 0 4px 12px -8px #484e3026;
  transition: border-color 160ms ease;
}
.journal-history .d-round-title h3 {
  font-size: 17px;
  overflow-wrap: anywhere;
}
.journal-history .d-round-title > span {
  color: #7e8574;
  font-size: 11px;
}
.journal-history .j-mini-cover {
  width: 48px;
  height: 54px;
  border-radius: 8px;
  border: 1px solid #dfd1b3;
  background: #eee4cd;
  color: #a68a52;
}
.journal-history .j-mini-cover.coop {
  color: #6a806a;
  background: #e5ebdd;
  border-color: #d0dac8;
}
.journal-history .j-mini-cover.team {
  color: #b1745b;
  background: #f1e2d8;
  border-color: #e2cbbb;
}
.journal-history .d-result {
  font-size: 11px;
  border-radius: 6px;
  max-width: 100%;
  overflow-wrap: anywhere;
}
.journal-history .d-memory {
  padding-left: 12px;
  border-left: 2px solid #e3dfcc;
  color: #69725f;
  font-size: 13px;
  overflow-wrap: anywhere;
  white-space: pre-line;
}
.journal-history .d-round-bottom {
  color: #808874;
  font-size: 10px;
}
.history-players,
.history-duration {
  display: flex;
  align-items: flex-start;
  gap: 6px;
}
.history-players {
  min-width: 0;
}
.history-players > svg,
.history-duration > svg {
  margin-top: 1px;
}
.history-duration {
  white-space: nowrap;
}
.d-round-top > svg {
  transition: transform 160ms var(--ease-out);
}
.journal-history .d-side-panel {
  border: 1px solid #e2e5d6;
  border-radius: 14px;
  background: #eff1e7;
}
.journal-history .d-side-panel .d-list-heading h2 {
  font-size: 12px;
}
.journal-history .d-side-panel .d-list-heading a {
  min-width: 32px;
  min-height: 36px;
  align-items: center;
  justify-content: center;
}
.journal-history .d-friend-grid a {
  min-width: 0;
}
.journal-history .d-friend-grid :deep(.d-player-label) {
  max-width: 64px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.journal-history .d-empty {
  border: 1px dashed #d9ddca;
  border-radius: 14px;
  background: #f2f3e9;
}
.filter-reveal-enter-active {
  transition:
    opacity 200ms var(--ease-out),
    transform 200ms var(--ease-out);
}
.filter-reveal-leave-active {
  transition:
    opacity 120ms var(--ease-out),
    transform 120ms var(--ease-out);
}
.filter-reveal-enter-from,
.filter-reveal-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
@media (hover: hover) and (pointer: fine) {
  .d-round-card:hover .d-round-top > svg {
    transform: translateX(3px);
  }
}
@media (max-width: 760px) {
  .d-day-group {
    padding-left: 15px;
  }
  .d-day-group .d-date-label {
    margin-left: -15px;
  }
  .journal-history .d-round-card {
    padding: 17px;
    border-radius: 12px;
  }
  .journal-history .d-round-title h3 {
    font-size: 16px;
  }
  .journal-history .d-memory {
    font-size: 12px;
  }
  .journal-history .j-mini-cover {
    width: 40px;
    height: 46px;
  }
  .journal-history .d-round-bottom {
    flex-wrap: wrap;
    gap: 8px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .filter-reveal-enter-from,
  .filter-reveal-leave-to,
  .d-round-card:hover .d-round-top > svg {
    transform: none;
  }
}
</style>
