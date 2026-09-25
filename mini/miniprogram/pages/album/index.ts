import { query, request } from '../../utils/api'
import { getGroupID, getUser, requireGroup, setGroupID } from '../../utils/session'
import { getPhoto } from '../../utils/photo'
import { errorMessage } from '../../utils/ui'
import type { Game, Page as RoundPage, Round } from '../../utils/types'

// AlbumPhoto keeps the originating round alongside its protected local image file.
interface AlbumPhoto {
  id: string
  roundID: string
  game: string
  path: string
}
// AlbumDay groups photos using date-only round semantics.
interface AlbumDay {
  date: string
  photos: AlbumPhoto[]
}

Page({
  data: {
    loading: true,
    loadingMore: false,
    error: '',
    photoError: '',
    filterError: '',
    from: '',
    to: '',
    gameIndex: 0,
    games: [{ id: '', name: '全部桌游' }] as Pick<Game, 'id' | 'name'>[],
    days: [] as AlbumDay[],
    total: 0,
    hasMore: false,
    filtered: false,
  },
  groupID: '',
  userID: '',
  generation: 0,
  active: true,
  rounds: [] as Round[],
  applied: { from: '', to: '', game: '' },
  errorIsMore: false,

  // onLoad accepts a deep link's explicit group before checking membership.
  onLoad(options: Record<string, string | undefined>) {
    if (options.group || options.groupId) setGroupID(options.group || options.groupId || '')
  },
  // onShow rechecks group access and refreshes the album after returning from a record.
  onShow() {
    void this.initialize()
  },
  // onHide clears private thumbnails until the next membership check.
  onHide() {
    this.generation++
    this.rounds = []
    this.setData({ days: [], total: 0, loading: true, loadingMore: false })
  },
  // onUnload invalidates outstanding list and photo requests.
  onUnload() {
    this.active = false
    this.generation++
  },
  // current rejects results after an account, group, or filter change.
  current(run: number): boolean {
    return this.active && run === this.generation && getUser()?.id === this.userID && getGroupID() === this.groupID
  },
  // initialize loads the group catalog used to label and filter photos.
  async initialize() {
    this.setData({ loading: true, error: '' })
    try {
      const context = await requireGroup()
      if (!context || !this.active) return
      if (this.groupID && this.groupID !== context.group.id) {
        this.applied = { from: '', to: '', game: '' }
        this.setData({ from: '', to: '', gameIndex: 0, days: [] })
      }
      this.groupID = context.group.id
      this.userID = context.user.id
      this.setData({ games: [{ id: '', name: '全部桌游' }, ...context.snapshot.games] })
      await this.load(false)
    } catch (cause) {
      if (this.active) this.setData({ error: errorMessage(cause), loading: false })
    }
  },
  // load applies validated filters and preserves existing photos during pagination.
  async load(more = false) {
    if (more && (this.data.loadingMore || !this.data.hasMore)) return
    const run = ++this.generation
    this.errorIsMore = more
    this.setData({ loading: !more, loadingMore: more, error: '', photoError: '' })
    if (!more) {
      this.rounds = []
      this.setData({ days: [], total: 0, hasMore: false })
    }
    try {
      const page = await request<RoundPage>(
        `/groups/${this.groupID}/rounds${query({ ...this.applied, has_photos: 'true', limit: 30, offset: this.rounds.length })}`,
      )
      if (!this.current(run)) return
      this.rounds = more ? [...this.rounds, ...page.items] : page.items
      const existing = new Map(this.data.days.flatMap(day => day.photos).map(photo => [photo.id, photo.path]))
      const days: AlbumDay[] = []
      for (const round of this.rounds) {
        let day = days.find(item => item.date === round.date)
        if (!day) {
          day = { date: round.date, photos: [] }
          days.push(day)
        }
        for (const id of round.photos)
          day.photos.push({
            id,
            roundID: round.id,
            game: this.data.games.find(game => game.id === round.game_id)?.name || '桌游',
            path: existing.get(id) || '',
          })
      }
      this.setData({
        days,
        total: page.total,
        hasMore: this.rounds.length < page.total,
        filtered: !!(this.applied.from || this.applied.to || this.applied.game),
      })
      void this.loadPhotos(run)
    } catch (cause) {
      if (this.current(run)) this.setData({ error: errorMessage(cause) })
    } finally {
      if (this.active && run === this.generation) this.setData({ loading: false, loadingMore: false })
    }
  },
  // loadPhotos downloads protected images in small batches without blocking the list.
  async loadPhotos(run: number) {
    const photos = this.data.days.flatMap(day => day.photos).filter(photo => !photo.path)
    for (let index = 0; index < photos.length; index += 4) {
      if (!this.current(run)) return
      await Promise.all(
        photos.slice(index, index + 4).map(async photo => {
          try {
            const path = await getPhoto(this.groupID, photo.id)
            if (!this.current(run)) return
            this.setData({
              days: this.data.days.map(day => ({
                ...day,
                photos: day.photos.map(item => (item.id === photo.id ? { ...item, path } : item)),
              })),
            })
          } catch (cause) {
            if (this.current(run)) this.setData({ photoError: errorMessage(cause) })
          }
        }),
      )
    }
  },
  // changeDate edits a draft date range without issuing intermediate requests.
  changeDate(event: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ [event.currentTarget.dataset.field === 'from' ? 'from' : 'to']: event.detail.value })
  },
  // changeGame edits the selected local catalog filter.
  changeGame(event: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ gameIndex: Number(event.detail.value) })
  },
  // apply validates the full interval before replacing the currently displayed results.
  apply() {
    if (this.data.from && this.data.to && this.data.from > this.data.to) {
      this.setData({ filterError: '开始日期不能晚于结束日期' })
      return
    }
    this.applied = { from: this.data.from, to: this.data.to, game: this.data.games[this.data.gameIndex]?.id || '' }
    this.setData({ filterError: '' })
    void this.load(false)
  },
  // clear resets every filter to the whole group's photo history.
  clear() {
    this.setData({ from: '', to: '', gameIndex: 0 })
    this.apply()
  },
  // more loads another page of photo-bearing rounds.
  more() {
    void this.load(true)
  },
  // retry repeats the failed pagination operation without losing older results.
  retry() {
    if (!this.groupID) void this.initialize()
    else void this.load(this.errorIsMore)
  },
  // retryPhotos retries only failed or missing photos.
  retryPhotos() {
    this.setData({ photoError: '' })
    void this.loadPhotos(this.generation)
  },
  // openRound keeps the selected photo attached to its original record.
  openRound(event: WechatMiniprogram.BaseEvent) {
    wx.navigateTo({ url: `/pages/round/index?group=${this.groupID}&id=${event.currentTarget.dataset.id}` })
  },
  // create opens an empty record in this album's current group.
  create() {
    wx.navigateTo({ url: `/pages/round-form/index?group=${this.groupID}` })
  },
})
