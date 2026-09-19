<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import JournalLayout from '@/components/layout/JournalLayout.vue'
import RequestStatus from '@/components/common/RequestStatus.vue'
import { useSession } from '@/stores/session'
import { provideGroupContext } from '@/composables/useGroupContext'
import { message } from '@/utils/error'
const route = useRoute()
const router = useRouter()
const { selected } = useSession()
const { snapshot, refresh, handleAccessError } = provideGroupContext(selected.value)
const loading = ref(true); const error = ref('')
async function load() {
  if (!selected.value) { await router.replace('/join'); return }
  loading.value = true; error.value = ''
  try { await refresh() } catch (cause) { error.value = message(cause); await handleAccessError(cause) }
  finally { loading.value = false }
}
onMounted(load)
</script>

<template>
<JournalLayout :group-name="snapshot?.group.name ?? ''" :section="String(route.meta.section ?? 'review')" :editor="!!route.meta.editor">
  <RequestStatus :loading="loading" :error="error" @retry="load" />
  <RouterView v-if="snapshot" v-slot="{ Component }"><component :is="Component" :key="route.fullPath" /></RouterView>
</JournalLayout>
</template>
