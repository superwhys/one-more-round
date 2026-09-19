import { ref, watch, onScopeDispose, type Ref } from 'vue'
import { previewGroupInvite } from '@/api/auth'
import { message } from '@/utils/error'
import type { InvitePreview } from '@/types/auth'

export function useInvitePreview(token: Ref<string>) {
  const preview = ref<InvitePreview | null>(null)
  const loading = ref(false)
  const error = ref('')
  let request = 0
  async function refresh() {
    const current = ++request
    preview.value = null; error.value = ''; loading.value = !!token.value
    if (!token.value) return
    try {
      const result = await previewGroupInvite(token.value)
      if (current === request) preview.value = result
    } catch (cause) {
      if (current === request) error.value = message(cause)
    } finally { if (current === request) loading.value = false }
  }
  watch(token, refresh, { immediate: true })
  onScopeDispose(() => { request++ })
  return { preview, loading, error, refresh }
}
