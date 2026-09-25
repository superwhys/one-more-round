import { ApiError, request, send } from './api'
import { getToken, setToken } from './credential'
import { clearPhotos } from './photo'
import type { User, Group, Snapshot } from './types'

export interface LoginResult extends User {
  token: string
  group_id?: string
}
let user: User | null = null
let groupID = ''
let pendingGroupToken = ''
// setInvitation remembers only the current app visit's invitation.
export function setInvitation(token: string): void {
  pendingGroupToken = token
}
// getInvitation reads the pending invitation for this app visit.
export function getInvitation(): string {
  return pendingGroupToken
}
// getUser returns the account only while its in-memory session exists.
export function getUser(): User | null {
  return getToken() ? user : null
}
// getGroupID returns the selected group identifier.
export function getGroupID(): string {
  return groupID
}
// setGroupID selects a group and remembers the account-scoped preference.
export function setGroupID(id: string): void {
  groupID = id
  if (user) wx.setStorageSync(`omr:group:${user.id}`, id)
}
// acceptLogin switches the current identity without persisting its credential.
export function acceptLogin(result: LoginResult): void {
  if (user && user.id !== result.id) clearSession()
  setToken(result.token)
  user = { id: result.id, email: result.email }
  groupID = result.group_id || wx.getStorageSync(`omr:group:${result.id}`) || ''
  pendingGroupToken = ''
}
// clearSession removes account-scoped drafts and private downloaded images.
export function clearSession(): void {
  setToken('')
  user = null
  groupID = ''
  pendingGroupToken = ''
  for (const key of wx.getStorageInfoSync().keys) if (key.startsWith('omr:draft:')) wx.removeStorageSync(key)
  clearPhotos()
}
// wxCode obtains a fresh, short-lived WeChat login proof.
export function wxCode(): Promise<string> {
  return new Promise((resolve, reject) =>
    wx.login({
      timeout: 10000,
      success: res => (res.code ? resolve(res.code) : reject(new Error('未能获取微信登录凭证'))),
      fail: () => reject(new Error('微信登录失败，请重试')),
    }),
  )
}
// requireSession validates the active account before opening protected pages.
export async function requireSession(): Promise<User | null> {
  if (!getToken()) {
    if (user) clearSession()
    user = null
    wx.reLaunch({ url: '/pages/login/index' })
    return null
  }
  try {
    user = await request<User>('/me')
    return user
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      clearSession()
      wx.reLaunch({ url: '/pages/login/index' })
      return null
    }
    throw error
  }
}
// requireGroup always refreshes membership and group-scoped resource IDs.
export async function requireGroup(): Promise<{ user: User; group: Group; snapshot: Snapshot } | null> {
  const current = await requireSession()
  if (!current) return null
  const groups = await request<Group[]>('/groups')
  const selected = groups.find(group => group.id === groupID) || groups[0]
  if (!selected) {
    setGroupID('')
    wx.navigateTo({ url: '/pages/setup/index' })
    return null
  }
  setGroupID(selected.id)
  const snapshot = await request<Snapshot>(`/groups/${selected.id}`)
  return { user: current, group: selected, snapshot }
}
// logout revokes the server session before clearing local account data.
export async function logout(): Promise<void> {
  await send('/auth/logout', {})
  clearSession()
  wx.reLaunch({ url: '/pages/login/index' })
}
