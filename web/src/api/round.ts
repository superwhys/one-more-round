import { ApiError, request, send } from './request'
import type { Round, Page, Recap, PublicRound, RoundShareStatus, RoundShareToken } from '@/types/journal'
import type { RoundQuery } from '@/types/round'
export function listRounds(groupId: string, query: RoundQuery = {}) {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) if (value !== undefined) params.set(key, String(value))
  return request<Page>(`/groups/${groupId}/rounds?${params}`)
}
export const getRound = (groupId: string, id: string) => request<Round>(`/groups/${groupId}/rounds/${id}`)
export const createRound = (groupId: string, round: Round, key: string) =>
  send<Round>(`/groups/${groupId}/rounds`, round, 'POST', key)
export const updateRound = (groupId: string, id: string, round: Round, key: string) =>
  send<Round>(`/groups/${groupId}/rounds/${id}`, round, 'PUT', key)
export const deleteRound = (groupId: string, id: string, version: number) =>
  send<void>(`/groups/${groupId}/rounds/${id}`, { version }, 'DELETE')
export const getRecap = (groupId: string, period: string) =>
  request<Recap>(`/groups/${groupId}/rounds/recap?${new URLSearchParams({ period })}`)
export const listRecycleBin = (groupId: string) => request<Round[]>(`/groups/${groupId}/rounds/recycle-bin`)
export const restoreRound = (groupId: string, id: string, version: number) =>
  send<Round>(`/groups/${groupId}/rounds/${id}/restore`, { version })
export const getRoundShareStatus = (groupId: string, id: string) =>
  request<RoundShareStatus>(`/groups/${groupId}/rounds/${id}/share`)
export const createRoundShare = (groupId: string, id: string) =>
  send<RoundShareToken>(`/groups/${groupId}/rounds/${id}/share`, {})
export const revokeRoundShare = (groupId: string, id: string) =>
  send<void>(`/groups/${groupId}/rounds/${id}/share`, {}, 'DELETE')
export const getPublicRound = (token: string) =>
  request<PublicRound>('/shared-rounds', { headers: { 'X-Round-Share': token } })
export async function getPublicRoundPhoto(token: string, id: string): Promise<Blob> {
  let response: Response
  try {
    response = await fetch(`/api/v1/shared-rounds/photos/${encodeURIComponent(id)}`, {
      headers: { Accept: 'image/jpeg', 'X-Round-Share': token },
      signal: AbortSignal.timeout(10_000),
    })
  } catch {
    throw new ApiError('照片加载中断或超时，请重试', 0)
  }
  if (response.ok) return response.blob()
  let error = '照片无法访问'
  try {
    const payload = (await response.json()) as { message?: unknown }
    if (typeof payload.message === 'string') error = payload.message
  } catch {
    /* Keep the transport fallback for non-JSON failures. */
  }
  throw new ApiError(error, response.status)
}
