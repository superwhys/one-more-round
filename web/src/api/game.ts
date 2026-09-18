import { request, send } from './request'
import type { Game } from '@/types/journal'
export const addGame = (groupId: string, name: string) => send<Game>(`/groups/${groupId}/games`, { name })
export const searchBGG = (groupId: string, query: string) => request<unknown>(`/groups/${groupId}/bgg/search?q=${encodeURIComponent(query)}`)
