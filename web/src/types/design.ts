export type Mode = 'individual' | 'team' | 'coop'
export type Outcome = 'win' | 'draw' | 'unknown' | 'loss'
export interface Player { id: string; name: string; color: string; linked: boolean }
export interface Game { id: string; name: string; english: string; theme: string; motif: string; caption: string }
export interface Team { id: string; name: string; players: string[]; score: string }
export interface Round {
  id: string; game: string; date: string; mode: Mode; players: string[]; winners: string[]
  outcome: Outcome; scores: Record<string, string>; teams: Team[]; memory: string
  photos: string[]; location: string; minutes: string; author: string; edited?: boolean
}
