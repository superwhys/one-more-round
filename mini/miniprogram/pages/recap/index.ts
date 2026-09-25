import { query, request } from '../../utils/api'
import { getGroupID, getUser, requireGroup, setGroupID } from '../../utils/session'
import { getPhoto } from '../../utils/photo'
import { errorMessage } from '../../utils/ui'
import { today } from '../../utils/round-form'
import type { Recap, Snapshot } from '../../utils/types'

const currentDate = today()

Page({
  data: {
    loading: true,
    error: '',
    photoError: '',
    kind: 'month',
    year: currentDate.slice(0, 4),
    month: Number(currentDate.slice(5, 7)) - 1,
    months: Array.from({ length: 12 }, (_value, index) => `${index + 1} 月`),
    recap: null as Recap | null,
    groupName: '',
    gameName: '',
    playerName: '',
    photos: [] as { id: string; path: string }[],
    generating: false,
    cardPath: '',
    saving: false,
    saveError: '',
  },
  groupID: '',
  userID: '',
  generation: 0,
  active: true,
  snapshot: null as Snapshot | null,

  // onLoad accepts a group deep link while keeping the default Beijing month.
  onLoad(options: Record<string, string | undefined>) {
    if (options.group || options.groupId) setGroupID(options.group || options.groupId || '')
    wx.hideShareMenu()
  },
  // onShow refreshes membership and highlights after records may have changed.
  onShow() {
    void this.initialize()
  },
  // onHide hides the group summary before the next membership check.
  onHide() {
    this.generation++
    this.clearCard()
    this.setData({ recap: null, photos: [], loading: true })
  },
  // onUnload invalidates pending requests and releases the generated private card.
  onUnload() {
    this.active = false
    this.generation++
    if (this.data.cardPath) wx.getFileSystemManager().unlink({ filePath: this.data.cardPath })
  },
  // current ignores results that belong to another account, group, or period.
  current(run: number): boolean {
    return this.active && run === this.generation && getUser()?.id === this.userID && getGroupID() === this.groupID
  },
  // initialize supplies names from the current group rather than persisting account data.
  async initialize() {
    this.setData({ loading: true, error: '' })
    try {
      const context = await requireGroup()
      if (!context || !this.active) return
      this.groupID = context.group.id
      this.userID = context.user.id
      this.snapshot = context.snapshot
      this.setData({ groupName: context.group.name })
      await this.load()
    } catch (cause) {
      if (this.active) this.setData({ loading: false, error: errorMessage(cause) })
    }
  },
  // load queries server-derived monthly or yearly counts with inclusive date semantics.
  async load() {
    if (!/^\d{4}$/.test(this.data.year)) {
      this.setData({ loading: false, error: '请输入四位年份' })
      return
    }
    const run = ++this.generation
    const period =
      this.data.kind === 'month' ? `${this.data.year}-${String(this.data.month + 1).padStart(2, '0')}` : this.data.year
    this.clearCard()
    this.setData({ loading: true, error: '', photoError: '', recap: null, photos: [] })
    try {
      const recap = await request<Recap>(`/groups/${this.groupID}/rounds/recap${query({ period })}`)
      if (!this.current(run)) return
      this.setData({
        recap,
        gameName: this.snapshot?.games.find(game => game.id === recap.top_game)?.name || '暂无',
        playerName: this.snapshot?.players.find(player => player.id === recap.top_player)?.name || '暂无',
        photos: recap.photos.map(id => ({ id, path: '' })),
      })
      void this.loadPhotos(run)
    } catch (cause) {
      if (this.current(run)) this.setData({ error: errorMessage(cause) })
    } finally {
      if (this.active && run === this.generation) this.setData({ loading: false })
    }
  },
  // switchKind requests the same chosen year at the selected summary granularity.
  switchKind(event: WechatMiniprogram.BaseEvent) {
    this.setData({ kind: event.currentTarget.dataset.kind === 'year' ? 'year' : 'month' })
    void this.load()
  },
  // inputYear accepts only four decimal digits and waits for explicit application.
  inputYear(event: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ year: event.detail.value.replace(/\D/g, '').slice(0, 4) })
  },
  // changeMonth updates the selected calendar month.
  changeMonth(event: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ month: Number(event.detail.value) })
  },
  // retry can recover an initial group load failure as well as a recap failure.
  retry() {
    if (!this.snapshot) void this.initialize()
    else void this.load()
  },
  // loadPhotos downloads only photos permitted to the active group member.
  async loadPhotos(run: number) {
    for (const photo of this.data.photos) {
      if (photo.path) continue
      try {
        const path = await getPhoto(this.groupID, photo.id)
        if (!this.current(run)) return
        this.setData({ photos: this.data.photos.map(item => (item.id === photo.id ? { ...item, path } : item)) })
      } catch (cause) {
        if (this.current(run)) this.setData({ photoError: errorMessage(cause) })
      }
    }
  },
  // retryPhotos reattempts missing photos without discarding successfully loaded highlights.
  retryPhotos() {
    this.setData({ photoError: '' })
    void this.loadPhotos(this.generation)
  },
  // previewPhoto opens the authenticated local image files.
  previewPhoto(event: WechatMiniprogram.BaseEvent) {
    const current = String(event.currentTarget.dataset.path || '')
    const urls = this.data.photos.map(photo => photo.path).filter(Boolean)
    if (current && urls.length) wx.previewImage({ current, urls })
  },
  // clearCard deletes a stale generated card whenever the period changes.
  clearCard() {
    if (this.data.cardPath) wx.getFileSystemManager().unlink({ filePath: this.data.cardPath })
    this.setData({ cardPath: '', saveError: '' })
  },
  // generateCard renders an exportable summary using the native canvas API.
  async generateCard() {
    const recap = this.data.recap
    if (!recap || this.data.generating) return
    const run = this.generation
    this.setData({ generating: true, saveError: '' })
    try {
      const ctx = wx.createCanvasContext('recap-card', this)
      const fit = (value: string, length = 22) =>
        Array.from(value).length > length ? `${Array.from(value).slice(0, length).join('')}…` : value
      ctx.setFillStyle('#f7f3e9')
      ctx.fillRect(0, 0, 600, 750)
      ctx.setFillStyle('#b16d47')
      ctx.fillRect(32, 32, 5, 686)
      ctx.setFillStyle('#526049')
      ctx.setFontSize(30)
      ctx.fillText('又一局 · 回顾', 65, 89)
      ctx.setFillStyle('#8b6c50')
      ctx.setFontSize(18)
      ctx.fillText(fit(this.data.groupName, 21), 65, 130)
      ctx.fillText(recap.period, 65, 163)
      ctx.setFillStyle('#30372b')
      ctx.setFontSize(84)
      ctx.fillText(String(recap.rounds), 65, 280)
      ctx.setFillStyle('#7a806f')
      ctx.setFontSize(21)
      ctx.fillText('局一起度过的好时光', 65, 324)
      const rows = [
        `玩过 ${recap.games} 款桌游`,
        `和 ${recap.players} 位朋友同桌`,
        `最常玩：${this.data.gameName}`,
        `最常参加：${this.data.playerName}`,
        recap.minutes ? `记录时长：${recap.minutes} 分钟` : '相聚的快乐，不必都用分钟计算',
      ]
      ctx.setFillStyle('#526049')
      ctx.setFontSize(21)
      rows.forEach((row, index) => ctx.fillText(fit(row), 65, 410 + index * 49))
      ctx.setFillStyle('#b16d47')
      ctx.setFontSize(17)
      ctx.fillText('记下每一局的输赢与相聚。', 65, 706)
      await new Promise<void>(resolve => ctx.draw(false, resolve))
      const path = await new Promise<string>((resolve, reject) =>
        wx.canvasToTempFilePath(
          {
            canvasId: 'recap-card',
            width: 600,
            height: 750,
            destWidth: 1200,
            destHeight: 1500,
            fileType: 'png',
            success: result => resolve(result.tempFilePath),
            fail: reject,
          },
          this,
        ),
      )
      if (!this.current(run)) {
        wx.getFileSystemManager().unlink({ filePath: path })
        return
      }
      this.clearCard()
      this.setData({ cardPath: path })
    } catch (cause) {
      if (this.active) this.setData({ saveError: `回顾卡片生成失败：${errorMessage(cause)}` })
    } finally {
      if (this.active) this.setData({ generating: false })
    }
  },
  // previewCard lets the user review the group information before saving or sharing it.
  previewCard() {
    if (this.data.cardPath) wx.previewImage({ current: this.data.cardPath, urls: [this.data.cardPath] })
  },
  // saveCard writes the explicit export to the user's photo library.
  async saveCard() {
    if (!this.data.cardPath || this.data.saving) return
    this.setData({ saving: true, saveError: '' })
    try {
      await new Promise<void>((resolve, reject) =>
        wx.saveImageToPhotosAlbum({ filePath: this.data.cardPath, success: () => resolve(), fail: reject }),
      )
      wx.showToast({ title: '已保存到相册', icon: 'success' })
    } catch {
      this.setData({ saveError: '未能保存。可点开卡片长按保存，或在小程序设置中允许访问相册。' })
    } finally {
      this.setData({ saving: false })
    }
  },
})
