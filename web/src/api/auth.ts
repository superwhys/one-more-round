import { request, send } from './request.ts'
import type { User } from '@/types/journal'
import type { InvitePreview, LoginResult, PasswordRegistration } from '@/types/auth'
export const getCurrentUser = () => request<User>('/me')
export const sendLoginCode = (email: string) => send<void>('/auth/code', { email })
export const login = (email: string, code: string, invite = '', groupToken = '') =>
  send<LoginResult>('/auth/login', { email, code, invite: groupToken ? '' : invite, group_token: groupToken })
export const loginPassword = (identifier: string, password: string) =>
  send<LoginResult>('/auth/password/login', { identifier, password })
export const registerPassword = (input: PasswordRegistration) =>
  send<LoginResult>('/auth/password/register', { ...input, invite: input.group_token ? '' : input.invite })
export const setPassword = (username: string, password: string) =>
  send<User>('/auth/password/set', { username, password })
export const previewGroupInvite = (token: string) => send<InvitePreview>('/auth/group-invite', { token })
export const logout = () => send<void>('/auth/logout', {})
