export interface RoundFilters {
  from: string; to: string; game: string; player: string; q: string; location: string; mode: string; outcome: string; has_photos: string
}
export interface RoundQuery extends Partial<RoundFilters> { limit?: number; offset?: number }
