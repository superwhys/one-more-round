import { useRouter } from 'vue-router'
import { useSession } from '@/stores/session'
import type { LoginResult } from '@/types/auth'

// useAuthCompletion restores the same account and group flow for every login method.
export function useAuthCompletion() {
  const router = useRouter()
  const session = useSession()

  return async (result: LoginResult) => {
    session.invitation.value = ''
    if (result.group_id) session.joinToken.value = ''
    try {
      sessionStorage.removeItem('omr:pending-login')
    } catch {
      /* optional recovery */
    }
    // Authentication has committed; a failed group refresh must not invite another submission.
    try {
      await session.acceptUser(result)
    } catch {
      session.error.value = '已登录，但小组列表加载失败，请重新加载'
    }
    if (result.group_id) session.selectGroup(result.group_id)
    await router.replace(
      result.group_id ? '/group' : session.joinToken.value || !session.selected.value ? '/join' : '/',
    )
  }
}
