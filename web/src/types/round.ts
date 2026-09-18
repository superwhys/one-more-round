export interface RoundFilters { from: string; to: string; game: string; player: string }
export interface RoundQuery extends Partial<RoundFilters> { limit?: number; offset?: number }
