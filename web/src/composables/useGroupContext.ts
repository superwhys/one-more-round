import { computed, inject, provide, ref, onScopeDispose } from 'vue'
import type { InjectionKey } from 'vue'
import { useRouter } from 'vue-router'
import { getGroup } from '@/api/group'
import { ApiError } from '@/api/request'
import { useSession } from '@/stores/session'
import { message } from '@/utils/error'
import type { Snapshot } from '@/types/journal'

function createGroupContext(groupId: string) {
  const session = useSession()
  const router = useRouter()
  const snapshot = ref<Snapshot | null>(null)
  const owner = computed(() => snapshot.value?.group.owner === session.user.value?.id)
  let generation = 0
  let active = true
  onScopeDispose(() => { generation++; active = false })
  async function refresh() {
    const run = ++generation
    const result = await getGroup(groupId)
    if (run === generation) snapshot.value = result
  }
  async function handleAccessError(error: unknown) {
    if (!active || !(error instanceof ApiError)) return
    if (error.status === 401) {
      snapshot.value = null
      session.expire()
      await router.replace('/login')
    } else if (error.status === 403) {
      try { await session.loadGroups() } catch (cause) { session.error.value = message(cause); return }
      if (session.selected.value !== groupId) {
        snapshot.value = null
        await router.replace('/')
      }
    }
  }
  const playerName = (id: string) => snapshot.value?.players.find(player => player.id === id)?.name ?? '玩家'
  const gameName = (id: string) => snapshot.value?.games.find(game => game.id === id)?.name ?? '桌游'
  const memberName = (id: string) => snapshot.value?.members.find(member => member.user_id === id)?.email ?? '历史成员'
  return { groupId, snapshot, owner, refresh, handleAccessError, playerName, gameName, memberName }
}
const groupKey: InjectionKey<ReturnType<typeof createGroupContext>> = Symbol('group')
export function provideGroupContext(groupId: string) {
  const context = createGroupContext(groupId)
  provide(groupKey, context)
  return context
}
export function useGroupContext() {
  const context = inject(groupKey)
  if (!context) throw new Error('小组页面必须位于小组布局内')
  return context
}
