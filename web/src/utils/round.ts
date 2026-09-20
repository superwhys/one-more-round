import type { Mode, Round, Player } from '@/types/journal'
import { today } from './date'

export const modeNames: Record<Mode, string> = { individual: '个人竞技', team: '组队竞技', coop: '合作' }
export function emptyRound(): Round {
  return {
    id: '',
    game_id: '',
    date: today(),
    mode: 'individual',
    outcome: '',
    players: [],
    winners: [],
    scores: {},
    teams: [],
    team_score: null,
    memory: '',
    minutes: null,
    photos: [],
    author: '',
    updated_by: '',
    updated_at: '0001-01-01T00:00:00Z',
    deleted_at: null,
    version: 0,
  }
}
export function resultLabel(r: Round, players: Player[]): string {
  if (r.outcome === 'unknown') return '♡ 未记结果，也是一段好时光'
  if (r.outcome === 'draw') return '＝ 全局平局'
  if (r.mode === 'coop') return r.outcome === 'win' ? '♔ 全队胜利' : '♡ 全队失败，下次再来'
  if (r.mode === 'team')
    return `♔ ${r.teams
      .filter(t => t.winner)
      .map(t => t.name)
      .join('、')}获胜`
  return `♔ ${r.winners.map(id => players.find(p => p.id === id)?.name ?? '玩家').join('、')}${r.winners.length > 1 ? '共同获胜' : '获胜'}`
}
