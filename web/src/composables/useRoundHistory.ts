import { ref, onMounted, onScopeDispose } from 'vue'
import { listRounds } from '@/api/round'
import { useGroupContext } from './useGroupContext'
import { message } from '@/utils/error'
import type { Page } from '@/types/journal'
import type { RoundFilters } from '@/types/round'

export function useRoundHistory(scope: Partial<RoundFilters> = {}) {
  const { groupId, handleAccessError } = useGroupContext()
  const page = ref<Page>({ items: [], total: 0, games: 0, players: 0, stats: [], activity: {} })
  const filters = ref<RoundFilters>({ from: '', to: '', game: '', player: '', q: '', location: '', mode: '', outcome: '', has_photos: '', ...scope })
  const loading = ref(true)
  const error = ref('')
  let generation = 0
  onScopeDispose(() => { generation++ })
  async function load(more = false) {
    const run = ++generation
    loading.value = true
    error.value = ''
    try {
      const result = await listRounds(groupId, { ...filters.value, ...scope, limit: 30, offset: more ? page.value.items.length : 0 })
      if (run !== generation) return
      page.value = more ? { ...result, items: [...page.value.items, ...result.items] } : result
    } catch (cause) {
      if (run === generation) { error.value = message(cause); await handleAccessError(cause) }
    } finally { if (run === generation) loading.value = false }
  }
  function apply(value: RoundFilters) { filters.value = { ...value, ...scope }; void load() }
  onMounted(() => { void load() })
  return { page, filters, loading, error, load, apply }
}
