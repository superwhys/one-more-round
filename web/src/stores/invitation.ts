import { ref, watch } from 'vue'

const storageKey = 'omr:pending-invitation'
const lifetime = 7 * 24 * 60 * 60 * 1000
type InvitationStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

// Each browser tab owns its pending invitation; it is never shared via localStorage.
export function createInvitationState(storage?: InvitationStorage) {
  const invitation = ref('')
  const joinToken = ref('')
  let expires = Date.now() + lifetime
  try {
    const saved: unknown = JSON.parse(storage?.getItem(storageKey) ?? 'null')
    if (saved && typeof saved === 'object' && 'expires' in saved && typeof saved.expires === 'number' && saved.expires > Date.now()) {
      expires = saved.expires
      if ('group' in saved && typeof saved.group === 'string') joinToken.value = saved.group
      if (!joinToken.value && 'trial' in saved && typeof saved.trial === 'string') invitation.value = saved.trial
    } else storage?.removeItem(storageKey)
  } catch { /* Storage may be disabled; invitations still work until reload. */ }

  watch([invitation, joinToken], () => {
    try {
      if (invitation.value || joinToken.value) storage?.setItem(storageKey, JSON.stringify({ trial: invitation.value, group: joinToken.value, expires }))
      else storage?.removeItem(storageKey)
    } catch { /* Optional refresh recovery; never block the invitation flow. */ }
  }, { flush: 'sync' })

  function clearInvitations() { invitation.value = ''; joinToken.value = '' }
  function captureInvitation(path: string, hash: string) {
    if (!hash) return false
    const fragment = hash.slice(1)
    const trial = new URLSearchParams(fragment).get('trial')
    if (path.replace(/\/+$/, '') !== '/join' && trial === null) return false
    clearInvitations()
    expires = Date.now() + lifetime
    if (path.replace(/\/+$/, '') === '/join') {
      try { joinToken.value = decodeURIComponent(fragment) }
      catch { joinToken.value = fragment }
    } else invitation.value = trial ?? ''
    return true
  }
  return { invitation, joinToken, clearInvitations, captureInvitation }
}
