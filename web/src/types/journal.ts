export interface User { id: string; email: string }
export interface Group { id: string; name: string; owner: string }
export interface Player { id: string; name: string; account: string | null }
export interface Game { id: string; name: string; original: string; bgg_id: number | null }
export interface Member { user_id: string; email: string }
export interface Snapshot { locations: string[]; group: Group; members: Member[]; players: Player[]; games: Game[]; claims: { user_id: string; player_id: string }[] }
export type Mode = 'individual' | 'team' | 'coop'
export type Outcome = 'win' | 'loss' | 'draw' | 'unknown'
export interface Team { id: string; name: string; players: string[]; score: string | null; winner: boolean }
export interface Round {
  id: string; game_id: string; date: string; mode: Mode; outcome: Outcome | ''; players: string[]; winners: string[]
  scores: Record<string, string | null>; teams: Team[]; team_score: string | null
  memory: string; location: string; minutes: number | null; photos: string[]
  author: string; updated_by: string; updated_at: string; version: number
  deleted_at: string | null
}
export interface Stat { game: string; player: string; mode: Mode; played: number; wins: number; samples: number }
export interface Page { activity: Record<string, { count: number; last_date: string }>; items: Round[]; total: number; games: number; players: number; stats: Stat[] }
export interface Invite { id: string; expires: string; revoked: boolean }
export interface Recap {
  period: string; from: string; to: string; rounds: number; games: number; players: number; minutes: number
  top_game: string; top_game_rounds: number; top_player: string; top_plays: number; photos: string[]
}
export interface Notification { id: string; group_id: string; kind: string; title: string; body: string; link: string; created: string; read_at: string | null }
export interface NotificationPage { items: Notification[]; unread: number }
export interface RoundShareStatus { active: boolean; created_at: string }
export interface RoundShareToken { token: string; created_at: string }
export interface PublicPlayer { name: string; score: string | null; winner: boolean }
export interface PublicTeam { name: string; players: string[]; score: string | null; winner: boolean }
export interface PublicRound {
  group_name: string; game_name: string; date: string; mode: Mode; outcome: Outcome
  players: PublicPlayer[]; teams: PublicTeam[]; team_score: string | null
  memory: string; photos: string[]
}
