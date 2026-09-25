import { request, send } from './request'

export interface Wishlist {
  game_ids: string[]
}

export const getWishlist = (groupId: string) => request<Wishlist>(`/groups/${groupId}/wishlist`)

export const addWishlistGame = (groupId: string, gameId: string) =>
  send<void>(`/groups/${groupId}/wishlist/${gameId}`, {})

export const removeWishlistGame = (groupId: string, gameId: string) =>
  send<void>(`/groups/${groupId}/wishlist/${gameId}`, {}, 'DELETE')
