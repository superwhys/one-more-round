import { request, send } from '../../utils/api'
import { getInvitation, requireSession, setGroupID, setInvitation } from '../../utils/session'
import type { Group } from '../../utils/types'
import { errorMessage, invitationToken } from '../../utils/ui'

// Setup creates the user's table or joins a verified invitation.
Page({
  data: {
    name: '',
    playerName: '',
    token: '',
    preview: null as { name: string; group_id: string } | null,
    groups: [] as Group[],
    busy: false,
    error: '',
  },
  // onShow refreshes this page when it becomes visible.
  async onShow() {
    try {
      if (!(await requireSession())) return
      this.setData({ groups: await request<Group[]>('/groups'), token: getInvitation() || this.data.token })
      if (this.data.token) await this.preview()
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    }
  },
  // input updates the editable field identified by the control.
  input(event: WechatMiniprogram.Input) {
    this.setData({ [event.currentTarget.dataset.field]: event.detail.value, preview: null })
  },
  // preview validates the invitation before displaying its group.
  async preview() {
    this.setData({ error: '', preview: null })
    try {
      const token = invitationToken(this.data.token)
      this.setData({ token, preview: await send<{ name: string; group_id: string }>('/auth/group-invite', { token }) })
      setInvitation(token)
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    }
  },
  // select updates the selected group or filter from its native control.
  select(event: WechatMiniprogram.TouchEvent) {
    setGroupID(event.currentTarget.dataset.id)
    wx.switchTab({ url: '/pages/review/index' })
  },
  // create creates the requested resource and displays its resulting state.
  async create() {
    if (this.data.busy) return
    this.setData({ busy: true, error: '' })
    try {
      const group = await send<Group>('/groups', {
        name: this.data.name.trim(),
        player_name: this.data.playerName.trim(),
      })
      setGroupID(group.id)
      wx.switchTab({ url: '/pages/group/index' })
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
  // join joins the verified invitation and selects its group.
  async join() {
    if (this.data.busy || !this.data.preview) return
    this.setData({ busy: true, error: '' })
    try {
      const id = await send<string>('/join', { token: this.data.token })
      setGroupID(id)
      setInvitation('')
      wx.switchTab({ url: '/pages/group/index' })
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
})
