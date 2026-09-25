import { send } from '../../utils/api'
import { acceptLogin, getUser, getInvitation, setInvitation, wxCode } from '../../utils/session'
import type { LoginResult } from '../../utils/session'
import { errorMessage, invitationToken } from '../../utils/ui'

// Login lets existing members attach WeChat before a separate account is created.
Page({
  data: {
    email: '',
    emailCode: '',
    invite: '',
    groupToken: '',
    groupName: '',
    existing: false,
    busy: false,
    error: '',
    sentAt: 0,
    countdown: 0,
  },
  timer: 0 as ReturnType<typeof setInterval> | 0,
  // onLoad initializes this page from its route and pending invitation.
  onLoad(options: Record<string, string>) {
    const token = options.group_token || options.join || getInvitation()
    this.setData({ groupToken: token, invite: options.invite || '' })
    if (token) {
      setInvitation(token)
      void this.preview()
    }
  },
  // onShow refreshes this page when it becomes visible.
  onShow() {
    if (getUser()) {
      if (this.data.groupToken) wx.redirectTo({ url: '/pages/setup/index' })
      else wx.switchTab({ url: '/pages/review/index' })
    }
  },
  // onUnload releases page timers before the page is destroyed.
  onUnload() {
    if (this.timer) clearInterval(this.timer)
  },
  // input updates the editable field identified by the control.
  input(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as string
    this.setData({ [field]: event.detail.value })
  },
  // toggle switches between WeChat entry and existing-email binding.
  toggle() {
    this.setData({ existing: !this.data.existing, error: '' })
  },
  // preview validates the invitation before displaying its group.
  async preview() {
    const input = this.data.groupToken
    this.setData({ groupName: '', error: '' })
    try {
      const token = invitationToken(input)
      const preview = await send<{ name: string }>('/auth/group-invite', { token })
      if (this.data.groupToken !== input) return
      this.setData({ groupToken: token, groupName: preview.name })
      setInvitation(token)
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    }
  },
  // cancelInvite discards the pending invitation from this app visit.
  cancelInvite() {
    setInvitation('')
    this.setData({ groupToken: '', groupName: '', error: '' })
  },
  // sendCode sends a limited-use email code and starts the resend countdown.
  async sendCode() {
    if (this.data.busy || this.data.countdown) return
    this.setData({ busy: true, error: '' })
    try {
      await send('/auth/code', {
        email: this.data.email.trim(),
        invite: this.data.invite.trim(),
        group_token: invitationToken(this.data.groupToken),
      })
      this.setData({ sentAt: Date.now(), countdown: 60 })
      if (this.timer) clearInterval(this.timer)
      this.timer = setInterval(() => {
        const seconds = Math.max(0, 60 - Math.floor((Date.now() - this.data.sentAt) / 1000))
        this.setData({ countdown: seconds })
        if (!seconds && this.timer) clearInterval(this.timer)
      }, 1000)
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
  // login exchanges WeChat and optional email proofs for one app session.
  async login() {
    if (this.data.busy) return
    this.setData({ busy: true, error: '' })
    try {
      const result = await send<LoginResult>('/auth/wx-login', {
        code: await wxCode(),
        invite: this.data.invite.trim(),
        group_token: invitationToken(this.data.groupToken),
        ...(this.data.existing ? { email: this.data.email.trim(), email_code: this.data.emailCode.trim() } : {}),
      })
      acceptLogin(result)
      wx.switchTab({ url: '/pages/review/index' })
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
})
