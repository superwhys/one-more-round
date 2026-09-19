<script setup lang="ts">
import { ref, onMounted } from 'vue'
import Modal from '@/components/common/AppModal.vue'
import { listInvites, createInvite as create, manageGroup } from '@/api/group'
import { message } from '@/utils/error'
import type { Invite } from '@/types/journal'
const props = defineProps<{ groupId: string }>()
const emit = defineEmits<{ close: []; accessError: [error: unknown] }>()
const invites = ref<Invite[]>([]); const inviteURL = ref(''); const error = ref(''); const busy = ref(false)
async function run(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true; error.value = ''
  try { await action() } catch (cause) { error.value = message(cause); emit('accessError', cause) } finally { busy.value = false }
}
function load() { return run(async () => { invites.value = await listInvites(props.groupId) }) }
function createInvite() { return run(async () => { inviteURL.value = (await create(props.groupId)).url; try { invites.value = await listInvites(props.groupId) } catch { error.value = '邀请已生成，列表刷新失败，请关闭后重新打开查看' } }) }
function revoke(id: string) { return run(async () => { await manageGroup(props.groupId, 'revoke', id); inviteURL.value = ''; try { invites.value = await listInvites(props.groupId) } catch { error.value = '邀请已撤销，列表刷新失败，请关闭后重新打开查看' } }) }
onMounted(load)
</script>

<template>
<Modal title="给朋友留个位置。" @close="$emit('close')"><p>链接 7 天有效，可邀请多位朋友。朋友验证邮箱即可注册并加入，无需额外邀请码。获得或被转发此链接的人都可加入并查看组内记录，请只分享给信任的朋友。</p><button class="d-button full" :disabled="busy" @click="createInvite">生成小组邀请</button><label v-if="inviteURL" class="d-field">复制邀请链接<input :value="inviteURL" readonly @focus="($event.target as HTMLInputElement).select()" /><small>链接仅本次展示，请妥善分享。</small></label><div v-for="i in invites" :key="i.id" class="j-invite"><span>{{ new Date(i.expires).toLocaleDateString('zh-CN') }} 到期 · {{ i.revoked ? '已撤销' : '已生成' }}</span><button v-if="!i.revoked" :disabled="busy" @click="revoke(i.id)">撤销</button></div><p v-if="error" class="j-error">{{ error }}</p></Modal>
</template>
