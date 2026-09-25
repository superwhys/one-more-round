<script setup lang="ts">
import { computed, onMounted, onScopeDispose, ref } from 'vue'
import { RouterLink } from 'vue-router'
import Icon from '@/components/common/AppIcon.vue'
import RequestStatus from '@/components/common/RequestStatus.vue'
import { photoURL } from '@/api/photo'
import { listRounds } from '@/api/round'
import { useGroupContext } from '@/composables/useGroupContext'
import { message } from '@/utils/error'
import type { Round } from '@/types/journal'

const { groupId, snapshot, handleAccessError } = useGroupContext()
const from = ref('')
const to = ref('')
const game = ref('')
const applied = ref({ from: '', to: '', game: '' })
const rounds = ref<Round[]>([])
const total = ref(0)
const loading = ref(true)
const loadingMore = ref(false)
const error = ref('')
const filterError = ref('')
const errorIsMore = ref(false)
const hasDraftFilter = computed(() => Boolean(from.value || to.value || game.value))
const hasFilter = computed(() => Boolean(applied.value.from || applied.value.to || applied.value.game))
const hasMore = computed(() => rounds.value.length < total.value)
const dateGroups = computed(() => {
  const groups = new Map<string, Round[]>()
  for (const round of rounds.value) {
    const group = groups.get(round.date) ?? []
    group.push(round)
    groups.set(round.date, group)
  }
  return [...groups].map(([date, items]) => ({ date, items }))
})
// gameName 从当前小组快照读取桌游名称。
const gameName = (id: string) => snapshot.value?.games.find(item => item.id === id)?.name ?? '桌游'
let generation = 0
onScopeDispose(() => {
  generation++
})

// load 按当前已应用的条件获取照片对局，并忽略过期请求。
async function load(more = false) {
  if (loadingMore.value && more) return
  const run = ++generation
  if (more) {
    loadingMore.value = true
  } else {
    loading.value = true
    rounds.value = []
    total.value = 0
  }
  error.value = ''
  errorIsMore.value = more
  try {
    const page = await listRounds(groupId, {
      ...applied.value,
      has_photos: 'true',
      limit: 30,
      offset: more ? rounds.value.length : 0,
    })
    if (run !== generation) return
    rounds.value = more ? [...rounds.value, ...page.items] : page.items
    total.value = page.total
  } catch (cause) {
    if (run !== generation) return
    error.value = message(cause)
    await handleAccessError(cause)
  } finally {
    if (run === generation) {
      loading.value = false
      loadingMore.value = false
    }
  }
}

// apply 验证日期范围后使用表单条件重新加载相册。
function apply() {
  filterError.value = from.value && to.value && from.value > to.value ? '开始日期不能晚于结束日期' : ''
  if (filterError.value) return
  applied.value = { from: from.value, to: to.value, game: game.value }
  void load()
}

// clear 清除全部筛选并重新显示本组照片。
function clear() {
  from.value = ''
  to.value = ''
  game.value = ''
  apply()
}

// retry 重试失败的首屏或后续分页。
function retry() {
  void load(errorIsMore.value)
}

onMounted(() => {
  void load()
})
</script>

<template>
  <header class="d-page-heading">
    <div>
      <p class="d-eyebrow">MOMENTS AROUND THE TABLE</p>
      <h1>聚会相册<span class="d-title-dot">。</span></h1>
      <p>翻翻一起玩过的照片，再回到那一局。</p>
    </div>
    <RouterLink to="/" class="d-text-link">返回时间线</RouterLink>
  </header>

  <form class="d-filters album-filters" @submit.prevent="apply">
    <label>
      桌游
      <select v-model="game">
        <option value="">全部桌游</option>
        <option v-for="item in snapshot?.games ?? []" :key="item.id" :value="item.id">{{ item.name }}</option>
      </select>
    </label>
    <label>开始日期<input v-model="from" type="date" /></label>
    <label>结束日期<input v-model="to" type="date" /></label>
    <div class="album-filter-actions">
      <button class="d-button" type="submit" :disabled="loading || loadingMore">查看照片</button>
      <button v-if="hasDraftFilter" class="d-text-link" type="button" @click="clear">清空筛选</button>
    </div>
    <p v-if="filterError" class="album-filter-error" role="alert">{{ filterError }}</p>
  </form>

  <RequestStatus :loading="loading" :error="error" @retry="retry" />
  <template v-if="!loading">
    <p v-if="rounds.length" class="album-count">共 {{ total }} 局留下了照片</p>
    <section v-for="day in dateGroups" :key="day.date" class="album-day" :aria-label="`${day.date} 的照片`">
      <div class="d-date-label">
        <time :datetime="day.date">{{ day.date }}</time
        ><span>那天的桌边时光</span>
      </div>
      <div class="album-grid">
        <template v-for="round in day.items" :key="round.id">
          <RouterLink
            v-for="(id, index) in round.photos"
            :key="id"
            :to="{ name: 'round-detail', params: { id: round.id } }"
            class="album-photo"
          >
            <img
              :src="photoURL(groupId, id)"
              :alt="`${gameName(round.game_id)}的第 ${index + 1} 张对局照片`"
              loading="lazy"
            />
            <span class="album-caption"
              ><strong>{{ gameName(round.game_id) }}</strong
              ><Icon name="arrow" :size="17"
            /></span>
          </RouterLink>
        </template>
      </div>
    </section>
    <div v-if="!rounds.length && !error" class="d-empty album-empty">
      <Icon name="photo" :size="34" />
      <h3>{{ hasFilter ? '这个范围还没有照片' : '相册里还没有照片' }}</h3>
      <p>{{ hasFilter ? '换个日期或桌游看看。' : '下次记局时，留下一张桌边回忆吧。' }}</p>
      <button v-if="hasFilter" class="d-button secondary" type="button" @click="clear">查看全部照片</button>
      <RouterLink v-else to="/rounds/new" class="d-button">记一局</RouterLink>
    </div>
    <button
      v-if="hasMore"
      class="d-button secondary full album-more"
      type="button"
      :disabled="loadingMore"
      @click="load(true)"
    >
      {{ loadingMore ? '正在加载照片…' : '加载更多照片' }}
    </button>
    <p v-else-if="rounds.length" class="d-end-note">— 照片先到这里，下一局待续 —</p>
  </template>
</template>

<style scoped>
.album-filters {
  grid-template-columns: repeat(3, minmax(0, 1fr)) auto;
  align-items: end;
  margin-bottom: 28px;
}
.album-filter-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 14px;
}
.album-filter-actions .d-text-link {
  min-height: 44px;
}
.album-filter-error {
  grid-column: 1 / -1;
  color: #9c4b39;
  font-size: 12px;
}
.album-count {
  margin-bottom: 22px;
  color: #7c826f;
  font-size: 12px;
}
.album-day {
  margin-bottom: 32px;
}
.album-day .d-date-label {
  margin-bottom: 16px;
}
.album-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 16px;
}
.album-photo {
  display: block;
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--d-line);
  border-radius: 11px;
  background: var(--d-paper);
  transition: border-color 150ms ease;
}
.album-photo img {
  display: block;
  width: 100%;
  aspect-ratio: 4 / 3;
  object-fit: cover;
}
.album-caption {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 12px;
  color: #596650;
  font-size: 12px;
}
.album-caption strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.album-caption svg {
  flex: none;
}
.album-empty {
  border: 1px dashed #d9ddca;
  border-radius: 14px;
  background: #f2f3e9;
}
.album-more {
  margin-top: 12px;
}
@media (hover: hover) and (pointer: fine) {
  .album-photo:hover {
    border-color: #bac1ac;
  }
}
@media (max-width: 960px) {
  .album-filters {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .album-filter-actions {
    grid-column: 1 / -1;
  }
}
@media (max-width: 600px) {
  .album-filters {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .album-filters label:first-child {
    grid-column: 1 / -1;
  }
  .album-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }
  .album-caption {
    padding: 9px;
    font-size: 11px;
  }
}
</style>
