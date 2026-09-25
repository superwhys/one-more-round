import { request, send } from './request'
import type { Game } from '@/types/journal'

export interface ExternalGame {
  bgg_id: number
  name: string
  year: number | null
  thumbnail?: string
}

export interface ExternalSearch {
  items: ExternalGame[]
  source: string
  source_url: string
}

export const addGame = (groupId: string, name: string) => send<Game>(`/groups/${groupId}/games`, { name })
export const searchBGG = (groupId: string, query: string) =>
  request<ExternalSearch>(`/groups/${groupId}/bgg/search?q=${encodeURIComponent(query)}`, {
    signal: AbortSignal.timeout(45_000),
  })
export const importGame = (groupId: string, bggId: number, name: string) =>
  send<Game>(`/groups/${groupId}/games/import`, { bgg_id: bggId, name })
export const syncCover = (groupId: string, gameId: string, bggId: number) =>
  send<Game>(`/groups/${groupId}/games/${gameId}/cover`, { bgg_id: bggId })
export const mergeGame = (groupId: string, sourceGameId: string, targetGameId: string) =>
  send<Game>(`/groups/${groupId}/games/${sourceGameId}/merge`, { target_game_id: targetGameId })
