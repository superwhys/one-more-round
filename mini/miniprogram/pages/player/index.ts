import { ApiError, query, request } from '../../utils/api'
import { getPhoto } from '../../utils/photo'
import { requireGroup, setGroupID } from '../../utils/session'
import type { Mode, Page as RoundPage, Player, Snapshot } from '../../utils/types'
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
} from '../game/history'
import type { HistoryCard, StatRow } from '../game/history'

Page({
  data: {
    id: '',
    groupId: '',
    snapshot: null as Snapshot | null,
    current: null as Player | null,
    page: emptyHistory(),
    filters: emptyFilters(),
    activeFilters: emptyFilters(),
    cards: [] as HistoryCard[],
    stats: [] as StatRow[],
    commonGames: [] as { id: string; name: string; count: number }[],
    modes,
    modeOptions,
    outcomeOptions,
    photoOptions,
    statMode: 'individual' as Mode,
    filterOpen: false,
    hasFilters: false,
    fixedGame: false,
    fixedPlayer: true,
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
    error: '',
  },
  generation: 0,

  // onLoad 固定当前玩家，成员身份仍由服务端检查。
  onLoad(options) {
    if (options.group) setGroupID(options.group)
    this.setData({ id: options.id || '' })
  },

  // onShow 切回页面时同步玩家参与和历史变动。
  onShow() {
    this.setData({ current: null, snapshot: null, page: emptyHistory(), cards: [], stats: [], commonGames: [] })
    void this.load()
  },

  // onUnload 使已离开页面的请求失效。
  onUnload() {
    this.generation += 1
  },

  // onPullDownRefresh 刷新当前玩家的查询范围。
  async onPullDownRefresh() {
    await this.load()
    wx.stopPullDownRefresh()
  },

  // onReachBottom 延续分页但不改变统计口径。
  onReachBottom() {
    this.more()
  },

  // reload 为模板提供不接收分页参数的重试入口。
  reload() {
    void this.load()
  },

  // load 同时用玩家条件读取完整统计和一页历史记录。
  async load(more = false) {
    const generation = ++this.generation
    this.setData({ loading: true, error: '' })
    try {
      const context = await requireGroup()
      if (!context || generation !== this.generation) return
      const current = context.snapshot.players.find(player => player.id === this.data.id) || null
      this.setData({
        groupId: context.group.id,
        snapshot: context.snapshot,
        current,
        gameOptions: [{ id: '', name: '全部桌游' }, ...context.snapshot.games],
        playerOptions: [{ id: '', name: '全部玩家' }, ...context.snapshot.players],
      })
      if (!current) return
      const result = await request<RoundPage>(
        `/groups/${context.group.id}/rounds${query({ ...this.data.activeFilters, player: current.id, limit: 30, offset: more ? this.data.page.items.length : 0 })}`,
      )
      if (generation !== this.generation) return
      const page = more ? { ...result, items: [...this.data.page.items, ...result.items] } : result
      const commonGames = context.snapshot.games
        .map(game => ({ id: game.id, name: game.name, count: page.activity[game.id]?.count || 0 }))
        .filter(game => game.count > 0)
        .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))
        .slice(0, 3)
      this.setData({
        page,
        commonGames,
        cards: historyCards(page.items, context.snapshot, more ? this.data.cards : []),
        stats: statistics(page.stats, context.snapshot, this.data.statMode, current.id),
      })
      void this.loadPhotos(generation)
    } catch (cause) {
      if (generation === this.generation) {
        this.setData({ error: errorMessage(cause) })
        if (cause instanceof ApiError && [401, 403].includes(cause.status))
          this.setData({ current: null, snapshot: null, page: emptyHistory(), cards: [], stats: [], commonGames: [] })
      }
    } finally {
      if (generation === this.generation) this.setData({ loading: false })
    }
  },

  // loadPhotos 顺序加载授权首图，单张失败保留详情入口。
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

  // more 防止重复分页请求产生重复卡片。
  more() {
    if (!this.data.loading && this.data.page.items.length < this.data.page.total) void this.load(true)
  },

  // toggleFilters 展开筛选项。
  toggleFilters() {
    this.setData({ filterOpen: !this.data.filterOpen })
  },

  // filterInput 暂存日期或关键词，等待用户应用条件。
  filterInput(event: WechatMiniprogram.Input | WechatMiniprogram.PickerChange) {
    const field = String(event.currentTarget.dataset.field)
    if (['q', 'from', 'to'].includes(field)) this.setData({ [`filters.${field}`]: String(event.detail.value) })
  },

  // filterPicker 将原生控件索引转换成筛选值。
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

  // applyFilters 使用同一组条件刷新常玩桌游、战绩与对局列表。
  applyFilters() {
    if (this.data.filters.from && this.data.filters.to && this.data.filters.from > this.data.filters.to)
      return toast('开始日期不能晚于结束日期')
    this.setData({
      activeFilters: { ...this.data.filters },
      hasFilters: Object.values(this.data.filters).some(Boolean),
    })
    void this.load()
  },

  // clearFilters 清空临时条件，但查询仍限定当前玩家。
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

  // quick 使用北京时间的本月或本年范围。
  quick(event: WechatMiniprogram.TouchEvent) {
    const dates = quickDates(String(event.currentTarget.dataset.period))
    this.setData({ 'filters.from': dates.from, 'filters.to': dates.to })
    this.applyFilters()
  },

  // changeStatMode 每次仅展示一种模式下的个人战绩。
  changeStatMode(event: WechatMiniprogram.TouchEvent) {
    const mode = event.currentTarget.dataset.mode as Mode
    if (!this.data.snapshot || !modes.some(item => item.value === mode)) return
    this.setData({ statMode: mode, stats: statistics(this.data.page.stats, this.data.snapshot, mode, this.data.id) })
  },

  // openGame 查看常玩桌游的组内资料。
  openGame(event: WechatMiniprogram.TouchEvent) {
    navigate(`/pages/game/index?id=${encodeURIComponent(String(event.currentTarget.dataset.id))}`)
  },

  // openRound 进入玩家参加过的某条对局。
  openRound(event: WechatMiniprogram.TouchEvent) {
    navigate(`/pages/round/index?id=${encodeURIComponent(String(event.currentTarget.dataset.id))}`)
  },

  // record 为当前小组开始新记录。
  record() {
    navigate('/pages/round-form/index')
  },

  // openGroup 回到小组查看有效玩家档案。
  openGroup() {
    navigate('/pages/group/index')
  },
})
