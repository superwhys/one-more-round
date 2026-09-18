import { request } from './request'
export function uploadPhoto(groupId: string, file: File) {
  const data = new FormData()
  data.append('photo', file)
  return request<{ id: string }>(`/groups/${groupId}/photos`, { method: 'POST', body: data, signal: AbortSignal.timeout(60_000) })
}
export const photoURL = (groupId: string, id: string, thumbnail = false) => `/api/v1/groups/${groupId}/photos/${id}${thumbnail ? '?size=thumb' : ''}`
