import { request, send } from './request'
import type { User } from '@/types/journal'
import type { InvitePreview, LoginResult } from '@/types/auth'
export const getCurrentUser = () => request<User>('/me')
export const sendLoginCode = (email: string, invite: string, groupToken = '') => send<void>('/auth/code', { email, invite: groupToken ? '' : invite, group_token: groupToken })
export const login = (email: string, code: string, groupToken = '') => send<LoginResult>('/auth/login', { email, code, group_token: groupToken })
export const previewGroupInvite = (token: string) => send<InvitePreview>('/auth/group-invite', { token })
export const logout = () => send<void>('/auth/logout', {})
