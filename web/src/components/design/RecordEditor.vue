<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { games, players, rounds, today, demoSession, gameById, modeNames } from '@/stores/designPreview'
import type { Mode, Outcome, Round, Team } from '@/types/design'
import Icon from '@/components/common/AppIcon.vue'
import GameCover from './GameCover.vue'
import PlayerAvatar from './PlayerAvatar.vue'
import Modal from '@/components/common/AppModal.vue'

const props = defineProps<{ editing?: Round; initialGame?: string }>()
const game = ref(props.editing?.game ?? props.initialGame ?? '')
const date = ref(props.editing?.date ?? today())
const selected = ref<string[]>([...(props.editing?.players ?? [])])
const mode = ref<Mode>(props.editing?.mode ?? 'individual')
const outcome = ref<Outcome | ''>(props.editing?.outcome ?? '')
const winners = ref<string[]>([...(props.editing?.winners ?? [])])
const scores = ref<Record<string, string>>({ ...props.editing?.scores })
const memory = ref(props.editing?.memory ?? '')
const minutes = ref(props.editing?.minutes ?? '')
const location = ref(props.editing?.location ?? '')
const photos = ref<string[]>([...(props.editing?.photos ?? [])])
const more = ref(!!props.editing)
const teamNames = ref(props.editing?.teams.length ? props.editing.teams.map((team) => team.name) : ['A 队', 'B 队'])
const assignments = ref<Record<string, string>>({})
const teamScores = ref<Record<string, string>>({})
const winningTeam = ref('')
props.editing?.teams.forEach((team, index) => {
  team.players.forEach((id) => { assignments.value[id] = String(index) })
  teamScores.value[String(index)] = team.score
  if (team.players.some((id) => winners.value.includes(id))) winningTeam.value = String(index)
})
const picker = ref(false)
const search = ref('')
const newGame = ref('')
const playerPicker = ref(false)
const newPlayer = ref('')
const error = ref('')
const pickerError = ref('')
const saved = ref<string | null>(null)
const fileInput = ref<HTMLInputElement>()
const matchedGames = computed(() => games.filter((item) => `${item.name} ${item.english}`.toLowerCase().includes(search.value.toLowerCase())))
const modes: Mode[] = ['individual', 'team', 'coop']

function togglePlayer(id: string) {
  selected.value = selected.value.includes(id) ? selected.value.filter((value) => value !== id) : [...selected.value, id]
  winners.value = winners.value.filter((value) => selected.value.includes(value))
}
function setMode(value: Mode) { mode.value = value; outcome.value = ''; winners.value = []; winningTeam.value = ''; error.value = '' }
function chooseOutcome(value: Outcome) { outcome.value = value; winners.value = []; winningTeam.value = '' }
function toggleWinner(id: string) { winners.value = winners.value.includes(id) ? winners.value.filter((value) => value !== id) : [...winners.value, id] }
function addGame() {
  if (!newGame.value.trim()) return
  const existing = games.find((item) => item.name === newGame.value.trim())
  if (existing) { game.value = existing.id; picker.value = false; return }
  const id = crypto.randomUUID()
  games.push({ id, name: newGame.value.trim(), english: 'OUR GAME', theme: 'sun', motif: '✳', caption: '新的游戏，新的故事。' })
  game.value = id; picker.value = false; newGame.value = ''
}
function addPlayer() {
  const name = newPlayer.value.trim()
  if (!name) return
  if (players.some((player) => player.name === name)) { pickerError.value = '已有同名玩家，请选择已有档案或换一个昵称'; return }
  const id = crypto.randomUUID()
  players.push({ id, name, color: '#d4c9ad', linked: false })
  selected.value.push(id); newPlayer.value = ''; pickerError.value = ''; playerPicker.value = false
}
function addPhotos(event: Event) {
  const input = event.target as HTMLInputElement
  for (const file of Array.from(input.files ?? [])) {
    if (photos.value.length >= 6) { error.value = '最多添加 6 张照片'; break }
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type) || file.size > 10 * 1024 * 1024) { error.value = '请选择不超过 10 MB 的 JPEG、PNG 或 WebP 图片'; continue }
    photos.value.push(URL.createObjectURL(file))
  }
  input.value = ''
}
function save() {
  error.value = ''
  if (!game.value) { error.value = '先选一款本局玩的桌游吧'; return }
  if (selected.value.length < (mode.value === 'individual' ? 2 : 1)) { error.value = mode.value === 'individual' ? '个人竞技至少选择 2 位玩家' : '至少选择 1 位玩家'; return }
  if (!date.value) { error.value = '请填写对局日期'; return }
  if (!outcome.value) { error.value = '请选择本局结果，也可以选择“未记结果”'; return }
  if (mode.value === 'individual' && outcome.value === 'win' && !winners.value.length) { error.value = '请点选获胜玩家，支持共同获胜'; return }
  if (Array.from(memory.value).length > 500) { error.value = '回忆最多 500 字'; return }
  if (minutes.value && (!/^\d+$/.test(minutes.value) || Number(minutes.value) <= 0)) { error.value = '时长请填写正整数分钟'; return }
  let teams: Team[] = []
  let winnerIds = [...winners.value]
  if (mode.value === 'team') {
    teams = teamNames.value.map((name, index) => ({ id: String(index), name, players: selected.value.filter((id) => assignments.value[id] === String(index)), score: teamScores.value[String(index)] ?? '' }))
    if (teams.some((team) => !team.players.length) || selected.value.some((id) => assignments.value[id] === undefined || assignments.value[id] === '')) { error.value = '请将每位玩家分入队伍，每队至少 1 人'; return }
    if (outcome.value === 'win' && !winningTeam.value) { error.value = '请选择获胜队伍'; return }
    winnerIds = outcome.value === 'win' ? teams.find((team) => team.id === winningTeam.value)?.players ?? [] : []
  }
  const id = props.editing?.id ?? crypto.randomUUID()
  const record: Round = { id, game: game.value, date: date.value, mode: mode.value, outcome: outcome.value, players: [...selected.value], winners: winnerIds, scores: { ...scores.value }, teams, memory: memory.value.trim(), photos: [...photos.value], location: location.value.trim(), minutes: minutes.value, author: props.editing?.author ?? 'lin', edited: !!props.editing }
  const index = rounds.findIndex((item) => item.id === id)
  if (index >= 0) rounds[index] = record
  else rounds.unshift(record)
  saved.value = id
}
function anotherRound() {
  outcome.value = ''; winners.value = []; scores.value = {}; memory.value = ''; photos.value = []; minutes.value = ''; location.value = ''
  teamScores.value = {}; winningTeam.value = ''; saved.value = null; more.value = false; date.value = today()
}
</script>

<template>
  <div v-if="saved" class="d-success d-surface">
    <div class="d-success-mark"><Icon name="check" :size="34" /></div><p class="d-eyebrow">ANOTHER GOOD MEMORY</p><h1>{{ editing ? '修改，已经记好了。' : '这一局，记下了。' }}</h1>
    <p>{{ gameById(game).name }} · {{ selected.length }} 位朋友<br />现在可以在回顾里找到这段好时光。</p>
    <small>演示记录仅在本次预览中保留，刷新后重置。</small>
    <RouterLink :to="`/design/rounds/${saved}`" class="d-button">查看记录<Icon name="arrow" :size="17" /></RouterLink>
    <button v-if="!editing" class="d-button secondary" @click="anotherRound">再记一局</button>
    <RouterLink to="/design" class="d-text-link">回到回顾</RouterLink>
  </div>
  <form v-else class="d-editor" @submit.prevent="save">
    <header class="d-page-heading compact"><p class="d-eyebrow">A LITTLE NOTE, A GOOD MEMORY</p><h1>{{ editing ? '再补充一点回忆。' : '刚刚，又玩了一局。' }}</h1><p>记下结果，也留住这一刻。</p></header>
    <div class="d-form-context"><span><Icon name="group" :size="16" />{{ demoSession.group }}</span><label><Icon name="calendar" :size="16" /><input v-model="date" type="date" aria-label="对局日期" required /></label></div>
    <section class="d-form-section"><div class="d-section-title"><h2><em>01</em>玩了什么</h2><span>必选</span></div>
      <button class="d-game-selector" type="button" @click="picker = true"><GameCover v-if="game" :game="gameById(game)" small /><span v-else class="d-add-icon"><Icon name="game" :size="27" /></span><span><strong>{{ game ? gameById(game).name : '选择一款桌游' }}</strong><small>{{ game ? '点击更换游戏' : '最近玩过的，或者认识点新的' }}</small></span><Icon name="down" /></button>
      <div v-if="!game" class="d-recent-games"><span>最近玩过</span><button v-for="item in games.slice(0, 3)" :key="item.id" type="button" @click="game = item.id">{{ item.name }}</button></div>
    </section>
    <section class="d-form-section"><div class="d-section-title"><h2><em>02</em>和谁一起</h2><span>已选 {{ selected.length }} 人</span></div>
      <div class="d-player-options"><button v-for="player in players" :key="player.id" type="button" :class="{ selected: selected.includes(player.id) }" :aria-pressed="selected.includes(player.id)" @click="togglePlayer(player.id)"><PlayerAvatar :id="player.id" /><span>{{ player.name }}{{ player.id === 'lin' ? '（我）' : '' }}</span><span class="d-selection-check"><Icon v-if="selected.includes(player.id)" name="check" :size="12" /></span></button><button type="button" class="d-add-player" @click="playerPicker = true"><Icon name="plus" />添加玩家</button></div>
      <button type="button" class="d-text-link" @click="selected = ['lin', 'zhou', 'keke', 'yuan']">还是上次这 4 位朋友</button>
    </section>
    <section class="d-form-section"><div class="d-section-title"><h2><em>03</em>结果怎么样</h2><span>分数可以不填</span></div>
      <div class="d-segment" aria-label="对局模式"><button v-for="item in modes" :key="item" type="button" :class="{ active: mode === item }" :aria-pressed="mode === item" @click="setMode(item)">{{ modeNames[item] }}</button></div>
      <template v-if="mode === 'team'">
        <p class="d-field-hint">先分队，每位玩家只属于一支队伍。</p>
        <div v-for="id in selected" :key="id" class="d-assignment"><PlayerAvatar :id="id" small /><span>{{ players.find((p) => p.id === id)?.name }}</span><select v-model="assignments[id]" :aria-label="`${players.find((p) => p.id === id)?.name}的队伍`"><option value="">选择队伍</option><option v-for="(name, i) in teamNames" :key="i" :value="String(i)">{{ name }}</option></select></div>
        <button type="button" class="d-text-link" @click="teamNames.push(`${String.fromCharCode(65 + teamNames.length)} 队`)">＋ 添加队伍</button>
      </template>
      <div class="d-outcome-options">
        <button type="button" :class="{ selected: outcome === 'win' }" @click="chooseOutcome('win')"><Icon name="cup" :size="17" />{{ mode === 'coop' ? '全队胜利' : mode === 'team' ? '选择获胜队' : '选择赢家' }}</button>
        <button v-if="mode === 'individual'" type="button" :class="{ selected: outcome === 'draw' }" @click="chooseOutcome('draw')">全局平局</button>
        <button v-if="mode === 'coop'" type="button" :class="{ selected: outcome === 'loss' }" @click="chooseOutcome('loss')">全队失败</button>
        <button type="button" :class="{ selected: outcome === 'unknown' }" @click="chooseOutcome('unknown')">未记结果</button>
      </div>
      <div v-if="mode === 'individual' && selected.length" class="d-score-list"><div class="d-score-labels"><span>{{ outcome === 'win' ? '点选赢家，可多人共同获胜' : '本局玩家' }}</span><span>分数（选填）</span></div><div v-for="id in selected" :key="id" class="d-score-row"><button type="button" :disabled="outcome !== 'win'" :class="{ winner: winners.includes(id) }" @click="toggleWinner(id)"><PlayerAvatar :id="id" small /><span>{{ players.find((p) => p.id === id)?.name }}</span><Icon v-if="winners.includes(id)" name="cup" :size="17" /></button><input :value="scores[id] ?? ''" type="number" step="any" placeholder="—" :aria-label="`${players.find((p) => p.id === id)?.name}的分数`" @input="scores[id] = ($event.target as HTMLInputElement).value" /></div></div>
      <div v-if="mode === 'team'" class="d-score-list"><div v-for="(name, i) in teamNames" :key="i" class="d-score-row"><button type="button" :disabled="outcome !== 'win'" :class="{ winner: winningTeam === String(i) }" @click="winningTeam = String(i)">{{ name }}<Icon v-if="winningTeam === String(i)" name="cup" :size="16" /></button><input :value="teamScores[String(i)] ?? ''" type="number" step="any" placeholder="队伍分数" :aria-label="`${name}分数`" @input="teamScores[String(i)] = ($event.target as HTMLInputElement).value" /></div></div>
      <label v-if="mode === 'coop'" class="d-field">团队分数（选填）<input :value="scores.team ?? ''" type="number" step="any" placeholder="空白不等于零" @input="scores.team = ($event.target as HTMLInputElement).value" /></label>
      <p class="d-field-hint">分数不决定输赢，结果由你确认。</p>
    </section>
    <section class="d-form-section d-extra-section"><button type="button" class="d-expand" :aria-expanded="more" @click="more = !more"><span><Icon name="heart" />留点回忆<small>照片、感想和更多</small></span><Icon :name="more ? 'close' : 'plus'" /></button>
      <div v-if="more" class="d-extra-fields"><label class="d-field">这一局，有什么值得记住？<textarea v-model="memory" rows="3" placeholder="比如：最后一张牌翻盘，大家都不服…"></textarea><small>{{ Array.from(memory).length }} / 500</small></label>
        <div class="d-upload-grid"><div v-for="(photo, i) in photos" :key="photo" class="d-upload-preview"><img :src="photo" alt="本地演示照片" /><button type="button" :aria-label="`移除第 ${i + 1} 张照片`" @click="photos.splice(i, 1)"><Icon name="close" :size="14" /></button></div><button v-if="photos.length < 6" type="button" class="d-upload-button" @click="fileInput?.click()"><Icon name="photo" :size="25" />添加照片</button></div>
        <input ref="fileInput" class="d-sr-only" type="file" accept="image/jpeg,image/png,image/webp" multiple aria-label="选择演示照片" @change="addPhotos" /><p class="d-field-hint">{{ photos.length }} / 6 张 · 每张不超过 10 MB · 仅本机预览，不上传</p>
        <div class="d-field-pair"><label class="d-field">玩了多久<input v-model="minutes" inputmode="numeric" placeholder="分钟，选填" /></label><label class="d-field">在哪里玩<input v-model="location" placeholder="地点，选填" /></label></div>
      </div>
    </section>
    <div class="d-save-bar"><p v-if="error" class="d-form-error" role="alert">{{ error }}</p><div><span><Icon name="group" :size="15" />保存到「{{ demoSession.group }}」<small>产品稿 · 仅演示保存</small></span><button class="d-button" type="submit"><Icon name="check" :size="18" />{{ editing ? '保存修改' : '保存这局' }}</button></div></div>
  </form>
  <Modal v-if="picker" title="今天玩了什么？" @close="picker = false"><label class="d-search"><Icon name="search" /><input v-model="search" placeholder="搜索桌游或中文别名" aria-label="搜索桌游" /></label><p class="d-field-hint">本组桌游</p><div class="d-picker-list"><button v-for="item in matchedGames" :key="item.id" @click="game = item.id; picker = false"><GameCover :game="item" small /><span><strong>{{ item.name }}</strong><small>{{ item.english }}</small></span><Icon v-if="game === item.id" name="check" /></button><p v-if="!matchedGames.length" class="d-field-hint">没有找到，直接记下游戏名称也可以。</p></div><div class="d-inline-form"><input v-model="newGame" aria-label="手动添加游戏名称" placeholder="手动添加：输入桌游名称" @keyup.enter="addGame" /><button class="d-button" @click="addGame">添加</button></div><p class="d-note">外部资料搜索在正式接入后开放。本稿展示本组搜索与手动添加。</p></Modal>
  <Modal v-if="playerPicker" title="还有哪位朋友？" @close="playerPicker = false"><p class="d-modal-copy">只要一个昵称，就能加入本局。朋友不用现在注册。</p><label class="d-field">玩家昵称<input v-model="newPlayer" placeholder="大家平时怎么称呼 TA？" @keyup.enter="addPlayer" /></label><p v-if="pickerError" role="alert" class="d-form-error">{{ pickerError }}</p><button class="d-button full" @click="addPlayer">添加并选中</button></Modal>
</template>
