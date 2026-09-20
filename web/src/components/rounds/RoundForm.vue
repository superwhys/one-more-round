<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { RouterLink } from 'vue-router'
import PlayerChip from '@/components/common/PlayerChip.vue'
import { message } from '@/utils/error'
import { emptyRound, modeNames } from '@/utils/round'
import { today } from '@/utils/date'
import type { Snapshot, User, Round, Mode, Game, Player } from '@/types/journal'
import { photoURL } from '@/api/photo'
const newID = () => crypto.randomUUID()
const maxPhotos = 3
const maxPhotoBytes = 2 * 1024 * 1024
const props = defineProps<{
  snapshot: Snapshot
  user: User
  editing?: Round
  initialGame?: string
  saving: boolean
  error: string
  saved: Round | null
  addItem: (kind: 'games' | 'players', name: string) => Promise<Game | Player>
  uploadPhoto: (file: File) => Promise<{ id: string }>
}>()
const emit = defineEmits<{ submit: [round: Round, key: string]; again: [] }>()
const draftKey = `omr:draft:${props.user.id}:${props.snapshot.group.id}:${props.editing?.id ?? 'new'}`
const form = ref<Round>(
  structuredClone(props.editing ? (JSON.parse(JSON.stringify(props.editing)) as Round) : emptyRound()),
)
form.value.game_id ||= props.initialGame ?? ''
const key = ref<string>(crypto.randomUUID())
const localError = ref('')
const notice = ref('')
const restored = ref(false)
const more = ref(!!props.editing)
const playerName = ref('')
const gameSearch = ref('')
const gamePickerOpen = ref(false)
const gameInput = ref<HTMLInputElement>()
const adding = ref(false)
interface Upload {
  file: File
  id: string
  state: 'uploading' | 'failed' | 'done'
  error: string
  progress: number
}
const uploads = ref<Upload[]>([])
const loadingPhotos = computed(() => uploads.value.some(p => p.state === 'uploading'))
const matchingGames = computed(() => {
  const query = gameSearch.value.trim().toLowerCase()
  return props.snapshot.games.filter(g => `${g.name} ${g.original}`.toLowerCase().includes(query))
})
try {
  const raw = localStorage.getItem(draftKey)
  if (raw) {
    const d = JSON.parse(raw) as { form: Round; key: string }
    if (d.form && Array.isArray(d.form.players) && typeof d.key === 'string') {
      form.value = d.form
      key.value = d.key
      restored.value = true
    }
  }
} catch {
  notice.value = '草稿无法读取，请重新填写'
}
const selfPlayer = props.snapshot.players.find(player => player.account === props.user.id)
if (!props.editing && !restored.value && selfPlayer && !form.value.players.length) form.value.players = [selfPlayer.id]
gameSearch.value = props.snapshot.games.find(game => game.id === form.value.game_id)?.name ?? ''
watch(
  [form, key],
  () => {
    if (props.saved) return
    try {
      localStorage.setItem(draftKey, JSON.stringify({ form: form.value, key: key.value }))
    } catch {
      notice.value = '此浏览器无法保存草稿，请保持页面打开'
    }
  },
  { deep: true },
)
function changeMode(mode: Mode) {
  form.value.mode = mode
  form.value.outcome = ''
  form.value.winners = []
  form.value.scores = {}
  form.value.team_score = null
  form.value.teams =
    mode === 'team'
      ? [
          { id: crypto.randomUUID(), name: 'A 队', players: [], winner: false, score: null },
          { id: crypto.randomUUID(), name: 'B 队', players: [], winner: false, score: null },
        ]
      : []
}
function toggle(id: string) {
  const r = form.value
  r.players = r.players.includes(id) ? r.players.filter(p => p !== id) : [...r.players, id]
  r.winners = r.winners.filter(p => r.players.includes(p))
  for (const p of Object.keys(r.scores)) if (!r.players.includes(p)) delete r.scores[p]
  for (const t of r.teams) t.players = t.players.filter(p => r.players.includes(p))
}
function assign(player: string, event: Event) {
  const id = (event.target as HTMLSelectElement).value
  for (const t of form.value.teams) t.players = t.players.filter(p => p !== player)
  form.value.teams.find(t => t.id === id)?.players.push(player)
}
function outcomeChanged() {
  form.value.winners = []
  for (const t of form.value.teams) t.winner = false
}
function searchGame(event: Event) {
  gameSearch.value = (event.target as HTMLInputElement).value
  form.value.game_id = ''
  gamePickerOpen.value = true
  localError.value = ''
}
function chooseGame(game: Game) {
  form.value.game_id = game.id
  gameSearch.value = game.name
  gamePickerOpen.value = false
  localError.value = ''
}
function closeGamePicker(event: FocusEvent) {
  if (!(event.currentTarget as HTMLElement).contains(event.relatedTarget as Node | null)) gamePickerOpen.value = false
}
async function add(kind: 'games' | 'players') {
  if (adding.value) return
  const name = kind === 'games' ? gameSearch.value.trim() : playerName.value
  if (!name.trim()) return
  adding.value = true
  localError.value = ''
  try {
    const item = await props.addItem(kind, name)
    if (kind === 'games') {
      form.value.game_id = item.id
      gameSearch.value = item.name
      gamePickerOpen.value = false
    } else {
      form.value.players.push(item.id)
      playerName.value = ''
    }
  } catch (e) {
    localError.value = message(e)
  } finally {
    adding.value = false
  }
}
function normalized(): Round {
  const r = JSON.parse(JSON.stringify(form.value)) as Round
  r.memory = r.memory.trim()
  r.location = r.location.trim()
  r.minutes = r.minutes === null || String(r.minutes) === '' ? null : Number(r.minutes)
  for (const p of Object.keys(r.scores)) r.scores[p] = r.scores[p]?.trim() || null
  r.team_score = r.team_score?.trim() || null
  for (const t of r.teams) t.score = t.score?.trim() || null
  return r
}
function save() {
  if (props.saving || loadingPhotos.value || adding.value) return
  localError.value = ''
  if (form.value.photos.length > maxPhotos) {
    localError.value = `最多 ${maxPhotos} 张照片，请先移除多余照片`
    more.value = true
    return
  }
  if (!form.value.game_id) {
    localError.value = '请从搜索结果中选择桌游，或手动添加后再保存'
    gamePickerOpen.value = true
    gameInput.value?.focus()
    return
  }
  emit('submit', normalized(), key.value)
}
watch(
  () => props.saved,
  value => {
    if (value) {
      try {
        localStorage.removeItem(draftKey)
      } catch {
        /* no persistent draft */
      }
    }
  },
  { flush: 'sync' },
)
function again() {
  const old = form.value
  form.value = { ...emptyRound(), game_id: old.game_id, players: [...old.players], date: today() }
  key.value = crypto.randomUUID()
  uploads.value = []
  emit('again')
  more.value = false
  restored.value = false
}
function discardDraft() {
  localStorage.removeItem(draftKey)
  form.value = props.editing ? (JSON.parse(JSON.stringify(props.editing)) as Round) : emptyRound()
  form.value.game_id ||= props.initialGame ?? ''
  if (!props.editing && selfPlayer) form.value.players = [selfPlayer.id]
  gameSearch.value = props.snapshot.games.find(game => game.id === form.value.game_id)?.name ?? ''
  key.value = crypto.randomUUID()
  restored.value = false
}
async function upload(item: Upload) {
  item.state = 'uploading'
  item.error = ''
  try {
    const result = await props.uploadPhoto(item.file)
    item.id = result.id
    item.state = 'done'
    item.progress = 100
    form.value.photos.push(item.id)
  } catch (e) {
    item.state = 'failed'
    item.error = message(e)
  }
}
function selectPhotos(event: Event) {
  const input = event.target as HTMLInputElement
  localError.value = ''
  for (const file of Array.from(input.files ?? [])) {
    if (form.value.photos.length + uploads.value.filter(p => p.state !== 'done').length >= maxPhotos) {
      localError.value = `最多 ${maxPhotos} 张照片`
      break
    }
    if (file.size > maxPhotoBytes || !['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      localError.value = '请选择不超过 2 MB 的 JPEG、PNG 或 WebP'
      continue
    }
    const item = ref<Upload>({ file, id: '', state: 'uploading', error: '', progress: 0 })
    uploads.value.push(item.value)
    void upload(item.value)
  }
  input.value = ''
}
</script>
<template>
  <section v-if="saved" class="d-success d-surface">
    <div class="d-success-mark">✓</div>
    <p class="d-eyebrow">ANOTHER GOOD MEMORY</p>
    <h1>这一局，记下了。</h1>
    <p>已保存到小组日记，朋友们也能回顾。</p>
    <RouterLink :to="`/rounds/${saved.id}`" class="d-button">查看记录 →</RouterLink
    ><button v-if="!editing" class="d-button secondary" @click="again">再记一局</button
    ><RouterLink to="/" class="d-text-link">回到回顾</RouterLink>
  </section>
  <form v-else class="d-editor" @submit.prevent="save">
    <header class="d-page-heading compact">
      <p class="d-eyebrow">A LITTLE NOTE, A GOOD MEMORY</p>
      <h1>{{ editing ? '再补充一点回忆。' : '刚刚，又玩了一局。' }}</h1>
      <p>记下结果，也留住这一刻。</p>
    </header>
    <p v-if="restored" class="j-notice">
      已恢复本账号在这个小组的草稿。<button type="button" @click="discardDraft">放弃草稿，重新填写</button>
    </p>
    <p v-if="notice" class="j-notice">{{ notice }}</p>
    <fieldset :disabled="saving" class="j-fieldset">
      <div class="d-form-context">
        <span>{{ snapshot.group.name }}</span
        ><label>对局日期<input v-model="form.date" type="date" required /></label>
      </div>
      <section class="d-form-section">
        <div class="d-section-title">
          <h2><em>01</em>玩了什么</h2>
          <span>必选</span>
        </div>
        <div class="d-field j-game-picker" @focusout="closeGamePicker">
          <label for="round-game">本局桌游</label
          ><input
            id="round-game"
            ref="gameInput"
            :value="gameSearch"
            type="search"
            maxlength="255"
            placeholder="搜索本组桌游或输入名称"
            autocomplete="off"
            role="combobox"
            aria-autocomplete="list"
            aria-controls="round-game-options"
            :aria-expanded="gamePickerOpen"
            :aria-invalid="!form.game_id && !!localError"
            @focus="gamePickerOpen = true"
            @input="searchGame"
          />
          <div v-if="gamePickerOpen" id="round-game-options" class="j-game-options" role="listbox">
            <button
              v-for="game in matchingGames"
              :key="game.id"
              class="j-game-option"
              :class="{ selected: form.game_id === game.id }"
              type="button"
              role="option"
              :aria-selected="form.game_id === game.id"
              @pointerdown.left.prevent="chooseGame(game)"
              @click="chooseGame(game)"
            >
              <span
                ><strong>{{ game.name }}</strong
                ><small v-if="game.original">{{ game.original }}</small></span
              ><span v-if="form.game_id === game.id" aria-hidden="true">✓</span>
            </button>
            <div v-if="!matchingGames.length" class="j-game-empty">
              <p>{{ gameSearch.trim() ? '本组还没有这款桌游' : '输入桌游名称开始搜索' }}</p>
              <button
                v-if="gameSearch.trim()"
                class="d-button secondary"
                type="button"
                :disabled="adding"
                @click="add('games')"
              >
                {{ adding ? '正在添加…' : `＋ 手动添加“${gameSearch.trim()}”` }}
              </button>
            </div>
          </div>
        </div>
      </section>
      <section class="d-form-section">
        <div class="d-section-title">
          <h2><em>02</em>和谁一起</h2>
          <span>已选 {{ form.players.length }} 人</span>
        </div>
        <div class="j-players">
          <button
            v-for="p in snapshot.players"
            :key="p.id"
            class="j-player"
            :class="{ selected: form.players.includes(p.id) }"
            type="button"
            :aria-pressed="form.players.includes(p.id)"
            @click="toggle(p.id)"
          >
            <PlayerChip :name="p.name" :me="p.account === user.id"
              ><span>{{ form.players.includes(p.id) ? '✓' : '＋' }}</span></PlayerChip
            >
          </button>
        </div>
        <div class="j-inline">
          <label class="d-field"
            >添加昵称玩家<input v-model="playerName" placeholder="朋友不注册也能参与" maxlength="255" /></label
          ><button
            class="d-button secondary"
            type="button"
            :disabled="adding || !playerName.trim()"
            @click="add('players')"
          >
            添加
          </button>
        </div>
      </section>
      <section class="d-form-section">
        <div class="d-section-title">
          <h2><em>03</em>怎么记结果</h2>
        </div>
        <div class="d-segment">
          <button
            v-for="(label, mode) in modeNames"
            :key="mode"
            type="button"
            :class="{ active: form.mode === mode }"
            :aria-pressed="form.mode === mode"
            @click="changeMode(mode)"
          >
            {{ label }}
          </button>
        </div>
        <label class="d-field"
          >本局结果<select v-model="form.outcome" required @change="outcomeChanged">
            <option value="">请选择结果</option>
            <option value="win">{{ form.mode === 'coop' ? '全队胜利' : '选择获胜者（可多选）' }}</option>
            <option v-if="form.mode === 'coop'" value="loss">全队失败</option>
            <option v-else value="draw">全局平局</option>
            <option value="unknown">未记结果</option>
          </select></label
        >
        <template v-if="form.mode === 'individual'"
          ><div v-for="id in form.players" :key="id" class="j-score">
            <strong>{{ snapshot.players.find(p => p.id === id)?.name }}</strong
            ><label v-if="form.outcome === 'win'" class="j-check"
              ><input v-model="form.winners" :value="id" type="checkbox" />获胜</label
            ><label class="j-score-value"
              >分数（选填）<input v-model="form.scores[id]" inputmode="decimal" placeholder="未填写"
            /></label></div
        ></template>
        <template v-if="form.mode === 'team'"
          ><div v-for="id in form.players" :key="id" class="j-score j-assignment">
            <label
              >{{ snapshot.players.find(p => p.id === id)?.name
              }}<select :value="form.teams.find(t => t.players.includes(id))?.id ?? ''" @change="assign(id, $event)">
                <option value="">选择队伍</option>
                <option v-for="t in form.teams" :key="t.id" :value="t.id">{{ t.name }}</option>
              </select></label
            >
          </div>
          <div v-for="(t, i) in form.teams" :key="t.id" class="j-team">
            <label class="d-field">队伍名称<input v-model="t.name" required /></label
            ><label v-if="form.outcome === 'win'" class="j-check"
              ><input v-model="t.winner" type="checkbox" />本队获胜</label
            ><label class="d-field">队伍分数（选填）<input v-model="t.score" inputmode="decimal" /></label
            ><button v-if="form.teams.length > 2" type="button" class="d-text-link" @click="form.teams.splice(i, 1)">
              移除队伍并重新分配队员
            </button>
          </div>
          <button
            type="button"
            class="d-button secondary"
            @click="
              form.teams.push({
                id: newID(),
                name: `${form.teams.length + 1} 队`,
                players: [],
                winner: false,
                score: null,
              })
            "
          >
            ＋ 添加队伍
          </button></template
        >
        <label v-if="form.mode === 'coop'" class="d-field"
          >团队分数（选填）<input v-model="form.team_score" inputmode="decimal"
        /></label>
        <p class="d-note">支持负数和最多 4 位小数；留空与零不同，不从分数推断结果。</p>
      </section>
      <section class="d-form-section">
        <button type="button" class="d-text-link" @click="more = !more">
          {{ more ? '收起' : '＋ 添加' }}回忆、照片和更多信息
        </button>
        <div v-if="more">
          <label class="d-field"
            >一句话回忆<textarea v-model="form.memory" rows="4" placeholder="这局有什么值得记住？" /><small
              >{{ Array.from(form.memory).length }} / 500 字</small
            ></label
          >
          <div class="j-inline">
            <label class="d-field">时长（分钟）<input v-model="form.minutes" type="number" min="1" step="1" /></label
            ><label class="d-field"
              >地点<input v-model="form.location" list="recent-locations" /><datalist id="recent-locations">
                <option v-for="place in snapshot.locations" :key="place" :value="place" /></datalist
            ></label>
          </div>
          <label class="d-field"
            >照片（最多 {{ maxPhotos }} 张，每张不超过 2 MB）<input
              type="file"
              multiple
              accept="image/jpeg,image/png,image/webp"
              @change="selectPhotos"
          /></label>
          <div class="j-photos">
            <div v-for="(id, i) in form.photos" :key="id">
              <img :src="photoURL(snapshot.group.id, id)" alt="本局照片" /><button
                type="button"
                @click="form.photos.splice(i, 1)"
              >
                移除
              </button>
            </div>
          </div>
          <div v-for="(item, i) in uploads.filter(p => p.state !== 'done')" :key="i" class="j-notice">
            {{ item.file.name }}<progress v-if="item.state === 'uploading'" aria-label="照片压缩或上传中" /><template
              v-else
              >{{ item.error }}<button type="button" @click="upload(item)">重试</button
              ><button type="button" @click="uploads.splice(uploads.indexOf(item), 1)">移除</button></template
            >
          </div>
          <p v-if="uploads.some(p => p.state === 'failed')" class="d-note">
            失败照片不会关联到记录，可以先保存其他内容。
          </p>
        </div>
      </section>
    </fieldset>
    <p v-if="localError || error" class="j-error" role="alert">{{ localError || error }}</p>
    <div class="j-save">
      <button class="d-button full" :disabled="saving || loadingPhotos || adding">
        {{ saving ? '正在保存…' : loadingPhotos ? '照片上传中…' : '保存这局' }}</button
      ><small>填写内容自动保留在当前设备</small>
    </div>
  </form>
</template>
