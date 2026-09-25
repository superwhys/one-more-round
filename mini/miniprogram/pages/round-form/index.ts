import { request, send } from '../../utils/api'
import { getGroupID, getUser, requireGroup, setGroupID } from '../../utils/session'
import { confirm, errorMessage } from '../../utils/ui'
import { getPhoto, uploadPhoto } from '../../utils/photo'
import type { Game, Mode, Outcome, Player, Round, Snapshot } from '../../utils/types'
import { anotherRound, copyRound, emptyRound, newID, normalizeRound, validateRound } from '../../utils/round-form'

type InputEvent = WechatMiniprogram.CustomEvent<{ value: string }>
// Upload keeps retryable local files separate from successfully associated photos.
interface Upload {
  key: string
  path: string
  size: number
  state: 'uploading' | 'failed'
  error: string
}
// ExternalSearch is the existing backend's explicitly requested BGG search result.
interface ExternalSearch {
  items: { bgg_id: number; name: string; year: number | null }[]
  source: string
  source_url: string
}

Page({
  data: {
    loading: true,
    loadFailed: false,
    saving: false,
    adding: false,
    uploading: false,
    error: '',
    notice: '',
    restored: false,
    editing: false,
    conflict: false,
    groupName: '',
    form: emptyRound(),
    minutesInput: '',
    memoryCount: 0,
    gameSearch: '',
    games: [] as Game[],
    playerName: '',
    players: [] as (Player & { selected: boolean })[],
    participants: [] as { id: string; name: string; score: string; winner: boolean; teamIndex: number }[],
    teamNames: ['选择队伍'] as string[],
    outcomes: [] as { value: Outcome | ''; label: string }[],
    outcomeIndex: 0,
    photos: [] as { id: string; path: string }[],
    uploads: [] as Upload[],
    more: false,
    savedID: '',
    external: null as ExternalSearch | null,
    externalName: '',
    externalID: 0,
  },
  snapshot: null as Snapshot | null,
  original: null as Round | null,
  groupID: '',
  userID: '',
  editingID: '',
  initialGame: '',
  draftKey: '',
  submissionKey: '',
  active: true,

  // onLoad resolves the account and group before reading its isolated draft.
  async onLoad(options: Record<string, string | undefined>) {
    const groupID = options.group || options.groupId
    if (groupID) setGroupID(groupID)
    this.editingID = options.id || ''
    this.initialGame = options.game || ''
    this.setData({ editing: !!this.editingID })
    await this.initialize(options.again || '')
  },
  // onShow refreshes games and players after returning from another page.
  onShow() {
    if (this.snapshot && !this.data.loading && !this.data.savedID) void this.refreshOptions()
  },
  // onUnload prevents delayed uploads from writing a different account's draft.
  onUnload() {
    this.active = false
  },
  // isCurrent guards every asynchronous update against account and group changes.
  isCurrent(): boolean {
    return this.active && getUser()?.id === this.userID && getGroupID() === this.groupID
  },

  // initialize loads the source record and restores only a structurally valid draft.
  async initialize(again = '') {
    this.setData({ loading: true, loadFailed: false, error: '' })
    try {
      const context = await requireGroup()
      if (!context) return
      this.groupID = context.group.id
      this.userID = context.user.id
      this.snapshot = context.snapshot
      this.draftKey = `omr:draft:${this.userID}:${this.groupID}:${this.editingID || 'new'}`
      const source = this.editingID || again
      this.original = this.editingID ? await request<Round>(`/groups/${this.groupID}/rounds/${source}`) : null
      if (this.original && context.group.owner !== this.userID && this.original.author !== this.userID)
        throw new Error('只有记录人和组主可以编辑这局')
      let form = this.original
        ? copyRound(this.original)
        : again
          ? anotherRound(await request<Round>(`/groups/${this.groupID}/rounds/${again}`))
          : emptyRound()
      form.game_id ||= this.initialGame
      if (!source) {
        const self = context.snapshot.players.find(player => player.account === this.userID)
        if (self) form.players = [self.id]
      }
      this.submissionKey = newID()
      let restored = false
      let minutesInput = form.minutes === null ? '' : String(form.minutes)
      if (!again) {
        try {
          const draft = wx.getStorageSync(this.draftKey) as { form?: Round; key?: string; minutes?: string } | undefined
          if (
            draft &&
            draft.form &&
            Array.isArray(draft.form.players) &&
            Array.isArray(draft.form.teams) &&
            Array.isArray(draft.form.photos) &&
            Array.isArray(draft.form.winners) &&
            draft.form.scores &&
            typeof draft.form.memory === 'string' &&
            typeof draft.key === 'string'
          ) {
            form = copyRound(draft.form)
            this.submissionKey = draft.key
            minutesInput =
              draft.minutes === undefined ? (form.minutes === null ? '' : String(form.minutes)) : draft.minutes
            restored = true
          }
        } catch {
          this.setData({ notice: '草稿无法读取，请重新填写' })
        }
      }
      if (!this.isCurrent()) return
      this.setData({
        form,
        minutesInput,
        restored,
        groupName: context.group.name,
        gameSearch: context.snapshot.games.find(game => game.id === form.game_id)?.name || '',
        more: !!source || !!form.memory || !!form.photos.length,
      })
      this.syncForm()
      void this.loadPhotos()
    } catch (cause) {
      this.setData({ error: errorMessage(cause), loadFailed: true })
    } finally {
      this.setData({ loading: false })
    }
  },
  // retryLoad retries a failed initial load without treating the tap event as a round ID.
  retryLoad() {
    void this.initialize()
  },
  // refreshOptions incorporates catalog additions without replacing form input.
  async refreshOptions() {
    try {
      const snapshot = await request<Snapshot>(`/groups/${this.groupID}`)
      if (!this.isCurrent()) return
      this.snapshot = snapshot
      this.syncForm()
    } catch (cause) {
      if (this.isCurrent()) this.setData({ notice: errorMessage(cause) })
    }
  },
  // syncForm prepares template-friendly selected states and persists the current input.
  syncForm() {
    if (!this.snapshot) return
    const form = this.data.form
    const term = this.data.gameSearch.trim().toLowerCase()
    const outcomes: { value: Outcome | ''; label: string }[] = [
      { value: '', label: '请选择本局结果' },
      { value: 'win', label: form.mode === 'coop' ? '全队胜利' : '选择获胜者（可多选）' },
      form.mode === 'coop' ? { value: 'loss', label: '全队失败' } : { value: 'draw', label: '全局平局' },
      { value: 'unknown', label: '未记结果' },
    ]
    this.setData({
      games: this.snapshot.games.filter(game => `${game.name} ${game.original || ''}`.toLowerCase().includes(term)),
      players: this.snapshot.players.map(player => ({ ...player, selected: form.players.includes(player.id) })),
      participants: form.players.map(id => ({
        id,
        name: this.snapshot?.players.find(player => player.id === id)?.name || '玩家',
        score: form.scores[id] ?? '',
        winner: form.winners.includes(id),
        teamIndex: form.teams.findIndex(team => team.players.includes(id)) + 1,
      })),
      teamNames: ['选择队伍', ...form.teams.map(team => team.name || '未命名队伍')],
      outcomes,
      outcomeIndex: outcomes.findIndex(outcome => outcome.value === form.outcome),
      memoryCount: Array.from(form.memory).length,
    })
    this.persistDraft()
  },
  // persistDraft retains the idempotency key alongside account-scoped form input.
  persistDraft() {
    if (!this.isCurrent() || !this.draftKey || this.data.savedID) return
    try {
      wx.setStorageSync(this.draftKey, {
        form: this.data.form,
        key: this.submissionKey,
        minutes: this.data.minutesInput,
      })
    } catch {
      this.setData({ notice: '设备无法保存草稿，请保持页面打开' })
    }
  },
  // inputField updates a simple round field without changing the submission key.
  inputField(event: InputEvent) {
    const field = String(event.currentTarget.dataset.field)
    if (!['date', 'memory', 'team_score'].includes(field) || this.data.saving) return
    this.setData({ [`form.${field}`]: event.detail.value })
    this.syncForm()
  },
  // inputMinutes preserves invalid and blank values until explicit validation on save.
  inputMinutes(event: InputEvent) {
    this.setData({ minutesInput: event.detail.value })
    this.persistDraft()
  },
  // searchGame searches only the local catalog until the explicit external search action.
  searchGame(event: InputEvent) {
    this.setData({ gameSearch: event.detail.value, 'form.game_id': '', external: null })
    this.syncForm()
  },
  // chooseGame selects a concrete local game instead of treating free text as an ID.
  chooseGame(event: WechatMiniprogram.BaseEvent) {
    const game = this.snapshot?.games.find(item => item.id === event.currentTarget.dataset.id)
    if (!game) return
    this.setData({ 'form.game_id': game.id, gameSearch: game.name, error: '' })
    this.syncForm()
  },
  // inputPlayer holds the optional nickname for a new player profile.
  inputPlayer(event: InputEvent) {
    this.setData({ playerName: event.detail.value })
  },
  // addItem adds and immediately selects a local game or a nickname player.
  async addItem(event: WechatMiniprogram.BaseEvent) {
    const kind = event.currentTarget.dataset.kind === 'games' ? 'games' : 'players'
    const name = (kind === 'games' ? this.data.gameSearch : this.data.playerName).trim()
    if (!name || this.data.adding || this.data.saving || !this.snapshot) return
    this.setData({ adding: true, error: '' })
    try {
      const item = await send<Game & Player>(`/groups/${this.groupID}/${kind}`, { name })
      if (!this.isCurrent()) return
      if (kind === 'games') {
        this.snapshot.games.push(item)
        this.setData({ 'form.game_id': item.id, gameSearch: item.name })
      } else {
        this.snapshot.players.push(item)
        this.setData({ 'form.players': [...this.data.form.players, item.id], playerName: '' })
      }
      this.syncForm()
    } catch (cause) {
      this.setData({ error: errorMessage(cause) })
    } finally {
      this.setData({ adding: false })
    }
  },
  // togglePlayer also removes stale scores, winners, and team membership.
  togglePlayer(event: WechatMiniprogram.BaseEvent) {
    if (this.data.saving) return
    const id = String(event.currentTarget.dataset.id)
    const form = copyRound(this.data.form)
    form.players = form.players.includes(id) ? form.players.filter(player => player !== id) : [...form.players, id]
    form.winners = form.winners.filter(player => form.players.includes(player))
    for (const player of Object.keys(form.scores)) if (!form.players.includes(player)) delete form.scores[player]
    for (const team of form.teams) team.players = team.players.filter(player => form.players.includes(player))
    this.setData({ form })
    this.syncForm()
  },
  // changeMode clears fields that have different meanings in another play mode.
  changeMode(event: WechatMiniprogram.BaseEvent) {
    const mode = event.currentTarget.dataset.mode as Mode
    if (this.data.saving || this.data.form.mode === mode) return
    const form = copyRound(this.data.form)
    Object.assign(form, {
      mode,
      outcome: '',
      winners: [],
      scores: {},
      team_score: null,
      teams:
        mode === 'team'
          ? [
              { id: newID(), name: 'A 队', players: [], score: null, winner: false },
              { id: newID(), name: 'B 队', players: [], score: null, winner: false },
            ]
          : [],
    })
    this.setData({ form })
    this.syncForm()
  },
  // changeOutcome clears winner selections when the result category changes.
  changeOutcome(event: InputEvent) {
    const form = copyRound(this.data.form)
    form.outcome = this.data.outcomes[Number(event.detail.value)].value
    form.winners = []
    form.teams.forEach(team => {
      team.winner = false
    })
    this.setData({ form })
    this.syncForm()
  },
  // toggleWinner records explicit winners, including a shared victory.
  toggleWinner(event: WechatMiniprogram.BaseEvent) {
    const id = String(event.currentTarget.dataset.id)
    const winners = this.data.form.winners
    this.setData({ 'form.winners': winners.includes(id) ? winners.filter(player => player !== id) : [...winners, id] })
    this.syncForm()
  },
  // inputScore stores decimal scores as strings to avoid floating-point rounding.
  inputScore(event: InputEvent) {
    this.setData({ [`form.scores.${event.currentTarget.dataset.id}`]: event.detail.value })
    this.syncForm()
  },
  // assignTeam ensures a selected player is assigned to at most one team.
  assignTeam(event: InputEvent) {
    const id = String(event.currentTarget.dataset.id)
    const form = copyRound(this.data.form)
    form.teams.forEach(team => {
      team.players = team.players.filter(player => player !== id)
    })
    form.teams[Number(event.detail.value) - 1]?.players.push(id)
    this.setData({ form })
    this.syncForm()
  },
  // inputTeam updates a team's optional score or display name.
  inputTeam(event: InputEvent) {
    const field = event.currentTarget.dataset.field === 'name' ? 'name' : 'score'
    this.setData({ [`form.teams[${Number(event.currentTarget.dataset.index)}].${field}`]: event.detail.value })
    this.syncForm()
  },
  // toggleTeamWinner explicitly records a winning team.
  toggleTeamWinner(event: WechatMiniprogram.BaseEvent) {
    const index = Number(event.currentTarget.dataset.index)
    this.setData({ [`form.teams[${index}].winner`]: !this.data.form.teams[index].winner })
    this.syncForm()
  },
  // addTeam adds a concrete team to the current record.
  addTeam() {
    this.setData({
      'form.teams': [
        ...this.data.form.teams,
        { id: newID(), name: `${this.data.form.teams.length + 1} 队`, players: [], score: null, winner: false },
      ],
    })
    this.syncForm()
  },
  // removeTeam leaves its players unassigned so validation requires reassignment.
  removeTeam(event: WechatMiniprogram.BaseEvent) {
    if (this.data.form.teams.length <= 2) return
    this.setData({
      'form.teams': this.data.form.teams.filter((_team, index) => index !== Number(event.currentTarget.dataset.index)),
    })
    this.syncForm()
  },
  // toggleMore reveals optional memories, duration, and photos.
  toggleMore() {
    this.setData({ more: !this.data.more })
  },
  // loadPhotos downloads protected images using the authenticated file helper.
  async loadPhotos() {
    const ids = [...this.data.form.photos]
    this.setData({ photos: ids.map(id => ({ id, path: '' })) })
    for (const id of ids) {
      try {
        const path = await getPhoto(this.groupID, id)
        if (!this.isCurrent()) return
        this.setData({ photos: this.data.photos.map(photo => (photo.id === id ? { id, path } : photo)) })
      } catch (cause) {
        if (this.isCurrent()) this.setData({ notice: `照片加载失败：${errorMessage(cause)}` })
      }
    }
  },
  // choosePhotos checks original files and uploads them sequentially.
  async choosePhotos() {
    const count = 3 - this.data.form.photos.length - this.data.uploads.length
    if (this.data.uploading || this.data.saving || count <= 0) return
    try {
      const result = await new Promise<WechatMiniprogram.ChooseMediaSuccessCallbackResult>((resolve, reject) =>
        wx.chooseMedia({ count, mediaType: ['image'], sizeType: ['original'], success: resolve, fail: reject }),
      )
      for (const file of result.tempFiles) {
        if (!this.isCurrent()) return
        const item: Upload = { key: newID(), path: file.tempFilePath, size: file.size, state: 'uploading', error: '' }
        this.setData({ uploads: [...this.data.uploads, item] })
        await this.uploadOne(item)
      }
    } catch (cause) {
      if (!String((cause as { errMsg?: string }).errMsg || '').includes('cancel'))
        this.setData({ error: errorMessage(cause) })
    }
  },
  // uploadOne retains a failed local file without blocking the rest of the form.
  async uploadOne(item: Upload) {
    this.setData({
      uploading: true,
      uploads: this.data.uploads.map(upload =>
        upload.key === item.key ? { ...upload, state: 'uploading', error: '' } : upload,
      ),
    })
    try {
      const photo = await uploadPhoto(this.groupID, item.path, item.size)
      if (!this.isCurrent()) return
      this.setData({
        'form.photos': [...this.data.form.photos, photo.id],
        photos: [...this.data.photos, { id: photo.id, path: item.path }],
        uploads: this.data.uploads.filter(upload => upload.key !== item.key),
      })
      this.persistDraft()
    } catch (cause) {
      if (this.isCurrent())
        this.setData({
          uploads: this.data.uploads.map(upload =>
            upload.key === item.key ? { ...upload, state: 'failed', error: errorMessage(cause) } : upload,
          ),
        })
    } finally {
      if (this.active) this.setData({ uploading: false })
    }
  },
  // retryUpload retries the same file after a failed upload.
  retryUpload(event: WechatMiniprogram.BaseEvent) {
    const item = this.data.uploads.find(upload => upload.key === event.currentTarget.dataset.key)
    if (item && !this.data.uploading) void this.uploadOne(item)
  },
  // removePhoto removes either a failed local file or a successfully uploaded reference.
  removePhoto(event: WechatMiniprogram.BaseEvent) {
    if (this.data.uploading || this.data.saving) return
    const id = String(event.currentTarget.dataset.id || '')
    const key = String(event.currentTarget.dataset.key || '')
    this.setData({
      'form.photos': this.data.form.photos.filter(photo => photo !== id),
      photos: this.data.photos.filter(photo => photo.id !== id),
      uploads: this.data.uploads.filter(upload => upload.key !== key),
    })
    this.persistDraft()
  },
  // discardDraft explicitly resets inputs and starts a fresh submission.
  async discardDraft() {
    if (!(await confirm('放弃草稿？', '当前设备保存的填写内容会被清除。'))) return
    const form = this.original ? copyRound(this.original) : emptyRound()
    form.game_id ||= this.initialGame
    this.submissionKey = newID()
    this.setData({
      form,
      minutesInput: form.minutes === null ? '' : String(form.minutes),
      restored: false,
      uploads: [],
      error: '',
      conflict: false,
      gameSearch: this.snapshot?.games.find(game => game.id === form.game_id)?.name || '',
    })
    this.syncForm()
    void this.loadPhotos()
  },
  // reloadConflict preserves the draft until the user chooses the server version.
  async reloadConflict() {
    if (!(await confirm('加载最新记录？', '这会替换当前编辑内容，请先保留需要的回忆文字。'))) return
    wx.removeStorageSync(this.draftKey)
    await this.initialize()
    this.setData({ conflict: false })
  },
  // save reuses the draft's idempotency key on every retry and clears only on success.
  async save() {
    if (this.data.saving || this.data.uploading || this.data.adding || !this.snapshot) return
    const form = normalizeRound(this.data.form, this.data.minutesInput)
    const error = validateRound(form, this.snapshot)
    if (error) {
      this.setData({ error })
      return
    }
    this.setData({ saving: true, error: '', conflict: false })
    this.persistDraft()
    try {
      const path = `/groups/${this.groupID}/rounds${this.editingID ? `/${this.editingID}` : ''}`
      const saved = await send<Round>(path, form, this.editingID ? 'PUT' : 'POST', this.submissionKey)
      if (!this.isCurrent()) return
      wx.removeStorageSync(this.draftKey)
      this.setData({ savedID: saved.id, form: saved, uploads: [] })
      if (this.editingID) wx.redirectTo({ url: `/pages/round/index?group=${this.groupID}&id=${saved.id}` })
    } catch (cause) {
      this.setData({
        error: errorMessage(cause),
        conflict: this.data.editing && (cause as { status?: number }).status === 409,
      })
    } finally {
      this.setData({ saving: false })
    }
  },
  // again starts today's next game without photos, previous results, or the old key.
  again() {
    const form = anotherRound(this.data.form)
    this.submissionKey = newID()
    this.setData({
      form,
      savedID: '',
      photos: [],
      uploads: [],
      minutesInput: '',
      more: false,
      restored: false,
      error: '',
    })
    this.syncForm()
  },
  // viewSaved opens the successfully persisted record.
  viewSaved() {
    wx.redirectTo({ url: `/pages/round/index?group=${this.groupID}&id=${this.data.savedID}` })
  },
  // searchExternal only contacts BGG following this explicit action.
  async searchExternal() {
    const name = this.data.gameSearch.trim()
    if (!name || this.data.adding) return
    this.setData({ adding: true, error: '' })
    try {
      this.setData({
        external: await request<ExternalSearch>(`/groups/${this.groupID}/bgg/search?q=${encodeURIComponent(name)}`, {
          timeout: 45000,
        }),
      })
    } catch (cause) {
      this.setData({ error: errorMessage(cause) })
    } finally {
      this.setData({ adding: false })
    }
  },
  // chooseExternal asks for the group's local game name before importing.
  chooseExternal(event: WechatMiniprogram.BaseEvent) {
    this.setData({
      externalID: Number(event.currentTarget.dataset.id),
      externalName: String(event.currentTarget.dataset.name),
    })
  },
  // inputExternalName preserves a Chinese alias independently of BGG's original name.
  inputExternalName(event: InputEvent) {
    this.setData({ externalName: event.detail.value })
  },
  // importExternal reuses an existing game when the BGG entry is already on the shelf.
  async importExternal() {
    if (!this.data.externalID || this.data.adding || !this.snapshot) return
    this.setData({ adding: true, error: '' })
    try {
      const game = await send<Game>(`/groups/${this.groupID}/games/import`, {
        bgg_id: this.data.externalID,
        name: this.data.externalName.trim(),
      })
      if (!this.snapshot.games.some(item => item.id === game.id)) this.snapshot.games.push(game)
      this.setData({ 'form.game_id': game.id, gameSearch: game.name, external: null, externalID: 0 })
      this.syncForm()
    } catch (cause) {
      this.setData({ error: errorMessage(cause) })
    } finally {
      this.setData({ adding: false })
    }
  },
})
