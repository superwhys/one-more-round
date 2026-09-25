import { query, request, send } from '../../utils/api'
import { requireGroup } from '../../utils/session'
import type { Game, Page as RoundPage, Snapshot } from '../../utils/types'
import { confirm, errorMessage, navigate, toast } from '../../utils/ui'

// ExternalGame 是后端返回的 BGG 搜索结果，不包含客户端凭据。
interface ExternalGame {
  bgg_id: number
  name: string
  year: number | null
  thumbnail?: string
}

// ExternalSearch 保留外部资料来源以便在结果中标明归属。
interface ExternalSearch {
  items: ExternalGame[]
  source: string
  source_url: string
}

// ShelfGame 提前计算 WXML 展示内容，避免在模板中调用 JavaScript。
interface ShelfGame extends Game {
  letter: string
  tone: string
  count: number
  lastDate: string
}

// shelfGames 将本组游戏与完整统计中的游玩次数合并。
function shelfGames(snapshot: Snapshot, activity: RoundPage['activity']): ShelfGame[] {
  return snapshot.games.map(game => {
    let hash = 0
    for (const character of game.id) hash = (hash * 31 + character.charCodeAt(0)) >>> 0
    return {
      ...game,
      letter: Array.from(game.name.trim())[0] || '局',
      tone: ['sage', 'clay', 'slate', 'wheat'][hash % 4]!,
      count: activity[game.id]?.count || 0,
      lastDate: activity[game.id]?.last_date || '',
    }
  })
}

Page({
  data: {
    groupId: '',
    owner: false,
    snapshot: null as Snapshot | null,
    games: [] as ShelfGame[],
    filteredGames: [] as ShelfGame[],
    search: '',
    loading: true,
    refreshing: false,
    activityLoaded: false,
    busy: false,
    error: '',
    addOpen: false,
    name: '',
    importId: 0,
    original: '',
    externalOpen: false,
    externalItems: [] as ExternalGame[],
    externalQuery: '',
    externalSearched: false,
    externalError: '',
    searching: false,
    linkingId: '',
    sourceURL: '',
  },
  searchSequence: 0,
  loadSequence: 0,

  // onShow 刷新小组，确保切组和详情修改后的桌游资料一致。
  onShow() {
    void this.load()
  },

  // onUnload 使退出页面后返回的请求失效。
  onUnload() {
    this.searchSequence += 1
    this.loadSequence += 1
  },

  // onPullDownRefresh 使用原生下拉刷新重新读取架上资料。
  async onPullDownRefresh() {
    await this.load()
    wx.stopPullDownRefresh()
  },

  // refresh 在独立滚动容器中保留原生下拉刷新交互。
  async refresh() {
    this.setData({ refreshing: true })
    try {
      await this.load()
    } finally {
      this.setData({ refreshing: false })
    }
  },

  // load 分别读取成员资料和完整游玩汇总，不根据分页记录推断次数。
  async load() {
    const sequence = ++this.loadSequence
    this.setData({ loading: true, error: '', activityLoaded: false, snapshot: null, games: [], filteredGames: [] })
    try {
      const context = await requireGroup()
      if (!context || sequence !== this.loadSequence) return
      this.setData({
        groupId: context.group.id,
        owner: context.group.owner === context.user.id,
        snapshot: context.snapshot,
        games: shelfGames(context.snapshot, {}),
      })
      this.filterGames()
      const page = await request<RoundPage>(`/groups/${context.group.id}/rounds?limit=1`)
      if (sequence !== this.loadSequence) return
      this.setData({ games: shelfGames(context.snapshot, page.activity), activityLoaded: true })
      this.filterGames()
    } catch (cause) {
      if (sequence === this.loadSequence) this.setData({ error: errorMessage(cause) })
    } finally {
      if (sequence === this.loadSequence) this.setData({ loading: false })
    }
  },

  // filterGames 先匹配本组名称与 BGG 原名，不触发外部服务。
  filterGames() {
    const term = this.data.search.trim().toLowerCase()
    this.setData({
      filteredGames: this.data.games.filter(game => `${game.name} ${game.original}`.toLowerCase().includes(term)),
    })
  },

  // onSearch 更新本地搜索词。
  onSearch(event: WechatMiniprogram.Input) {
    this.setData({ search: event.detail.value })
    this.filterGames()
  },

  // openGame 进入选中游戏的组内详情。
  openGame(event: WechatMiniprogram.TouchEvent) {
    navigate(`/pages/game/index?id=${encodeURIComponent(String(event.currentTarget.dataset.id))}`)
  },

  // openWishlist 查看全组共享的想玩清单。
  openWishlist() {
    navigate('/pages/wishlist/index')
  },

  // openRecord 为当前小组开始一条新记录。
  openRecord() {
    navigate('/pages/round-form/index')
  },

  // openAdd 保留搜索名称作为手动添加或 BGG 查询的起点。
  openAdd() {
    this.setData({ addOpen: true, name: this.data.search.trim(), importId: 0, original: '' })
  },

  // closeAdd 关闭表单，不影响已保存的架上资料。
  closeAdd() {
    if (!this.data.busy) this.setData({ addOpen: false })
  },

  // onName 收集本组习惯使用的名称。
  onName(event: WechatMiniprogram.Input) {
    this.setData({ name: event.detail.value })
  },

  // saveGame 通过同一表单保存手动条目或确认的 BGG 导入条目。
  async saveGame() {
    const name = this.data.name.trim()
    if (!name) return toast('请输入桌游名称')
    if (this.data.busy) return
    this.setData({ busy: true, error: '' })
    try {
      const base = `/groups/${this.data.groupId}/games`
      if (this.data.importId) {
        await send<Game>(`${base}/import`, { bgg_id: this.data.importId, name })
      } else {
        await send<Game>(base, { name })
      }
      this.setData({ addOpen: false, name: '', importId: 0 })
      toast('桌游已添加')
      await this.load()
    } catch (cause) {
      this.setData({ error: errorMessage(cause) })
    } finally {
      this.setData({ busy: false })
    }
  },

  // openExternal 仅在用户主动请求时搜索 BGG。
  openExternal() {
    const term = this.data.search.trim()
    if (!term) return toast('先输入要找的桌游名称')
    this.setData({ externalOpen: true, linkingId: '', externalQuery: term })
    void this.searchExternal()
  },

  // searchFromAdd 用添加表单的名称进行外部查询。
  searchFromAdd() {
    const term = this.data.name.trim()
    if (!term) return toast('先输入要找的桌游名称')
    this.setData({ addOpen: false, externalOpen: true, linkingId: '', externalQuery: term })
    void this.searchExternal()
  },

  // onExternalQuery 使旧查询结果失效，防止选择与当前输入不符的资料。
  onExternalQuery(event: WechatMiniprogram.Input) {
    this.searchSequence += 1
    this.setData({
      externalQuery: event.detail.value,
      externalItems: [],
      externalSearched: false,
      externalError: '',
      searching: false,
    })
  },

  // searchExternal 在超时或授权失败时保留手动添加入口。
  async searchExternal() {
    const term = this.data.externalQuery.trim()
    if (!term) return toast('请输入桌游名称')
    if (this.data.searching) return
    const sequence = ++this.searchSequence
    this.setData({ searching: true, externalError: '', externalItems: [], externalSearched: false })
    try {
      const result = await request<ExternalSearch>(`/groups/${this.data.groupId}/bgg/search${query({ q: term })}`, {
        timeout: 45000,
      })
      if (sequence !== this.searchSequence) return
      this.setData({ externalItems: result.items || [], sourceURL: result.source_url, externalSearched: true })
    } catch (cause) {
      if (sequence === this.searchSequence) this.setData({ externalError: errorMessage(cause) })
    } finally {
      if (sequence === this.searchSequence) this.setData({ searching: false })
    }
  },

  // closeExternal 取消界面上的搜索结果接收。
  closeExternal() {
    if (this.data.busy) return
    this.searchSequence += 1
    this.setData({ externalOpen: false, searching: false, linkingId: '' })
  },

  // manualFromExternal 外部查询不可用时仍可仅用名称记下桌游。
  manualFromExternal() {
    const name = this.data.externalQuery.trim()
    this.closeExternal()
    this.setData({ addOpen: true, name, importId: 0, original: '' })
  },

  // chooseExternal 先检测重复 BGG 条目，合并必须由组主显式确认。
  async chooseExternal(event: WechatMiniprogram.TouchEvent) {
    const selected = this.data.externalItems.find(item => item.bgg_id === Number(event.currentTarget.dataset.id))
    if (!selected || this.data.busy) return
    const source = this.data.snapshot?.games.find(game => game.id === this.data.linkingId)
    if (!source) {
      this.closeExternal()
      this.setData({ addOpen: true, importId: selected.bgg_id, original: selected.name, name: selected.name })
      return
    }
    const target = this.data.snapshot?.games.find(game => game.id !== source.id && game.bgg_id === selected.bgg_id)
    if (target && !this.data.owner) return toast('架上已有这款桌游，请联系组主合并')
    if (
      target &&
      !(await confirm(
        '合并桌游',
        `将「${source.name}」合并到「${target.name}」？历史对局和想玩状态会保留，使用已有桌游的本组名称。`,
      ))
    )
      return
    this.setData({ busy: true, externalError: '' })
    try {
      if (target) {
        await send<Game>(`/groups/${this.data.groupId}/games/${source.id}/merge`, { target_game_id: target.id })
        toast('桌游已合并，历史已保留')
      } else {
        const updated = await send<Game>(`/groups/${this.data.groupId}/games/${source.id}/cover`, {
          bgg_id: selected.bgg_id,
        })
        toast(updated.cover ? '已关联 BGG 并同步封面' : '已关联 BGG，暂无封面')
      }
      this.searchSequence += 1
      this.setData({ externalOpen: false, linkingId: '' })
      await this.load()
    } catch (cause) {
      this.setData({ externalError: errorMessage(cause) })
    } finally {
      this.setData({ busy: false })
    }
  },

  // syncCover 已关联条目直接同步，手动条目先由用户选择 BGG 结果。
  async syncCover(event: WechatMiniprogram.TouchEvent) {
    const game = this.data.snapshot?.games.find(item => item.id === event.currentTarget.dataset.id)
    if (!game || this.data.busy) return
    if (!game.bgg_id) {
      this.setData({ linkingId: game.id, externalOpen: true, externalQuery: game.name })
      await this.searchExternal()
      return
    }
    this.setData({ busy: true, error: '' })
    try {
      const updated = await send<Game>(`/groups/${this.data.groupId}/games/${game.id}/cover`, { bgg_id: game.bgg_id })
      toast(updated.cover ? '封面已经同步' : 'BGG 暂无这款桌游的封面')
      await this.load()
    } catch (cause) {
      this.setData({ error: errorMessage(cause) })
    } finally {
      this.setData({ busy: false })
    }
  },

  // copySource 提供可复制的外部资料来源，避免在小程序内跳转未配置的域名。
  copySource() {
    wx.setClipboardData({ data: this.data.sourceURL || 'https://boardgamegeek.com' })
  },

  // stopPropagation 防止点击弹层内容触发遮罩关闭。
  stopPropagation() {},
})
