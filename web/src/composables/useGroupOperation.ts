import { ref, onScopeDispose } from 'vue'
import { useGroupContext } from './useGroupContext'
import { message } from '@/utils/error'

export function useGroupOperation() {
  const { handleAccessError } = useGroupContext()
  const busy = ref(false)
  const error = ref('')
  const notice = ref('')
  let active = true
  onScopeDispose(() => {
    active = false
  })
  async function run(action: () => Promise<void>) {
    if (busy.value) return
    busy.value = true
    error.value = ''
    try {
      await action()
    } catch (cause) {
      if (active) {
        error.value = message(cause)
        await handleAccessError(cause)
      }
    } finally {
      if (active) busy.value = false
    }
  }
  return { busy, error, notice, run }
}
