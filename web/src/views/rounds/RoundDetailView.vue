<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import Modal from '@/components/common/AppModal.vue'
import PlayerChip from '@/components/common/PlayerChip.vue'
import RequestStatus from '@/components/common/RequestStatus.vue'
import { useGroupContext } from '@/composables/useGroupContext'
import { useGroupOperation } from '@/composables/useGroupOperation'

import { useRoute, useRouter } from 'vue-router'
import { useSession } from '@/stores/session'
import { useRound } from '@/composables/useRound'
import { deleteRound as remove } from '@/api/round'
import { photoURL } from '@/api/photo'
import { modeNames, resultLabel } from '@/utils/round'
const route = useRoute(); const router = useRouter()
const { user } = useSession()
const { groupId, snapshot, owner, playerName, gameName, memberName } = useGroupContext()
const { current, loading, error: loadError, load } = useRound(String(route.params.id))
const { busy, error, run } = useGroupOperation()
const editable = computed(() => owner.value || current.value?.author === user.value?.id)
const modal = ref<'' | 'delete' | 'photo'>(''); const fullPhoto = ref('')
function deleteRound() { if (!current.value || !editable.value) return; const round = current.value; return run(async () => { await remove(groupId, round.id, round.version); modal.value = ''; await router.push('/') }) }
</script>

<template>
<RequestStatus :loading="loading" :error="loadError || error" @retry="load" /><template v-if="snapshot && current"><header class="d-page-heading compact"><p class="d-eyebrow">A DAY TO REMEMBER</p><h1>{{ gameName(current.game_id) }}</h1><p>{{ current.date }} · {{ modeNames[current.mode] }}</p></header><section class="d-surface j-detail"><h2 class="d-result">{{ resultLabel(current, snapshot.players) }}</h2><div class="j-players"><RouterLink v-for="id in current.players" :key="id" :to="`/players/${id}`" class="j-player"><PlayerChip :name="playerName(id)"><span v-if="current.scores?.[id] != null">{{ current.scores[id] }} 分</span></PlayerChip></RouterLink></div><div v-for="t in current.teams" :key="t.id" class="j-team"><strong>{{ t.name }} {{ t.winner ? '♔ 获胜' : '' }}</strong><p>{{ t.players.map(playerName).join('、') }}<span v-if="t.score != null"> · {{ t.score }} 分</span></p></div><p v-if="current.team_score != null">团队分数：{{ current.team_score }}</p><blockquote v-if="current.memory">{{ current.memory }}</blockquote><div class="j-photos"><button v-for="id in current.photos" :key="id" @click="fullPhoto = id; modal = 'photo'"><img :src="photoURL(groupId, id)" alt="放大查看对局照片" /></button></div><p v-if="current.location || current.minutes">{{ current.location }} {{ current.minutes ? `· ${current.minutes} 分钟` : '' }}</p><p class="d-note">记录人：{{ memberName(current.author) }}<br />最近修改：{{ memberName(current.updated_by) }} · {{ new Date(current.updated_at).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' }) }}（北京时间）</p><RouterLink :to="`/games/${current.game_id}`" class="d-text-link">查看这款桌游的故事 →</RouterLink><div v-if="editable" class="j-actions"><RouterLink :to="`/edit/${current.id}`" class="d-button secondary">编辑记录</RouterLink><button class="d-text-link" @click="modal = 'delete'">删除这局</button></div></section><Modal v-if="modal === 'delete'" title="删除这段对局记录？" @close="modal = ''"><p>删除后会移入回收站，7 天内可以恢复；时间线和统计会立即更新。</p><p v-if="error" class="j-error">{{ error }}</p><div class="d-modal-actions"><button class="d-button secondary" @click="modal = ''">再想想</button><button class="d-button" :disabled="busy" @click="deleteRound">删除这局</button></div></Modal>
<Modal v-if="modal === 'photo'" title="那天的桌边时光" wide @close="modal = ''"><img class="d-lightbox" :src="photoURL(groupId, fullPhoto)" alt="对局回忆大图" /></Modal></template>
</template>
