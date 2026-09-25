<script setup lang="ts">
import { computed, onMounted, onScopeDispose, ref } from 'vue'
import { useRoute } from 'vue-router'
import PlayerChip from '@/components/common/PlayerChip.vue'
import RequestStatus from '@/components/common/RequestStatus.vue'
import BggAttribution from '@/components/games/BggAttribution.vue'
import favicon from '@/assets/favicon.svg'
import { getPublicRound, getPublicRoundPhoto } from '@/api/round'
import type { PublicRound } from '@/types/journal'
import { modeNames } from '@/utils/round'
import { message } from '@/utils/error'
import '@/styles/theme.css'
import '@/styles/journal.css'

const route = useRoute()
const token = route.hash.slice(1)
const round = ref<PublicRound | null>(null)
const photoURLs = ref<string[]>([])
const loading = ref(true)
const error = ref('')

const result = computed(() => {
  if (!round.value) return ''
  if (round.value.outcome === 'unknown') return '♡ 未记结果，也是一段好时光'
  if (round.value.outcome === 'draw') return '＝ 全局平局'
  if (round.value.mode === 'coop') return round.value.outcome === 'win' ? '♔ 全队胜利' : '♡ 全队失败，下次再来'
  if (round.value.mode === 'team')
    return `♔ ${round.value.teams
      .filter(team => team.winner)
      .map(team => team.name)
      .join('、')}获胜`
  const winners = round.value.players.filter(player => player.winner).map(player => player.name)
  return `♔ ${winners.join('、')}${winners.length > 1 ? '共同获胜' : '获胜'}`
})

async function load() {
  loading.value = true
  error.value = ''
  round.value = null
  clearPhotos()
  try {
    const shared = await getPublicRound(token)
    const photos = await Promise.all(shared.photos.map(id => getPublicRoundPhoto(token, id)))
    photoURLs.value = photos.map(photo => URL.createObjectURL(photo))
    round.value = shared
  } catch (cause) {
    error.value = message(cause)
  } finally {
    loading.value = false
  }
}

function clearPhotos() {
  for (const url of photoURLs.value) URL.revokeObjectURL(url)
  photoURLs.value = []
}

onMounted(load)
onScopeDispose(clearPhotos)
</script>

<template>
  <div class="journal-app journal-root j-public-share">
    <header class="j-public-header">
      <img :src="favicon" alt="" /><span>又一局<small>记下每一局的输赢与相聚</small></span>
    </header>
    <main class="j-public-main">
      <RequestStatus :loading="loading" :error="error" @retry="load" />
      <template v-if="round">
        <header class="d-page-heading compact">
          <div>
            <p class="d-eyebrow">{{ round.group_name }} 分享的桌边回忆</p>
            <h1>{{ round.game_name }}</h1>
            <p>{{ round.date }} · {{ modeNames[round.mode] }}</p>
          </div>
        </header>
        <article class="d-surface j-detail">
          <h2 class="d-result" :class="round.outcome">{{ result }}</h2>
          <div class="j-players">
            <span v-for="player in round.players" :key="player.name" class="j-player"
              ><PlayerChip :name="player.name"
                ><span v-if="player.score != null">{{ player.score }} 分</span></PlayerChip
              ></span
            >
          </div>
          <div v-for="team in round.teams" :key="team.name" class="j-team">
            <strong>{{ team.name }} {{ team.winner ? '♔ 获胜' : '' }}</strong>
            <p>
              {{ team.players.join('、') }}<span v-if="team.score != null"> · {{ team.score }} 分</span>
            </p>
          </div>
          <p v-if="round.team_score != null">团队分数：{{ round.team_score }}</p>
          <blockquote v-if="round.memory">{{ round.memory }}</blockquote>
          <div v-if="photoURLs.length" class="j-public-photos">
            <img v-for="(url, index) in photoURLs" :key="round.photos[index]" :src="url" alt="分享的对局照片" />
          </div>
        </article>
      </template>
    </main>
    <footer class="j-public-footer">
      <p>这是一条由小组成员主动公开的对局记录。链接被撤销或记录被删除后将无法访问。</p>
      <BggAttribution />
    </footer>
  </div>
</template>

<style scoped>
.j-public-footer {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}
</style>
