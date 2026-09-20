import { ref, onMounted, onScopeDispose } from 'vue'
import { getRound } from '@/api/round'
import { useGroupContext } from './useGroupContext'
import { message } from '@/utils/error'
import type { Round } from '@/types/journal'
export function useRound(id: string) {
  const { groupId, handleAccessError } = useGroupContext()
  const current = ref<Round | null>(null)
  const loading = ref(true)
  const error = ref('')
  let generation = 0
  onScopeDispose(() => {
    generation++
  })
  async function load() {
    const run = ++generation
    loading.value = true
    error.value = ''
    try {
      const round = await getRound(groupId, id)
      if (run === generation) current.value = round
    } catch (cause) {
      if (run === generation) {
        error.value = message(cause)
        await handleAccessError(cause)
      }
    } finally {
      if (run === generation) loading.value = false
    }
  }
  onMounted(load)
  return { current, loading, error, load }
}
