<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { rounds, dateLabel, gameById, modeNames, playerName, resultLabel } from '@/stores/designPreview'
import { useDesignPreview } from '@/composables/useDesignPreview'
import Icon from '@/components/common/AppIcon.vue'
import Modal from '@/components/common/AppModal.vue'
import Avatar from '@/components/design/PlayerAvatar.vue'
const route = useRoute()
const itemId = computed(() => String(route.params.id))
const currentRound = computed(() => rounds.find((r) => r.id === itemId.value))
const fullPhoto = ref('')
function deleteRound() { const i = rounds.findIndex((r) => r.id === itemId.value); if (i >= 0) rounds.splice(i, 1); modal.value = ''; void router.push('/design'); notify('演示记录已删除，统计同步更新') }
const modal = ref('')
const router = useRouter()
const { notify } = useDesignPreview()
</script>

<template>
<template v-if="currentRound">
          <div class="d-detail-heading"><div><p class="d-eyebrow">{{ dateLabel(currentRound.date) }}</p><RouterLink :to="`/design/games/${currentRound.game}`"><h1>{{ gameById(currentRound.game).name }}<Icon name="arrow" :size="25" /></h1></RouterLink><p>{{ modeNames[currentRound.mode] }}<i>·</i>{{ currentRound.players.length }} 位朋友的共同回忆</p></div><div class="d-detail-actions"><RouterLink :to="`/design/edit/${currentRound.id}`" class="d-button secondary"><Icon name="edit" :size="16" />编辑</RouterLink><button class="d-icon-button" aria-label="删除这条演示记录" @click="modal = 'delete'"><Icon name="more" /></button></div></div>
          <div class="d-detail-columns"><div><section class="d-detail-result d-surface"><div class="d-result-heading"><span class="d-result-emblem"><Icon :name="currentRound.outcome === 'win' ? 'cup' : 'heart'" :size="30" /></span><div><small>这一局的结果</small><h2>{{ resultLabel(currentRound) }}</h2></div></div><div v-if="currentRound.mode === 'team'" class="d-team-results"><div v-for="team in currentRound.teams" :key="team.id"><strong>{{ team.name }} {{ team.players.some((id) => currentRound?.winners.includes(id)) ? '· 获胜' : '' }}</strong><span>{{ team.players.map(playerName).join('、') }}</span><small>队伍分数：{{ team.score || '未填写' }}</small></div></div><div v-else-if="currentRound.mode === 'coop'" class="d-note">团队分数：{{ currentRound.scores.team || '未填写' }}</div><div v-else class="d-detail-players"><RouterLink v-for="id in currentRound.players" :key="id" :to="`/design/players/${id}`" :class="{ winner: currentRound?.winners.includes(id) }"><Avatar :id="id" /><strong>{{ playerName(id) }}</strong><span>{{ currentRound.scores[id] === undefined || currentRound.scores[id] === '' ? '未填分数' : `${currentRound.scores[id]} 分` }}</span><Icon v-if="currentRound?.winners.includes(id)" name="cup" :size="15" /></RouterLink></div></section>
            <section class="d-memory-detail d-surface"><div class="d-list-heading"><h2><Icon name="heart" :size="18" />这一局，还记得</h2></div><blockquote>{{ currentRound.memory || '有些好时光，还没来得及写下来。' }}</blockquote><div class="d-detail-photos"><button v-for="photo in currentRound.photos" :key="photo" aria-label="查看回忆大图" @click="fullPhoto = photo; modal = 'photo'"><img :src="photo" alt="本局回忆示意照片" /></button></div><p v-if="currentRound.photos.includes('/design-assets/table-memory.png')" class="d-photo-caption">示意照片 · 为产品稿生成，非真实聚会记录</p></section></div>
            <aside class="d-detail-side d-surface"><h2>关于这次相聚</h2><div><Icon name="calendar" /><span><small>什么时候</small>{{ dateLabel(currentRound.date) }}</span></div><div><Icon name="clock" /><span><small>玩了多久</small>{{ currentRound.minutes ? `${currentRound.minutes} 分钟` : '未填写' }}</span></div><div><Icon name="pin" /><span><small>在哪里玩</small>{{ currentRound.location || '未填写' }}</span></div><div><Avatar :id="currentRound.author" small /><span><small>{{ currentRound.edited ? '最近由阿林修改 · 本次预览' : '谁记下了这一局' }}</small>{{ playerName(currentRound.author) }}</span></div><RouterLink :to="`/design/new?game=${currentRound.game}`" class="d-button secondary full">再玩一次<Icon name="arrow" :size="17" /></RouterLink></aside></div>
<Modal v-if="modal === 'delete'" title="删除这段对局记录？" @close="modal = ''"><p class="d-modal-copy">这条演示记录会从小组回顾中移除，相关统计同步更新。</p><div class="d-modal-actions"><button class="d-button secondary" @click="modal = ''">再想想</button><button class="d-button" @click="deleteRound">删除这局</button></div></Modal><Modal v-if="modal === 'photo'" title="那天的桌边时光" wide @close="modal = ''"><img class="d-lightbox" :src="fullPhoto" alt="对局回忆大图" /><p class="d-note">示意素材，仅用于产品稿评审。</p></Modal></template><div v-else class="d-empty">这页回忆不存在。</div>
</template>
