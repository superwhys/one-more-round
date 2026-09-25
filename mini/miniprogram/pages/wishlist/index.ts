import { request, send } from '../../utils/api'
import { requireGroup } from '../../utils/session'
import type { Game, Snapshot } from '../../utils/types'
import { errorMessage, navigate, toast } from '../../utils/ui'

Page({
  data: {
    groupId: '',
    snapshot: null as Snapshot | null,
    gameIds: [] as string[],
    wishedGames: [] as Game[],
    availableGames: [] as Game[],
    selectedIndex: -1,
    selectedName: '',
    loading: true,
    busy: false,
    error: '',
  },
  generation: 0,

  // onShow 切回清单时同步其他成员修改的共享想玩状态。
  onShow() {
    void this.load()
  },

  // onUnload 忽略已经离开页面时才返回的数据。
  onUnload() {
    this.generation += 1
  },

  // onPullDownRefresh 提供原生刷新入口。
  async onPullDownRefresh() {
    await this.load()
    wx.stopPullDownRefresh()
  },

  // load 将请求失败与真正的空清单区分展示。
  async load() {
    const generation = ++this.generation
    this.setData({
      loading: true,
      error: '',
      snapshot: null,
      wishedGames: [],
      availableGames: [],
      selectedIndex: -1,
      selectedName: '',
    })
    try {
      const context = await requireGroup()
      if (!context || generation !== this.generation) return
      const result = await request<{ game_ids: string[] }>(`/groups/${context.group.id}/wishlist`)
      if (generation !== this.generation) return
      this.setData({ groupId: context.group.id, snapshot: context.snapshot, gameIds: result.game_ids || [] })
      this.updateGames()
    } catch (cause) {
      if (generation === this.generation) this.setData({ error: errorMessage(cause) })
    } finally {
      if (generation === this.generation) this.setData({ loading: false })
    }
  },

  // updateGames 将已有桌游分成已想玩和可添加两部分。
  updateGames() {
    const games = this.data.snapshot?.games || []
    this.setData({
      wishedGames: games.filter(game => this.data.gameIds.includes(game.id)),
      availableGames: games.filter(game => !this.data.gameIds.includes(game.id)),
      selectedIndex: -1,
      selectedName: '',
    })
  },

  // selectGame 记录原生选择器选中的现有桌游。
  selectGame(event: WechatMiniprogram.PickerChange) {
    const index = Number(event.detail.value)
    this.setData({ selectedIndex: index, selectedName: this.data.availableGames[index]?.name || '' })
  },

  // add 等待服务端确认后再更新共享清单。
  async add() {
    const game = this.data.availableGames[this.data.selectedIndex]
    if (!game || this.data.busy) return
    this.setData({ busy: true, error: '' })
    try {
      await send<void>(`/groups/${this.data.groupId}/wishlist/${game.id}`, {})
      this.setData({ gameIds: [...this.data.gameIds, game.id] })
      this.updateGames()
      toast('已加入想玩清单')
    } catch (cause) {
      this.setData({ error: errorMessage(cause) })
    } finally {
      this.setData({ busy: false })
    }
  },

  // remove 只移除想玩状态，不删除桌游或对局。
  async remove(event: WechatMiniprogram.TouchEvent) {
    if (this.data.busy) return
    const id = String(event.currentTarget.dataset.id)
    this.setData({ busy: true, error: '' })
    try {
      await send<void>(`/groups/${this.data.groupId}/wishlist/${id}`, {}, 'DELETE')
      this.setData({ gameIds: this.data.gameIds.filter(gameId => gameId !== id) })
      this.updateGames()
      toast('已移出想玩清单')
    } catch (cause) {
      this.setData({ error: errorMessage(cause) })
    } finally {
      this.setData({ busy: false })
    }
  },

  // openGame 查看已选择的桌游详情。
  openGame(event: WechatMiniprogram.TouchEvent) {
    navigate(`/pages/game/index?id=${encodeURIComponent(String(event.currentTarget.dataset.id))}`)
  },

  // record 沿用选中的桌游开始记局。
  record(event: WechatMiniprogram.TouchEvent) {
    navigate(`/pages/round-form/index?game=${encodeURIComponent(String(event.currentTarget.dataset.id))}`)
  },

  // openShelf 返回本组桌游架。
  openShelf() {
    wx.switchTab({ url: '/pages/games/index' })
  },
})
