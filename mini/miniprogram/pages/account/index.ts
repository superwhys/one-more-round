import { send } from '../../utils/api'
import { acceptLogin, getUser, logout, requireSession } from '../../utils/session'
import type { LoginResult } from '../../utils/session'
import { confirm, errorMessage } from '../../utils/ui'

// Account links an unused email to a WeChat-only identity without merging data.
Page({
  data: { user: getUser(), email: '', code: '', busy: false, error: '', countdown: 0 },
  timer: 0 as ReturnType<typeof setInterval> | 0,
  // onShow refreshes this page when it becomes visible.
  async onShow() {
    try {
      this.setData({ user: await requireSession() })
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    }
  },
  // onUnload releases page timers before the page is destroyed.
  onUnload() {
    if (this.timer) clearInterval(this.timer)
  },
  // input updates the editable field identified by the control.
  input(event: WechatMiniprogram.Input) {
    this.setData({ [event.currentTarget.dataset.field]: event.detail.value })
  },
  // sendCode sends a limited-use email code and starts the resend countdown.
  async sendCode() {
    if (this.data.busy || this.data.countdown) return
    this.setData({ busy: true, error: '' })
    try {
      await send('/auth/code', { email: this.data.email.trim() })
      const expires = Date.now() + 60000
      this.setData({ countdown: 60 })
      this.timer = setInterval(() => {
        this.setData({ countdown: Math.max(0, Math.ceil((expires - Date.now()) / 1000)) })
        if (!this.data.countdown && this.timer) clearInterval(this.timer)
      }, 1000)
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
  // bind verifies and attaches an email without merging accounts.
  async bind() {
    if (this.data.busy) return
    this.setData({ busy: true, error: '' })
    try {
      const result = await send<LoginResult>('/auth/wx-bind-email', {
        email: this.data.email.trim(),
        code: this.data.code.trim(),
      })
      acceptLogin(result)
      this.setData({ user: getUser(), code: '' })
      wx.showToast({ title: '邮箱已绑定' })
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
  // logout confirms sign-out and revokes the current server session.
  async logout() {
    if (!(await confirm('退出登录', '将清除当前设备上的对局草稿和已下载的组内照片。'))) return
    try {
      await logout()
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    }
  },
})
