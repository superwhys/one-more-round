import { request, send } from '../../utils/api'
import { requireSession, setGroupID } from '../../utils/session'
import type { NotificationPage } from '../../utils/types'
import { errorMessage, navigate } from '../../utils/ui'

// Notifications preserves server-owned read state and group context.
Page({
  data: { page: { items: [], unread: 0 } as NotificationPage, loading: false, busy: false, error: '' },
  // onShow refreshes this page when it becomes visible.
  onShow() {
    void this.load()
  },
  // load refreshes the authorized data for this page.
  async load() {
    this.setData({ loading: true, error: '' })
    try {
      if (await requireSession()) this.setData({ page: await request<NotificationPage>('/notifications') })
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ loading: false })
    }
  },
  // open opens the selected record with its group context.
  async open(event: WechatMiniprogram.TouchEvent) {
    const item = this.data.page.items.find(value => value.id === event.currentTarget.dataset.id)
    if (!item || this.data.busy) return
    this.setData({ busy: true, error: '' })
    try {
      if (!item.read_at) await send(`/notifications/${item.id}/read`, {})
      if (item.group_id) setGroupID(item.group_id)
      const round = item.link.match(/^\/rounds\/([^/?#]+)/)
      if (round)
        navigate(`/pages/round/index?group=${encodeURIComponent(item.group_id)}&id=${encodeURIComponent(round[1])}`)
      else navigate('/pages/group/index')
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
  // readAll persists read state for each unread notification.
  async readAll() {
    if (this.data.busy) return
    this.setData({ busy: true, error: '' })
    try {
      for (const item of this.data.page.items) if (!item.read_at) await send(`/notifications/${item.id}/read`, {})
      await this.load()
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
})
