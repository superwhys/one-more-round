<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import Icon from '@/components/common/AppIcon.vue'
import RequestStatus from '@/components/common/RequestStatus.vue'
import RoundOverview from '@/components/rounds/RoundOverview.vue'
import { getRecap } from '@/api/round'
import { photoURL } from '@/api/photo'
import { useGroupContext } from '@/composables/useGroupContext'
import { today } from '@/utils/date'
import { message } from '@/utils/error'
import type { Recap } from '@/types/journal'

const { groupId, snapshot, handleAccessError } = useGroupContext()
const kind = ref<'month' | 'year'>('month')
const currentDate = today()
const year = ref(currentDate.slice(0, 4))
const month = ref(currentDate.slice(5, 7))
const months = Array.from({ length: 12 }, (_, index) => ({
  value: String(index + 1).padStart(2, '0'),
  label: `${index + 1} 月`,
}))
const recap = ref<Recap | null>(null)
const loading = ref(true)
const error = ref('')
const period = computed(() => (kind.value === 'month' ? `${year.value}-${month.value}` : year.value))
const gameName = computed(() => snapshot.value?.games.find(item => item.id === recap.value?.top_game)?.name ?? '暂无')
const playerName = computed(
  () => snapshot.value?.players.find(item => item.id === recap.value?.top_player)?.name ?? '暂无',
)
const overview = computed(() => ({
  total: recap.value?.rounds ?? 0,
  games: recap.value?.games ?? 0,
  players: recap.value?.players ?? 0,
  activity: {},
  items: [],
  stats: [],
}))
async function load() {
  if (!/^\d{4}$/.test(year.value)) {
    loading.value = false
    error.value = '请输入四位年份'
    return
  }
  loading.value = true
  error.value = ''
  try {
    recap.value = await getRecap(groupId, period.value)
  } catch (cause) {
    error.value = message(cause)
    await handleAccessError(cause)
  } finally {
    loading.value = false
  }
}
function switchKind(value: 'month' | 'year') {
  kind.value = value
  void load()
}
function downloadCard() {
  if (!recap.value || !snapshot.value) return
  const canvas = document.createElement('canvas')
  canvas.width = 1200
  canvas.height = 1500
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  ctx.fillStyle = '#f7f4eb'
  ctx.fillRect(0, 0, canvas.width, canvas.height)
  ctx.fillStyle = '#b16d47'
  ctx.fillRect(70, 70, 12, 1360)
  ctx.fillStyle = '#4f5846'
  ctx.font = '700 58px system-ui'
  ctx.fillText('又一局 · 回顾', 135, 180)
  ctx.fillStyle = '#8b6c50'
  ctx.font = '36px system-ui'
  ctx.fillText(`${snapshot.value.group.name} · ${recap.value.period}`, 135, 245)
  ctx.fillStyle = '#2f352c'
  ctx.font = '700 170px system-ui'
  ctx.fillText(String(recap.value.rounds), 135, 510)
  ctx.font = '36px system-ui'
  ctx.fillStyle = '#747b69'
  ctx.fillText('局一起度过的好时光', 380, 505)
  const rows = [
    `玩过 ${recap.value.games} 款桌游`,
    `和 ${recap.value.players} 位朋友同桌`,
    `最常玩：${gameName.value}`,
    `最常参加：${playerName.value}`,
    recap.value.minutes ? `记录时长：${recap.value.minutes} 分钟` : '还有很多时光，不必都用分钟计算',
  ]
  ctx.font = '42px system-ui'
  ctx.fillStyle = '#4f5846'
  rows.forEach((row, index) => ctx.fillText(row, 135, 700 + index * 125))
  ctx.font = '34px system-ui'
  ctx.fillStyle = '#b16d47'
  ctx.fillText('记下每一局的输赢与相聚。', 135, 1370)
  const link = document.createElement('a')
  link.download = `又一局-${recap.value.period}.png`
  link.href = canvas.toDataURL('image/png')
  link.click()
}
onMounted(load)
</script>

<template>
  <header class="d-page-heading">
    <div>
      <p class="d-eyebrow">MEMORIES IN NUMBERS</p>
      <h1>这一段相聚<span class="d-title-dot">。</span></h1>
      <p>把散落的每一局，拼成值得回看的时光。</p>
    </div>
    <RouterLink to="/" class="d-text-link">返回时间线</RouterLink>
  </header>
  <section class="d-surface j-recap-controls">
    <div class="d-segment small">
      <button :class="{ active: kind === 'month' }" @click="switchKind('month')">月度回顾</button
      ><button :class="{ active: kind === 'year' }" @click="switchKind('year')">年度回顾</button>
    </div>
    <div class="j-recap-period">
      <label class="d-field j-recap-period-field"
        >年份<input
          v-model="year"
          type="text"
          inputmode="numeric"
          maxlength="4"
          pattern="[0-9]{4}"
          placeholder="例如 2026"
          @input="year = year.replace(/\D/g, '').slice(0, 4)"
          @change="load" /></label
      ><label v-if="kind === 'month'" class="d-field j-recap-period-field"
        >月份<select v-model="month" @change="load">
          <option v-for="item in months" :key="item.value" :value="item.value">{{ item.label }}</option>
        </select></label
      >
    </div>
  </section>
  <RequestStatus :loading="loading" :error="error" @retry="load" />
  <template v-if="recap"
    ><RoundOverview :page="overview" />
    <section class="d-surface j-recap-highlight">
      <h2>{{ recap.period }} 的桌边故事</h2>
      <div class="j-recap-lines">
        <p>
          <span>最常玩</span><strong>{{ gameName }} · {{ recap.top_game_rounds }} 局</strong>
        </p>
        <p>
          <span>最常参加</span><strong>{{ playerName }} · {{ recap.top_plays }} 次</strong>
        </p>
        <p>
          <span>记录时长</span><strong>{{ recap.minutes ? `${recap.minutes} 分钟` : '暂无记录' }}</strong>
        </p>
      </div>
      <button class="d-button" @click="downloadCard"><Icon name="download" />下载分享卡片</button>
    </section>
    <section v-if="recap.photos.length" class="d-surface">
      <h2>这段时间的照片</h2>
      <div class="j-recap-photos">
        <img v-for="id in recap.photos" :key="id" :src="photoURL(groupId, id)" alt="回顾照片" loading="lazy" />
      </div>
    </section>
    <div v-if="!recap.rounds" class="d-empty">
      <h2>这段时间还没有对局。</h2>
      <RouterLink to="/rounds/new" class="d-button">记下第一局</RouterLink>
    </div></template
  >
</template>
