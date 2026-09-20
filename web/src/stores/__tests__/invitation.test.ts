import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createInvitationState } from '../invitation.ts'

function storage() {
  const data = new Map<string, string>()
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => {
      data.set(key, value)
    },
    removeItem: (key: string) => {
      data.delete(key)
    },
  }
}

test('group invitation survives reload after the URL fragment is removed', () => {
  const tab = storage()
  const initial = createInvitationState(tab)
  assert.equal(initial.captureInvitation('/join', '#group-token'), true)
  assert.equal(initial.invitation.value, '')
  const reloaded = createInvitationState(tab)
  assert.equal(reloaded.joinToken.value, 'group-token')
  assert.equal(reloaded.captureInvitation('/login', ''), false)
  assert.equal(reloaded.joinToken.value, 'group-token')
})

test('trial and group invitations replace each other without mixing registration credentials', () => {
  const state = createInvitationState(storage())
  state.captureInvitation('/login', '#trial=trial-token')
  assert.equal(state.invitation.value, 'trial-token')
  state.captureInvitation('/join/', '#second-group')
  assert.equal(state.invitation.value, '')
  assert.equal(state.joinToken.value, 'second-group')
  state.captureInvitation('/login', '#trial=new-trial')
  assert.equal(state.joinToken.value, '')
  assert.equal(state.invitation.value, 'new-trial')
})

test('success, cancellation and logout can clear invitations including refresh recovery', () => {
  const tab = storage()
  const state = createInvitationState(tab)
  state.captureInvitation('/join', '#group-token')
  state.clearInvitations()
  const reloaded = createInvitationState(tab)
  assert.equal(reloaded.joinToken.value, '')
  assert.equal(reloaded.invitation.value, '')
})

test('pending invitations are isolated by browser tab', () => {
  const first = createInvitationState(storage())
  first.captureInvitation('/join', '#group-token')
  assert.equal(createInvitationState(storage()).joinToken.value, '')
})

test('expired or malformed recovery data cannot restore an invitation', () => {
  const tab = storage()
  for (const value of [
    'broken-json',
    JSON.stringify({ group: 'expired', expires: Date.now() - 1 }),
    JSON.stringify({ group: {}, expires: Date.now() + 10000 }),
  ]) {
    tab.setItem('omr:pending-invitation', value)
    assert.equal(createInvitationState(tab).joinToken.value, '')
  }
})

test('unrelated anchors and malformed encoding do not break navigation', () => {
  const state = createInvitationState(storage())
  state.captureInvitation('/join', '#group-token')
  assert.equal(state.captureInvitation('/group', '#main-content'), false)
  assert.equal(state.joinToken.value, 'group-token')
  assert.doesNotThrow(() => state.captureInvitation('/join', '#%invalid'))
})

test('unavailable browser storage still permits the current invitation flow', () => {
  const state = createInvitationState({
    getItem: () => {
      throw new Error('disabled')
    },
    setItem: () => {
      throw new Error('disabled')
    },
    removeItem: () => {
      throw new Error('disabled')
    },
  })
  state.captureInvitation('/join', '#group-token')
  assert.equal(state.joinToken.value, 'group-token')
  assert.doesNotThrow(state.clearInvitations)
})
