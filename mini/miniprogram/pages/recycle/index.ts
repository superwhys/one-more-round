import { request, send } from '../../utils/api'
import { getGroupID, requireGroup } from '../../utils/session'
import type { Round } from '../../utils/types'
import { confirm, errorMessage } from '../../utils/ui'

// Recycle restores only records the current member is allowed to manage.
Page({
  data: { items: [] as (Round & { name: string; canRestore: boolean })[], loading: false, busy: false, error: '' },
  // onShow refreshes this page when it becomes visible.
  onShow() {
    void this.load()
  },
  // load refreshes the authorized data for this page.
  async load() {
    this.setData({ loading: true, error: '', items: [] })
    try {
      const current = await requireGroup()
      if (!current) return
      const items = await request<Round[]>(`/groups/${current.group.id}/rounds/recycle-bin`)
      this.setData({
        items: items.map(round => ({
          ...round,
          name: current.snapshot.games.find(game => game.id === round.game_id)?.name || '桌游',
          canRestore: round.author === current.user.id || current.group.owner === current.user.id,
        })),
      })
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ loading: false })
    }
  },
  // restore restores a deleted round using its observed version.
  async restore(event: WechatMiniprogram.TouchEvent) {
    const item = this.data.items.find(round => round.id === event.currentTarget.dataset.id)
    if (!item || this.data.busy || !(await confirm('恢复这条对局？', '恢复后将重新显示在时间线和统计中。'))) return
    this.setData({ busy: true, error: '' })
    try {
      await send(`/groups/${getGroupID()}/rounds/${item.id}/restore`, { version: item.version })
      await this.load()
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
})
