<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import Modal from '@/components/common/AppModal.vue'
import PlayerChip from '@/components/common/PlayerChip.vue'
import RequestStatus from '@/components/common/RequestStatus.vue'
import { useGroupContext } from '@/composables/useGroupContext'
import { useGroupOperation } from '@/composables/useGroupOperation'
import { useSession } from '@/stores/session'
import { useRound } from '@/composables/useRound'
import { createRoundShare, deleteRound as remove, getRoundShareStatus, revokeRoundShare } from '@/api/round'
import { photoURL } from '@/api/photo'
import { modeNames, resultLabel } from '@/utils/round'
import { message } from '@/utils/error'

const route = useRoute()
const router = useRouter()
const { user } = useSession()
const { groupId, snapshot, owner, playerName, gameName, memberName } = useGroupContext()
const { current, loading, error: loadError, load } = useRound(String(route.params.id))
const { busy, error, run } = useGroupOperation()
const editable = computed(() => owner.value || current.value?.author === user.value?.id)
const modal = ref<'' | 'delete' | 'photo' | 'share'>('')
const fullPhoto = ref('')
const shareActive = ref(false)
const shareURL = ref('')
const shareError = ref('')
const copied = ref(false)
const shareBusy = ref(false)

function openPhoto(id: string) {
  fullPhoto.value = id
  modal.value = 'photo'
}

function deleteRound() {
  if (!current.value || !editable.value) return
  const round = current.value
  return run(async () => {
    await remove(groupId, round.id, round.version)
    modal.value = ''
    await router.push('/')
  })
}

async function openShare() {
  if (!current.value || !editable.value) return
  modal.value = 'share'
  shareURL.value = ''
  shareError.value = ''
  copied.value = false
  shareBusy.value = true
  try {
    shareActive.value = (await getRoundShareStatus(groupId, current.value.id)).active
  } catch (cause) {
    shareError.value = message(cause)
  } finally {
    shareBusy.value = false
  }
}

async function generateShare() {
  if (!current.value || shareBusy.value) return
  shareBusy.value = true
  shareError.value = ''
  copied.value = false
  try {
    const share = await createRoundShare(groupId, current.value.id)
    shareURL.value = `${window.location.origin}/share#${share.token}`
    shareActive.value = true
  } catch (cause) {
    shareError.value = message(cause)
  } finally {
    shareBusy.value = false
  }
}

async function copyShare() {
  try {
    await navigator.clipboard.writeText(shareURL.value)
    copied.value = true
  } catch {
    shareError.value = '复制失败，请长按链接手动复制'
  }
}

async function revokeShare() {
  if (!current.value || shareBusy.value) return
  shareBusy.value = true
  shareError.value = ''
  try {
    await revokeRoundShare(groupId, current.value.id)
    shareActive.value = false
    shareURL.value = ''
    copied.value = false
  } catch (cause) {
    shareError.value = message(cause)
  } finally {
    shareBusy.value = false
  }
}
</script>

<template>
  <RequestStatus :loading="loading" :error="loadError || error" @retry="load" />
  <template v-if="snapshot && current">
    <header class="d-page-heading compact">
      <p class="d-eyebrow">A DAY TO REMEMBER</p>
      <h1>{{ gameName(current.game_id) }}</h1>
      <p>{{ current.date }} · {{ modeNames[current.mode] }}</p>
    </header>
    <section class="d-surface j-detail">
      <h2 class="d-result">{{ resultLabel(current, snapshot.players) }}</h2>
      <div class="j-players">
        <RouterLink v-for="id in current.players" :key="id" :to="`/players/${id}`" class="j-player">
          <PlayerChip :name="playerName(id)"
            ><span v-if="current.scores?.[id] != null">{{ current.scores[id] }} 分</span></PlayerChip
          >
        </RouterLink>
      </div>
      <div v-for="team in current.teams" :key="team.id" class="j-team">
        <strong>{{ team.name }} {{ team.winner ? '♔ 获胜' : '' }}</strong>
        <p>
          {{ team.players.map(playerName).join('、') }}<span v-if="team.score != null"> · {{ team.score }} 分</span>
        </p>
      </div>
      <p v-if="current.team_score != null">团队分数：{{ current.team_score }}</p>
      <blockquote v-if="current.memory">{{ current.memory }}</blockquote>
      <div class="j-photos">
        <button v-for="id in current.photos" :key="id" @click="openPhoto(id)">
          <img :src="photoURL(groupId, id)" alt="放大查看对局照片" />
        </button>
      </div>
      <p v-if="current.location || current.minutes">
        {{ current.location }} {{ current.minutes ? `· ${current.minutes} 分钟` : '' }}
      </p>
      <p class="d-note">
        记录人：{{ memberName(current.author) }}<br />最近修改：{{ memberName(current.updated_by) }} ·
        {{ new Date(current.updated_at).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' }) }}（北京时间）
      </p>
      <RouterLink :to="`/games/${current.game_id}`" class="d-text-link">查看这款桌游的故事 →</RouterLink>
      <div v-if="editable" class="j-actions">
        <button class="d-button" @click="openShare">分享这局</button>
        <RouterLink :to="`/edit/${current.id}`" class="d-button secondary">编辑记录</RouterLink>
        <button class="d-text-link" @click="modal = 'delete'">删除这局</button>
      </div>
    </section>
    <Modal v-if="modal === 'delete'" title="删除这段对局记录？" @close="modal = ''">
      <p>删除后会移入回收站，7 天内可以恢复；时间线、统计和公开分享会立即失效。</p>
      <p v-if="error" class="j-error">{{ error }}</p>
      <div class="d-modal-actions">
        <button class="d-button secondary" @click="modal = ''">再想想</button
        ><button class="d-button" :disabled="busy" @click="deleteRound">删除这局</button>
      </div>
    </Modal>
    <Modal v-if="modal === 'share'" title="把这一局分享出去" @close="modal = ''">
      <p>
        获得链接的人无需登录，可以看到小组名、游戏、日期、玩家昵称、结果、回忆和照片。不会显示账号、记录人或其他组内内容。
      </p>
      <p v-if="shareActive && !shareURL" class="j-notice">当前已有公开链接。重新生成后，旧链接会立即失效。</p>
      <button class="d-button full" :disabled="shareBusy" @click="generateShare">
        {{ shareActive ? '重新生成分享链接' : '生成分享链接' }}
      </button>
      <label v-if="shareURL" class="d-field"
        >分享链接<input :value="shareURL" readonly @focus="($event.target as HTMLInputElement).select()"
      /></label>
      <button v-if="shareURL" class="d-button secondary full" @click="copyShare">
        {{ copied ? '已复制' : '复制链接' }}
      </button>
      <button v-if="shareActive" class="d-text-link j-share-revoke" :disabled="shareBusy" @click="revokeShare">
        撤销公开链接
      </button>
      <p v-if="shareError" class="j-error">{{ shareError }}</p>
    </Modal>
    <Modal v-if="modal === 'photo'" title="那天的桌边时光" wide @close="modal = ''"
      ><img class="d-lightbox" :src="photoURL(groupId, fullPhoto)" alt="对局回忆大图"
    /></Modal>
  </template>
</template>
