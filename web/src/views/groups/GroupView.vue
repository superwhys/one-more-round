<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import Icon from '@/components/common/AppIcon.vue'
import Modal from '@/components/common/AppModal.vue'
import PlayerChip from '@/components/common/PlayerChip.vue'
import RequestStatus from '@/components/common/RequestStatus.vue'
import { useGroupContext } from '@/composables/useGroupContext'
import { useGroupOperation } from '@/composables/useGroupOperation'

import GroupInviteDialog from '@/components/groups/GroupInviteDialog.vue'
import { addPlayer, exportGroup, manageGroup } from '@/api/group'
import { useSession } from '@/stores/session'
import { useRoundHistory } from '@/composables/useRoundHistory'
import type { GroupAction } from '@/types/group'
const router = useRouter()
const session = useSession()
const { user } = session
const { groupId, snapshot, owner, refresh, handleAccessError, memberName, playerName } = useGroupContext()
const { page, loading, error: loadError, load } = useRoundHistory()
const { busy, error, notice, run } = useGroupOperation()
const name = ref('')
const claimPlayer = ref('')
const profileName = ref('')
const profileMode = ref<'create' | 'link'>('create')
const addingOpen = ref(false)
const inviteOpen = ref(false)
const confirmOpen = ref(false)
const myPlayer = computed(() => snapshot.value?.players.find(player => player.account === user.value?.id))
const myClaim = computed(() => snapshot.value?.claims.find(claim => claim.user_id === user.value?.id))
const claimedPlayer = computed(() => snapshot.value?.players.find(player => player.id === myClaim.value?.player_id))
const unlinkedPlayers = computed(() => snapshot.value?.players.filter(player => !player.account) ?? [])
const members = computed(() => {
  const current = snapshot.value
  if (!current) return []
  return current.members
    .map(member => ({
      ...member,
      playerName: current.players.find(player => player.account === member.user_id)?.name,
    }))
    .sort((a, b) => Number(b.user_id === current.group.owner) - Number(a.user_id === current.group.owner))
})
const confirmation = ref<{ action: GroupAction; target: string; title: string }>({
  action: 'remove',
  target: '',
  title: '',
})
function confirm(action: GroupAction, target: string, title: string) {
  confirmation.value = { action, target, title }
  confirmOpen.value = true
}
function openAddForm() {
  addingOpen.value = true
  name.value = ''
}
function add() {
  return run(async () => {
    await addPlayer(groupId, name.value)
    name.value = ''
    addingOpen.value = false
    notice.value = '已经添加，可以开始记局了'
    try {
      await refresh()
    } catch {
      notice.value = '玩家已添加，刷新失败，请重新加载'
    }
  })
}
function manage(action: GroupAction, target = '', value = '', success = '已保存') {
  return run(async () => {
    await manageGroup(groupId, action, target, value)
    confirmOpen.value = false
    notice.value = success
    if (action === 'claim' || action === 'claim-new') {
      claimPlayer.value = ''
      profileName.value = ''
    }
    await session.loadGroups()
    if (session.selected.value !== groupId) {
      await router.replace('/')
      return
    }
    try {
      await refresh()
    } catch {
      notice.value = '已保存，刷新失败，请重新加载'
    }
  })
}
function requestProfile() {
  return profileMode.value === 'create'
    ? manage('claim-new', '', profileName.value, '已创建玩家档案，等待组主确认关联')
    : manage('claim', claimPlayer.value, '', '关联申请已提交，等待组主确认')
}
function retry() {
  return run(async () => {
    await refresh()
    await load()
  })
}
function downloadBackup() {
  return run(async () => {
    const blob = await exportGroup(groupId)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `又一局-${snapshot.value?.group.name ?? '小组'}-${new Date().toISOString().slice(0, 10)}.zip`
    link.click()
    window.setTimeout(() => URL.revokeObjectURL(url), 0)
    notice.value = '备份已经下载'
  })
}
</script>

<template>
  <template v-if="snapshot && user"
    ><RequestStatus
      :loading="loading"
      :error="loadError || error"
      :notice="notice"
      @retry="retry"
      @dismiss="notice = ''" />
    <header class="d-page-heading">
      <div>
        <p class="d-eyebrow">OUR LITTLE CIRCLE</p>
        <h1>{{ snapshot.group.name }}<span class="d-title-dot">。</span></h1>
        <p>桌子不大，刚好坐得下这些老朋友。</p>
      </div>
      <button v-if="owner" class="d-button" @click="inviteOpen = true">＋ 邀请朋友</button>
    </header>
    <div class="d-group-banner">
      <div class="d-group-banner-art">局<span>OUR TABLE</span></div>
      <div>
        <span class="d-pill">私密小组</span>
        <h2>有空就聚，再来一局。</h2>
        <p>{{ snapshot.players.length }} 位玩家 · {{ snapshot.members.length }} 位成员 · {{ page.total }} 段回忆</p>
      </div>
    </div>
    <div class="d-group-columns">
      <section class="d-surface">
        <div class="d-list-heading">
          <h2>同桌的朋友<small>玩家档案</small></h2>
          <button class="d-text-link" @click="openAddForm">＋ 添加玩家</button>
        </div>
        <RouterLink v-for="p in snapshot.players" :key="p.id" :to="`/players/${p.id}`" class="d-member-row"
          ><PlayerChip
            :name="p.name"
            :me="p.account === user.id"
            :subtitle="p.account ? '已关联账号' : '昵称玩家 · 无访问权限'" /><Icon name="arrow"
        /></RouterLink>
        <p v-if="!snapshot.players.length" class="d-note">只有昵称，也能留下战绩。先添加一位同桌的朋友吧。</p>
        <div v-if="myPlayer" class="j-profile-status">
          <strong>我的玩家档案：{{ myPlayer.name }}</strong
          ><span>新建对局时会默认选中“我”。</span>
        </div>
        <div v-else-if="myClaim" class="j-profile-status pending">
          <strong>关联申请待确认</strong
          ><span>申请关联「{{ claimedPlayer?.name ?? '玩家档案' }}」，组主确认后会标记为“我”。</span>
        </div>
        <div v-else class="j-profile-setup">
          <h3>哪一个是你？</h3>
          <p>创建新档案，或者认领以前留下的战绩。两种方式都需要组主确认。</p>
          <div class="d-segment small" aria-label="玩家档案建立方式">
            <button
              type="button"
              :class="{ active: profileMode === 'create' }"
              :aria-pressed="profileMode === 'create'"
              @click="profileMode = 'create'"
            >
              创建新档案</button
            ><button
              type="button"
              :class="{ active: profileMode === 'link' }"
              :aria-pressed="profileMode === 'link'"
              @click="profileMode = 'link'"
            >
              关联已有档案
            </button>
          </div>
          <form v-if="profileMode === 'create'" @submit.prevent="requestProfile">
            <label class="d-field"
              >你的玩家昵称<input
                v-model="profileName"
                required
                maxlength="255"
                placeholder="大家平时怎么称呼你？" /></label
            ><button class="d-button secondary full" :disabled="busy || !profileName.trim()">创建并申请关联</button>
          </form>
          <form v-else @submit.prevent="requestProfile">
            <label class="d-field"
              >选择以前的玩家档案<select v-model="claimPlayer" required>
                <option value="">请选择</option>
                <option v-for="p in unlinkedPlayers" :key="p.id" :value="p.id">{{ p.name }}</option>
              </select></label
            >
            <p v-if="!unlinkedPlayers.length" class="d-note">当前没有可关联的历史档案，可以创建一个新的。</p>
            <button class="d-button secondary full" :disabled="busy || !claimPlayer">申请关联</button>
          </form>
        </div>
      </section>
      <section class="d-surface">
        <div class="d-list-heading">
          <h2>能一起回顾的人<small>账号成员</small></h2>
        </div>
        <div v-for="m in members" :key="m.user_id" class="j-member">
          <div class="j-member-account">
            <strong>{{ m.email }}</strong>
            <span class="d-pill">{{ m.user_id === snapshot.group.owner ? '组主' : '成员' }}</span>
          </div>
          <p v-if="m.playerName" class="j-member-player">关联玩家：{{ m.playerName }}</p>
          <div v-if="owner && m.user_id !== user.id" class="j-actions">
            <button class="d-text-link" @click="confirm('transfer', m.user_id, '转让组主身份？你将成为普通成员。')">
              转让组主</button
            ><button
              class="d-text-link"
              @click="confirm('remove', m.user_id, '移除此成员？历史参与记录保留，访问权限立即撤销。')"
            >
              移除
            </button>
          </div>
        </div>
        <template v-if="owner"
          ><div v-for="c in snapshot.claims" :key="c.user_id" class="j-notice">
            <p>{{ memberName(c.user_id) }} 申请关联 {{ playerName(c.player_id) }}</p>
            <button :disabled="busy" @click="manage('approve', c.user_id)">确认关联</button
            ><button :disabled="busy" @click="manage('reject', c.user_id)">拒绝</button>
          </div>
          <form class="j-inline" @submit.prevent="manage('rename', '', name)">
            <label class="d-field">修改小组名称<input v-model="name" required maxlength="255" /></label
            ><button class="d-button secondary" :disabled="busy">保存</button>
          </form></template
        ><button
          v-else
          class="d-text-link"
          @click="confirm('remove', user.id, '退出小组？历史记录保留，退出后无法继续查看。')"
        >
          退出小组
        </button>
      </section>
    </div>
    <section class="d-surface j-data-tools">
      <div>
        <h2>数据与恢复</h2>
        <p class="d-note">删除的对局可在 7 天内恢复；组主还可以导出 JSON、CSV 与照片备份。</p>
      </div>
      <div class="j-actions">
        <RouterLink to="/recycle-bin" class="d-button secondary"><Icon name="archive" />回收站</RouterLink
        ><button v-if="owner" class="d-button secondary" :disabled="busy" @click="downloadBackup">
          <Icon name="download" />导出小组备份
        </button>
      </div>
    </section>
    <Modal v-if="addingOpen" title="给朋友留个位置。" @close="addingOpen = false"
      ><form @submit.prevent="add">
        <label class="d-field">玩家昵称<input v-model="name" required maxlength="255" autofocus /></label>
        <p v-if="error" class="j-error" role="alert">{{ error }}</p>
        <button class="d-button full" :disabled="busy">保存</button>
      </form></Modal
    >
    <Modal v-if="confirmOpen" title="确认这项操作" @close="confirmOpen = false"
      ><p>{{ confirmation.title }}</p>
      <p v-if="error" class="j-error">{{ error }}</p>
      <div class="d-modal-actions">
        <button class="d-button secondary" @click="confirmOpen = false">取消</button
        ><button class="d-button" :disabled="busy" @click="manage(confirmation.action, confirmation.target)">
          确认
        </button>
      </div></Modal
    >
    <GroupInviteDialog
      v-if="inviteOpen"
      :group-id="groupId"
      @close="inviteOpen = false"
      @access-error="handleAccessError"
  /></template>
</template>
