import type { Mode, Page as RoundPage, Round, Snapshot, Stat } from '../../utils/types'

export const modes = [
  { value: 'individual', label: '个人竞技' },
  { value: 'team', label: '组队竞技' },
  { value: 'coop', label: '合作' },
]
export const modeOptions = [{ value: '', label: '全部模式' }, ...modes]
export const outcomeOptions = [
  { value: '', label: '全部结果' },
  { value: 'win', label: '胜利 / 成功' },
  { value: 'loss', label: '失败' },
  { value: 'draw', label: '平局' },
  { value: 'unknown', label: '未记结果' },
]
export const photoOptions = [
  { value: '', label: '照片不限' },
  { value: 'true', label: '有照片' },
  { value: 'false', label: '无照片' },
]

// HistoryFilters 与服务端列表和统计共用的筛选契约保持一致。
export interface HistoryFilters {
  from: string
  to: string
  game: string
  player: string
  q: string
  mode: string
  outcome: string
  has_photos: string
}

// HistoryCard 将模板不支持的结果计算提前完成。
export interface HistoryCard extends Round {
  gameName: string
  playerNames: string
  result: string
  modeName: string
  photoPath: string
  photoFailed: boolean
}

// StatRow 保留分游戏与模式的样本数，不生成综合战绩排名。
export interface StatRow extends Stat {
  key: string
  gameName: string
  playerName: string
  rate: string
  result: string
}

// emptyHistory 为每个页面生成独立分页状态。
export function emptyHistory(): RoundPage {
  return { activity: {}, items: [], total: 0, games: 0, players: 0, stats: [] }
}

// emptyFilters 不引入隐式日期范围或模式限制。
export function emptyFilters(): HistoryFilters {
  return { from: '', to: '', game: '', player: '', q: '', mode: '', outcome: '', has_photos: '' }
}

// roundResult 仅根据显式结果生成描述，绝不从分数推断获胜者。
function roundResult(round: Round, snapshot: Snapshot): string {
  if (round.outcome === 'unknown' || !round.outcome) return '♡ 未记结果，也是一段好时光'
  if (round.outcome === 'draw') return '＝ 全局平局'
  if (round.mode === 'coop') return round.outcome === 'win' ? '♔ 全队胜利' : '♡ 全队失败，下次再来'
  if (round.mode === 'team')
    return `♔ ${round.teams
      .filter(team => team.winner)
      .map(team => team.name)
      .join('、')}获胜`
  const names = round.winners.map(id => snapshot.players.find(player => player.id === id)?.name || '玩家').join('、')
  return `♔ ${names}${round.winners.length > 1 ? '共同获胜' : '获胜'}`
}

// historyCards 复用已加载的本地图片，但不把图片路径作为永久公开地址。
export function historyCards(items: Round[], snapshot: Snapshot, previous: HistoryCard[] = []): HistoryCard[] {
  return items.map(round => ({
    ...round,
    gameName: snapshot.games.find(game => game.id === round.game_id)?.name || '桌游',
    playerNames: round.players.map(id => snapshot.players.find(player => player.id === id)?.name || '玩家').join('、'),
    result: roundResult(round, snapshot),
    modeName: modes.find(mode => mode.value === round.mode)?.label || '',
    photoPath: previous.find(card => card.id === round.id)?.photoPath || '',
    photoFailed: false,
  }))
}

// statistics 使用完整查询统计而非当前分页列表计算胜率。
export function statistics(items: Stat[], snapshot: Snapshot, mode: Mode, playerId = ''): StatRow[] {
  return items
    .filter(item => item.mode === mode && (!playerId || item.player === playerId))
    .map(item => ({
      ...item,
      key: `${item.game}:${item.player}:${item.mode}`,
      gameName: snapshot.games.find(game => game.id === item.game)?.name || '桌游',
      playerName: snapshot.players.find(player => player.id === item.player)?.name || '玩家',
      rate: item.samples ? `${Math.round((item.wins / item.samples) * 100)}%` : '暂无数据',
      result: item.samples ? `${item.wins} ${mode === 'coop' ? '成功' : '胜'} / ${item.samples} 局` : '',
    }))
}

// filterSelection 将原生选择器索引转换为 API 值及可读标签。
export function filterSelection(
  field: string,
  index: number,
  snapshot: Snapshot,
): { value: string; label: string } | null {
  if (field === 'game') {
    const item = [{ id: '', name: '全部桌游' }, ...snapshot.games][index]
    return item ? { value: item.id, label: item.name } : null
  }
  if (field === 'player') {
    const item = [{ id: '', name: '全部玩家' }, ...snapshot.players][index]
    return item ? { value: item.id, label: item.name } : null
  }
  const options =
    field === 'mode' ? modeOptions : field === 'outcome' ? outcomeOptions : field === 'has_photos' ? photoOptions : []
  return options[index] || null
}

// quickDates 按北京时间生成本月和本年的日期语义范围。
export function quickDates(period: string): { from: string; to: string } {
  const to = new Date(Date.now() + 8 * 60 * 60 * 1000).toISOString().slice(0, 10)
  return { from: period === 'month' ? `${to.slice(0, 7)}-01` : `${to.slice(0, 4)}-01-01`, to }
}
