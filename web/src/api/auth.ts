import { request, send } from './request'
import type { User } from '@/types/journal'
export const getCurrentUser = () => request<User>('/me')
export const sendLoginCode = (email: string, invite: string) => send<void>('/auth/code', { email, invite })
export const login = (email: string, code: string) => send<User>('/auth/login', { email, code })
export const logout = () => send<void>('/auth/logout', {})
