import type { Mode, Player, Round, Snapshot } from './types'

export const modeNames: Record<Mode, string> = { individual: '个人竞技', team: '组队竞技', coop: '合作' }

// newID creates a submission identifier that stays with the persisted draft.
export function newID(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`
}

// today returns a date in Beijing independently of the device timezone.
export function today(): string {
  return new Date(Date.now() + 8 * 60 * 60 * 1000).toISOString().slice(0, 10)
}

// beijingTime formats an audit timestamp in the product's default timezone.
export function beijingTime(value: string): string {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return ''
  return new Date(date.getTime() + 8 * 60 * 60 * 1000).toISOString().slice(0, 16).replace('T', ' ')
}

// emptyRound starts a new record without reusing results, photos, or server metadata.
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
    version: 0,
    deleted_at: null,
  }
}

// copyRound isolates mutable form arrays from server responses and saved drafts.
export function copyRound(round: Round): Round {
  return JSON.parse(JSON.stringify(round)) as Round
}

// anotherRound preserves only the selected game and players from the previous record.
export function anotherRound(round: Round): Round {
  return { ...emptyRound(), game_id: round.game_id, players: [...round.players] }
}

// normalizeRound preserves an explicit zero while representing blank scores as null.
export function normalizeRound(round: Round, minutes: string): Round {
  const result = copyRound(round)
  result.memory = result.memory.trim()
  result.minutes = minutes.trim() === '' ? null : Number(minutes)
  for (const id of Object.keys(result.scores)) result.scores[id] = result.scores[id]?.trim() || null
  result.team_score = result.team_score?.trim() || null
  for (const team of result.teams) {
    team.name = team.name.trim()
    team.score = team.score?.trim() || null
  }
  return result
}

// validateRound mirrors the established domain rules before a network submission.
export function validateRound(round: Round, snapshot?: Snapshot): string {
  const parsed = new Date(`${round.date}T00:00:00Z`)
  if (
    !/^\d{4}-\d{2}-\d{2}$/.test(round.date) ||
    !Number.isFinite(parsed.getTime()) ||
    parsed.toISOString().slice(0, 10) !== round.date
  )
    return '请填写有效的对局日期'
  if (!round.game_id || (snapshot && !snapshot.games.some(game => game.id === round.game_id)))
    return '请从本组选择桌游，或先手动添加'
  if (!round.players.length) return '请至少选择一位玩家'
  if (new Set(round.players).size !== round.players.length || round.players.some(id => !id)) return '玩家不能重复'
  if (snapshot && round.players.some(id => !snapshot.players.some(player => player.id === id)))
    return '部分玩家已不在本组，请重新选择'
  if (Array.from(round.memory).length > 500) return '回忆最多 500 字'
  if (round.photos.length > 3 || new Set(round.photos).size !== round.photos.length || round.photos.some(id => !id))
    return '照片最多 3 张，不能重复'
  if (round.minutes !== null && (!Number.isInteger(round.minutes) || round.minutes <= 0)) return '时长必须为正整数分钟'
  const validScore = (value: string | null) => value === null || /^-?(0|[1-9][0-9]{0,11})(\.[0-9]{1,4})?$/.test(value)
  if (
    !validScore(round.team_score) ||
    Object.keys(round.scores).some(id => !round.players.includes(id) || !validScore(round.scores[id]))
  )
    return '分数支持负数，最多 12 位整数和 4 位小数'
  if (!round.outcome) return '请选择本局结果'
  if (round.mode === 'individual') {
    if (round.players.length < 2) return '个人竞技至少两位玩家；单人挑战请选合作'
    if (!['win', 'draw', 'unknown'].includes(round.outcome) || round.teams.length || round.team_score !== null)
      return '个人竞技结果不合法'
    if (
      (round.outcome === 'win') !== round.winners.length > 0 ||
      new Set(round.winners).size !== round.winners.length ||
      round.winners.some(id => !round.players.includes(id))
    )
      return '请选择获胜玩家，或清空不适用的结果'
  } else if (round.mode === 'coop') {
    if (
      !['win', 'loss', 'unknown'].includes(round.outcome) ||
      round.teams.length ||
      round.winners.length ||
      Object.keys(round.scores).length
    )
      return '合作模式请记录全队结果与团队分数'
  } else if (round.mode === 'team') {
    if (
      round.teams.length < 2 ||
      round.team_score !== null ||
      Object.keys(round.scores).length ||
      round.winners.length ||
      !['win', 'draw', 'unknown'].includes(round.outcome)
    )
      return '组队至少两队，结果和分数需填写在队伍上'
    const assigned: string[] = []
    const teamIDs = new Set<string>()
    for (const team of round.teams) {
      if (!team.id || teamIDs.has(team.id) || !team.name.trim() || !team.players.length)
        return '每队需要名称和至少一位玩家'
      if (!validScore(team.score)) return '队伍分数支持负数，最多 12 位整数和 4 位小数'
      teamIDs.add(team.id)
      assigned.push(...team.players)
    }
    if (
      assigned.length !== round.players.length ||
      new Set(assigned).size !== assigned.length ||
      assigned.some(id => !round.players.includes(id))
    )
      return '每位玩家必须恰好属于一队'
    if ((round.outcome === 'win') !== round.teams.some(team => team.winner)) return '请选择获胜队伍，或清空不适用的结果'
  } else return '请选择有效的玩法'
  return ''
}

// resultLabel renders results without deriving a winner from optional scores.
export function resultLabel(round: Round, players: Player[]): string {
  if (round.outcome === 'unknown') return '未记结果，也是一段好时光'
  if (round.outcome === 'draw') return '全局平局'
  if (round.mode === 'coop') return round.outcome === 'win' ? '全队胜利' : '全队失败，下次再来'
  if (round.mode === 'team')
    return `${round.teams
      .filter(team => team.winner)
      .map(team => team.name)
      .join('、')}获胜`
  return `${round.winners.map(id => players.find(player => player.id === id)?.name || '玩家').join('、')}${round.winners.length > 1 ? '共同获胜' : '获胜'}`
}
