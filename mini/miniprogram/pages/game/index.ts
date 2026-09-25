import { ApiError, query, request, send } from '../../utils/api'
import { getPhoto } from '../../utils/photo'
import { requireGroup, setGroupID } from '../../utils/session'
import type { Game, Mode, Page as RoundPage, Snapshot } from '../../utils/types'
import { errorMessage, navigate, toast } from '../../utils/ui'
import {
  emptyFilters,
  emptyHistory,
  filterSelection,
  historyCards,
  modeOptions,
  modes,
  outcomeOptions,
  photoOptions,
  quickDates,
  statistics,
} from './history'
import type { HistoryCard, StatRow } from './history'

Page({
  data: {
    id: '',
    groupId: '',
    snapshot: null as Snapshot | null,
    current: null as Game | null,
    page: emptyHistory(),
    filters: emptyFilters(),
    activeFilters: emptyFilters(),
    cards: [] as HistoryCard[],
    stats: [] as StatRow[],
    modes,
    modeOptions,
    outcomeOptions,
    photoOptions,
    statMode: 'individual' as Mode,
    filterOpen: false,
    hasFilters: false,
    fixedGame: true,
    fixedPlayer: false,
    gameOptions: [] as { id: string; name: string }[],
    playerOptions: [] as { id: string; name: string }[],
    filterLabels: {
      game: '全部桌游',
      player: '全部玩家',
      mode: '全部模式',
      outcome: '全部结果',
      has_photos: '照片不限',
    },
    filterIndices: { game: 0, player: 0, mode: 0, outcome: 0, has_photos: 0 },
    loading: true,
    busy: false,
    error: '',
    saveError: '',
    name: '',
  },
  generation: 0,

  // onLoad 固定当前桌游，显式组参数只用于成员身份验证后的读取。
  onLoad(options) {
    if (options.group) setGroupID(options.group)
    this.setData({ id: options.id || '' })
  },

  // onShow 返回详情时重新读取修改后的资料与统计。
  onShow() {
    this.setData({ current: null, snapshot: null, page: emptyHistory(), cards: [], stats: [] })
    void this.load()
  },

  // onUnload 阻止旧请求写回已销毁的页面。
  onUnload() {
    this.generation += 1
  },

  // onPullDownRefresh 刷新当前筛选范围。
  async onPullDownRefresh() {
    await this.load()
    wx.stopPullDownRefresh()
  },

  // onReachBottom 自动延续当前筛选下的历史分页。
  onReachBottom() {
    this.more()
  },

  // reload 提供按钮事件入口，避免把事件对象误当成分页标识。
  reload() {
    void this.load()
  },

  // load 始终把固定桌游加入列表和统计的同一查询。
  async load(more = false) {
    const generation = ++this.generation
    this.setData({ loading: true, error: '' })
    try {
      const context = await requireGroup()
      if (!context || generation !== this.generation) return
      const current = context.snapshot.games.find(game => game.id === this.data.id) || null
      this.setData({
        groupId: context.group.id,
        snapshot: context.snapshot,
        current,
        gameOptions: [{ id: '', name: '全部桌游' }, ...context.snapshot.games],
        playerOptions: [{ id: '', name: '全部玩家' }, ...context.snapshot.players],
      })
      if (!current) return
      const result = await request<RoundPage>(
        `/groups/${context.group.id}/rounds${query({ ...this.data.activeFilters, game: current.id, limit: 30, offset: more ? this.data.page.items.length : 0 })}`,
      )
      if (generation !== this.generation) return
      const page = more ? { ...result, items: [...this.data.page.items, ...result.items] } : result
      this.setData({
        page,
        cards: historyCards(page.items, context.snapshot, more ? this.data.cards : []),
        stats: statistics(page.stats, context.snapshot, this.data.statMode),
      })
      void this.loadPhotos(generation)
    } catch (cause) {
      if (generation === this.generation) {
        this.setData({ error: errorMessage(cause) })
        if (cause instanceof ApiError && [401, 403].includes(cause.status))
          this.setData({ current: null, snapshot: null, page: emptyHistory(), cards: [], stats: [] })
      }
    } finally {
      if (generation === this.generation) this.setData({ loading: false })
    }
  },

  // loadPhotos 顺序读取需要成员授权的首图，照片失败不阻止查看其他内容。
  async loadPhotos(generation: number) {
    const cards = this.data.cards
    for (let index = 0; index < cards.length; index += 1) {
      const card = cards[index]!
      if (generation !== this.generation) return
      if (!card.photos.length || card.photoPath) continue
      try {
        const photoPath = await getPhoto(this.data.groupId, card.photos[0]!)
        if (generation === this.generation) this.setData({ [`cards[${index}].photoPath`]: photoPath })
      } catch {
        if (generation === this.generation) this.setData({ [`cards[${index}].photoFailed`]: true })
      }
    }
  },

  // more 保持分页期间只有一个历史请求。
  more() {
    if (!this.data.loading && this.data.page.items.length < this.data.page.total) void this.load(true)
  },

  // toggleFilters 展开原生筛选控件。
  toggleFilters() {
    this.setData({ filterOpen: !this.data.filterOpen })
  },

  // filterInput 更新回忆搜索或日期，应用前不发起请求。
  filterInput(event: WechatMiniprogram.Input | WechatMiniprogram.PickerChange) {
    const field = String(event.currentTarget.dataset.field)
    if (['q', 'from', 'to'].includes(field)) this.setData({ [`filters.${field}`]: String(event.detail.value) })
  },

  // filterPicker 把选择器下标映射为服务端接受的标识。
  filterPicker(event: WechatMiniprogram.PickerChange) {
    if (!this.data.snapshot) return
    const field = String(event.currentTarget.dataset.field)
    const index = Number(event.detail.value)
    const selected = filterSelection(field, index, this.data.snapshot)
    if (selected)
      this.setData({
        [`filters.${field}`]: selected.value,
        [`filterLabels.${field}`]: selected.label,
        [`filterIndices.${field}`]: index,
      })
  },

  // applyFilters 确认日期范围后让列表与统计一起刷新。
  applyFilters() {
    if (this.data.filters.from && this.data.filters.to && this.data.filters.from > this.data.filters.to)
      return toast('开始日期不能晚于结束日期')
    this.setData({
      activeFilters: { ...this.data.filters },
      hasFilters: Object.values(this.data.filters).some(Boolean),
    })
    void this.load()
  },

  // clearFilters 只清空用户条件，保留本页固定的桌游。
  clearFilters() {
    this.setData({
      filters: emptyFilters(),
      activeFilters: emptyFilters(),
      hasFilters: false,
      filterLabels: {
        game: '全部桌游',
        player: '全部玩家',
        mode: '全部模式',
        outcome: '全部结果',
        has_photos: '照片不限',
      },
      filterIndices: { game: 0, player: 0, mode: 0, outcome: 0, has_photos: 0 },
    })
    void this.load()
  },

  // quick 按北京时间快捷选择本月或今年。
  quick(event: WechatMiniprogram.TouchEvent) {
    const dates = quickDates(String(event.currentTarget.dataset.period))
    this.setData({ 'filters.from': dates.from, 'filters.to': dates.to })
    this.applyFilters()
  },

  // changeStatMode 分开展示三类战绩，不混算不同模式胜率。
  changeStatMode(event: WechatMiniprogram.TouchEvent) {
    const mode = event.currentTarget.dataset.mode as Mode
    if (!this.data.snapshot || !modes.some(item => item.value === mode)) return
    this.setData({ statMode: mode, stats: statistics(this.data.page.stats, this.data.snapshot, mode) })
  },

  // onName 收集中文别名，不修改 BGG 原名。
  onName(event: WechatMiniprogram.Input) {
    this.setData({ name: event.detail.value })
  },

  // rename 保存本组习惯使用的名称后刷新资料。
  async rename() {
    const name = this.data.name.trim()
    if (!name) return toast('请输入本组名称')
    if (this.data.busy) return
    this.setData({ busy: true, saveError: '' })
    try {
      await send<void>(`/groups/${this.data.groupId}/manage`, { action: 'alias', target: this.data.id, value: name })
      this.setData({ name: '' })
      toast('本组名称已保存')
      await this.load()
    } catch (cause) {
      this.setData({ saveError: errorMessage(cause) })
    } finally {
      this.setData({ busy: false })
    }
  },

  // copyBGG 复制资料来源地址，避免外部站点依赖影响组内记录。
  copyBGG() {
    if (this.data.current?.bgg_id)
      wx.setClipboardData({ data: `https://boardgamegeek.com/boardgame/${this.data.current.bgg_id}` })
  },

  // record 沿用当前桌游开始新的对局。
  record() {
    navigate(`/pages/round-form/index?game=${encodeURIComponent(this.data.id)}`)
  },

  // openRound 打开某条组内对局详情。
  openRound(event: WechatMiniprogram.TouchEvent) {
    navigate(`/pages/round/index?id=${encodeURIComponent(String(event.currentTarget.dataset.id))}`)
  },

  // openShelf 返回桌游架寻找可用的条目。
  openShelf() {
    navigate('/pages/games/index')
  },
})
