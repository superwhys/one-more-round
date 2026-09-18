<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { createGroup, joinGroup } from '@/api/group'
import { useSession } from '@/stores/session'
import { message } from '@/utils/error'
const router = useRouter()
const session = useSession()
const { selected, joinToken } = session
const name = ref(''); const error = ref(''); const busy = ref(false)
async function run(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true; error.value = ''
  try { await action() } catch (cause) { error.value = message(cause) } finally { busy.value = false }
}
function create() { return run(async () => { const group = await createGroup(name.value); await session.loadGroups(); session.selectGroup(group.id); name.value = ''; await router.push('/') }) }
function join() { return run(async () => { const id = await joinGroup(joinToken.value); await session.loadGroups(); session.selectGroup(id); joinToken.value = ''; await router.push('/') }) }
function cancel() { joinToken.value = ''; void router.push('/') }
</script>

<template>
<section class="j-auth d-surface"><p v-if="error" class="j-error" role="alert">{{ error }}</p><p class="d-eyebrow">OUR LITTLE CIRCLE</p><h1>为朋友们，留张桌子。</h1><form @submit.prevent="create"><label class="d-field">小组名称<input v-model="name" maxlength="255" required placeholder="比如：周五不散场" /></label><button class="d-button full" :disabled="busy">创建小组</button></form><hr /><form @submit.prevent="join"><label class="d-field">小组邀请码<input v-model="joinToken" required autocomplete="off" /></label><p class="d-note">加入后可查看历史记录；这份邀请不授予首次注册资格。</p><button class="d-button secondary full" :disabled="busy">加入小组</button></form><button v-if="selected" class="d-text-link" @click="cancel">暂不加入</button></section>
</template>
