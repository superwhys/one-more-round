import { request, send } from './request'
import type { Round, Page } from '@/types/journal'
import type { RoundQuery } from '@/types/round'
export function listRounds(groupId: string, query: RoundQuery = {}) {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) if (value !== undefined) params.set(key, String(value))
  return request<Page>(`/groups/${groupId}/rounds?${params}`)
}
export const getRound = (groupId: string, id: string) => request<Round>(`/groups/${groupId}/rounds/${id}`)
export const createRound = (groupId: string, round: Round, key: string) => send<Round>(`/groups/${groupId}/rounds`, round, 'POST', key)
export const updateRound = (groupId: string, id: string, round: Round, key: string) => send<Round>(`/groups/${groupId}/rounds/${id}`, round, 'PUT', key)
export const deleteRound = (groupId: string, id: string, version: number) => send<void>(`/groups/${groupId}/rounds/${id}`, { version }, 'DELETE')
