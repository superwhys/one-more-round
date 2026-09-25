import { query, request, send } from '../../utils/api'
import { API_ORIGIN } from '../../utils/config'
import { getGroupID, getUser, requireGroup, setGroupID } from '../../utils/session'
import { confirm, errorMessage } from '../../utils/ui'
import { getPhoto, getPublicPhoto } from '../../utils/photo'
import type {
  PublicRound,
  Round,
  RoundComment,
  RoundCommentPage,
  RoundShareStatus,
  RoundShareToken,
  Snapshot,
  Player,
  Member,
} from '../../utils/types'
import { beijingTime, modeNames, newID, resultLabel } from '../../utils/round-form'

// CommentView adds display names and a single nesting level to comment responses.
interface CommentView extends RoundComment {
  name: string
  time: string
  removable: boolean
  replies: CommentView[]
}
// DetailView contains only the fields required by both private and public detail templates.
interface DetailView {
  game: string
  group: string
  date: string
  mode: string
  result: string
  players: { id: string; name: string; score: string | null; winner: boolean }[]
  teams: { name: string; names: string; score: string | null; winner: boolean }[]
  teamScore: string | null
  memory: string
  minutes: number | null
  author: string
  updated: string
  gameID: string
}

Page({
  data: {
    loading: true,
    error: '',
    photoError: '',
    busy: false,
    editable: false,
    public: false,
    detail: null as DetailView | null,
    photos: [] as { id: string; path: string }[],
    comments: [] as CommentView[],
    commentsLoading: false,
    commentsError: '',
    commentsMore: false,
    comment: '',
    commentCount: 0,
    replyID: '',
    replyName: '',
    shareOpen: false,
    shareBusy: false,
    shareActive: false,
    shareToken: '',
    shareError: '',
  },
  round: null as Round | null,
  snapshot: null as Snapshot | null,
  allComments: [] as RoundComment[],
  groupID: '',
  roundID: '',
  userID: '',
  token: '',
  commentKey: '',
  active: true,
  generation: 0,

  // onLoad routes explicit public tokens separately from authenticated group records.
  onLoad(options: Record<string, string | undefined>) {
    this.token = options.token || ''
    this.roundID = options.id || ''
    if (!this.token && (options.group || options.groupId)) setGroupID(options.group || options.groupId || '')
    this.commentKey = newID()
    this.setData({ public: !!this.token })
    wx.hideShareMenu()
  },
  // onShow reloads after an edit and rechecks membership before showing private content.
  onShow() {
    void this.load()
  },
  // onHide clears private projections before a later visit revalidates membership.
  onHide() {
    this.generation++
    this.round = null
    this.snapshot = null
    this.allComments = []
    this.setData({
      detail: null,
      photos: [],
      comments: [],
      shareToken: '',
      shareOpen: false,
      loading: true,
      commentsLoading: false,
    })
    wx.hideShareMenu()
  },
  // onUnload invalidates delayed response handlers.
  onUnload() {
    this.active = false
    this.generation++
  },
  // current checks that an asynchronous result still belongs to the displayed account.
  current(run: number): boolean {
    return (
      this.active &&
      run === this.generation &&
      (!!this.token || (getUser()?.id === this.userID && getGroupID() === this.groupID))
    )
  },

  // load fetches an authorized detail projection and then its independently retryable photos.
  async load() {
    const run = ++this.generation
    this.setData({
      loading: true,
      error: '',
      photoError: '',
      detail: null,
      photos: [],
      comments: [],
      editable: false,
      shareOpen: false,
      shareToken: '',
    })
    try {
      if (this.token) {
        const round = await request<PublicRound>('/shared-rounds', { headers: { 'X-Round-Share': this.token } })
        if (!this.current(run)) return
        let result = '未记结果，也是一段好时光'
        if (round.outcome === 'draw') result = '全局平局'
        else if (round.mode === 'coop' && round.outcome !== 'unknown')
          result = round.outcome === 'win' ? '全队胜利' : '全队失败，下次再来'
        else if (round.outcome === 'win')
          result = `${
            round.mode === 'team'
              ? round.teams
                  .filter(team => team.winner)
                  .map(team => team.name)
                  .join('、')
              : round.players
                  .filter(player => player.winner)
                  .map(player => player.name)
                  .join('、')
          }获胜`
        this.setData({
          detail: {
            game: round.game_name,
            group: round.group_name,
            date: round.date,
            mode: modeNames[round.mode],
            result,
            players: round.players.map((player, index) => ({ ...player, id: String(index) })),
            teams: round.teams.map(team => ({ ...team, names: team.players.join('、') })),
            teamScore: round.team_score,
            memory: round.memory,
            minutes: null,
            author: '',
            updated: '',
            gameID: '',
          },
          shareToken: this.token,
        })
        wx.showShareMenu({ menus: ['shareAppMessage'] })
        void this.loadPhotos(round.photos, run)
      } else {
        const context = await requireGroup()
        if (!context) return
        this.groupID = context.group.id
        this.userID = context.user.id
        this.snapshot = context.snapshot
        const round = await request<Round>(`/groups/${this.groupID}/rounds/${this.roundID}`)
        if (!this.current(run)) return
        this.round = round
        const players = context.snapshot.players
        this.setData({
          detail: {
            game: context.snapshot.games.find(game => game.id === round.game_id)?.name || '桌游',
            group: context.group.name,
            date: round.date,
            mode: modeNames[round.mode],
            result: resultLabel(round, players),
            players: round.players.map(id => ({
              id,
              name: players.find(player => player.id === id)?.name || '玩家',
              score: round.scores[id] ?? null,
              winner: round.winners.includes(id),
            })),
            teams: round.teams.map(team => ({
              ...team,
              names: team.players.map(id => players.find(player => player.id === id)?.name || '玩家').join('、'),
            })),
            teamScore: round.team_score,
            memory: round.memory,
            minutes: round.minutes,
            author: this.memberName(round.author),
            updated: `${this.memberName(round.updated_by)} · ${beijingTime(round.updated_at)}（北京时间）`,
            gameID: round.game_id,
          },
          editable: context.group.owner === this.userID || round.author === this.userID,
        })
        void this.loadPhotos(round.photos, run)
        void this.loadComments(false)
      }
    } catch (cause) {
      if (this.active && run === this.generation) this.setData({ error: errorMessage(cause) })
    } finally {
      if (this.active && run === this.generation) this.setData({ loading: false })
    }
  },
  // memberName prefers the group's linked player nickname and handles historical members.
  memberName(id: string): string {
    return (
      this.snapshot?.players.find((player: Player) => player.account === id)?.name ||
      this.snapshot?.members.find((member: Member) => member.user_id === id)?.email ||
      '历史成员'
    )
  },
  // loadPhotos keeps a failed image from hiding the record's text.
  async loadPhotos(ids: string[], run: number) {
    this.setData({ photoError: '', photos: ids.map(id => ({ id, path: '' })) })
    for (const id of ids) {
      try {
        const path = this.token ? await getPublicPhoto(this.token, id) : await getPhoto(this.groupID, id)
        if (!this.current(run)) return
        this.setData({ photos: this.data.photos.map(photo => (photo.id === id ? { id, path } : photo)) })
      } catch (cause) {
        if (this.current(run)) this.setData({ photoError: errorMessage(cause) })
      }
    }
  },
  // retryPhotos retries all displayed photo IDs after a transport failure.
  retryPhotos() {
    void this.loadPhotos(
      this.data.photos.map(photo => photo.id),
      this.generation,
    )
  },
  // previewPhoto opens only the authenticated local photo files already downloaded.
  previewPhoto(event: WechatMiniprogram.BaseEvent) {
    const urls = this.data.photos.map(photo => photo.path).filter(Boolean)
    const current = String(event.currentTarget.dataset.path || '')
    if (current && urls.length) wx.previewImage({ current, urls })
  },
  // edit preserves the record ID and version through the edit page.
  edit() {
    if (this.data.editable) wx.navigateTo({ url: `/pages/round-form/index?group=${this.groupID}&id=${this.roundID}` })
  },
  // again requests a fresh form that copies only this record's game and players.
  again() {
    wx.navigateTo({ url: `/pages/round-form/index?group=${this.groupID}&again=${this.roundID}` })
  },
  // openGame navigates to this game's group-only history.
  openGame() {
    if (!this.token) wx.navigateTo({ url: `/pages/game/index?group=${this.groupID}&id=${this.data.detail?.gameID}` })
  },
  // openPlayer navigates to a participating player's group-only history.
  openPlayer(event: WechatMiniprogram.BaseEvent) {
    if (!this.token)
      wx.navigateTo({ url: `/pages/player/index?group=${this.groupID}&id=${event.currentTarget.dataset.id}` })
  },
  // remove requires confirmation and sends the last observed record version.
  async remove() {
    if (!this.round || !this.data.editable || this.data.busy) return
    if (!(await confirm('删除这段对局记录？', '记录会移入回收站，7 天内可恢复；时间线、统计和公开分享立即失效。')))
      return
    this.setData({ busy: true, error: '' })
    try {
      await send(`/groups/${this.groupID}/rounds/${this.roundID}`, { version: this.round.version }, 'DELETE')
      wx.reLaunch({ url: '/pages/review/index' })
    } catch (cause) {
      this.setData({ error: errorMessage(cause) })
    } finally {
      this.setData({ busy: false })
    }
  },
  // loadComments supports full pagination and ignores responses from a previous page load.
  async loadComments(more = false) {
    if (this.data.commentsLoading || this.token) return
    const run = this.generation
    this.setData({ commentsLoading: true, commentsError: '' })
    try {
      const page = await request<RoundCommentPage>(
        `/groups/${this.groupID}/rounds/${this.roundID}/comments${query({ offset: more ? this.allComments.length : 0, limit: 100 })}`,
      )
      if (!this.current(run)) return
      this.allComments = more ? [...this.allComments, ...page.items] : page.items
      this.setData({ commentsMore: this.allComments.length < page.total })
      this.renderComments()
    } catch (cause) {
      if (this.current(run)) this.setData({ commentsError: errorMessage(cause) })
    } finally {
      if (this.active) this.setData({ commentsLoading: false })
    }
  },
  // moreComments loads the next page without dropping already visible discussion.
  moreComments() {
    void this.loadComments(true)
  },
  // retryComments refreshes comments after a failed request.
  retryComments() {
    void this.loadComments(false)
  },
  // renderComments builds the supported one-level reply tree.
  renderComments() {
    const comments = [...this.allComments].sort((left, right) =>
      left.created === right.created ? left.id.localeCompare(right.id) : left.created.localeCompare(right.created),
    )
    const view = (comment: RoundComment): CommentView => ({
      ...comment,
      name: this.memberName(comment.author),
      time: beijingTime(comment.created),
      removable: this.snapshot?.group.owner === this.userID || comment.author === this.userID,
      replies: [],
    })
    this.setData({
      comments: comments
        .filter(comment => !comment.parent_id)
        .map(comment => ({
          ...view(comment),
          replies: comments.filter(reply => reply.parent_id === comment.id).map(view),
        })),
    })
  },
  // inputComment counts Unicode characters consistently with the backend.
  inputComment(event: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ comment: event.detail.value, commentCount: Array.from(event.detail.value).length })
  },
  // reply only targets root comments, preserving a single reply level.
  reply(event: WechatMiniprogram.BaseEvent) {
    this.setData({
      replyID: String(event.currentTarget.dataset.id),
      replyName: String(event.currentTarget.dataset.name),
    })
  },
  // cancelReply restores the root comment composer.
  cancelReply() {
    this.setData({ replyID: '', replyName: '' })
  },
  // submitComment reuses its key after failure and clears input only after success.
  async submitComment() {
    const run = this.generation
    const body = this.data.comment.trim()
    if (!body || this.data.commentCount > 500 || this.data.busy) return
    this.setData({ busy: true, commentsError: '' })
    try {
      const saved = await send<RoundComment>(
        `/groups/${this.groupID}/rounds/${this.roundID}/comments`,
        { body, parent_id: this.data.replyID || null },
        'POST',
        this.commentKey,
      )
      if (!this.current(run)) return
      this.allComments = [...this.allComments.filter((comment: RoundComment) => comment.id !== saved.id), saved]
      this.commentKey = newID()
      this.setData({ comment: '', commentCount: 0, replyID: '', replyName: '' })
      this.renderComments()
    } catch (cause) {
      this.setData({ commentsError: errorMessage(cause) })
    } finally {
      this.setData({ busy: false })
    }
  },
  // removeComment confirms irreversible deletion of a root and its replies.
  async removeComment(event: WechatMiniprogram.BaseEvent) {
    const id = String(event.currentTarget.dataset.id)
    if (this.data.busy || !(await confirm('删除这条评论？', '删除后无法恢复；这条评论下面的回复也会一起删除。'))) return
    this.setData({ busy: true, commentsError: '' })
    try {
      await send(`/groups/${this.groupID}/rounds/${this.roundID}/comments/${id}`, {}, 'DELETE')
      this.allComments = this.allComments.filter(
        (comment: RoundComment) => comment.id !== id && comment.parent_id !== id,
      )
      this.renderComments()
      if (this.data.replyID === id) this.cancelReply()
    } catch (cause) {
      this.setData({ commentsError: errorMessage(cause) })
    } finally {
      this.setData({ busy: false })
    }
  },
  // openShare shows current public access status before offering an explicit grant.
  async openShare() {
    if (!this.data.editable || this.data.shareBusy) return
    this.setData({ shareOpen: true, shareBusy: true, shareError: '' })
    try {
      this.setData({
        shareActive: (await request<RoundShareStatus>(`/groups/${this.groupID}/rounds/${this.roundID}/share`)).active,
      })
    } catch (cause) {
      this.setData({ shareError: errorMessage(cause) })
    } finally {
      this.setData({ shareBusy: false })
    }
  },
  // closeShare dismisses the sharing controls without changing an existing grant.
  closeShare() {
    this.setData({ shareOpen: false })
  },
  // generateShare creates the revocable public projection only after the disclosure is confirmed.
  async generateShare() {
    const run = this.generation
    if (
      this.data.shareBusy ||
      !(await confirm(
        '生成公开分享？',
        '获得链接的人无需登录，可以看到小组名、游戏、日期、玩家昵称、结果、回忆和照片。不会显示账号、评论或其他组内内容。重新生成会使旧链接失效。',
      ))
    )
      return
    this.setData({ shareBusy: true, shareError: '' })
    try {
      const share = await send<RoundShareToken>(`/groups/${this.groupID}/rounds/${this.roundID}/share`, {})
      if (!this.current(run)) return
      this.setData({ shareToken: share.token, shareActive: true })
      wx.showShareMenu({ menus: ['shareAppMessage'] })
    } catch (cause) {
      this.setData({ shareError: errorMessage(cause) })
    } finally {
      this.setData({ shareBusy: false })
    }
  },
  // copyShare copies the same revocable token for opening in a normal browser.
  copyShare() {
    if (this.data.shareToken) wx.setClipboardData({ data: `${API_ORIGIN}/share#${this.data.shareToken}` })
  },
  // revokeShare removes public access and hides further native sharing affordances.
  async revokeShare() {
    if (this.data.shareBusy || !(await confirm('撤销公开分享？', '已发送的分享将无法再查看这局。'))) return
    this.setData({ shareBusy: true, shareError: '' })
    try {
      await send(`/groups/${this.groupID}/rounds/${this.roundID}/share`, {}, 'DELETE')
      this.setData({ shareActive: false, shareToken: '' })
      wx.hideShareMenu()
    } catch (cause) {
      this.setData({ shareError: errorMessage(cause) })
    } finally {
      this.setData({ shareBusy: false })
    }
  },
  // onShareAppMessage shares only an explicitly generated public token.
  onShareAppMessage() {
    if (!this.data.shareToken || !this.data.detail)
      return { title: '又一局 · 记下输赢与相聚', path: '/pages/review/index', imageUrl: '/pages/round/share-card.png' }
    return {
      title: `${this.data.detail.game} · ${this.data.detail.date} 的桌边回忆`,
      path: `/pages/round/index?token=${encodeURIComponent(this.data.shareToken)}`,
      imageUrl: '/pages/round/share-card.png',
    }
  },
})
