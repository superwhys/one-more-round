import { download, send } from '../../utils/api'
import { requireGroup, setGroupID } from '../../utils/session'
import type { Snapshot, Member, Player } from '../../utils/types'
import { confirm, errorMessage, navigate } from '../../utils/ui'

// Group keeps membership and historical player profiles distinct.
Page({
  data: {
    snapshot: null as Snapshot | null,
    userID: '',
    owner: false,
    members: [] as (Member & { name: string })[],
    claims: [] as { user_id: string; name: string; player: string }[],
    unlinked: [] as Player[],
    myPlayer: '',
    myClaim: false,
    playerName: '',
    newProfile: '',
    groupName: '',
    claimIndex: 0,
    busy: false,
    loading: false,
    error: '',
  },
  generation: 0,
  // onShow refreshes this page when it becomes visible.
  onShow() {
    void this.load()
  },
  // onHide invalidates private content when leaving this page.
  onHide() {
    this.generation++
    this.setData({ snapshot: null })
  },
  // load refreshes the authorized data for this page.
  async load() {
    const run = ++this.generation
    this.setData({ loading: true, error: '' })
    try {
      const current = await requireGroup()
      if (!current || run !== this.generation) return
      const snapshot = current.snapshot
      this.setData({
        snapshot,
        userID: current.user.id,
        owner: current.user.id === current.group.owner,
        groupName: current.group.name,
        members: snapshot.members.map(member => ({
          ...member,
          name: snapshot.players.find(player => player.account === member.user_id)?.name || member.email || '微信成员',
        })),
        claims: snapshot.claims.map(claim => ({
          ...claim,
          name: snapshot.members.find(member => member.user_id === claim.user_id)?.email || '微信成员',
          player: snapshot.players.find(player => player.id === claim.player_id)?.name || '玩家',
        })),
        unlinked: snapshot.players.filter(player => !player.account),
        myPlayer: snapshot.players.find(player => player.account === current.user.id)?.name || '',
        myClaim: snapshot.claims.some(claim => claim.user_id === current.user.id),
      })
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ loading: false })
    }
  },
  // input updates the editable field identified by the control.
  input(event: WechatMiniprogram.Input) {
    this.setData({ [event.currentTarget.dataset.field]: event.detail.value })
  },
  // pick updates the selected historical player for an association request.
  pick(event: WechatMiniprogram.PickerChange) {
    this.setData({ claimIndex: Number(event.detail.value) })
  },
  // add creates a nickname player and refreshes the group.
  async add() {
    if (this.data.busy || !this.data.snapshot) return
    this.setData({ busy: true, error: '' })
    try {
      await send(`/groups/${this.data.snapshot?.group.id}/players`, { name: this.data.playerName.trim() })
      this.setData({ playerName: '' })
      await this.load()
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
  // manage submits the authorized group action after required confirmation.
  async manage(event: WechatMiniprogram.TouchEvent) {
    if (this.data.busy || !this.data.snapshot) return
    const action = event.currentTarget.dataset.action as string
    let target = (event.currentTarget.dataset.target as string) || ''
    const value =
      action === 'rename' ? this.data.groupName.trim() : action === 'claim-new' ? this.data.newProfile.trim() : ''
    if (action === 'claim') target = this.data.unlinked[this.data.claimIndex]?.id || ''
    if (
      ['remove', 'transfer', 'reject'].includes(action) &&
      !(await confirm(
        '确认操作',
        action === 'transfer'
          ? '转让后你将成为普通成员，组主权限交给对方。'
          : action === 'remove'
            ? '移除成员或退出后，访问权限立即撤销，历史参与记录保留。'
            : '拒绝这次玩家关联申请？',
      ))
    )
      return
    this.setData({ busy: true, error: '' })
    try {
      await send(`/groups/${this.data.snapshot?.group.id}/manage`, { action, target, value })
      this.setData({ newProfile: '' })
      if (action === 'remove' && target === this.data.userID) {
        setGroupID('')
        wx.redirectTo({ url: '/pages/setup/index' })
        return
      }
      await this.load()
      wx.showToast({ title: '已保存' })
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
  // backup downloads the authorized group archive for explicit local export.
  async backup() {
    if (this.data.busy || !this.data.snapshot) return
    this.setData({ busy: true, error: '' })
    try {
      const path = await download(`/groups/${this.data.snapshot?.group.id}/export`)
      await new Promise<void>((resolve, reject) =>
        wx.shareFileMessage({
          filePath: path,
          fileName: '又一局-小组备份.zip',
          success: () => resolve(),
          fail: reject,
        }),
      )
    } catch (error) {
      this.setData({ error: errorMessage(error) })
    } finally {
      this.setData({ busy: false })
    }
  },
  // player opens the selected historical player profile.
  player(event: WechatMiniprogram.TouchEvent) {
    navigate(`/pages/player/index?id=${event.currentTarget.dataset.id}`)
  },
  // go opens the page selected by this navigation control.
  go(event: WechatMiniprogram.TouchEvent) {
    navigate(event.currentTarget.dataset.url)
  },
})
