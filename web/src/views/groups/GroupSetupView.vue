<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { createGroup, joinGroup } from '@/api/group'
import { useInvitePreview } from '@/composables/useInvitePreview'
import { useSession } from '@/stores/session'
import { message } from '@/utils/error'
const router = useRouter()
const session = useSession()
const { selected, groups, joinToken } = session
const { preview, loading, error: inviteError, refresh } = useInvitePreview(joinToken)
const alreadyMember = computed(() => groups.value.some(group => group.id === preview.value?.group_id))
const name = ref(''); const playerName = ref(''); const tokenInput = ref(''); const error = ref(''); const busy = ref(false)
async function run(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true; error.value = ''
  try { await action() } catch (cause) { error.value = message(cause) } finally { busy.value = false }
}
function create() { return run(async () => { const group = await createGroup(name.value, playerName.value); await session.loadGroups(); session.selectGroup(group.id); name.value = ''; playerName.value = ''; await router.push('/') }) }
function join() {
  if (!preview.value) return
  const invitedGroupID = preview.value.group_id
  const isMember = alreadyMember.value
  return run(async () => {
    const id = isMember ? invitedGroupID : await joinGroup(joinToken.value)
    session.clearInvitations()
    try { await session.loadGroups() }
    catch { session.error.value = '已加入小组，但列表加载失败，请重新加载' }
    session.selectGroup(id)
    await router.push('/group')
  })
}
function inspect() { joinToken.value = tokenInput.value.trim(); error.value = '' }
function cancel() { session.clearInvitations(); tokenInput.value = ''; error.value = ''; if (selected.value) void router.push('/') }
</script>

<template>
<section class="j-auth d-surface">
  <p class="d-eyebrow">OUR LITTLE CIRCLE</p>
  <template v-if="joinToken">
    <h1>朋友为你留了个位置。</h1>
    <p v-if="loading" role="status">正在确认小组邀请…</p>
    <p v-if="inviteError" class="j-error" role="alert">{{ inviteError }} <button type="button" class="d-text-link" @click="refresh">重新检查</button></p>
    <template v-if="preview"><h2>{{ preview.name }}</h2><p>{{ alreadyMember ? '你已经是小组成员，可以直接进入。' : '加入后可以查看组内回忆，也可以记录新的对局。' }}</p><button class="d-button full" :disabled="busy" @click="join">{{ busy ? '请稍候…' : alreadyMember ? '进入小组' : '加入小组' }}</button></template>
    <p v-if="error" class="j-error" role="alert">{{ error }}</p>
    <button class="d-text-link" :disabled="busy" @click="cancel">{{ alreadyMember ? '返回' : '暂不加入' }}</button>
  </template>
  <template v-else>
    <h1>为朋友们，留张桌子。</h1><p v-if="error" class="j-error" role="alert">{{ error }}</p>
    <form @submit.prevent="create"><label class="d-field">小组名称<input v-model="name" maxlength="255" required placeholder="比如：周五不散场" /></label><label class="d-field">你的玩家昵称<input v-model="playerName" maxlength="255" required placeholder="大家平时怎么称呼你？" /><small>会创建并关联为你在这个小组里的玩家档案。</small></label><button class="d-button full" :disabled="busy">创建小组</button></form><hr />
    <form @submit.prevent="inspect"><label class="d-field">小组邀请码<input v-model="tokenInput" required autocomplete="off" /></label><p class="d-note">也可以直接打开朋友分享的小组邀请链接。</p><button class="d-button secondary full" :disabled="busy || !tokenInput.trim()">查看邀请</button></form>
    <button v-if="selected" class="d-text-link" :disabled="busy" @click="cancel">返回小组</button>
  </template>
</section>
</template>
