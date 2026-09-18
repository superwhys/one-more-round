import { reactive } from 'vue'

import type { Player, Game, Round, Mode } from '@/types/design'
import { today, dateLabel } from '@/utils/date'
export { today, dateLabel }

export const players = reactive<Player[]>([
  { id: 'lin', name: '阿林', color: '#e4b99b', linked: true },
  { id: 'zhou', name: '小周', color: '#b6c4af', linked: true },
  { id: 'keke', name: '可可', color: '#d9b8bc', linked: true },
  { id: 'yuan', name: '阿远', color: '#b1c2cc', linked: false },
  { id: 'orange', name: '橙子', color: '#dfc887', linked: false },
  { id: 'xia', name: '小夏', color: '#c5bfd4', linked: true },
])

export const games = reactive<Game[]>([
  { id: 'splendor', name: '璀璨宝石', english: 'SPLENDOR', theme: 'gem', motif: '◇', caption: '一点运气，一点小心机。' },
  { id: 'azul', name: '花砖物语', english: 'AZUL', theme: 'tile', motif: '✥', caption: '把好时光，拼成漂亮的样子。' },
  { id: 'codenames', name: '行动代号', english: 'CODENAMES', theme: 'code', motif: '⌁', caption: '看看谁最懂你的暗号。' },
  { id: 'pandemic', name: '瘟疫危机', english: 'PANDEMIC', theme: 'leaf', motif: '✳', caption: '这一次，我们站在同一边。' },
  { id: 'catan', name: '卡坦岛', english: 'CATAN', theme: 'sun', motif: '☀', caption: '路还很长，朋友就在桌边。' },
])

const makeRound = (round: Partial<Round> & Pick<Round, 'id' | 'game' | 'date'>): Round => ({
  mode: 'individual', players: ['lin', 'zhou', 'keke', 'yuan'], winners: ['keke'], outcome: 'win',
  scores: {}, teams: [], memory: '', photos: [], location: '阿林家的餐桌', minutes: '', author: 'lin', ...round,
})

export const rounds = reactive<Round[]>([
  makeRound({ id: 'r1', game: 'splendor', date: '2026-09-18', scores: { lin: '12', zhou: '13', keke: '16', yuan: '11' }, minutes: '35', memory: '以为胜券在握，结果可可最后一张牌直接翻盘。阿远说这局不算，必须再来一局。', photos: ['/design-assets/table-memory.png'] }),
  makeRound({ id: 'r2', game: 'azul', date: '2026-09-18', players: ['lin', 'zhou', 'keke'], winners: ['lin', 'zhou'], scores: { lin: '72', zhou: '72', keke: '65' }, minutes: '40', memory: '人生第一次和小周共同获胜，值得纪念。今天大家拼的花砖都很好看。' }),
  makeRound({ id: 'r3', game: 'codenames', date: '2026-09-12', mode: 'team', players: ['lin', 'zhou', 'keke', 'yuan', 'orange', 'xia'], winners: ['lin', 'keke', 'orange'], teams: [{ id: 'a', name: '红队', players: ['lin', 'keke', 'orange'], score: '8' }, { id: 'b', name: '蓝队', players: ['zhou', 'yuan', 'xia'], score: '6' }], minutes: '25', memory: '“月亮，三个。”一个词让大家讨论了整整五分钟。默契这件事，还是要多练练。', location: '巷口桌游小馆' }),
  makeRound({ id: 'r4', game: 'pandemic', date: '2026-09-12', mode: 'coop', winners: [], outcome: 'win', minutes: '55', memory: '最后一回合惊险获胜！这一次，所有人都是 MVP。', location: '巷口桌游小馆' }),
  makeRound({ id: 'r5', game: 'catan', date: '2026-09-05', winners: [], outcome: 'unknown', players: ['lin', 'zhou', 'yuan', 'orange'], minutes: '80', memory: '忘了谁赢，只记得那天笑得很大声。', location: '橙子家' }),
  makeRound({ id: 'r6', game: 'splendor', date: '2026-09-05', winners: ['lin'], players: ['lin', 'keke', 'orange'], minutes: '30', memory: '饭后消食的一局，刚刚好。', location: '橙子家' }),
  makeRound({ id: 'r7', game: 'azul', date: '2026-08-29', winners: [], outcome: 'draw', players: ['lin', 'zhou'], minutes: '25', memory: '一场难得的平局。下次见分晓。' }),
])

export const modeNames: Record<Mode, string> = { individual: '个人竞技', team: '组队竞技', coop: '合作' }
export const playerName = (id: string) => players.find((p) => p.id === id)?.name ?? '未知玩家'
export const gameById = (id: string) => games.find((game) => game.id === id) ?? games[0]!
export const gameRounds = (id: string) => rounds.filter((round) => round.game === id)
export function resultLabel(round: Round): string {
  if (round.outcome === 'unknown') return '未记结果，也是一段好时光'
  if (round.outcome === 'draw') return '全局平局'
  if (round.mode === 'coop') return round.outcome === 'win' ? '全队胜利' : '全队失败，下次再来'
  if (round.mode === 'team') return `${round.teams.find((team) => team.players.some((id) => round.winners.includes(id)))?.name ?? '队伍'}获胜`
  return `${round.winners.map(playerName).join('、')}${round.winners.length > 1 ? '共同获胜' : '获胜'}`
}

export function performance(list: Round[], playerId?: string) {
  const valid = list.filter((r) => r.outcome !== 'unknown' && (!playerId || r.players.includes(playerId)))
  const wins = valid.filter((r) => r.mode === 'coop' ? r.outcome === 'win' : !!playerId && r.winners.includes(playerId)).length
  return { total: valid.length, wins, rate: valid.length ? Math.round(wins / valid.length * 100) : null }
}

export const demoSession = reactive({ group: '周五不散场', joined: false, invited: false })
