<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import RequestStatus from '@/components/common/RequestStatus.vue'
import { listRecycleBin, restoreRound } from '@/api/round'
import { useGroupContext } from '@/composables/useGroupContext'
import { message } from '@/utils/error'
import { useSession } from '@/stores/session'
import type { Round } from '@/types/journal'

const { user } = useSession()
const { groupId, snapshot, gameName, handleAccessError } = useGroupContext()
const items = ref<Round[]>([])
const loading = ref(true)
const error = ref('')
const busy = ref('')
const notice = ref('')
async function load() {
  loading.value = true
  error.value = ''
  try {
    items.value = await listRecycleBin(groupId)
  } catch (cause) {
    error.value = message(cause)
    await handleAccessError(cause)
  } finally {
    loading.value = false
  }
}
async function restore(item: Round) {
  busy.value = item.id
  error.value = ''
  try {
    await restoreRound(groupId, item.id, item.version)
    items.value = items.value.filter(value => value.id !== item.id)
    notice.value = '对局已经恢复到时间线'
  } catch (cause) {
    error.value = message(cause)
    await handleAccessError(cause)
  } finally {
    busy.value = ''
  }
}
const canRestore = (item: Round) => item.author === user.value?.id || snapshot.value?.group.owner === user.value?.id
onMounted(load)
</script>
<template>
  <header class="d-page-heading">
    <div>
      <p class="d-eyebrow">SEVEN DAYS TO RETURN</p>
      <h1>回收站<span class="d-title-dot">。</span></h1>
      <p>删除的对局保留 7 天，到期后会永久清理。</p>
    </div>
    <RouterLink to="/group" class="d-text-link">返回小组</RouterLink>
  </header>
  <RequestStatus :loading="loading" :error="error" :notice="notice" @retry="load" @dismiss="notice = ''" />
  <section class="d-surface">
    <div v-for="item in items" :key="item.id" class="j-recycle-row">
      <div>
        <strong>{{ gameName(item.game_id) }}</strong
        ><span
          >{{ item.date }} · 删除于 {{ item.deleted_at ? new Date(item.deleted_at).toLocaleString('zh-CN') : '' }}</span
        >
      </div>
      <button v-if="canRestore(item)" class="d-button secondary" :disabled="!!busy" @click="restore(item)">
        {{ busy === item.id ? '正在恢复…' : '恢复' }}</button
      ><small v-else>仅记录人或组主可恢复</small>
    </div>
    <div v-if="!items.length && !loading" class="d-empty">
      <h2>回收站是空的。</h2>
      <p>这里没有等待恢复的对局。</p>
    </div>
  </section>
</template>
