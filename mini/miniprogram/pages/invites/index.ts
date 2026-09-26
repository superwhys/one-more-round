import { request, send } from '../../utils/api'
import { getGroupID, requireGroup } from '../../utils/session'
import type { Invite } from '../../utils/types'
import { confirm, errorMessage, invitationToken } from '../../utils/ui'
import { appShare } from '../../utils/share'

// Invites exposes explicitly created, revocable invitations to the group owner.
Page({
  data: {
    items: [] as (Invite & { date: string })[],
    url: '',
    token: '',
    name: '',
    owner: false,
    busy: false,
    error: '',
  },
  // onShow refreshes this page when it becomes visible.
  onShow() {
    wx.showShareMenu({ menus: ['shareAppMessage'] })
    void this.load()
  },
  // load refreshes the authorized data for this page.
  async load() {
    try {
      const current = await requireGroup()
      if (!current) return
      if (current.group.owner !== current.user.id) throw new Error('只有组主可以管理邀请')
      this.setData({
        owner: true,
        name: current.group.name,
        items: (await request<Invite[]>(`/groups/${getGroupID()}/invites`)).map(invite => ({
          ...invite,
          date: invite.expires.slice(0, 10),
        })),
      })
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    }
  },
  // create creates the requested resource and displays its resulting state.
  async create() {
    if (
      this.data.busy ||
      !(await confirm('生成邀请', '链接 7 天有效，可多人使用。获得或被转发邀请的人都能加入，请只分享给信任的朋友。'))
    )
      return
    this.setData({ busy: true, error: '' })
    try {
      const result = await send<{ url: string }>(`/groups/${getGroupID()}/invites`, {})
      this.setData({ url: result.url, token: invitationToken(result.url) })
      await this.load()
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
  // revoke revokes an invitation after confirming its effect.
  async revoke(event: WechatMiniprogram.TouchEvent) {
    if (this.data.busy || !(await confirm('撤销邀请', '此邀请将立即失效，已经加入的成员不受影响。'))) return
    this.setData({ busy: true, error: '' })
    try {
      await send(`/groups/${getGroupID()}/manage`, { action: 'revoke', target: event.currentTarget.dataset.id })
      this.setData({ url: '', token: '' })
      await this.load()
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
  // copy copies only the invitation currently shown to its creator.
  copy() {
    if (this.data.url) wx.setClipboardData({ data: this.data.url })
  },
  // onShareAppMessage keeps the menu on the home card and the explicit button on its invitation.
  onShareAppMessage(options: WechatMiniprogram.Page.IShareAppMessageOption) {
    if (options.from !== 'button' || !this.data.token) return appShare()
    return {
      title: `来「${this.data.name}」，一起记下每一局`,
      imageUrl: '/pages/round/share-card.png',
      path: `/pages/login/index?group_token=${encodeURIComponent(this.data.token)}`,
    }
  },
})
