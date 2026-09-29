import { request, send } from './request.ts'
import type { User } from '@/types/journal'
import type { InvitePreview, LoginResult } from '@/types/auth'
export const getCurrentUser = () => request<User>('/me')
export const sendLoginCode = (email: string) => send<void>('/auth/code', { email })
export const login = (email: string, code: string, invite = '', groupToken = '') =>
  send<LoginResult>('/auth/login', { email, code, invite: groupToken ? '' : invite, group_token: groupToken })
export const previewGroupInvite = (token: string) => send<InvitePreview>('/auth/group-invite', { token })
export const logout = () => send<void>('/auth/logout', {})
