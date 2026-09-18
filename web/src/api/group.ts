import { request, send } from './request'
import type { Group, Snapshot, Invite, Player } from '@/types/journal'
import type { GroupAction } from '@/types/group'
export const listGroups = () => request<Group[]>('/groups')
export const createGroup = (name: string) => send<Group>('/groups', { name })
export const joinGroup = (token: string) => send<string>('/join', { token })
export const getGroup = (id: string) => request<Snapshot>(`/groups/${id}`)
export const addPlayer = (id: string, name: string) => send<Player>(`/groups/${id}/players`, { name })
export const manageGroup = (id: string, action: GroupAction, target = '', value = '') => send<void>(`/groups/${id}/manage`, { action, target, value })
export const listInvites = (id: string) => request<Invite[]>(`/groups/${id}/invites`)
export const createInvite = (id: string) => send<{ url: string }>(`/groups/${id}/invites`, {})
