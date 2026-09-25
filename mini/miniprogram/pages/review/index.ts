import { query, request } from '../../utils/api'
import { requireGroup } from '../../utils/session'
import type { Group, Page as RoundPage, Round, Snapshot } from '../../utils/types'
import { getPhoto } from '../../utils/photo'
import { errorMessage, navigate } from '../../utils/ui'

interface Card extends Round {
  gameName: string
  playerNames: string
  result: string
  image: string
}
// Review displays the same filtered timeline and statistics as the web app.
Page({
  data: {
    group: null as Group | null,
    snapshot: null as Snapshot | null,
    items: [] as Card[],
    total: 0,
    gameCount: 0,
    playerCount: 0,
    loading: false,
    refreshing: false,
    error: '',
    filterOpen: false,
    photoOptions: ['照片不限', '有照片', '无照片'],
    photoIndex: 0,
    filters: { from: '', to: '', game: '', player: '', q: '', mode: '', outcome: '', has_photos: '' },
    games: ['全部桌游'],
    players: ['全部玩家'],
    gameIndex: 0,
    playerIndex: 0,
    modes: ['全部模式', '个人竞技', '组队竞技', '合作'],
    outcomes: ['全部结果', '胜利', '失败', '平局', '未记结果'],
    modeIndex: 0,
    outcomeIndex: 0,
  },
  generation: 0,
  activeFilters: { from: '', to: '', game: '', player: '', q: '', mode: '', outcome: '', has_photos: '' },
  // onShow refreshes this page when it becomes visible.
  onShow() {
    void this.load()
  },
  // onHide invalidates private content when leaving this page.
  onHide() {
    this.generation++
    this.setData({ items: [], loading: false, refreshing: false })
  },
  // onPullDownRefresh refreshes the timeline and finishes the native pull gesture.
  onPullDownRefresh() {
    this.setData({ refreshing: true })
    void this.load().finally(() => this.setData({ refreshing: false }))
  },
  // onReachBottom loads the next page only when more records exist.
  onReachBottom() {
    if (!this.data.loading && this.data.items.length < this.data.total) void this.load(true)
  },
  // load refreshes the authorized data for this page.
  async load(more = false) {
    if (this.data.loading) return
    const generation = ++this.generation
    this.setData({ loading: true, error: '' })
    try {
      const current = await requireGroup()
      if (!current || generation !== this.generation) return
      const changed = this.data.group?.id !== current.group.id
      if (changed)
        this.setData({
          items: [],
          filters: { from: '', to: '', game: '', player: '', q: '', mode: '', outcome: '', has_photos: '' },
          gameIndex: 0,
          playerIndex: 0,
          modeIndex: 0,
          outcomeIndex: 0,
          photoIndex: 0,
        })
      if (!more || changed) this.activeFilters = { ...this.data.filters }
      const offset = more && !changed ? this.data.items.length : 0
      const page = await request<RoundPage>(
        `/groups/${current.group.id}/rounds${query({ ...this.activeFilters, limit: 20, offset })}`,
      )
      if (generation !== this.generation) return
      const cards = page.items.map(round => ({
        ...round,
        gameName: current.snapshot.games.find(game => game.id === round.game_id)?.name || '桌游',
        playerNames: round.players
          .map(id => current.snapshot.players.find(player => player.id === id)?.name || '玩家')
          .join(' · '),
        result: resultLabel(round, current.snapshot),
        image: '',
      }))
      this.setData({
        group: current.group,
        snapshot: current.snapshot,
        items: offset ? [...this.data.items, ...cards] : cards,
        total: page.total,
        gameCount: page.games,
        playerCount: page.players,
        games: ['全部桌游', ...current.snapshot.games.map(game => game.name)],
        players: ['全部玩家', ...current.snapshot.players.map(player => player.name)],
      })
      for (let index = 0; index < cards.length; index++) {
        if (!cards[index].photos.length) continue
        try {
          const image = await getPhoto(current.group.id, cards[index].photos[0])
          if (generation === this.generation) this.setData({ [`items[${offset + index}].image`]: image })
        } catch {
          /* The card remains accessible if its photo is unavailable. */
        }
      }
    } catch (error) {
      if (generation === this.generation) this.setData({ error: errorMessage(error) })
    } finally {
      if (generation === this.generation) this.setData({ loading: false })
    }
  },
  // input updates the editable field identified by the control.
  input(event: WechatMiniprogram.Input) {
    this.setData({ [`filters.${event.currentTarget.dataset.field}`]: event.detail.value })
  },
  // select updates the selected group or filter from its native control.
  select(event: WechatMiniprogram.PickerChange) {
    const field = event.currentTarget.dataset.field as 'game' | 'player' | 'mode' | 'outcome'
    const index = Number(event.detail.value)
    const values =
      field === 'game'
        ? this.data.snapshot?.games.map(value => value.id) || []
        : field === 'player'
          ? this.data.snapshot?.players.map(value => value.id) || []
          : field === 'mode'
            ? ['individual', 'team', 'coop']
            : ['win', 'loss', 'draw', 'unknown']
    this.setData({ [`filters.${field}`]: index ? values[index - 1] : '', [`${field}Index`]: index })
  },
  // photos sets the explicit photo-presence filter.
  photos(event: WechatMiniprogram.PickerChange) {
    const index = Number(event.detail.value)
    this.setData({ photoIndex: index, 'filters.has_photos': ['', 'true', 'false'][index] })
  },
  // clearFilters restores the unfiltered timeline and its statistics.
  clearFilters() {
    this.setData({
      filters: { from: '', to: '', game: '', player: '', q: '', mode: '', outcome: '', has_photos: '' },
      gameIndex: 0,
      playerIndex: 0,
      modeIndex: 0,
      outcomeIndex: 0,
      photoIndex: 0,
    })
    void this.load()
  },
  // filter toggles the timeline filter editor.
  filter() {
    this.setData({ filterOpen: !this.data.filterOpen })
  },
  // search applies the selected timeline filters.
  search() {
    void this.load()
  },
  // open opens the selected record with its group context.
  open(event: WechatMiniprogram.TouchEvent) {
    navigate(`/pages/round/index?group=${this.data.group?.id}&id=${event.currentTarget.dataset.id}`)
  },
  // go opens the page selected by this navigation control.
  go(event: WechatMiniprogram.TouchEvent) {
    navigate(event.currentTarget.dataset.url)
  },
  // record opens the new-round form for the current group.
  record() {
    navigate('/pages/round-form/index')
  },
})
// resultLabel expresses outcomes in text so result does not rely on color.
function resultLabel(round: Round, snapshot: Snapshot): string {
  if (round.outcome === 'unknown') return '♡ 未记结果'
  if (round.outcome === 'draw') return '＝ 全局平局'
  if (round.mode === 'coop') return round.outcome === 'win' ? '♔ 全队胜利' : '♡ 全队失败'
  const names =
    round.mode === 'team'
      ? round.teams.filter(team => team.winner).map(team => team.name)
      : round.winners.map(id => snapshot.players.find(player => player.id === id)?.name || '玩家')
  return `♔ ${names.join('、')}${names.length > 1 ? '共同获胜' : '获胜'}`
}
