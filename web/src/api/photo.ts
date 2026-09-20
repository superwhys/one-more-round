import { request } from './request'
import { preparePhoto } from '@/utils/photo'

// Serialize preprocessing as well as network requests, including retries.
let pendingUpload: Promise<unknown> = Promise.resolve()
export function uploadPhoto(groupId: string, file: File) {
  const result = pendingUpload.then(async () => {
    const data = new FormData()
    data.append('photo', await preparePhoto(file))
    return request<{ id: string }>(`/groups/${groupId}/photos`, { method: 'POST', body: data, signal: AbortSignal.timeout(60_000) })
  })
  pendingUpload = result.catch(() => undefined)
  return result
}
export const photoURL = (groupId: string, id: string) => `/api/v1/groups/${groupId}/photos/${id}`
